// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest_test

// Adversarial, throwaway. Entire-API fixed-point sweep: for every singleton
// <ns>.config / <ns>.update pair the middleware exposes, read the config, write
// its OWN current values back (only the fields <ns>.update accepts), re-read,
// and report ANY field that changed. Writing a thing's own values back must be
// a no-op; where it isn't, the middleware (and anything built on it) is not a
// fixed point — the normalization/derive/side-effect class at the source.
//
// NOT committed. Run against a disposable box:
//   TF_ACC=1 TRUENAS_ENDPOINT=... TRUENAS_API_KEY=... \
//     go test ./internal/acctest/ -run TestAPIFixedPoint_Sweep -v -timeout 40m

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// volatileTop: top-level keys that legitimately move between two reads.
var volatileTop = map[string]bool{
	"id": true, "state": true, "uptime": true, "uptime_seconds": true,
	"datetime": true, "job": true,
}

func TestAPIFixedPoint_Sweep(t *testing.T) {
	acctest.PreCheck(t)      // Client() panics, not skips, without a reachable box
	methods := getMethods(t) // name -> accepts(raw)
	ctx := context.Background()
	c := acctest.Client()

	// Singleton pairs: X.config with a sibling X.update.
	var bases []string
	for name := range methods {
		if !strings.HasSuffix(name, ".config") {
			continue
		}
		base := strings.TrimSuffix(name, ".config")
		if _, ok := methods[base+".update"]; ok {
			bases = append(bases, base)
		}
	}
	sort.Strings(bases)
	t.Logf("singleton config/update pairs discovered: %d", len(bases))

	type finding struct {
		base  string
		diffs []string
	}
	var findings []finding
	var errored int

	for _, base := range bases {
		before, err := callMap(ctx, c, base+".config")
		if err != nil {
			t.Logf("read %-24s err: %v", base, err)
			errored++
			continue
		}
		keys := acceptedKeys(methods[base+".update"])
		if len(keys) == 0 {
			continue
		}
		payload := map[string]any{}
		for _, k := range keys {
			if v, ok := before[k]; ok {
				payload[k] = v
			}
		}
		if len(payload) == 0 {
			continue
		}
		if _, err := c.Call(ctx, base+".update", payload); err != nil {
			// A write-back that the middleware rejects is itself suspicious
			// (it returned this value but won't accept it back), but many are
			// benign (a field became invalid in context). Record, don't fail.
			t.Logf("UPDATE %-24s rejected own values: %v", base, truncate(err.Error(), 160))
			errored++
			continue
		}
		after, err := callMap(ctx, c, base+".config")
		if err != nil {
			t.Logf("re-read %-22s err: %v", base, err)
			errored++
			continue
		}
		if d := mapDiff(before, after); len(d) > 0 {
			sort.Strings(d)
			findings = append(findings, finding{base, d})
		}
	}

	t.Logf("swept %d singletons (%d read/update errors)", len(bases), errored)
	if len(findings) == 0 {
		t.Logf("no fixed-point violations among singletons")
		return
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\n=== %d SINGLETON FIXED-POINT VIOLATIONS (write-back-own-values changed the server) ===\n", len(findings))
	for _, f := range findings {
		fmt.Fprintf(&b, "\n%s:\n", f.base)
		for _, d := range f.diffs {
			fmt.Fprintf(&b, "    %s\n", d)
		}
	}
	t.Errorf("%s", b.String())
}

func callMap(ctx context.Context, c interface {
	Call(context.Context, string, ...any) (json.RawMessage, error)
}, method string, args ...any) (map[string]any, error) {
	raw, err := c.Call(ctx, method, args...)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("decode %s: %w", method, err)
	}
	return m, nil
}

// acceptedKeys pulls the top-level writable property names from a singleton
// update's accepts: [ {additionalProperties:false, properties:{k:{...}}} ].
func acceptedKeys(accepts json.RawMessage) []string {
	if len(accepts) == 0 {
		return nil
	}
	var params []map[string]any
	if err := json.Unmarshal(accepts, &params); err != nil || len(params) == 0 {
		return nil
	}
	props, _ := params[len(params)-1]["properties"].(map[string]any)
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	return keys
}

func mapDiff(a, b map[string]any) []string {
	fa, fb := map[string]any{}, map[string]any{}
	flattenSweep("", a, fa)
	flattenSweep("", b, fb)
	seen := map[string]bool{}
	for k := range fa {
		seen[k] = true
	}
	for k := range fb {
		seen[k] = true
	}
	var out []string
	for k := range seen {
		top := k
		if i := strings.IndexByte(k, '.'); i >= 0 {
			top = k[:i]
		}
		if volatileTop[top] {
			continue
		}
		sa := fmt.Sprintf("%v", fa[k])
		sb := fmt.Sprintf("%v", fb[k])
		if sa != sb {
			out = append(out, fmt.Sprintf("%s: %q -> %q", k, sa, sb))
		}
	}
	return out
}

func flattenSweep(prefix string, v any, out map[string]any) {
	switch t := v.(type) {
	case map[string]any:
		for k, vv := range t {
			p := k
			if prefix != "" {
				p = prefix + "." + k
			}
			flattenSweep(p, vv, out)
		}
	case []any:
		for i, vv := range t {
			flattenSweep(fmt.Sprintf("%s[%d]", prefix, i), vv, out)
		}
	default:
		out[prefix] = v
	}
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

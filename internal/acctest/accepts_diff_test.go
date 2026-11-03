// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
	"github.com/truenas/terraform-provider-truenas/internal/provider"
)

// TestAcceptsDiff_Audit is a live API-contract guard against the create-only-
// field class of update bugs (the truenas_vm bootloader/secure_boot crash, the
// zvol volblocksize-as-updatable bug, GH #25 share_type). For each resource it
// diffs the TrueNAS method `accepts` for `<ns>.create` vs `<ns>.update`
// (core.get_methods): a field the create method accepts but the update method
// does NOT is create-only. If such a field maps to a writable (Optional/Required)
// schema attribute that is NOT marked RequiresReplace, the provider will resend
// it on an in-place update and the API will reject it — so this test fails.
//
// It needs a live box (it reads the server's method schemas): TF_ACC=1 plus the
// usual TRUENAS_* env. Report-only by default; the acceptsDiffRatchet allowlist
// records create-only writable fields that are intentionally handled another way
// (e.g. stripped from the update payload while kept RequiresReplace, like
// share_type — proven by TestAccDataset_shareTypeUpdate). It only shrinks.
//
// Scope: top-level accepts fields only, mapped to top-level schema attributes by
// name (with apiFieldAlias for the few that differ). The RequiresReplace-but-
// still-emitted subtlety is covered instead by each resource's update acceptance
// step (guaranteed to exist by TestLifecycleCoverage_Audit).
func TestAcceptsDiff_Audit(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Set TF_ACC=1 (and TRUENAS_* env) to run the live accepts-diff audit")
	}
	root := repoRoot(t)
	methods := getMethods(t)

	ctx := context.Background()
	p := provider.New("test")()

	var violations []string
	var reported int
	for _, ctor := range providerResources(ctx, p) {
		r := ctor()
		var md fwresource.MetadataResponse
		r.Metadata(ctx, fwresource.MetadataRequest{ProviderTypeName: "truenas"}, &md)
		var sc fwresource.SchemaResponse
		r.Schema(ctx, fwresource.SchemaRequest{}, &sc)

		ns := resourceMethodNamespace(t, root, md.TypeName)
		if ns == "" {
			continue // no create+update pair (singleton config, or dynamic method names)
		}
		createAccepts, okC := methods[ns+".create"]
		updateAccepts, okU := methods[ns+".update"]
		if !okC || !okU {
			continue
		}
		createOnly := map[string]bool{}
		for f := range acceptsFields(createAccepts) {
			if !acceptsFields(updateAccepts)[f] {
				createOnly[f] = true
			}
		}
		if len(createOnly) == 0 {
			continue
		}
		dir := filepath.Join(root, "internal", "resources", pkgDir(root, md.TypeName))
		schemaSrc := readFile(t, filepath.Join(dir, "schema.go"))
		pkgSrc := packageSource(dir)
		for name, attr := range sc.Schema.Attributes {
			if name == "id" || !isWritable(attr) {
				continue
			}
			apiField := name
			if a, ok := apiFieldAlias[md.TypeName+"."+name]; ok {
				apiField = a
			}
			if !createOnly[apiField] {
				continue
			}
			reported++
			if attrHasRequiresReplace(schemaSrc, name) {
				continue // a change recreates; correct handling for a create-only field
			}
			if fieldStrippedInUpdate(pkgSrc, apiField) {
				continue // explicitly deleted from the update payload
			}
			if acceptsDiffRatchet[md.TypeName+"."+name] {
				continue
			}
			violations = append(violations,
				md.TypeName+"."+name+" (api field "+apiField+" accepted by "+ns+".create but not "+ns+
					".update, and the attribute is not RequiresReplace — it will be resent on update and rejected)")
		}
	}

	sort.Strings(violations)
	t.Logf("accepts-diff: checked create-only writable fields, %d create-only/writable matches, %d violation(s)", reported, len(violations))
	for _, v := range violations {
		t.Errorf("%s", v)
	}
}

// apiFieldAlias maps a schema attribute name to the API field name when they
// differ. Keyed "<type_name>.<attr>".
var apiFieldAlias = map[string]string{
	"truenas_dataset.dedup": "deduplication",
	"truenas_zvol.dedup":    "deduplication",
}

// acceptsDiffRatchet allowlists create-only writable fields handled correctly by
// a means this source scan cannot see — the update payload simply never includes
// the field (omission), or the field is applied through a different API call. It
// is NOT for fields handled by RequiresReplace or an explicit delete() (those are
// detected automatically). Each entry names how the field is handled; verify that
// before adding one, and keep the list shrinking.
var acceptsDiffRatchet = map[string]bool{
	// app.updatePayload builds a minimal payload (custom_compose_config_string +
	// values only); version/train/catalog_app are never sent on update.
	"truenas_app.version": true,
	// pool.update is called with {autotrim} only. deduplication/checksum are
	// applied through a separate pool.dataset.update on the pool's root dataset;
	// name cannot change (the pool is keyed by id and refuses rename).
	"truenas_pool.checksum":      true,
	"truenas_pool.deduplication": true,
	"truenas_pool.name":          true,
	// group_create is a create-time flag ("also create a primary group"). It is
	// never added to user.updatePayload.
	"truenas_user.group_create": true,
}

func getMethods(t *testing.T) map[string]json.RawMessage {
	t.Helper()
	raw, err := acctest.Client().CallRead(context.Background(), "core.get_methods")
	if err != nil {
		t.Fatalf("core.get_methods: %v", err)
	}
	var m map[string]struct {
		Accepts json.RawMessage `json:"accepts"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("decode methods: %v", err)
	}
	out := map[string]json.RawMessage{}
	for k, v := range m {
		out[k] = v.Accepts
	}
	return out
}

// acceptsFields collects the top-level accepted field names from a method's
// `accepts` schema (a list of param schemas), recursing through oneOf/anyOf/allOf
// so union-typed params like pool.dataset.create are covered.
func acceptsFields(accepts json.RawMessage) map[string]bool {
	out := map[string]bool{}
	if len(accepts) == 0 {
		return out
	}
	var params []map[string]any
	if json.Unmarshal(accepts, &params) != nil {
		return out
	}
	var walk func(s map[string]any)
	walk = func(s map[string]any) {
		if props, ok := s["properties"].(map[string]any); ok {
			for k := range props {
				out[k] = true
			}
		}
		for _, key := range []string{"oneOf", "anyOf", "allOf"} {
			if subs, ok := s[key].([]any); ok {
				for _, sub := range subs {
					if sm, ok := sub.(map[string]any); ok {
						walk(sm)
					}
				}
			}
		}
	}
	for _, p := range params {
		walk(p)
	}
	return out
}

var reMethodLit = regexp.MustCompile(`"([a-z_]+(?:\.[a-z_]+)*)\.(create|update)"`)

// resourceMethodNamespace returns the API namespace (e.g. "pool.dataset") that a
// resource package calls BOTH <ns>.create and <ns>.update on, scanned from its
// non-test Go source. Empty if there is no single such pair (an update-only
// singleton config, or more than one candidate pair — too ambiguous to audit).
func resourceMethodNamespace(t *testing.T, root, typeName string) string {
	t.Helper()
	dir := filepath.Join(root, "internal", "resources", pkgDir(root, typeName))
	gofiles, _ := filepath.Glob(filepath.Join(dir, "*.go"))
	hasCreate, hasUpdate := map[string]bool{}, map[string]bool{}
	for _, g := range gofiles {
		if strings.HasSuffix(g, "_test.go") {
			continue
		}
		for _, m := range reMethodLit.FindAllStringSubmatch(readFile(t, g), -1) {
			if m[2] == "create" {
				hasCreate[m[1]] = true
			} else {
				hasUpdate[m[1]] = true
			}
		}
	}
	var pairs []string
	for ns := range hasCreate {
		if hasUpdate[ns] {
			pairs = append(pairs, ns)
		}
	}
	if len(pairs) == 1 {
		return pairs[0]
	}
	return ""
}

// pkgDir maps a resource type name to its package directory under
// internal/resources. The directory is the type name without the "truenas_"
// prefix for all current resources; verified to exist, else scanned for.
func pkgDir(root, typeName string) string {
	d := strings.TrimPrefix(typeName, "truenas_")
	if _, err := os.Stat(filepath.Join(root, "internal", "resources", d)); err == nil {
		return d
	}
	// Fallback: find the package whose source registers this TypeName.
	dirs, _ := filepath.Glob(filepath.Join(root, "internal", "resources", "*"))
	suffix := strings.TrimPrefix(typeName, "truenas")
	for _, dir := range dirs {
		gofiles, _ := filepath.Glob(filepath.Join(dir, "*.go"))
		for _, g := range gofiles {
			if strings.HasSuffix(g, "_test.go") {
				continue
			}
			b, _ := os.ReadFile(g)
			if strings.Contains(string(b), `ProviderTypeName + "`+suffix+`"`) {
				return filepath.Base(dir)
			}
		}
	}
	return d
}

// attrHasRequiresReplace reports whether the attribute carries a RequiresReplace
// or RequiresReplaceIf plan modifier. It handles both forms this codebase uses:
// an inline `schema.XAttribute{ ... }` block, and a helper call like
// `vmStrAttrReplace(...)` whose body (or name) carries the modifier.
func attrHasRequiresReplace(schemaSrc, attr string) bool {
	m := regexp.MustCompile(`"` + regexp.QuoteMeta(attr) + `":\s*([A-Za-z_][A-Za-z0-9_.]*)\s*([({])`).FindStringSubmatch(schemaSrc)
	if m == nil {
		return false
	}
	ident, delim := m[1], m[2]
	if delim == "{" { // inline schema.XAttribute{ ... }
		start := strings.Index(schemaSrc, m[0]) + len(m[0])
		rest := schemaSrc[start:]
		end := len(rest)
		if next := regexp.MustCompile(`\n\s*"[a-z0-9_]+":\s*`).FindStringIndex(rest); next != nil {
			end = next[0]
		}
		return hasReplaceToken(rest[:end])
	}
	// helper call: the modifier is in the helper's name or its body.
	if hasReplaceToken(ident) {
		return true
	}
	body := regexp.MustCompile(`(?s)func\s+` + regexp.QuoteMeta(ident) + `\s*\(.*?\n}`).FindString(schemaSrc)
	return hasReplaceToken(body)
}

func hasReplaceToken(s string) bool {
	// planmod.ReplaceFromKnown* (certificate create-only inputs, container.image)
	// is a conditional replace too: a known prior value that changes replaces,
	// and Update never resends the attribute. Pre-2026-11-02 the audit missed it
	// and reported 9 false positives on 28.0.
	return strings.Contains(s, "RequiresReplace") || strings.Contains(s, "replaceIf") ||
		strings.Contains(s, "ReplaceFromKnown")
}

// fieldStrippedInUpdate reports whether the package explicitly removes the API
// field from an update payload (delete(p, "field") / delete(payload, "field")).
func fieldStrippedInUpdate(pkgSrc, apiField string) bool {
	return regexp.MustCompile(`delete\(\s*\w+\s*,\s*"` + regexp.QuoteMeta(apiField) + `"\s*\)`).MatchString(pkgSrc)
}

// packageSource concatenates a package's non-test Go source.
func packageSource(dir string) string {
	gofiles, _ := filepath.Glob(filepath.Join(dir, "*.go"))
	var b strings.Builder
	for _, g := range gofiles {
		if strings.HasSuffix(g, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(g)
		if err == nil {
			b.Write(raw)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestImportIgnore_Justified guards against the class of bug behind GH #32: an
// attribute was excluded from ImportStateVerify (via ImportStateVerifyIgnore or
// the ImportReapplyNoop helper) because the provider didn't read it back — which
// is a round-trip bug, not a legitimate exemption, and the ignore hid it.
//
// Every ignored attribute must appear in importIgnoreJustified with a reason
// stating why it genuinely cannot round-trip (a write-only secret the API never
// returns, a create-time-only flag, or a server-owned value that churns). "We
// don't read it" is NOT a valid reason — fix the read instead. The list only
// shrinks: when a field starts round-tripping, drop its ignore and its entry.
//
// Pure source scan (no TF_ACC): it reads the acceptance_test.go sources.
func TestImportIgnore_Justified(t *testing.T) {
	root := repoRoot(t)
	ignores := scanImportIgnores(t, root) // type.attr -> true

	if os.Getenv("COVERAGE_AUDIT_EMIT") == "1" {
		var keys []string
		for k := range ignores {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var b strings.Builder
		b.WriteString("\nvar importIgnoreJustified = map[string]string{\n")
		for _, k := range keys {
			fmt.Fprintf(&b, "\t%q: %q,\n", k, "TODO")
		}
		b.WriteString("}\n")
		t.Log(b.String())
	}

	var keys []string
	for k := range ignores {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if reason, ok := importIgnoreJustified[k]; !ok || strings.TrimSpace(reason) == "" || reason == "TODO" {
			t.Errorf("%s is excluded from ImportStateVerify but has no justification in importIgnoreJustified — "+
				"add a reason proving it cannot round-trip (a write-only secret, a create-only flag, or a server-owned "+
				"value), or fix the read so it round-trips and drop the ignore", k)
		}
	}
}

var (
	reISVIList    = regexp.MustCompile(`ImportStateVerifyIgnore:\s*\[\]string\{([^}]*)\}`)
	reReapply     = regexp.MustCompile(`ImportReapplyNoop\(([^)]*)\)`)
	reQuotedToken = regexp.MustCompile(`"([^"]+)"`)
)

// scanImportIgnores returns the set of "<type>.<attr>" excluded from
// ImportStateVerify across every acceptance_test.go, attributing each to the
// resource type(s) its package registers.
func scanImportIgnores(t *testing.T, root string) map[string]bool {
	t.Helper()
	// Hunt tests carry ImportStateVerifyIgnore lists too; an unjustified
	// ignore there hides a round-trip bug just as well as one in
	// acceptance_test.go does (2026-10-30: three such ignores slipped in).
	files, err := filepath.Glob(filepath.Join(root, "internal", "resources", "*", "acceptance_test.go"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	hunt, err := filepath.Glob(filepath.Join(root, "internal", "resources", "*", "hunt_test.go"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	files = append(files, hunt...)
	out := map[string]bool{}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		src := string(raw)
		types := packageResourceTypes(t, filepath.Dir(f))
		if len(types) == 0 {
			continue
		}
		var attrs []string
		// ImportStateVerifyIgnore: []string{"a", "b"}
		for _, m := range reISVIList.FindAllStringSubmatch(src, -1) {
			for _, q := range reQuotedToken.FindAllStringSubmatch(m[1], -1) {
				attrs = append(attrs, q[1])
			}
		}
		// ImportReapplyNoop(addr, config, "a", "b") — drop the first quoted token
		// (the resource address), the rest are ignored attributes.
		for _, m := range reReapply.FindAllStringSubmatch(src, -1) {
			q := reQuotedToken.FindAllStringSubmatch(m[1], -1)
			for i, qq := range q {
				if i == 0 {
					continue // resource address literal
				}
				attrs = append(attrs, qq[1])
			}
		}
		for _, a := range attrs {
			for _, tn := range types {
				out[tn+"."+a] = true
			}
		}
	}
	return out
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/provider"
)

// TestLifecycleCoverage_Audit checks, for every managed resource, that its
// acceptance test exercises the Terraform lifecycle phases that a plain
// create+destroy test skips — UPDATE and IMPORT. These are the phases where
// the create path is reused incorrectly (update sends create-only fields:
// GH #25/#26; a create-only field forces replace on an imported resource:
// GH #29) or a read-back races a transient state (GH #28). The attribute
// auditor (TestAttributeCoverage_Audit) proves every field is *set*; this
// proves every resource is *updated and imported* at least once.
//
// Signals, scanned from the acceptance_test.go source (no TF_ACC, no box):
//   - UPDATE: some resource.Test has a Steps block with 2+ apply steps
//     (2+ `Config:` entries in one test func) — a second apply that can
//     diff an attribute in place.
//   - IMPORT: some step sets `ImportState: true`.
//
// Report-only by default; COVERAGE_AUDIT_STRICT=1 fails on any gap not on the
// lifecycleRatchet allowlist. The ratchet only ever SHRINKS — a new resource
// must ship update + import steps, not a ratchet entry.
func TestLifecycleCoverage_Audit(t *testing.T) {
	root := repoRoot(t)
	funcs := acceptanceTestFuncs(t, root)

	ctx := context.Background()
	p := provider.New("test")()

	type gap struct {
		typeName           string
		noUpdate, noImport bool
	}
	var gaps []gap
	var total, okUpdate, okImport int

	for _, ctor := range providerResources(ctx, p) {
		r := ctor()
		var md fwresource.MetadataResponse
		r.Metadata(ctx, fwresource.MetadataRequest{ProviderTypeName: "truenas"}, &md)

		if coverageSkip[md.TypeName] {
			continue // acceptance test intentionally skipped (external dependency)
		}
		total++

		chunks := funcs[md.TypeName]
		var hasUpdate, hasImport bool
		for _, c := range chunks {
			if countConfigSteps(c) >= 2 {
				hasUpdate = true
			}
			if reImportStep.MatchString(c) {
				hasImport = true
			}
		}
		if hasUpdate {
			okUpdate++
		}
		if hasImport {
			okImport++
		}
		if !hasUpdate || !hasImport {
			gaps = append(gaps, gap{md.TypeName, !hasUpdate, !hasImport})
		}
	}

	sort.Slice(gaps, func(i, j int) bool { return gaps[i].typeName < gaps[j].typeName })
	var b strings.Builder
	fmt.Fprintf(&b, "\nLifecycle coverage over %d resources: update %d/%d, import %d/%d\n",
		total, okUpdate, total, okImport, total)
	for _, g := range gaps {
		var miss []string
		if g.noUpdate {
			miss = append(miss, "update")
		}
		if g.noImport {
			miss = append(miss, "import")
		}
		fmt.Fprintf(&b, "  %-28s missing: %s\n", g.typeName, strings.Join(miss, ", "))
	}
	t.Log(b.String())

	// Emit a copy-pasteable ratchet map: `COVERAGE_AUDIT_EMIT=1 go test -run LifecycleCoverage -v`.
	if os.Getenv("COVERAGE_AUDIT_EMIT") == "1" {
		var e strings.Builder
		e.WriteString("\nvar lifecycleRatchet = map[string]lifecyclePhases{\n")
		for _, g := range gaps {
			fmt.Fprintf(&e, "\t%q: {noUpdate: %v, noImport: %v},\n", g.typeName, g.noUpdate, g.noImport)
		}
		e.WriteString("}\n")
		t.Log(e.String())
	}

	for _, g := range gaps {
		allow := lifecycleRatchet[g.typeName]
		if g.noUpdate && !allow.noUpdate {
			t.Errorf("%s: acceptance test never exercises UPDATE (add a second apply step with a changed attribute, or — only for accepted debt — add to lifecycleRatchet)", g.typeName)
		}
		if g.noImport && !allow.noImport {
			t.Errorf("%s: acceptance test never exercises IMPORT (add an ImportState step, or — only for accepted debt — add to lifecycleRatchet)", g.typeName)
		}
	}
}

// reImportStep matches an import step: an inline `ImportState: true` TestStep,
// a call to the ImportReapplyNoop helper (which builds an import step plus
// an empty-plan re-apply — see acctest/import_steps.go), or a config-driven
// `import {` block in a step's HCL.
var reImportStep = regexp.MustCompile(`(?m)\bImportState:\s*true\b|\bImportReapplyNoop\(|^\s*import\s*\{`)

// countConfigSteps counts `Config:` TestStep fields in one test-func body. A
// func with 2+ is a multi-apply (update) test; 1 is create-only.
func countConfigSteps(funcBody string) int {
	return len(regexp.MustCompile(`(?m)^\s*Config:`).FindAllString(funcBody, -1))
}

// acceptanceTestFuncs maps a resource type name to the bodies of the
// acceptance test functions (those that call resource.Test) in its own
// package. A file's test funcs are attributed only to the resource type(s)
// that package registers (its Metadata `TypeName`), never to a dependency
// resource the config merely references — so a join resource used as a
// fixture in another package's update test is not falsely credited.
func acceptanceTestFuncs(t *testing.T, root string) map[string][]string {
	t.Helper()
	// Every resource-package test file counts (acceptance tests are split
	// across files such as force_destroy_test.go); hunt_* files reproduce
	// known bugs and are not coverage.
	all, err := filepath.Glob(filepath.Join(root, "internal", "resources", "*", "*_test.go"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	var files []string
	for _, f := range all {
		if !strings.HasPrefix(filepath.Base(f), "hunt_") {
			files = append(files, f)
		}
	}
	out := map[string][]string{}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		types := packageResourceTypes(t, filepath.Dir(f))
		for _, body := range splitTopLevelFuncs(string(raw)) {
			if !strings.Contains(body, "resource.Test(") {
				continue
			}
			for _, tn := range types {
				out[tn] = append(out[tn], body)
			}
		}
	}
	return out
}

var reTypeName = regexp.MustCompile(`TypeName\s*=\s*req\.ProviderTypeName\s*\+\s*"(_[a-z0-9_]+)"`)

// packageResourceTypes returns the resource type names a package registers,
// read from its non-test Go source (`resp.TypeName = req.ProviderTypeName +
// "_x"` in the Metadata method).
func packageResourceTypes(t *testing.T, dir string) []string {
	t.Helper()
	gofiles, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatalf("glob %s: %v", dir, err)
	}
	seen := map[string]bool{}
	var types []string
	for _, g := range gofiles {
		if strings.HasSuffix(g, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(g)
		if err != nil {
			t.Fatalf("read %s: %v", g, err)
		}
		for _, m := range reTypeName.FindAllStringSubmatch(string(raw), -1) {
			tn := "truenas" + m[1]
			if !seen[tn] {
				seen[tn] = true
				types = append(types, tn)
			}
		}
	}
	return types
}

// splitTopLevelFuncs returns each top-level `func ...` body in a Go source
// file (from one `func` keyword to the next). Good enough for test files,
// which declare funcs only at column 0.
func splitTopLevelFuncs(src string) []string {
	idx := regexp.MustCompile(`(?m)^func `).FindAllStringIndex(src, -1)
	var out []string
	for i, loc := range idx {
		end := len(src)
		if i+1 < len(idx) {
			end = idx[i+1][0]
		}
		out = append(out, src[loc[0]:end])
	}
	return out
}

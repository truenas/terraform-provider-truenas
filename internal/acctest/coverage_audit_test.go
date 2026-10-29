// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/provider"
)

// TestAttributeCoverage_Audit reports, for every managed resource, which
// writable (Optional or Required) schema attributes are never exercised by the
// resource's acceptance test config. An attribute that is set in a create step
// and then left to an update step is what surfaces update-payload and drift
// bugs (see the truenas_vm create-only-fields fix); an attribute never set in
// any acceptance config is untested surface.
//
// This is a plain unit test (no TF_ACC, no live box): it reads the schema and
// greps the acceptance_test.go source. It FAILS on any writable attribute not
// exercised by an acceptance config and not in the coverageRatchet allowlist;
// TestCoverageRatchet_Justified additionally fails on any bare (unreasoned)
// ratchet entry, so unexercised attributes cannot hide as silent debt.
func TestAttributeCoverage_Audit(t *testing.T) {
	root := repoRoot(t)
	testSrc := acceptanceSources(t, root)

	ctx := context.Background()
	p := provider.New("test")()

	var totalAttrs, totalCovered int
	type gap struct {
		typeName string
		missing  []string
		total    int
	}
	var gaps []gap

	for _, ctor := range providerResources(ctx, p) {
		r := ctor()
		var md fwresource.MetadataResponse
		r.Metadata(ctx, fwresource.MetadataRequest{ProviderTypeName: "truenas"}, &md)
		var sc fwresource.SchemaResponse
		r.Schema(ctx, fwresource.SchemaRequest{}, &sc)

		if coverageSkip[md.TypeName] {
			continue // acceptance test intentionally skipped (external dependency)
		}
		src, ok := testSrc[md.TypeName]
		if !ok {
			t.Errorf("%s: no acceptance_test.go references resource %q (add one, or add to coverageSkip with a reason)", md.TypeName, md.TypeName)
			continue
		}

		var missing []string
		var writable int
		for name, attr := range sc.Schema.Attributes {
			if name == "id" || !isWritable(attr) {
				continue
			}
			writable++
			totalAttrs++
			if attrExercised(src, name) {
				totalCovered++
			} else {
				missing = append(missing, name)
			}
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			gaps = append(gaps, gap{md.TypeName, missing, writable})
		}
	}

	sort.Slice(gaps, func(i, j int) bool { return len(gaps[i].missing) > len(gaps[j].missing) })
	var b strings.Builder
	fmt.Fprintf(&b, "\nAttribute coverage: %d/%d writable attributes exercised (%.0f%%)\n",
		totalCovered, totalAttrs, 100*float64(totalCovered)/float64(totalAttrs))
	fmt.Fprintf(&b, "%d resource(s) with unexercised attributes:\n", len(gaps))
	for _, g := range gaps {
		fmt.Fprintf(&b, "  %-28s %d/%d missing: %s\n", g.typeName, len(g.missing), g.total, strings.Join(g.missing, ", "))
	}
	t.Log(b.String())

	// Emit mode: print a copy-pasteable ratchet map of the current gaps, so the
	// allowlist can be regenerated. `COVERAGE_AUDIT_EMIT=1 go test -run Audit -v`.
	if os.Getenv("COVERAGE_AUDIT_EMIT") == "1" {
		var e strings.Builder
		e.WriteString("\nvar coverageRatchet = map[string]bool{\n")
		for _, g := range gaps {
			for _, m := range g.missing {
				fmt.Fprintf(&e, "\t%q: true,\n", g.typeName+"."+m)
			}
		}
		e.WriteString("}\n")
		t.Log(e.String())
	}

	// Enforce the ratchet: any writable attribute not exercised and not on the
	// (shrinking) allowlist fails. New attributes must be exercised, not ratcheted.
	for _, g := range gaps {
		for _, m := range g.missing {
			if !ratchetAllows(g.typeName, m) {
				t.Errorf("%s.%s: writable attribute not exercised by any acceptance config (add it to the test, or — only for accepted debt — to coverageRatchet)", g.typeName, m)
			}
		}
	}
}

// coverageSkip lists resources whose acceptance test is intentionally skipped
// because create validates against an external endpoint absent from the lab
// (a container registry, a vCenter/ESXi host, the TrueNAS Connect cloud). They
// are excluded from the attribute-coverage requirement.
var coverageSkip = map[string]bool{
	"truenas_app_registry":      true, // app.registry.create validates creds against a live registry
	"truenas_vmware":            true, // vmware.create validates against a live vCenter/ESXi
	"truenas_tn_connect_config": true, // tn_connect validates against the TrueNAS Connect cloud
}

func isWritable(a rschema.Attribute) bool {
	return a.IsOptional() || a.IsRequired()
}

// attrExercised reports whether the HCL config sets this attribute (an
// `attr =` assignment somewhere in the acceptance source).
func attrExercised(src, name string) bool {
	re := regexp.MustCompile(`(?m)\b` + regexp.QuoteMeta(name) + `\s*=`)
	return re.MatchString(src)
}

func providerResources(ctx context.Context, p interface {
	Resources(context.Context) []func() fwresource.Resource
}) []func() fwresource.Resource {
	return p.Resources(ctx)
}

// acceptanceSources maps a resource type name to the concatenated source of
// every resource-package test file that instantiates it (`resource "truenas_x"`).
// All *_test.go files count, not just acceptance_test.go, so acceptance tests
// split into their own files (e.g. force_destroy_test.go) are seen. hunt_*
// files are excluded: they reproduce known bugs and are not coverage.
func acceptanceSources(t *testing.T, root string) map[string]string {
	t.Helper()
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
	// Match both literal-quote blocks (resource "truenas_x") and escaped-quote
	// blocks built inside Go string literals (fmt.Sprintf("resource \"truenas_x\"")).
	blockRe := regexp.MustCompile(`resource\s+\\?"(truenas_[a-z0-9_]+)\\?"`)
	out := map[string]string{}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		s := string(b)
		seen := map[string]bool{}
		for _, m := range blockRe.FindAllStringSubmatch(s, -1) {
			if seen[m[1]] {
				continue
			}
			seen[m[1]] = true
			out[m[1]] += "\n" + s
		}
	}
	return out
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// this file is internal/acctest/coverage_audit_test.go
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest_test

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/truenas/terraform-provider-truenas/internal/planmod"
	"github.com/truenas/terraform-provider-truenas/internal/provider"
	"github.com/truenas/terraform-provider-truenas/internal/resources/dataset"
)

// TestRenameNeverReplaces_Audit guards hunt finding F8: until 2026-10-30 the
// `name` of truenas_dataset and truenas_zvol carried RequiresReplace, so a
// rename in config destroyed the dataset (and its files) and created an empty
// one. `name` now renames in place on the guid identity; a replace-forcing
// plan modifier (RequiresReplace, RequiresReplaceIf, planmod.ReplaceFrom*) on
// it would bring the data loss back. Pure schema walk, no TF_ACC.
func TestRenameNeverReplaces_Audit(t *testing.T) {
	ctx := context.Background()
	p := provider.New("test")()
	seen := map[string]bool{}
	for _, f := range providerResources(ctx, p) {
		r := f()
		var md fwresource.MetadataResponse
		r.Metadata(ctx, fwresource.MetadataRequest{ProviderTypeName: "truenas"}, &md)
		if md.TypeName != "truenas_dataset" && md.TypeName != "truenas_zvol" {
			continue
		}
		seen[md.TypeName] = true
		var sr fwresource.SchemaResponse
		r.Schema(ctx, fwresource.SchemaRequest{}, &sr)
		a, ok := sr.Schema.Attributes["name"]
		if !ok {
			t.Errorf("%s: no name attribute", md.TypeName)
			continue
		}
		mods := attrModifiers("name", a)["name"]
		for _, m := range mods {
			if replaceModifier(ctx, m) {
				t.Errorf("%s.name carries the replace-forcing plan modifier %q. F8: a rename must stay an in-place "+
					"pool.dataset.rename (the destroy half of a replacement took the data with it, see "+
					"docs-dev/hunt-findings-2026-10-30.md F8). Remove it.", md.TypeName, m.Description(ctx))
			}
		}
	}
	for _, want := range []string{"truenas_dataset", "truenas_zvol"} {
		if !seen[want] {
			t.Errorf("%s not found among provider resources; audit walker is broken", want)
		}
	}
}

// replaceModifier reports a plan modifier that can force a replacement: the
// framework's RequiresReplace / RequiresReplaceIf* (their descriptions say the
// resource is destroyed and recreated) or planmod.ReplaceFrom*.
func replaceModifier(ctx context.Context, m planmodifier.Describer) bool {
	if hasUnconditionalRequiresReplace(ctx, []planmodifier.Describer{m}) || anyFromKnown([]planmodifier.Describer{m}) {
		return true
	}
	d := strings.ToLower(m.Description(ctx))
	if strings.Contains(d, "destroy and recreate") || strings.Contains(d, "recreate") || strings.Contains(d, "replace") {
		return true
	}
	// A RequiresReplaceIf with a custom description is caught by its type
	// (stringplanmodifier.requiresReplaceIfModifier and friends).
	typ := strings.ToLower(fmt.Sprintf("%T", m))
	return strings.Contains(typ, "requiresreplace") || strings.Contains(typ, "replacefrom")
}

// TestRenameNeverReplaces_DetectorSelfTest pins replaceModifier to the real
// modifiers, so the audit cannot pass by failing to recognise them.
func TestRenameNeverReplaces_DetectorSelfTest(t *testing.T) {
	ctx := context.Background()
	for _, m := range replaceModifierSamples() {
		if !replaceModifier(ctx, m) {
			t.Errorf("replace-forcing modifier %q not detected", m.Description(ctx))
		}
	}
	for _, m := range nonReplaceModifierSamples() {
		if replaceModifier(ctx, m) {
			t.Errorf("harmless modifier %q misclassified as replace-forcing", m.Description(ctx))
		}
	}
}

// deleteCallSiteExempt lists pool.dataset.delete call sites outside
// dataset.DeleteDataset that are allowed, with why. Only cleanup tooling that
// never runs inside a Terraform operation belongs here.
var deleteCallSiteExempt = map[string]string{
	"internal/loadtest/sweep.go": "load-test leftover sweeper: deletes only the tf-acc-* datasets the load test created, never user data, and runs from the CLI, not from the provider",
}

// TestDatasetDeleteCallSites_Audit guards the v1.6.1 destroy guard (F8): every
// pool.dataset.delete the provider issues must go through dataset.DeleteDataset,
// the helper that refuses to take child datasets and snapshots with it unless
// force_destroy is set (the callers add the contents / holds-data checks). A
// direct call elsewhere would bypass the guard. Pure source scan of internal/,
// no TF_ACC.
func TestDatasetDeleteCallSites_Audit(t *testing.T) {
	root := repoRoot(t)
	var offenders []string
	var inHelper int
	err := filepath.Walk(filepath.Join(root, "internal"), func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !strings.Contains(string(src), "pool.dataset.delete") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			return err
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING || !strings.Contains(lit.Value, "pool.dataset.delete") {
				return true
			}
			fn := enclosingFunc(f, lit.Pos())
			line := fset.Position(lit.Pos()).Line
			if rel == filepath.Join("internal", "resources", "dataset", "delete.go") && fn == "DeleteDataset" {
				inHelper++
				return true
			}
			if _, ok := deleteCallSiteExempt[filepath.ToSlash(rel)]; ok {
				return true
			}
			offenders = append(offenders, rel+":"+itoa(line)+" (in "+fn+")")
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if inHelper == 0 {
		t.Fatal("no pool.dataset.delete inside dataset.DeleteDataset; the audit's reference point moved")
	}
	sort.Strings(offenders)
	for _, o := range offenders {
		t.Errorf("%s calls pool.dataset.delete directly, bypassing the force_destroy / non-empty guard; use "+
			"dataset.DeleteDataset (F8, v1.6.1 destroy guard)", o)
	}
}

func enclosingFunc(f *ast.File, pos token.Pos) string {
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Pos() <= pos && pos <= fd.End() {
			return fd.Name.Name
		}
	}
	return "<file scope>"
}

func itoa(i int) string { return strconv.Itoa(i) }

// replaceModifierSamples are every replace-forcing modifier the provider may
// use; nonReplaceModifierSamples the common harmless ones.
func replaceModifierSamples() []planmodifier.Describer {
	return []planmodifier.Describer{
		stringplanmodifier.RequiresReplace(),
		stringplanmodifier.RequiresReplaceIfConfigured(),
		stringplanmodifier.RequiresReplaceIf(func(context.Context, planmodifier.StringRequest, *stringplanmodifier.RequiresReplaceIfFuncResponse) {}, "custom", "custom"),
		int64planmodifier.RequiresReplace(),
		boolplanmodifier.RequiresReplace(),
		planmod.ReplaceFromKnownString(),
		planmod.ReplaceFromKnownStringFold(),
		planmod.ReplaceFromKnownBool(),
	}
}

func nonReplaceModifierSamples() []planmodifier.Describer {
	return []planmodifier.Describer{
		stringplanmodifier.UseStateForUnknown(),
		int64planmodifier.UseStateForUnknown(),
		dataset.IDFollowsName(),
		dataset.MountpointFollowsName(),
	}
}

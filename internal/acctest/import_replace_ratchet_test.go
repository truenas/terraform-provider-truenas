// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest_test

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/truenas/terraform-provider-truenas/internal/planmod"
	"github.com/truenas/terraform-provider-truenas/internal/provider"
)

// TestImportReplace_Audit guards cluster C1 (BUGS B-002..B-009): an attribute
// with the framework's unconditional RequiresReplace() that import leaves null
// makes `import` + `plan` destroy and recreate the object the user adopted.
//
// Every attribute carrying the stock RequiresReplace() must be listed in
// importReplaceReadBack with where import reads it from the API. An attribute
// the API does not return must use planmod.ReplaceFromKnown* instead (a null
// prior value then records in place) and must not be listed. Pure schema walk,
// no TF_ACC.
func TestImportReplace_Audit(t *testing.T) {
	ctx := context.Background()
	stock, fromKnown := requiresReplaceAttrs(ctx)
	if os.Getenv("COVERAGE_AUDIT_EMIT") == "1" {
		var b strings.Builder
		b.WriteString("\nvar importReplaceReadBack = map[string]string{\n")
		for _, k := range stock {
			fmt.Fprintf(&b, "\t%q: %q,\n", k, importReplaceReadBack[k])
		}
		b.WriteString("}\n")
		t.Log(b.String())
	}

	if len(stock) == 0 || len(fromKnown) == 0 {
		t.Fatalf("schema walk found %d stock / %d from-known RequiresReplace attributes; walker is broken", len(stock), len(fromKnown))
	}

	stockSet := map[string]bool{}
	for _, k := range stock {
		stockSet[k] = true
		if reason := strings.TrimSpace(importReplaceReadBack[k]); reason == "" {
			if p, ok := importReplacePending[k]; ok && strings.TrimSpace(p) != "" {
				continue
			}
			t.Errorf("%s has an unconditional RequiresReplace() but no entry in importReplaceReadBack. If import "+
				"reads it from the API, add the entry saying where; if not, use planmod.ReplaceFromKnown* so a null "+
				"value after import records in place instead of replacing the object", k)
		}
	}
	for k := range importReplaceReadBack {
		if !stockSet[k] {
			t.Errorf("importReplaceReadBack entry %s no longer has a stock RequiresReplace(); remove it", k)
		}
	}
	for _, k := range fromKnown {
		if _, ok := importReplaceReadBack[k]; ok {
			t.Errorf("%s uses planmod.ReplaceFromKnown*; remove it from importReplaceReadBack", k)
		}
	}
}

// TestImportReplace_DetectorSelfTest pins the walker to the real modifiers.
func TestImportReplace_DetectorSelfTest(t *testing.T) {
	ctx := context.Background()
	stockAttr := rschema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}}
	if !hasUnconditionalRequiresReplace(ctx, describers(stockAttr.PlanModifiers)) {
		t.Fatal("stock RequiresReplace() not detected; the framework description string may have changed")
	}
	fk := rschema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{planmod.ReplaceFromKnownString()}}
	if hasUnconditionalRequiresReplace(ctx, describers(fk.PlanModifiers)) || !anyFromKnown(describers(fk.PlanModifiers)) {
		t.Fatal("planmod.ReplaceFromKnownString misclassified")
	}
}

func anyFromKnown(mods []planmodifier.Describer) bool {
	for _, m := range mods {
		if planmod.IsReplaceFromKnown(m) {
			return true
		}
	}
	return false
}

// attrModifiers returns the plan modifiers of a resource schema attribute and
// its nested attributes (keyed by dotted path relative to the attribute).
func attrModifiers(name string, a rschema.Attribute) map[string][]planmodifier.Describer {
	out := map[string][]planmodifier.Describer{}
	var nested map[string]rschema.Attribute
	switch v := a.(type) {
	case rschema.StringAttribute:
		out[name] = describers(v.PlanModifiers)
	case rschema.BoolAttribute:
		out[name] = describers(v.PlanModifiers)
	case rschema.Int64Attribute:
		out[name] = describers(v.PlanModifiers)
	case rschema.Float64Attribute:
		out[name] = describers(v.PlanModifiers)
	case rschema.NumberAttribute:
		out[name] = describers(v.PlanModifiers)
	case rschema.ListAttribute:
		out[name] = describers(v.PlanModifiers)
	case rschema.SetAttribute:
		out[name] = describers(v.PlanModifiers)
	case rschema.MapAttribute:
		out[name] = describers(v.PlanModifiers)
	case rschema.ObjectAttribute:
		out[name] = describers(v.PlanModifiers)
	case rschema.SingleNestedAttribute:
		out[name] = describers(v.PlanModifiers)
		nested = v.Attributes
	case rschema.ListNestedAttribute:
		out[name] = describers(v.PlanModifiers)
		nested = v.NestedObject.Attributes
	case rschema.SetNestedAttribute:
		out[name] = describers(v.PlanModifiers)
		nested = v.NestedObject.Attributes
	case rschema.MapNestedAttribute:
		out[name] = describers(v.PlanModifiers)
		nested = v.NestedObject.Attributes
	}
	for n, na := range nested {
		for k, m := range attrModifiers(name+"."+n, na) {
			out[k] = m
		}
	}
	return out
}

// requiresReplaceAttrs walks every provider resource schema and returns the
// "<type>.<attr>" paths carrying the stock RequiresReplace() and those
// carrying planmod.ReplaceFromKnown*.
func requiresReplaceAttrs(ctx context.Context) (stock, fromKnown []string) {
	p := provider.New("test")()
	for _, f := range providerResources(ctx, p) {
		r := f()
		var md fwresource.MetadataResponse
		r.Metadata(ctx, fwresource.MetadataRequest{ProviderTypeName: "truenas"}, &md)
		var sr fwresource.SchemaResponse
		r.Schema(ctx, fwresource.SchemaRequest{}, &sr)
		for name, a := range sr.Schema.Attributes {
			for path, mods := range attrModifiers(name, a) {
				key := md.TypeName + "." + path
				if hasUnconditionalRequiresReplace(ctx, mods) {
					stock = append(stock, key)
				}
				if anyFromKnown(mods) {
					fromKnown = append(fromKnown, key)
				}
			}
		}
	}
	sort.Strings(stock)
	sort.Strings(fromKnown)
	return stock, fromKnown
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest_test

import (
	"context"
	"sort"
	"testing"

	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/truenas/terraform-provider-truenas/internal/provider"
)

// stockRequiresReplaceDesc is the description of the framework's
// unconditional RequiresReplace() plan modifiers. Our RequiresReplaceIf
// wrappers return a different description and are not flagged.
const stockRequiresReplaceDesc = "If the value of this attribute changes, Terraform will destroy and recreate the resource."

// hasUnconditionalRequiresReplace reports whether any plan modifier is the
// framework's stock RequiresReplace().
func hasUnconditionalRequiresReplace(ctx context.Context, mods []planmodifier.Describer) bool {
	for _, m := range mods {
		if m.Description(ctx) == stockRequiresReplaceDesc {
			return true
		}
	}
	return false
}

// attrIsDefaultReplaceTrap reports whether a schema attribute (Bool, Int64 or
// String) has both a Default and an unconditional RequiresReplace.
func attrIsDefaultReplaceTrap(ctx context.Context, a any) bool {
	switch v := a.(type) {
	case rschema.BoolAttribute:
		return v.Default != nil && hasUnconditionalRequiresReplace(ctx, describers(v.PlanModifiers))
	case rschema.Int64Attribute:
		return v.Default != nil && hasUnconditionalRequiresReplace(ctx, describers(v.PlanModifiers))
	case rschema.StringAttribute:
		return v.Default != nil && hasUnconditionalRequiresReplace(ctx, describers(v.PlanModifiers))
	}
	return false
}

func describers[T planmodifier.Describer](mods []T) []planmodifier.Describer {
	out := make([]planmodifier.Describer, len(mods))
	for i, m := range mods {
		out[i] = m
	}
	return out
}

// defaultReplaceTraps walks every provider resource schema and returns
// "<type>.<attr>" for each trap attribute, plus the resource/attribute counts.
func defaultReplaceTraps(ctx context.Context) (traps []string, resources, attrs int) {
	p := provider.New("test")()
	for _, f := range providerResources(ctx, p) {
		r := f()
		var md fwresource.MetadataResponse
		r.Metadata(ctx, fwresource.MetadataRequest{ProviderTypeName: "truenas"}, &md)
		var sr fwresource.SchemaResponse
		r.Schema(ctx, fwresource.SchemaRequest{}, &sr)
		resources++
		for name, a := range sr.Schema.Attributes {
			attrs++
			if attrIsDefaultReplaceTrap(ctx, a) {
				traps = append(traps, md.TypeName+"."+name)
			}
		}
	}
	sort.Strings(traps)
	return traps, resources, attrs
}

func TestImmutableDefaultTrap_Audit(t *testing.T) {
	ctx := context.Background()
	traps, resources, attrs := defaultReplaceTraps(ctx)
	if resources == 0 || attrs == 0 {
		t.Fatalf("audit walked %d resources / %d attributes; schema walk is broken", resources, attrs)
	}
	trapSet := map[string]bool{}
	for _, k := range traps {
		trapSet[k] = true
		if _, ok := immutableDefaultTrapRatchet[k]; !ok {
			t.Errorf("%s has a schema Default and an unconditional RequiresReplace(): upgrading from null state "+
				"would replace every instance. Use a conditional RequiresReplaceIf (see truenas_certificate.store_privatekey).", k)
		}
	}
	for k := range immutableDefaultTrapRatchet {
		if !trapSet[k] {
			t.Errorf("ratchet entry %s is no longer a trap; remove it", k)
		}
	}
	for _, known := range []string{"truenas_certificate.store_privatekey", "truenas_replication_task.transport"} {
		if trapSet[known] {
			t.Errorf("%s must use RequiresReplaceIf but was flagged as an unconditional trap", known)
		}
	}
}

// TestImmutableDefaultTrap_DetectorSelfTest pins the stock-string match to the
// real framework modifier, so a framework wording change cannot silently
// disable detection.
func TestImmutableDefaultTrap_DetectorSelfTest(t *testing.T) {
	ctx := context.Background()
	trap := rschema.BoolAttribute{
		Optional:      true,
		Computed:      true,
		Default:       booldefault.StaticBool(true),
		PlanModifiers: []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
	}
	if !attrIsDefaultReplaceTrap(ctx, trap) {
		t.Fatal("detector missed a Default + RequiresReplace() attribute; the stock description string may have changed")
	}
	noDefault := rschema.BoolAttribute{
		Optional:      true,
		PlanModifiers: []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
	}
	if attrIsDefaultReplaceTrap(ctx, noDefault) {
		t.Fatal("detector flagged an attribute with no Default")
	}
	noReplace := rschema.BoolAttribute{
		Optional: true,
		Computed: true,
		Default:  booldefault.StaticBool(true),
	}
	if attrIsDefaultReplaceTrap(ctx, noReplace) {
		t.Fatal("detector flagged an attribute with no RequiresReplace")
	}
}

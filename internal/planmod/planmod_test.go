// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package planmod

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var objType = tftypes.Object{AttributeTypes: map[string]tftypes.Type{"a": tftypes.String}}

func raw(null bool) tftypes.Value {
	if null {
		return tftypes.NewValue(objType, nil)
	}
	return tftypes.NewValue(objType, map[string]tftypes.Value{"a": tftypes.NewValue(tftypes.String, "x")})
}

func TestReplaceFromKnownString(t *testing.T) {
	cases := []struct {
		name                string
		stateNull, planNull bool // whole-resource state/plan
		stateVal, planVal   types.String
		want                bool
	}{
		{"create", true, false, types.StringNull(), types.StringValue("a"), false},
		{"destroy", false, true, types.StringValue("a"), types.StringNull(), false},
		{"null prior (import) records in place", false, false, types.StringNull(), types.StringValue("a"), false},
		{"unchanged", false, false, types.StringValue("a"), types.StringValue("a"), false},
		{"changed from known", false, false, types.StringValue("a"), types.StringValue("b"), true},
		{"known to null", false, false, types.StringValue("a"), types.StringNull(), true},
		{"known to unknown", false, false, types.StringValue("a"), types.StringUnknown(), true},
	}
	for _, c := range cases {
		req := planmodifier.StringRequest{
			State:      tfsdk.State{Raw: raw(c.stateNull)},
			Plan:       tfsdk.Plan{Raw: raw(c.planNull)},
			StateValue: c.stateVal,
			PlanValue:  c.planVal,
		}
		var resp planmodifier.StringResponse
		ReplaceFromKnownString().PlanModifyString(context.Background(), req, &resp)
		if resp.RequiresReplace != c.want {
			t.Errorf("%s: RequiresReplace = %v, want %v", c.name, resp.RequiresReplace, c.want)
		}
	}
}

func TestReplaceFromKnownVariants(t *testing.T) {
	ctx := context.Background()
	st, pl := tfsdk.State{Raw: raw(false)}, tfsdk.Plan{Raw: raw(false)}

	var b planmodifier.BoolResponse
	ReplaceFromKnownBool().PlanModifyBool(ctx, planmodifier.BoolRequest{State: st, Plan: pl, StateValue: types.BoolNull(), PlanValue: types.BoolValue(true)}, &b)
	if b.RequiresReplace {
		t.Error("bool: null prior must not replace")
	}
	ReplaceFromKnownBool().PlanModifyBool(ctx, planmodifier.BoolRequest{State: st, Plan: pl, StateValue: types.BoolValue(false), PlanValue: types.BoolValue(true)}, &b)
	if !b.RequiresReplace {
		t.Error("bool: known change must replace")
	}

	var i planmodifier.Int64Response
	ReplaceFromKnownInt64().PlanModifyInt64(ctx, planmodifier.Int64Request{State: st, Plan: pl, StateValue: types.Int64Value(1), PlanValue: types.Int64Value(2)}, &i)
	if !i.RequiresReplace {
		t.Error("int64: known change must replace")
	}

	var m planmodifier.MapResponse
	ReplaceFromKnownMap().PlanModifyMap(ctx, planmodifier.MapRequest{State: st, Plan: pl, StateValue: types.MapNull(types.Int64Type),
		PlanValue: types.MapValueMust(types.Int64Type, map[string]attr.Value{"a": types.Int64Value(1)})}, &m)
	if m.RequiresReplace {
		t.Error("map: null prior must not replace")
	}

	for _, mod := range []any{ReplaceFromKnownString(), ReplaceFromKnownBool(), ReplaceFromKnownInt64(),
		ReplaceFromKnownList(), ReplaceFromKnownMap(), ReplaceFromKnownObject()} {
		if !IsReplaceFromKnown(mod) {
			t.Errorf("IsReplaceFromKnown(%T) = false", mod)
		}
	}
	if IsReplaceFromKnown(struct{}{}) {
		t.Error("IsReplaceFromKnown(other) = true")
	}
}

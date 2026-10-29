// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package normalize

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestKeepStateIfEquivalent(t *testing.T) {
	ctx := context.Background()
	m := KeepStateIfEquivalent(CIDRType{})
	cases := []struct {
		name                string
		config, state, plan types.String
		want                types.String
	}{
		{"equivalent keeps state", types.StringValue("10.0.0.1"), types.StringValue("10.0.0.1/32"), types.StringValue("10.0.0.1"), types.StringValue("10.0.0.1/32")},
		{"different keeps plan", types.StringValue("10.0.0.2"), types.StringValue("10.0.0.1/32"), types.StringValue("10.0.0.2"), types.StringValue("10.0.0.2")},
		{"create (null state)", types.StringValue("10.0.0.1"), types.StringNull(), types.StringValue("10.0.0.1"), types.StringValue("10.0.0.1")},
		{"unknown config", types.StringUnknown(), types.StringValue("10.0.0.1/32"), types.StringUnknown(), types.StringUnknown()},
		{"null config", types.StringNull(), types.StringValue("10.0.0.1/32"), types.StringValue("10.0.0.1/32"), types.StringValue("10.0.0.1/32")},
	}
	for _, c := range cases {
		resp := &planmodifier.StringResponse{PlanValue: c.plan}
		m.PlanModifyString(ctx, planmodifier.StringRequest{ConfigValue: c.config, StateValue: c.state, PlanValue: c.plan}, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("%s: %v", c.name, resp.Diagnostics)
		}
		if !resp.PlanValue.Equal(c.want) {
			t.Errorf("%s: plan = %v, want %v", c.name, resp.PlanValue, c.want)
		}
	}

	// JSONSubset is strict at plan time: removing key "b" from the config must
	// plan an update even though config ⊆ state (shtasks request, B-119/B-120).
	js := KeepStateIfEquivalent(JSONSubsetType{})
	resp := &planmodifier.StringResponse{PlanValue: types.StringValue(`{"a":1}`)}
	js.PlanModifyString(ctx, planmodifier.StringRequest{ConfigValue: types.StringValue(`{"a":1}`), StateValue: types.StringValue(`{"a":1,"b":2}`), PlanValue: types.StringValue(`{"a":1}`)}, resp)
	if resp.PlanValue.ValueString() != `{"a":1}` {
		t.Errorf("key removal was hidden: %v", resp.PlanValue)
	}
	resp = &planmodifier.StringResponse{PlanValue: types.StringValue("{\n \"b\": 2, \"a\": 1.0}")}
	js.PlanModifyString(ctx, planmodifier.StringRequest{ConfigValue: resp.PlanValue, StateValue: types.StringValue(`{"a":1,"b":2}`), PlanValue: resp.PlanValue}, resp)
	if resp.PlanValue.ValueString() != `{"a":1,"b":2}` {
		t.Errorf("respelled document did not keep state: %v", resp.PlanValue)
	}
	resp = &planmodifier.StringResponse{PlanValue: types.StringValue(`{"a":1,"b":2}`)}
	js.PlanModifyString(ctx, planmodifier.StringRequest{ConfigValue: types.StringValue(`{"a":1,"b":2}`), StateValue: types.StringValue(`{"a":1}`), PlanValue: types.StringValue(`{"a":1,"b":2}`)}, resp)
	if resp.PlanValue.ValueString() != `{"a":1,"b":2}` {
		t.Errorf("superset config was dropped: %v", resp.PlanValue)
	}
	if m.Description(ctx) == "" || m.MarkdownDescription(ctx) == "" {
		t.Error("empty description")
	}
}

func TestKeepStateIfEquivalentList(t *testing.T) {
	ctx := context.Background()
	list := func(elem attr.Type, vals ...attr.Value) types.List {
		l, d := types.ListValue(elem, vals)
		if d.HasError() {
			t.Fatal(d)
		}
		return l
	}
	s := func(v string) attr.Value { return types.StringValue(v) }
	c := func(v string) attr.Value { return NewCIDRValue(v) }

	unordered := KeepStateIfEquivalentList(UnorderedListOf(types.StringType))
	cfg := list(types.StringType, s("b"), s("a"))
	st := list(types.StringType, s("a"), s("b"))
	resp := &planmodifier.ListResponse{PlanValue: cfg}
	unordered.PlanModifyList(ctx, planmodifier.ListRequest{ConfigValue: cfg, StateValue: st, PlanValue: cfg}, resp)
	if !resp.PlanValue.Equal(st) {
		t.Errorf("unordered: plan = %v, want state", resp.PlanValue)
	}

	ordered := KeepStateIfEquivalentList(types.ListType{ElemType: CIDRType{}})
	cfg = list(CIDRType{}, c("10.0.0.1"), c("2001:db8::1"))
	st = list(CIDRType{}, c("10.0.0.1/32"), c("2001:0db8::1/128"))
	resp = &planmodifier.ListResponse{PlanValue: cfg}
	ordered.PlanModifyList(ctx, planmodifier.ListRequest{ConfigValue: cfg, StateValue: st, PlanValue: cfg}, resp)
	if !resp.PlanValue.Equal(st) {
		t.Errorf("ordered: plan = %v, want state", resp.PlanValue)
	}
	swapped := list(CIDRType{}, c("2001:db8::1"), c("10.0.0.1"))
	resp = &planmodifier.ListResponse{PlanValue: swapped}
	ordered.PlanModifyList(ctx, planmodifier.ListRequest{ConfigValue: swapped, StateValue: st, PlanValue: swapped}, resp)
	if !resp.PlanValue.Equal(swapped) {
		t.Errorf("ordered list ignored order: plan = %v", resp.PlanValue)
	}
}

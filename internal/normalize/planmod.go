// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package normalize

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

const keepStateDescription = "If the configured value is equivalent to the value in state (the server's spelling), " +
	"the plan keeps the state value, so a different spelling plans no change."

// KeepStateIfEquivalent returns a string plan modifier that plans the prior
// state value when the configured value is semantically equal to it under
// the attribute's custom type t (pass the same type as CustomType, for
// example normalize.CIDRType{}).
//
// Semantic equality only runs after Create, Update and Read, so on its own it
// cannot stop a plan diff when state holds the server spelling and the config
// another one: after `terraform import`, or after a config edit that only
// changes the spelling. For JSONSubset the plan-time comparison is strict
// EqualJSON (see stringsEquivalent), so key removals are always planned. Terraform accepts a planned value that differs from
// the config when it equals the prior state, and treats that as the provider
// saying the two are the same. Put this modifier before RequiresReplace (or
// planmod.ReplaceFromKnown): an equivalent spelling then plans no change at
// all instead of an update or a replacement (BUGS C1/C2: B-002, B-005,
// B-118..B-124).
func KeepStateIfEquivalent(t basetypes.StringTypable) planmodifier.String {
	return keepStateString{t: t}
}

type keepStateString struct{ t basetypes.StringTypable }

func (m keepStateString) Description(context.Context) string         { return keepStateDescription }
func (m keepStateString) MarkdownDescription(context.Context) string { return keepStateDescription }

func (m keepStateString) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.StateValue.IsNull() || req.StateValue.IsUnknown() ||
		req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() ||
		req.PlanValue.IsUnknown() || req.PlanValue.Equal(req.StateValue) {
		return
	}
	eq, diags := stringsEquivalent(ctx, m.t, req.ConfigValue, req.StateValue)
	resp.Diagnostics.Append(diags...)
	if eq {
		resp.PlanValue = req.StateValue
	}
}

// stringsEquivalent reports whether the configured value is semantically
// equal to the state value under type t.
//
// JSONSubset is the exception: at plan time it is compared strictly
// (EqualJSON). State often holds an earlier config (semantic equality kept the
// configured spelling), so "config ⊆ state" would also hold after the user
// removes a key, and the removal would never be planned. A plan modifier
// cannot tell server-added keys from previously configured ones, so the safe
// choice is: whole-document respelling plans nothing, anything else plans an
// update (after import that is one update that converges).
func stringsEquivalent(ctx context.Context, t basetypes.StringTypable, config, state basetypes.StringValue) (bool, diag.Diagnostics) {
	var diags diag.Diagnostics
	cfg, d := t.ValueFromString(ctx, config)
	diags.Append(d...)
	st, d := t.ValueFromString(ctx, state)
	diags.Append(d...)
	if diags.HasError() {
		return false, diags
	}
	if _, ok := st.(JSONSubset); ok {
		return EqualJSON(config.ValueString(), state.ValueString()), diags
	}
	sem, ok := st.(basetypes.StringValuableWithSemanticEquals)
	if !ok {
		return false, diags
	}
	eq, d := sem.StringSemanticEquals(ctx, cfg)
	diags.Append(d...)
	return eq, diags
}

// KeepStateIfEquivalentList is KeepStateIfEquivalent for list attributes.
// Pass the attribute's list type: normalize.UnorderedListOf(elem) compares in
// any order; a plain types.ListType{ElemType: normalize.CIDRType{}} compares
// element by element with the element type's semantic equality.
func KeepStateIfEquivalentList(t basetypes.ListTypable) planmodifier.List {
	return keepStateList{t: t}
}

type keepStateList struct{ t basetypes.ListTypable }

func (m keepStateList) Description(context.Context) string         { return keepStateDescription }
func (m keepStateList) MarkdownDescription(context.Context) string { return keepStateDescription }

func (m keepStateList) PlanModifyList(ctx context.Context, req planmodifier.ListRequest, resp *planmodifier.ListResponse) {
	if req.StateValue.IsNull() || req.StateValue.IsUnknown() ||
		req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() ||
		req.PlanValue.IsUnknown() || req.PlanValue.Equal(req.StateValue) {
		return
	}
	cfg, d := m.t.ValueFromList(ctx, req.ConfigValue)
	resp.Diagnostics.Append(d...)
	st, d := m.t.ValueFromList(ctx, req.StateValue)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	var eq bool
	if sem, ok := st.(basetypes.ListValuableWithSemanticEquals); ok {
		eq, d = sem.ListSemanticEquals(ctx, cfg)
	} else {
		eq, d = elementsEquivalent(ctx, req.ConfigValue.Elements(), req.StateValue.Elements())
	}
	resp.Diagnostics.Append(d...)
	if eq {
		resp.PlanValue = req.StateValue
	}
}

func elementsEquivalent(ctx context.Context, config, state []attr.Value) (bool, diag.Diagnostics) {
	var diags diag.Diagnostics
	if len(config) != len(state) {
		return false, diags
	}
	for i := range config {
		eq, d := elementEqual(ctx, config[i], state[i])
		diags.Append(d...)
		if !eq {
			return false, diags
		}
	}
	return true, diags
}

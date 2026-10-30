// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package planmod

import (
	"context"
	"reflect"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// ResetWhenUnsetDescription is the plan-modifier description shown in docs.
const ResetWhenUnsetDescription = "When this attribute is removed from the configuration after having been set, " +
	"it is reset to the server default. When it was never configured (for example after `terraform import`, or " +
	"on a resource created with it unset), the value on the server is kept."

// ResetWhenUnset* replaces a schema Default on an Optional+Computed attribute
// that has a server default.
//
// A plain Default makes the plan write the default whenever the configuration
// is silent. That is right when the user *removes* an attribute (B-113/B-114:
// "removing comment/ro/networks left the old value on the share"), and wrong
// when the configuration was never responsible for the value: importing an
// existing share with a minimal config rewrote its ro/enabled/networks on the
// first apply (hunt finding F4), and upgrading from a provider that stored a
// different value flipped it (F3).
//
// These modifiers tell the two cases apart with a flag in the resource's
// private state: when the attribute is set in config the flag is recorded;
// when it is later null and the flag is present the plan resets it to the
// default once and clears the flag; when it is null without the flag the
// server value (prior state) is kept. State written before this modifier
// existed carries no flags, so the first removal of an attribute on such a
// resource keeps the value — set it explicitly once to reset it.
type resetWhenUnset struct{ def attr.Value }

// ResetWhenUnsetString is ResetWhenUnset for strings.
func ResetWhenUnsetString(def string) planmodifier.String {
	return resetWhenUnset{types.StringValue(def)}
}

// ResetWhenUnsetBool is ResetWhenUnset for bools.
func ResetWhenUnsetBool(def bool) planmodifier.Bool { return resetWhenUnset{types.BoolValue(def)} }

// ResetWhenUnsetInt64 is ResetWhenUnset for int64s.
func ResetWhenUnsetInt64(def int64) planmodifier.Int64 { return resetWhenUnset{types.Int64Value(def)} }

// ResetWhenUnsetList is ResetWhenUnset for lists; def is usually an empty list.
func ResetWhenUnsetList(def types.List) planmodifier.List { return resetWhenUnset{def} }

// ResetWhenUnsetObject is ResetWhenUnset for nested objects.
func ResetWhenUnsetObject(def types.Object) planmodifier.Object { return resetWhenUnset{def} }

// IsResetWhenUnset reports whether m is one of the ResetWhenUnset* modifiers.
func IsResetWhenUnset(m any) bool { _, ok := m.(resetWhenUnset); return ok }

// ResetWhenUnsetDefault returns the server default a ResetWhenUnset* modifier
// resets to (for audits that compare schema defaults with the server).
func ResetWhenUnsetDefault(m any) (attr.Value, bool) {
	r, ok := m.(resetWhenUnset)
	if !ok {
		return nil, false
	}
	return r.def, true
}

func (resetWhenUnset) Description(context.Context) string         { return ResetWhenUnsetDescription }
func (resetWhenUnset) MarkdownDescription(context.Context) string { return ResetWhenUnsetDescription }

// privateData is the subset of privatestate.ProviderData the modifier uses.
type privateData interface {
	GetKey(context.Context, string) ([]byte, diag.Diagnostics)
	SetKey(context.Context, string, []byte) diag.Diagnostics
}

// isNil is true for a nil interface or a typed nil pointer (the framework's
// *ProviderData may be either).
func isNil(i any) bool {
	if i == nil {
		return true
	}
	v := reflect.ValueOf(i)
	return v.Kind() == reflect.Pointer && v.IsNil()
}

// decide returns the value to plan (nil = leave the framework's plan alone).
func (m resetWhenUnset) decide(ctx context.Context, p path.Path, in, out privateData, cfg, state attr.Value) (attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics
	if isNil(in) {
		in = nil
	}
	if isNil(out) {
		out = nil
	}
	key := "planmod.managed." + p.String()
	if !cfg.IsNull() {
		// Configured: remember that Terraform owns this value. Unknown config
		// (an expression) still counts as configured.
		if out != nil {
			diags.Append(out.SetKey(ctx, key, []byte("1"))...)
		}
		return nil, diags
	}
	var flagged bool
	if in != nil {
		raw, d := in.GetKey(ctx, key)
		diags.Append(d...)
		flagged = len(raw) > 0
	}
	if flagged {
		// Previously configured, now removed: reset to the server default once.
		if out != nil {
			diags.Append(out.SetKey(ctx, key, nil)...)
		}
		return m.def, diags
	}
	if !state.IsNull() && !state.IsUnknown() {
		// Never configured (import, upgrade, created unset): keep what the
		// server has.
		return state, diags
	}
	// Create with the attribute unset: leave it unknown so the API default
	// applies and Read fills it in.
	return nil, diags
}

func (m resetWhenUnset) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	v, d := m.decide(ctx, req.Path, req.Private, resp.Private, req.ConfigValue, req.StateValue)
	resp.Diagnostics.Append(d...)
	if v != nil {
		resp.PlanValue = v.(basetypes.StringValue)
	}
}

func (m resetWhenUnset) PlanModifyBool(ctx context.Context, req planmodifier.BoolRequest, resp *planmodifier.BoolResponse) {
	v, d := m.decide(ctx, req.Path, req.Private, resp.Private, req.ConfigValue, req.StateValue)
	resp.Diagnostics.Append(d...)
	if v != nil {
		resp.PlanValue = v.(basetypes.BoolValue)
	}
}

func (m resetWhenUnset) PlanModifyList(ctx context.Context, req planmodifier.ListRequest, resp *planmodifier.ListResponse) {
	v, d := m.decide(ctx, req.Path, req.Private, resp.Private, req.ConfigValue, req.StateValue)
	resp.Diagnostics.Append(d...)
	if v != nil {
		resp.PlanValue = v.(basetypes.ListValue)
	}
}

func (m resetWhenUnset) PlanModifyObject(ctx context.Context, req planmodifier.ObjectRequest, resp *planmodifier.ObjectResponse) {
	v, d := m.decide(ctx, req.Path, req.Private, resp.Private, req.ConfigValue, req.StateValue)
	resp.Diagnostics.Append(d...)
	if v != nil {
		resp.PlanValue = v.(basetypes.ObjectValue)
	}
}

func (m resetWhenUnset) PlanModifyInt64(ctx context.Context, req planmodifier.Int64Request, resp *planmodifier.Int64Response) {
	v, d := m.decide(ctx, req.Path, req.Private, resp.Private, req.ConfigValue, req.StateValue)
	resp.Diagnostics.Append(d...)
	if v != nil {
		resp.PlanValue = v.(basetypes.Int64Value)
	}
}

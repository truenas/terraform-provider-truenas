// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

// Package planmod holds plan modifiers shared across resources.
//
// ReplaceFromKnown* is the import-safe RequiresReplace. An attribute the API
// never returns (a create-time directive such as snapshot "recursive" or
// certificate "create_type") reads back as null after `terraform import`. With
// the framework's unconditional RequiresReplace(), the first plan after import
// compares the configured value against that null and destroys and recreates
// an object the user only adopted. These modifiers require replacement only
// when the prior state value is known and differs from the plan: a null prior
// value records the configured value in place (an update that sends nothing for
// it), and every later change still replaces. Update must therefore never send
// these attributes; it only stores the planned value. (#29, BUGS C1)
package planmod

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// FromKnownDescription is the plan-modifier description shown in docs.
const FromKnownDescription = "If the value of this attribute changes from a known value, Terraform will destroy and " +
	"recreate the resource. A null prior value (after import) records the configured value in place."

type replaceFromKnown struct{}

// ReplaceFromKnownString is the import-safe RequiresReplace for strings.
func ReplaceFromKnownString() planmodifier.String { return replaceFromKnown{} }

// ReplaceFromKnownBool is the import-safe RequiresReplace for bools.
func ReplaceFromKnownBool() planmodifier.Bool { return replaceFromKnown{} }

// ReplaceFromKnownInt64 is the import-safe RequiresReplace for int64s.
func ReplaceFromKnownInt64() planmodifier.Int64 { return replaceFromKnown{} }

// ReplaceFromKnownList is the import-safe RequiresReplace for lists.
func ReplaceFromKnownList() planmodifier.List { return replaceFromKnown{} }

// ReplaceFromKnownMap is the import-safe RequiresReplace for maps.
func ReplaceFromKnownMap() planmodifier.Map { return replaceFromKnown{} }

// ReplaceFromKnownObject is the import-safe RequiresReplace for objects.
func ReplaceFromKnownObject() planmodifier.Object { return replaceFromKnown{} }

// IsReplaceFromKnown reports whether a plan modifier is one of the
// ReplaceFromKnown* modifiers. Used by the provider-wide import ratchet.
func IsReplaceFromKnown(m any) bool {
	switch m.(type) {
	case replaceFromKnown, replaceFromKnownFold:
		return true
	}
	return false
}

// FromKnownFoldDescription is the plan-modifier description for the
// case-insensitive variant.
const FromKnownFoldDescription = "If the value of this attribute changes from a known value, ignoring letter case, " +
	"Terraform will destroy and recreate the resource. A null prior value (after import) or a change in letter case " +
	"only records the configured value in place."

type replaceFromKnownFold struct{}

// ReplaceFromKnownStringFold is ReplaceFromKnownString for a case-insensitive
// create-only string: a change in letter case only (the server may read back
// "nfsv4" for a configured "NFSV4") records the configured value in place.
func ReplaceFromKnownStringFold() planmodifier.String { return replaceFromKnownFold{} }

func (replaceFromKnownFold) Description(context.Context) string { return FromKnownFoldDescription }
func (replaceFromKnownFold) MarkdownDescription(context.Context) string {
	return FromKnownFoldDescription
}

func (replaceFromKnownFold) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() || req.StateValue.IsNull() {
		return
	}
	if req.PlanValue.IsNull() || req.PlanValue.IsUnknown() {
		resp.RequiresReplace = true
		return
	}
	if !strings.EqualFold(req.PlanValue.ValueString(), req.StateValue.ValueString()) {
		resp.RequiresReplace = true
	}
}

func (replaceFromKnown) Description(context.Context) string         { return FromKnownDescription }
func (replaceFromKnown) MarkdownDescription(context.Context) string { return FromKnownDescription }

// requiresReplace mirrors the framework's RequiresReplace (no replace on
// create, destroy, or an unchanged value) except that a null prior value never
// forces replacement.
func requiresReplace(stateRaw, planRaw tftypes.Value, stateValue, planValue attr.Value) bool {
	if stateRaw.IsNull() || planRaw.IsNull() {
		return false
	}
	if stateValue.IsNull() {
		return false
	}
	return !planValue.Equal(stateValue)
}

func (replaceFromKnown) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if requiresReplace(req.State.Raw, req.Plan.Raw, req.StateValue, req.PlanValue) {
		resp.RequiresReplace = true
	}
}

func (replaceFromKnown) PlanModifyBool(_ context.Context, req planmodifier.BoolRequest, resp *planmodifier.BoolResponse) {
	if requiresReplace(req.State.Raw, req.Plan.Raw, req.StateValue, req.PlanValue) {
		resp.RequiresReplace = true
	}
}

func (replaceFromKnown) PlanModifyInt64(_ context.Context, req planmodifier.Int64Request, resp *planmodifier.Int64Response) {
	if requiresReplace(req.State.Raw, req.Plan.Raw, req.StateValue, req.PlanValue) {
		resp.RequiresReplace = true
	}
}

func (replaceFromKnown) PlanModifyList(_ context.Context, req planmodifier.ListRequest, resp *planmodifier.ListResponse) {
	if requiresReplace(req.State.Raw, req.Plan.Raw, req.StateValue, req.PlanValue) {
		resp.RequiresReplace = true
	}
}

func (replaceFromKnown) PlanModifyMap(_ context.Context, req planmodifier.MapRequest, resp *planmodifier.MapResponse) {
	if requiresReplace(req.State.Raw, req.Plan.Raw, req.StateValue, req.PlanValue) {
		resp.RequiresReplace = true
	}
}

func (replaceFromKnown) PlanModifyObject(_ context.Context, req planmodifier.ObjectRequest, resp *planmodifier.ObjectResponse) {
	if requiresReplace(req.State.Raw, req.Plan.Raw, req.StateValue, req.PlanValue) {
		resp.RequiresReplace = true
	}
}

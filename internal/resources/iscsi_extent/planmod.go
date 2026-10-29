// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_extent

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// disk and path are both Computed, but their server values are derived from
// type and disk: TrueNAS stores a DISK extent's zvol in path (so path ==
// disk) and reports disk as null ("") for a FILE extent. UseStateForUnknown
// held the old value across a disk or type change and made the apply fail
// with "inconsistent result" after the server had already changed. These
// modifiers plan the derived value instead.

// diskFollowsType plans disk = "" for a FILE extent when the config leaves
// disk unset.
type diskFollowsType struct{}

func (diskFollowsType) Description(context.Context) string {
	return "Plans \"\" for a FILE extent; otherwise keeps the prior value while type is unchanged."
}

func (m diskFollowsType) MarkdownDescription(ctx context.Context) string { return m.Description(ctx) }

func (diskFollowsType) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if !req.ConfigValue.IsNull() {
		return
	}
	var typ types.String
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("type"), &typ)...)
	if typ.IsUnknown() {
		resp.PlanValue = types.StringUnknown()
		return
	}
	if typ.ValueString() == "FILE" {
		resp.PlanValue = types.StringValue("")
		return
	}
	resp.PlanValue = keepIfSameType(ctx, req.State, req.StateValue, typ, &resp.Diagnostics)
}

// pathFollowsDisk plans path = disk for a DISK extent when the config leaves
// path unset.
type pathFollowsDisk struct{}

func (pathFollowsDisk) Description(context.Context) string {
	return "Plans the disk value for a DISK extent; otherwise keeps the prior value while type is unchanged."
}

func (m pathFollowsDisk) MarkdownDescription(ctx context.Context) string { return m.Description(ctx) }

func (pathFollowsDisk) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if !req.ConfigValue.IsNull() {
		return
	}
	var typ types.String
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("type"), &typ)...)
	if typ.IsUnknown() {
		resp.PlanValue = types.StringUnknown()
		return
	}
	if typ.ValueString() == "DISK" {
		var disk types.String
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("disk"), &disk)...)
		if disk.IsNull() || disk.IsUnknown() {
			resp.PlanValue = types.StringUnknown()
			return
		}
		resp.PlanValue = disk
		return
	}
	resp.PlanValue = keepIfSameType(ctx, req.State, req.StateValue, typ, &resp.Diagnostics)
}

// keepIfSameType returns the prior state value when the resource exists and
// its type is unchanged, and unknown otherwise.
func keepIfSameType(ctx context.Context, state tfsdk.State, stateValue, planType types.String, diags *diag.Diagnostics) types.String {
	if state.Raw.IsNull() || stateValue.IsNull() {
		return types.StringUnknown()
	}
	var stateType types.String
	diags.Append(state.GetAttribute(ctx, path.Root("type"), &stateType)...)
	if !stateType.Equal(planType) {
		return types.StringUnknown()
	}
	return stateValue
}

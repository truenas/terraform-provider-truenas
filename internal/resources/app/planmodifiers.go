// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package app

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// The read-only attributes state, human_version and upgrade_available used to
// show "(known after apply)" on every update (B-137). They are kept from state
// unless the update changes what they are derived from: human_version and
// upgrade_available follow the installed version (an upgrade), state follows
// running. An app in a transitional or failed state (DEPLOYING, CRASHED, ...)
// may settle differently after any update, so its state stays unknown.

// upgradePlanned reports whether the update upgrades the app (true when that
// cannot be known yet). It reads the configuration, not the plan: the plan
// value of an unset version is still unknown while these modifiers run (its
// own UseStateForUnknown has not been applied to the plan they see), and an
// unset version keeps the state value.
func upgradePlanned(ctx context.Context, cfg tfsdk.Config, prior tfsdk.State) (bool, bool) {
	var cv, sv types.String
	if cfg.GetAttribute(ctx, path.Root("version"), &cv).HasError() ||
		prior.GetAttribute(ctx, path.Root("version"), &sv).HasError() {
		return false, false
	}
	if cv.IsUnknown() {
		return true, true
	}
	return needsUpgrade(&AppModel{Version: cv}, &AppModel{Version: sv}), true
}

// runningChangePlanned reports whether the update starts or stops the app
// (true when that cannot be known yet), from the configuration like
// upgradePlanned.
func runningChangePlanned(ctx context.Context, cfg tfsdk.Config, prior tfsdk.State) (bool, bool) {
	var pr, sr types.Bool
	if cfg.GetAttribute(ctx, path.Root("running"), &pr).HasError() ||
		prior.GetAttribute(ctx, path.Root("running"), &sr).HasError() {
		return false, false
	}
	if pr.IsUnknown() {
		return true, true
	}
	return !pr.IsNull() && !pr.Equal(sr), true
}

// keepVersionDerivedString keeps human_version from state unless the app is
// upgraded.
type keepVersionDerivedString struct{}

func (keepVersionDerivedString) Description(context.Context) string {
	return "Keeps the prior value unless the app version changes."
}

func (m keepVersionDerivedString) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (keepVersionDerivedString) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.StateValue.IsNull() || req.Plan.Raw.IsNull() || !req.PlanValue.IsUnknown() {
		return
	}
	if up, ok := upgradePlanned(ctx, req.Config, req.State); ok && !up {
		resp.PlanValue = req.StateValue
	}
}

// keepVersionDerivedBool keeps upgrade_available from state unless the app is
// upgraded.
type keepVersionDerivedBool struct{}

func (keepVersionDerivedBool) Description(context.Context) string {
	return "Keeps the prior value unless the app version changes."
}

func (m keepVersionDerivedBool) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (keepVersionDerivedBool) PlanModifyBool(ctx context.Context, req planmodifier.BoolRequest, resp *planmodifier.BoolResponse) {
	if req.StateValue.IsNull() || req.Plan.Raw.IsNull() || !req.PlanValue.IsUnknown() {
		return
	}
	if up, ok := upgradePlanned(ctx, req.Config, req.State); ok && !up {
		resp.PlanValue = req.StateValue
	}
}

// keepSettledState keeps state from prior state when the app is RUNNING or
// STOPPED and the update neither starts/stops nor upgrades it.
type keepSettledState struct{}

func (keepSettledState) Description(context.Context) string {
	return "Keeps a settled (RUNNING/STOPPED) prior value unless the app is started, stopped or upgraded."
}

func (m keepSettledState) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (keepSettledState) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.StateValue.IsNull() || req.Plan.Raw.IsNull() || !req.PlanValue.IsUnknown() {
		return
	}
	if s := req.StateValue.ValueString(); s != "RUNNING" && s != "STOPPED" {
		return
	}
	run, ok1 := runningChangePlanned(ctx, req.Config, req.State)
	up, ok2 := upgradePlanned(ctx, req.Config, req.State)
	if ok1 && ok2 && !run && !up {
		resp.PlanValue = req.StateValue
	}
}

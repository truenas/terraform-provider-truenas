// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// statusFollowsRunning keeps the prior status in the plan unless the update
// starts or stops the VM. status is derived from the run state only, so an
// update that leaves running alone does not change it; without this every
// update showed status as "(known after apply)". running is Optional+Computed
// and keeps its state value when unset, so a null config value means
// unchanged. (BUGS B-137)
type statusFollowsRunning struct{}

func (statusFollowsRunning) Description(_ context.Context) string {
	return "Keeps the prior status unless running changes."
}

func (m statusFollowsRunning) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (statusFollowsRunning) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.StateValue.IsNull() || req.StateValue.IsUnknown() || !req.PlanValue.IsUnknown() {
		return
	}
	var cfgRunning, stateRunning types.Bool
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("running"), &cfgRunning)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("running"), &stateRunning)...)
	if resp.Diagnostics.HasError() || cfgRunning.IsUnknown() {
		return
	}
	if cfgRunning.IsNull() || cfgRunning.Equal(stateRunning) {
		resp.PlanValue = req.StateValue
	}
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_portal

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// portFromState plans listen[].port as the prior value at the same index,
// or unknown for a new listen entry. port is Computed-only; for an entry
// added to the list the framework planned null, and the server's port then
// failed the apply with "inconsistent result" (UseStateForUnknown leaves a
// null alone).
type portFromState struct{}

func (portFromState) Description(context.Context) string {
	return "Keeps the prior port for an existing listen entry; unknown for a new one."
}

func (m portFromState) MarkdownDescription(ctx context.Context) string { return m.Description(ctx) }

func (portFromState) PlanModifyInt64(_ context.Context, req planmodifier.Int64Request, resp *planmodifier.Int64Response) {
	if !req.StateValue.IsNull() && !req.StateValue.IsUnknown() {
		resp.PlanValue = req.StateValue
		return
	}
	resp.PlanValue = types.Int64Unknown()
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

// keepLocalSpecialSmallBlockSize fails the plan when special_small_block_size
// goes from a size set on the dataset itself to INHERIT. Reverting it is a
// change to where the dataset's future small blocks are written, made by a
// one-word edit, so it is refused here rather than applied; set the parent's
// value explicitly instead, or revert it outside Terraform and refresh.
// Creating a dataset with INHERIT, and keeping INHERIT on a dataset that
// already inherits, are unaffected.
type keepLocalSpecialSmallBlockSize struct{}

func (keepLocalSpecialSmallBlockSize) Description(context.Context) string {
	return "refuses to change special_small_block_size from a size to INHERIT"
}

func (m keepLocalSpecialSmallBlockSize) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (keepLocalSpecialSmallBlockSize) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.StateValue.IsNull() || req.StateValue.IsUnknown() ||
		req.PlanValue.IsNull() || req.PlanValue.IsUnknown() {
		return
	}
	if isInherit(req.StateValue.ValueString()) || !isInherit(req.PlanValue.ValueString()) {
		return
	}
	resp.Diagnostics.AddAttributeError(req.Path,
		"special_small_block_size cannot be changed from a size to INHERIT",
		"This dataset has special_small_block_size set to "+req.StateValue.ValueString()+
			" on the dataset itself. Changing it to INHERIT is refused, because it changes "+
			"where the dataset's future small blocks are written. Set the size you want "+
			"explicitly instead.")
}

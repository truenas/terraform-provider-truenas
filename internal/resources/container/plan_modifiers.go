// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package container

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// idmapRequiresReplace replaces the container only when idmap really changes:
// container.update does not accept idmap, but the stored text is opaque JSON,
// so a different spelling of the same configuration (import reads the server's
// form; `slice = null` vs the slice the server picked) must not replace. A
// null prior value (state written before idmap was read on import) records in
// place, like planmod.ReplaceFromKnown*. (BUGS B-005)
func idmapRequiresReplace() planmodifier.String {
	return stringplanmodifier.RequiresReplaceIf(
		func(_ context.Context, req planmodifier.StringRequest, resp *stringplanmodifier.RequiresReplaceIfFuncResponse) {
			if req.StateValue.IsNull() || req.PlanValue.IsNull() {
				return
			}
			if req.PlanValue.IsUnknown() {
				resp.RequiresReplace = true
				return
			}
			resp.RequiresReplace = !idmapEquivalent(req.StateValue.ValueString(), req.PlanValue.ValueString())
		},
		"Replaced only when the idmap configuration changes (a different JSON spelling of the same value does not replace).",
		"Replaced only when the idmap configuration changes (a different JSON spelling of the same value does not replace).",
	)
}

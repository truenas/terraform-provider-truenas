// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package container

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
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

// keepUnlessChanged keeps the prior value of a server-derived attribute in the
// plan unless the configured value of the attribute it is derived from
// changes: status follows running, dataset follows name (container.update
// renames the dataset with the container). An empty input means the value is
// never changed by an update (default_network: it follows the container's NIC
// devices, which container.update does not touch; a device change shows up on
// the next refresh). The prior value is kept even when it is null. Without
// this, every update showed these as "(known after apply)". A null config value for an Optional+Computed
// input (running) keeps its state value, so it counts as unchanged.
// (BUGS B-137)
type keepUnlessChanged struct {
	input string
}

func (m keepUnlessChanged) Description(_ context.Context) string {
	if m.input == "" {
		return "Keeps the prior value on update."
	}
	return "Keeps the prior value unless " + m.input + " changes."
}

func (m keepUnlessChanged) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m keepUnlessChanged) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.State.Raw.IsNull() || req.StateValue.IsUnknown() || !req.PlanValue.IsUnknown() {
		return
	}
	var unchanged bool
	switch m.input {
	case "":
		unchanged = true
	case "running":
		var cfg, state types.Bool
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(m.input), &cfg)...)
		resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root(m.input), &state)...)
		unchanged = !cfg.IsUnknown() && (cfg.IsNull() || cfg.Equal(state))
	default:
		var cfg, state types.String
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(m.input), &cfg)...)
		resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root(m.input), &state)...)
		unchanged = !cfg.IsUnknown() && cfg.Equal(state)
	}
	if !resp.Diagnostics.HasError() && unchanged {
		resp.PlanValue = req.StateValue
	}
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package static_route

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// StaticRouteModel is the Terraform state model for truenas_static_route.
type StaticRouteModel struct {
	ID          types.Int64  `tfsdk:"id"`
	Destination types.String `tfsdk:"destination"` // CIDR, e.g. "10.20.0.0/16"
	Gateway     types.String `tfsdk:"gateway"`
	Description types.String `tfsdk:"description"`
}

// staticRouteAPI is the JSON wire format for a TrueNAS static route object.
type staticRouteAPI struct {
	ID          int64  `json:"id"`
	Destination string `json:"destination"`
	Gateway     string `json:"gateway"`
	Description string `json:"description"`
}

// responseToModel maps an API response onto a Terraform model.
func responseToModel(_ context.Context, api *staticRouteAPI, m *StaticRouteModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.Int64Value(api.ID)
	m.Destination = types.StringValue(api.Destination)
	m.Gateway = types.StringValue(api.Gateway)
	m.Description = types.StringValue(api.Description)
	return diags
}

// apiPayload builds the map[string]any payload for staticroute.create /
// staticroute.update.
func (m *StaticRouteModel) apiPayload(_ context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	return map[string]any{
		"destination": m.Destination.ValueString(),
		"gateway":     m.Gateway.ValueString(),
		"description": m.Description.ValueString(),
	}, diags
}

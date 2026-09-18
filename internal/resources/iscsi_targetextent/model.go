// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_targetextent

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TargetExtentModel is the Terraform state model for truenas_iscsi_targetextent.
type TargetExtentModel struct {
	ID     types.Int64 `tfsdk:"id"`
	Target types.Int64 `tfsdk:"target"` // target ID, RequiresReplace
	Extent types.Int64 `tfsdk:"extent"` // extent ID, RequiresReplace
	LunID  types.Int64 `tfsdk:"lunid"`  // Optional+Computed; null -> auto-assign
}

// TargetExtentDataSourceModel is the Terraform state model for the
// truenas_iscsi_targetextent data source.
type TargetExtentDataSourceModel struct {
	ID     types.Int64 `tfsdk:"id"`
	Target types.Int64 `tfsdk:"target"`
	Extent types.Int64 `tfsdk:"extent"`
	LunID  types.Int64 `tfsdk:"lunid"`
}

// targetExtentAPI is the JSON wire format for a TrueNAS iSCSI target/extent
// association object.
type targetExtentAPI struct {
	ID     int64 `json:"id"`
	Target int64 `json:"target"`
	Extent int64 `json:"extent"`
	LunID  int64 `json:"lunid"`
}

// responseToModel maps an API response onto a Terraform resource model.
func responseToModel(_ context.Context, api *targetExtentAPI, m *TargetExtentModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.Int64Value(api.ID)
	m.Target = types.Int64Value(api.Target)
	m.Extent = types.Int64Value(api.Extent)
	m.LunID = types.Int64Value(api.LunID)
	return diags
}

// responseToDataSourceModel maps an API response onto a Terraform data
// source model.
func responseToDataSourceModel(api *targetExtentAPI, m *TargetExtentDataSourceModel) {
	m.ID = types.Int64Value(api.ID)
	m.Target = types.Int64Value(api.Target)
	m.Extent = types.Int64Value(api.Extent)
	m.LunID = types.Int64Value(api.LunID)
}

// apiPayload builds the map[string]any payload for iscsi.targetextent.create
// / iscsi.targetextent.update. target and extent are always included; lunid
// is only included when known (non-null, non-unknown) so the API can
// auto-assign a LUN ID when it is omitted.
func (m *TargetExtentModel) apiPayload(_ context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	p := map[string]any{
		"target": m.Target.ValueInt64(),
		"extent": m.Extent.ValueInt64(),
	}
	if !m.LunID.IsNull() && !m.LunID.IsUnknown() {
		p["lunid"] = m.LunID.ValueInt64()
	}
	return p, diags
}

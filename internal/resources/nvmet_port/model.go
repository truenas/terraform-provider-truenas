// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_port

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// NVMetPortModel is the Terraform state model for truenas_nvmet_port.
type NVMetPortModel struct {
	ID             types.Int64  `tfsdk:"id"`
	AddrTrtype     types.String `tfsdk:"addr_trtype"`      // Required + RequiresReplace; TCP or RDMA
	AddrTraddr     types.String `tfsdk:"addr_traddr"`      // Required
	AddrTrsvcid    types.Int64  `tfsdk:"addr_trsvcid"`     // Optional+Computed+USFU (default 4420)
	Enabled        types.Bool   `tfsdk:"enabled"`          // Optional+Computed+USFU
	InlineDataSize types.Int64  `tfsdk:"inline_data_size"` // Optional+Computed+USFU; nullable
	MaxQueueSize   types.Int64  `tfsdk:"max_queue_size"`   // Optional+Computed+USFU; nullable
	PIEnable       types.Bool   `tfsdk:"pi_enable"`        // Optional+Computed+USFU; nullable
	// Computed
	Index      types.Int64  `tfsdk:"index"`       // Computed+USFU
	AddrAdrfam types.String `tfsdk:"addr_adrfam"` // Computed+USFU
}

// NVMetPortDataSourceModel is the read-only model for the truenas_nvmet_port
// datasource. It mirrors NVMetPortModel exactly; the datasource keeps its own
// model + schema pair so schema/model drift is caught independently by
// TestNVMetPortDataSourceModel_MatchesSchema.
type NVMetPortDataSourceModel struct {
	ID             types.Int64  `tfsdk:"id"`
	AddrTrtype     types.String `tfsdk:"addr_trtype"`
	AddrTraddr     types.String `tfsdk:"addr_traddr"`
	AddrTrsvcid    types.Int64  `tfsdk:"addr_trsvcid"`
	Enabled        types.Bool   `tfsdk:"enabled"`
	InlineDataSize types.Int64  `tfsdk:"inline_data_size"`
	MaxQueueSize   types.Int64  `tfsdk:"max_queue_size"`
	PIEnable       types.Bool   `tfsdk:"pi_enable"`
	Index          types.Int64  `tfsdk:"index"`
	AddrAdrfam     types.String `tfsdk:"addr_adrfam"`
}

// nvmetPortAPI is the JSON wire format for a TrueNAS NVMe-oF port object
// (nvmet.port.*).
type nvmetPortAPI struct {
	ID             int64  `json:"id"`
	AddrTrtype     string `json:"addr_trtype"`
	AddrTraddr     string `json:"addr_traddr"`
	AddrTrsvcid    int64  `json:"addr_trsvcid"`
	Enabled        bool   `json:"enabled"`
	InlineDataSize *int64 `json:"inline_data_size"`
	MaxQueueSize   *int64 `json:"max_queue_size"`
	PIEnable       *bool  `json:"pi_enable"`
	Index          int64  `json:"index"`
	AddrAdrfam     string `json:"addr_adrfam"`
}

// responseToModel maps an API response onto a Terraform model. Nil pointer
// fields (inline_data_size, max_queue_size, pi_enable) map to null rather
// than a zero value, so that an update payload built from this model omits
// them (see guardedFields) instead of sending an explicit 0/false that
// overwrites a server-side null.
func responseToModel(_ context.Context, api *nvmetPortAPI, m *NVMetPortModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.AddrTrtype = types.StringValue(api.AddrTrtype)
	m.AddrTraddr = types.StringValue(api.AddrTraddr)
	m.AddrTrsvcid = types.Int64Value(api.AddrTrsvcid)
	m.Enabled = types.BoolValue(api.Enabled)
	m.Index = types.Int64Value(api.Index)
	m.AddrAdrfam = types.StringValue(api.AddrAdrfam)

	if api.InlineDataSize != nil {
		m.InlineDataSize = types.Int64Value(*api.InlineDataSize)
	} else {
		m.InlineDataSize = types.Int64Null()
	}

	if api.MaxQueueSize != nil {
		m.MaxQueueSize = types.Int64Value(*api.MaxQueueSize)
	} else {
		m.MaxQueueSize = types.Int64Null()
	}

	if api.PIEnable != nil {
		m.PIEnable = types.BoolValue(*api.PIEnable)
	} else {
		m.PIEnable = types.BoolNull()
	}

	return diags
}

// responseToDataSourceModel maps an API response onto an
// NVMetPortDataSourceModel using the same nil-pointer-to-null rules as
// responseToModel.
func responseToDataSourceModel(_ context.Context, api *nvmetPortAPI, m *NVMetPortDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.AddrTrtype = types.StringValue(api.AddrTrtype)
	m.AddrTraddr = types.StringValue(api.AddrTraddr)
	m.AddrTrsvcid = types.Int64Value(api.AddrTrsvcid)
	m.Enabled = types.BoolValue(api.Enabled)
	m.Index = types.Int64Value(api.Index)
	m.AddrAdrfam = types.StringValue(api.AddrAdrfam)

	if api.InlineDataSize != nil {
		m.InlineDataSize = types.Int64Value(*api.InlineDataSize)
	} else {
		m.InlineDataSize = types.Int64Null()
	}

	if api.MaxQueueSize != nil {
		m.MaxQueueSize = types.Int64Value(*api.MaxQueueSize)
	} else {
		m.MaxQueueSize = types.Int64Null()
	}

	if api.PIEnable != nil {
		m.PIEnable = types.BoolValue(*api.PIEnable)
	} else {
		m.PIEnable = types.BoolNull()
	}

	return diags
}

// guardedFields builds the set of optional payload fields shared by create
// and update: addr_trsvcid, enabled, inline_data_size, max_queue_size,
// pi_enable. Each is included only when its model value is known (not null,
// not unknown).
func (m *NVMetPortModel) guardedFields() map[string]any {
	p := map[string]any{}

	if !m.AddrTrsvcid.IsNull() && !m.AddrTrsvcid.IsUnknown() {
		p["addr_trsvcid"] = m.AddrTrsvcid.ValueInt64()
	}
	if !m.Enabled.IsNull() && !m.Enabled.IsUnknown() {
		p["enabled"] = m.Enabled.ValueBool()
	}
	if !m.InlineDataSize.IsNull() && !m.InlineDataSize.IsUnknown() {
		p["inline_data_size"] = m.InlineDataSize.ValueInt64()
	}
	if !m.MaxQueueSize.IsNull() && !m.MaxQueueSize.IsUnknown() {
		p["max_queue_size"] = m.MaxQueueSize.ValueInt64()
	}
	if !m.PIEnable.IsNull() && !m.PIEnable.IsUnknown() {
		p["pi_enable"] = m.PIEnable.ValueBool()
	}

	return p
}

// createPayload builds the map[string]any payload for nvmet.port.create.
// "addr_trtype" and "addr_traddr" are always included; the remaining fields
// are guarded (see guardedFields).
func (m *NVMetPortModel) createPayload(_ context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	payload := m.guardedFields()
	payload["addr_trtype"] = m.AddrTrtype.ValueString()
	payload["addr_traddr"] = m.AddrTraddr.ValueString()

	return payload, diags
}

// updatePayload builds the map[string]any payload for nvmet.port.update. It
// never includes "addr_trtype": trtype changes are handled via
// RequiresReplace, and the API may reject "addr_trtype" in an update call.
// "addr_traddr" is always included since it is Required but not
// RequiresReplace.
func (m *NVMetPortModel) updatePayload(_ context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	payload := m.guardedFields()
	payload["addr_traddr"] = m.AddrTraddr.ValueString()

	return payload, diags
}

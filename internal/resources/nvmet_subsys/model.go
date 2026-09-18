// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_subsys

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// NVMetSubsysModel is the Terraform state model for truenas_nvmet_subsys.
type NVMetSubsysModel struct {
	ID           types.Int64  `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`           // Required + RequiresReplace
	SubNQN       types.String `tfsdk:"subnqn"`         // Optional+Computed+USFU; server derives when unset
	AllowAnyHost types.Bool   `tfsdk:"allow_any_host"` // Optional+Computed+USFU
	ANA          types.Bool   `tfsdk:"ana"`            // Optional+Computed+USFU; nullable in API
	IEEEOUI      types.String `tfsdk:"ieee_oui"`       // Optional+Computed+USFU; nullable
	PIEnable     types.Bool   `tfsdk:"pi_enable"`      // Optional+Computed+USFU; nullable
	QIDMax       types.Int64  `tfsdk:"qid_max"`        // Optional+Computed+USFU; nullable
	Serial       types.String `tfsdk:"serial"`         // Computed + USFU (server-assigned)
}

// NVMetSubsysDataSourceModel is the read-only model for the
// truenas_nvmet_subsys datasource. It mirrors NVMetSubsysModel exactly: there
// is no write-only field to exclude here, but the datasource keeps its own
// model + schema pair so schema/model drift is caught independently by
// TestNVMetSubsysDataSourceModel_MatchesSchema.
type NVMetSubsysDataSourceModel struct {
	ID           types.Int64  `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	SubNQN       types.String `tfsdk:"subnqn"`
	AllowAnyHost types.Bool   `tfsdk:"allow_any_host"`
	ANA          types.Bool   `tfsdk:"ana"`
	IEEEOUI      types.String `tfsdk:"ieee_oui"`
	PIEnable     types.Bool   `tfsdk:"pi_enable"`
	QIDMax       types.Int64  `tfsdk:"qid_max"`
	Serial       types.String `tfsdk:"serial"`
}

// nvmetSubsysAPI is the JSON wire format for a TrueNAS NVMe-oF subsystem
// object (nvmet.subsys.*).
type nvmetSubsysAPI struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	SubNQN       string  `json:"subnqn"`
	AllowAnyHost bool    `json:"allow_any_host"`
	ANA          *bool   `json:"ana"`
	IEEEOUI      *string `json:"ieee_oui"`
	PIEnable     *bool   `json:"pi_enable"`
	QIDMax       *int64  `json:"qid_max"`
	Serial       string  `json:"serial"`
}

// responseToModel maps an API response onto a Terraform model. Nil pointer
// fields (ana, pi_enable, qid_max) map to null; ieee_oui maps to empty string.
func responseToModel(_ context.Context, api *nvmetSubsysAPI, m *NVMetSubsysModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.Name = types.StringValue(api.Name)
	m.SubNQN = types.StringValue(api.SubNQN)
	m.AllowAnyHost = types.BoolValue(api.AllowAnyHost)
	m.Serial = types.StringValue(api.Serial)

	if api.ANA != nil {
		m.ANA = types.BoolValue(*api.ANA)
	} else {
		m.ANA = types.BoolNull()
	}

	if api.IEEEOUI != nil {
		m.IEEEOUI = types.StringValue(*api.IEEEOUI)
	} else {
		m.IEEEOUI = types.StringValue("")
	}

	if api.PIEnable != nil {
		m.PIEnable = types.BoolValue(*api.PIEnable)
	} else {
		m.PIEnable = types.BoolNull()
	}

	if api.QIDMax != nil {
		m.QIDMax = types.Int64Value(*api.QIDMax)
	} else {
		m.QIDMax = types.Int64Null()
	}

	return diags
}

// responseToDataSourceModel maps an API response onto an
// NVMetSubsysDataSourceModel using the same nil-pointer-to-null rules
// as responseToModel.
func responseToDataSourceModel(_ context.Context, api *nvmetSubsysAPI, m *NVMetSubsysDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.Name = types.StringValue(api.Name)
	m.SubNQN = types.StringValue(api.SubNQN)
	m.AllowAnyHost = types.BoolValue(api.AllowAnyHost)
	m.Serial = types.StringValue(api.Serial)

	if api.ANA != nil {
		m.ANA = types.BoolValue(*api.ANA)
	} else {
		m.ANA = types.BoolNull()
	}

	if api.IEEEOUI != nil {
		m.IEEEOUI = types.StringValue(*api.IEEEOUI)
	} else {
		m.IEEEOUI = types.StringValue("")
	}

	if api.PIEnable != nil {
		m.PIEnable = types.BoolValue(*api.PIEnable)
	} else {
		m.PIEnable = types.BoolNull()
	}

	if api.QIDMax != nil {
		m.QIDMax = types.Int64Value(*api.QIDMax)
	} else {
		m.QIDMax = types.Int64Null()
	}

	return diags
}

// guardedFields builds the set of optional payload fields shared by create
// and update: subnqn, allow_any_host, ana, ieee_oui, pi_enable, qid_max.
// Each is included only when its model value is known (not null, not
// unknown).
//
// subnqn and ieee_oui use a three-way rule instead of the simple guard:
// null/unknown are omitted (no opinion), an explicit empty string sends an
// explicit JSON null (clears the server-side value), and any other value is
// sent as-is. Without this, an explicit "" would be indistinguishable from
// "unset" and could never clear a previously-set subnqn/ieee_oui.
func (m *NVMetSubsysModel) guardedFields() map[string]any {
	p := map[string]any{}

	if !m.SubNQN.IsNull() && !m.SubNQN.IsUnknown() {
		if m.SubNQN.ValueString() == "" {
			p["subnqn"] = nil
		} else {
			p["subnqn"] = m.SubNQN.ValueString()
		}
	}
	if !m.AllowAnyHost.IsNull() && !m.AllowAnyHost.IsUnknown() {
		p["allow_any_host"] = m.AllowAnyHost.ValueBool()
	}
	if !m.ANA.IsNull() && !m.ANA.IsUnknown() {
		p["ana"] = m.ANA.ValueBool()
	}
	if !m.IEEEOUI.IsNull() && !m.IEEEOUI.IsUnknown() {
		if m.IEEEOUI.ValueString() == "" {
			p["ieee_oui"] = nil
		} else {
			p["ieee_oui"] = m.IEEEOUI.ValueString()
		}
	}
	if !m.PIEnable.IsNull() && !m.PIEnable.IsUnknown() {
		p["pi_enable"] = m.PIEnable.ValueBool()
	}
	if !m.QIDMax.IsNull() && !m.QIDMax.IsUnknown() {
		p["qid_max"] = m.QIDMax.ValueInt64()
	}

	return p
}

// createPayload builds the map[string]any payload for nvmet.subsys.create.
// "name" is always included; the remaining fields are guarded (see
// guardedFields).
func (m *NVMetSubsysModel) createPayload(_ context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	payload := m.guardedFields()
	payload["name"] = m.Name.ValueString()

	return payload, diags
}

// updatePayload builds the map[string]any payload for nvmet.subsys.update.
// It never includes "name": name changes are handled via RequiresReplace,
// and the API may reject "name" in an update call.
func (m *NVMetSubsysModel) updatePayload(_ context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	return m.guardedFields(), diags
}

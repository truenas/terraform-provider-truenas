// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package snmp_config

import (
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// snmpConfigResourceID is the fixed Terraform ID for this singleton resource:
// there is exactly one SNMP service configuration per TrueNAS system, and it
// is never created or deleted on TrueNAS itself.
const snmpConfigResourceID = "snmp_config"

// SNMPConfigModel is the Terraform state model for truenas_snmp_config.
//
// V3Password and V3PrivPassphrase are write-only secrets: the API never
// returns usable values for them (v3_password always reads back as "",
// masked), so they are Optional+Sensitive only (never Computed) and are
// never touched by responseToModel.
type SNMPConfigModel struct {
	ID               types.String `tfsdk:"id"` // fixed: "snmp_config"
	Community        types.String `tfsdk:"community"`
	Contact          types.String `tfsdk:"contact"`
	Location         types.String `tfsdk:"location"`
	LogLevel         types.Int64  `tfsdk:"loglevel"`
	Options          types.String `tfsdk:"options"`
	Traps            types.Bool   `tfsdk:"traps"`
	Zilstat          types.Bool   `tfsdk:"zilstat"`
	V3               types.Bool   `tfsdk:"v3"`
	V3Username       types.String `tfsdk:"v3_username"`
	V3AuthType       types.String `tfsdk:"v3_authtype"`
	V3Password       types.String `tfsdk:"v3_password"`       // write-only, Sensitive
	V3PrivProto      types.String `tfsdk:"v3_privproto"`      // nullable in API
	V3PrivPassphrase types.String `tfsdk:"v3_privpassphrase"` // write-only, Sensitive; nullable in API
}

// SNMPConfigDataSourceModel is the read-only model for the
// truenas_snmp_config datasource. It has no secret fields (v3_password,
// v3_privpassphrase): the API never returns usable values for them, so
// datasource attributes for them would always read as empty/unknown.
type SNMPConfigDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Community   types.String `tfsdk:"community"`
	Contact     types.String `tfsdk:"contact"`
	Location    types.String `tfsdk:"location"`
	LogLevel    types.Int64  `tfsdk:"loglevel"`
	Options     types.String `tfsdk:"options"`
	Traps       types.Bool   `tfsdk:"traps"`
	Zilstat     types.Bool   `tfsdk:"zilstat"`
	V3          types.Bool   `tfsdk:"v3"`
	V3Username  types.String `tfsdk:"v3_username"`
	V3AuthType  types.String `tfsdk:"v3_authtype"`
	V3PrivProto types.String `tfsdk:"v3_privproto"`
}

// snmpConfigAPI mirrors the JSON object returned by snmp.config and accepted
// (as a subset) by snmp.update. v3_password is present in the response
// (always "", masked) but is intentionally never read into either model.
type snmpConfigAPI struct {
	ID               int64   `json:"id"`
	Community        string  `json:"community"`
	Contact          string  `json:"contact"`
	Location         string  `json:"location"`
	LogLevel         int64   `json:"loglevel"`
	Options          string  `json:"options"`
	Traps            bool    `json:"traps"`
	Zilstat          bool    `json:"zilstat"`
	V3               bool    `json:"v3"`
	V3Username       string  `json:"v3_username"`
	V3AuthType       string  `json:"v3_authtype"`
	V3Password       string  `json:"v3_password"`
	V3PrivProto      *string `json:"v3_privproto"`
	V3PrivPassphrase *string `json:"v3_privpassphrase"`
}

// responseToModel maps an API response onto a Terraform model. V3Password
// and V3PrivPassphrase are NOT set here (write-only, never stored in state).
func responseToModel(api *snmpConfigAPI, m *SNMPConfigModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(snmpConfigResourceID)
	m.Community = types.StringValue(api.Community)
	m.Contact = types.StringValue(api.Contact)
	m.Location = types.StringValue(api.Location)
	m.LogLevel = types.Int64Value(api.LogLevel)
	m.Options = types.StringValue(api.Options)
	m.Traps = types.BoolValue(api.Traps)
	m.Zilstat = types.BoolValue(api.Zilstat)
	m.V3 = types.BoolValue(api.V3)
	m.V3Username = types.StringValue(api.V3Username)
	m.V3AuthType = types.StringValue(api.V3AuthType)

	if api.V3PrivProto != nil {
		m.V3PrivProto = types.StringValue(*api.V3PrivProto)
	} else {
		m.V3PrivProto = types.StringValue("")
	}

	// NOTE: V3Password and V3PrivPassphrase are NOT set here (write-only).

	return diags
}

// responseToDataSourceModel maps an API response onto a
// SNMPConfigDataSourceModel.
func responseToDataSourceModel(api *snmpConfigAPI, m *SNMPConfigDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(snmpConfigResourceID)
	m.Community = types.StringValue(api.Community)
	m.Contact = types.StringValue(api.Contact)
	m.Location = types.StringValue(api.Location)
	m.LogLevel = types.Int64Value(api.LogLevel)
	m.Options = types.StringValue(api.Options)
	m.Traps = types.BoolValue(api.Traps)
	m.Zilstat = types.BoolValue(api.Zilstat)
	m.V3 = types.BoolValue(api.V3)
	m.V3Username = types.StringValue(api.V3Username)
	m.V3AuthType = types.StringValue(api.V3AuthType)

	if api.V3PrivProto != nil {
		m.V3PrivProto = types.StringValue(*api.V3PrivProto)
	} else {
		m.V3PrivProto = types.StringValue("")
	}

	return diags
}

// updatePayload builds the snmp.update argument.
//
// community/contact/location/loglevel/options/traps/zilstat/v3/v3_username/
// v3_authtype are guarded: each is only included when known
// (Optional+Computed).
//
// v3_privproto is nullable on the API: it is omitted when null/unknown, sent
// as nil when explicitly set to "", and sent as the value otherwise.
//
// v3_password and v3_privpassphrase are write-only secrets: each is included
// only when known and non-null (a null/unknown value means "leave the
// current value alone" — never send an empty string or nil for these).
func (m *SNMPConfigModel) updatePayload() map[string]any {
	p := map[string]any{}

	if !m.Community.IsNull() && !m.Community.IsUnknown() {
		p["community"] = m.Community.ValueString()
	}
	if !m.Contact.IsNull() && !m.Contact.IsUnknown() {
		p["contact"] = m.Contact.ValueString()
	}
	if !m.Location.IsNull() && !m.Location.IsUnknown() {
		p["location"] = m.Location.ValueString()
	}
	if !m.LogLevel.IsNull() && !m.LogLevel.IsUnknown() {
		p["loglevel"] = m.LogLevel.ValueInt64()
	}
	if !m.Options.IsNull() && !m.Options.IsUnknown() {
		p["options"] = m.Options.ValueString()
	}
	if !m.Traps.IsNull() && !m.Traps.IsUnknown() {
		p["traps"] = m.Traps.ValueBool()
	}
	if !m.Zilstat.IsNull() && !m.Zilstat.IsUnknown() {
		p["zilstat"] = m.Zilstat.ValueBool()
	}
	if !m.V3.IsNull() && !m.V3.IsUnknown() {
		p["v3"] = m.V3.ValueBool()
	}
	if !m.V3Username.IsNull() && !m.V3Username.IsUnknown() {
		p["v3_username"] = m.V3Username.ValueString()
	}
	if !m.V3AuthType.IsNull() && !m.V3AuthType.IsUnknown() {
		p["v3_authtype"] = m.V3AuthType.ValueString()
	}
	if !m.V3PrivProto.IsNull() && !m.V3PrivProto.IsUnknown() {
		if v := m.V3PrivProto.ValueString(); v != "" {
			p["v3_privproto"] = v
		} else {
			p["v3_privproto"] = nil
		}
	}
	if !m.V3Password.IsNull() && !m.V3Password.IsUnknown() {
		p["v3_password"] = m.V3Password.ValueString()
	}
	if !m.V3PrivPassphrase.IsNull() && !m.V3PrivPassphrase.IsUnknown() {
		p["v3_privpassphrase"] = m.V3PrivPassphrase.ValueString()
	}

	return p
}

// deleteWarningDiagnostics builds the warning diagnostic emitted by Delete.
// Delete makes NO client calls: SNMP settings are system-critical monitoring
// configuration, so removing this resource from Terraform state must never
// reset the box's SNMP configuration. Splitting this into its own function
// keeps Delete's "no client calls" contract independently unit-testable.
func deleteWarningDiagnostics() diag.Diagnostics {
	var diags diag.Diagnostics
	diags.AddWarning(
		"SNMP configuration left in place",
		"SNMP configuration left in place; removed from Terraform state only",
	)
	return diags
}

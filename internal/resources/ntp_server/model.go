// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package ntp_server

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// NTPServerModel is the Terraform state model for truenas_ntp_server.
type NTPServerModel struct {
	ID      types.Int64  `tfsdk:"id"`
	Address types.String `tfsdk:"address"`
	Burst   types.Bool   `tfsdk:"burst"`
	IBurst  types.Bool   `tfsdk:"iburst"`
	Prefer  types.Bool   `tfsdk:"prefer"`
	MinPoll types.Int64  `tfsdk:"minpoll"`
	MaxPoll types.Int64  `tfsdk:"maxpoll"`
	Force   types.Bool   `tfsdk:"force"` // write-only validation bypass; never populated from API responses
}

// NTPServerDataSourceModel is NTPServerModel without the write-only force field.
type NTPServerDataSourceModel struct {
	ID      types.Int64  `tfsdk:"id"`
	Address types.String `tfsdk:"address"`
	Burst   types.Bool   `tfsdk:"burst"`
	IBurst  types.Bool   `tfsdk:"iburst"`
	Prefer  types.Bool   `tfsdk:"prefer"`
	MinPoll types.Int64  `tfsdk:"minpoll"`
	MaxPoll types.Int64  `tfsdk:"maxpoll"`
}

func responseToDataSourceModel(api *ntpServerAPI, m *NTPServerDataSourceModel) {
	m.ID = types.Int64Value(api.ID)
	m.Address = types.StringValue(api.Address)
	m.Burst = types.BoolValue(api.Burst)
	m.IBurst = types.BoolValue(api.IBurst)
	m.Prefer = types.BoolValue(api.Prefer)
	m.MinPoll = types.Int64Value(api.MinPoll)
	m.MaxPoll = types.Int64Value(api.MaxPoll)
}

// ntpServerAPI is the JSON wire format for a TrueNAS NTP server object.
type ntpServerAPI struct {
	ID      int64  `json:"id"`
	Address string `json:"address"`
	Burst   bool   `json:"burst"`
	IBurst  bool   `json:"iburst"`
	Prefer  bool   `json:"prefer"`
	MinPoll int64  `json:"minpoll"`
	MaxPoll int64  `json:"maxpoll"`
}

// responseToModel maps an API response onto a Terraform model. Force is
// intentionally never set here: it is a write-only validation bypass and has
// no corresponding field in API responses.
func responseToModel(_ context.Context, api *ntpServerAPI, m *NTPServerModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.Int64Value(api.ID)
	m.Address = types.StringValue(api.Address)
	m.Burst = types.BoolValue(api.Burst)
	m.IBurst = types.BoolValue(api.IBurst)
	m.Prefer = types.BoolValue(api.Prefer)
	m.MinPoll = types.Int64Value(api.MinPoll)
	m.MaxPoll = types.Int64Value(api.MaxPoll)
	return diags
}

// apiPayload builds the map[string]any payload for system.ntpserver.create /
// system.ntpserver.update. address is always included; burst, iburst,
// prefer, minpoll, maxpoll, and force are only included when set (non-null,
// non-unknown).
func (m *NTPServerModel) apiPayload(_ context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	p := map[string]any{
		"address": m.Address.ValueString(),
	}
	if !m.Burst.IsNull() && !m.Burst.IsUnknown() {
		p["burst"] = m.Burst.ValueBool()
	}
	if !m.IBurst.IsNull() && !m.IBurst.IsUnknown() {
		p["iburst"] = m.IBurst.ValueBool()
	}
	if !m.Prefer.IsNull() && !m.Prefer.IsUnknown() {
		p["prefer"] = m.Prefer.ValueBool()
	}
	if !m.MinPoll.IsNull() && !m.MinPoll.IsUnknown() {
		p["minpoll"] = m.MinPoll.ValueInt64()
	}
	if !m.MaxPoll.IsNull() && !m.MaxPoll.IsUnknown() {
		p["maxpoll"] = m.MaxPoll.ValueInt64()
	}
	if !m.Force.IsNull() && !m.Force.IsUnknown() {
		p["force"] = m.Force.ValueBool()
	}
	return p, diags
}

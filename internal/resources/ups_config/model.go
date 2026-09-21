// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package ups_config

import (
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// upsConfigResourceID is the fixed Terraform ID for this singleton resource:
// there is exactly one UPS service configuration per TrueNAS system, and it
// is never created or deleted on TrueNAS itself.
const upsConfigResourceID = "ups_config"

// UPSConfigModel is the Terraform state model for truenas_ups_config.
//
// MonPwd is a write-only secret: the API never returns a usable value for
// it, so it is Optional+Sensitive only (never Computed) and is never touched
// by responseToModel.
//
// CompleteIdentifier is a server-derived, Computed-only value (it is never
// sent to ups.update and has no UseStateForUnknown plan modifier, since it
// is recomputed from other fields on every read).
type UPSConfigModel struct {
	ID                 types.String `tfsdk:"id"` // fixed: "ups_config"
	Identifier         types.String `tfsdk:"identifier"`
	Mode               types.String `tfsdk:"mode"`
	RemoteHost         types.String `tfsdk:"remotehost"`
	RemotePort         types.Int64  `tfsdk:"remoteport"`
	Driver             types.String `tfsdk:"driver"`
	Port               types.String `tfsdk:"port"`
	Options            types.String `tfsdk:"options"`
	OptionsUPSD        types.String `tfsdk:"optionsupsd"`
	Description        types.String `tfsdk:"description"`
	Shutdown           types.String `tfsdk:"shutdown"`
	ShutdownTimer      types.Int64  `tfsdk:"shutdowntimer"`
	ShutdownCmd        types.String `tfsdk:"shutdowncmd"` // nullable
	MonUser            types.String `tfsdk:"monuser"`
	MonPwd             types.String `tfsdk:"monpwd"` // write-only, Sensitive
	ExtraUsers         types.String `tfsdk:"extrausers"`
	RMonitor           types.Bool   `tfsdk:"rmonitor"`
	PowerDown          types.Bool   `tfsdk:"powerdown"`
	HostSync           types.Int64  `tfsdk:"hostsync"`
	NoCommWarnTime     types.Int64  `tfsdk:"nocommwarntime"` // nullable
	CompleteIdentifier types.String `tfsdk:"complete_identifier"`
}

// UPSConfigDataSourceModel is the read-only model for the
// truenas_ups_config datasource. It has no monpwd field: the API never
// returns a usable value for it, so a datasource attribute for it would
// always read as empty/unknown.
type UPSConfigDataSourceModel struct {
	ID                 types.String `tfsdk:"id"`
	Identifier         types.String `tfsdk:"identifier"`
	Mode               types.String `tfsdk:"mode"`
	RemoteHost         types.String `tfsdk:"remotehost"`
	RemotePort         types.Int64  `tfsdk:"remoteport"`
	Driver             types.String `tfsdk:"driver"`
	Port               types.String `tfsdk:"port"`
	Options            types.String `tfsdk:"options"`
	OptionsUPSD        types.String `tfsdk:"optionsupsd"`
	Description        types.String `tfsdk:"description"`
	Shutdown           types.String `tfsdk:"shutdown"`
	ShutdownTimer      types.Int64  `tfsdk:"shutdowntimer"`
	ShutdownCmd        types.String `tfsdk:"shutdowncmd"`
	MonUser            types.String `tfsdk:"monuser"`
	ExtraUsers         types.String `tfsdk:"extrausers"`
	RMonitor           types.Bool   `tfsdk:"rmonitor"`
	PowerDown          types.Bool   `tfsdk:"powerdown"`
	HostSync           types.Int64  `tfsdk:"hostsync"`
	NoCommWarnTime     types.Int64  `tfsdk:"nocommwarntime"`
	CompleteIdentifier types.String `tfsdk:"complete_identifier"`
}

// upsConfigAPI mirrors the JSON object returned by ups.config and accepted
// (as a subset) by ups.update. monpwd is present in the response (masked)
// but is intentionally never read into either model. shutdowncmd and
// nocommwarntime are nullable on the API side, hence the pointer types.
// complete_identifier is server-derived and read-only.
type upsConfigAPI struct {
	ID                 int64   `json:"id"`
	Identifier         string  `json:"identifier"`
	Mode               string  `json:"mode"`
	RemoteHost         string  `json:"remotehost"`
	RemotePort         int64   `json:"remoteport"`
	Driver             string  `json:"driver"`
	Port               string  `json:"port"`
	Options            string  `json:"options"`
	OptionsUPSD        string  `json:"optionsupsd"`
	Description        string  `json:"description"`
	Shutdown           string  `json:"shutdown"`
	ShutdownTimer      int64   `json:"shutdowntimer"`
	ShutdownCmd        *string `json:"shutdowncmd"`
	MonUser            string  `json:"monuser"`
	ExtraUsers         string  `json:"extrausers"`
	RMonitor           bool    `json:"rmonitor"`
	PowerDown          bool    `json:"powerdown"`
	HostSync           int64   `json:"hostsync"`
	NoCommWarnTime     *int64  `json:"nocommwarntime"`
	CompleteIdentifier string  `json:"complete_identifier"`
}

// responseToModel maps an API response onto a Terraform model. MonPwd is
// NOT set here (write-only, never stored in state).
func responseToModel(api *upsConfigAPI, m *UPSConfigModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(upsConfigResourceID)
	m.Identifier = types.StringValue(api.Identifier)
	m.Mode = types.StringValue(api.Mode)
	m.RemoteHost = types.StringValue(api.RemoteHost)
	m.RemotePort = types.Int64Value(api.RemotePort)
	m.Driver = types.StringValue(api.Driver)
	m.Port = types.StringValue(api.Port)
	m.Options = types.StringValue(api.Options)
	m.OptionsUPSD = types.StringValue(api.OptionsUPSD)
	m.Description = types.StringValue(api.Description)
	m.Shutdown = types.StringValue(api.Shutdown)
	m.ShutdownTimer = types.Int64Value(api.ShutdownTimer)
	m.MonUser = types.StringValue(api.MonUser)
	m.ExtraUsers = types.StringValue(api.ExtraUsers)
	m.RMonitor = types.BoolValue(api.RMonitor)
	m.PowerDown = types.BoolValue(api.PowerDown)
	m.HostSync = types.Int64Value(api.HostSync)
	m.CompleteIdentifier = types.StringValue(api.CompleteIdentifier)

	if api.ShutdownCmd != nil {
		m.ShutdownCmd = types.StringValue(*api.ShutdownCmd)
	} else {
		m.ShutdownCmd = types.StringValue("")
	}

	if api.NoCommWarnTime != nil {
		m.NoCommWarnTime = types.Int64Value(*api.NoCommWarnTime)
	} else {
		m.NoCommWarnTime = types.Int64Value(0)
	}

	// NOTE: MonPwd is NOT set here (write-only).

	return diags
}

// responseToDataSourceModel maps an API response onto a
// UPSConfigDataSourceModel.
func responseToDataSourceModel(api *upsConfigAPI, m *UPSConfigDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(upsConfigResourceID)
	m.Identifier = types.StringValue(api.Identifier)
	m.Mode = types.StringValue(api.Mode)
	m.RemoteHost = types.StringValue(api.RemoteHost)
	m.RemotePort = types.Int64Value(api.RemotePort)
	m.Driver = types.StringValue(api.Driver)
	m.Port = types.StringValue(api.Port)
	m.Options = types.StringValue(api.Options)
	m.OptionsUPSD = types.StringValue(api.OptionsUPSD)
	m.Description = types.StringValue(api.Description)
	m.Shutdown = types.StringValue(api.Shutdown)
	m.ShutdownTimer = types.Int64Value(api.ShutdownTimer)
	m.MonUser = types.StringValue(api.MonUser)
	m.ExtraUsers = types.StringValue(api.ExtraUsers)
	m.RMonitor = types.BoolValue(api.RMonitor)
	m.PowerDown = types.BoolValue(api.PowerDown)
	m.HostSync = types.Int64Value(api.HostSync)
	m.CompleteIdentifier = types.StringValue(api.CompleteIdentifier)

	if api.ShutdownCmd != nil {
		m.ShutdownCmd = types.StringValue(*api.ShutdownCmd)
	} else {
		m.ShutdownCmd = types.StringValue("")
	}

	if api.NoCommWarnTime != nil {
		m.NoCommWarnTime = types.Int64Value(*api.NoCommWarnTime)
	} else {
		m.NoCommWarnTime = types.Int64Value(0)
	}

	return diags
}

// updatePayload builds the ups.update argument.
//
// identifier/mode/remotehost/remoteport/driver/port/options/optionsupsd/
// description/shutdown/shutdowntimer/monuser/extrausers/rmonitor/powerdown/
// hostsync are guarded: each is only included when known
// (Optional+Computed).
//
// shutdowncmd and nocommwarntime are nullable on the API: each is omitted
// when null/unknown, sent as nil when explicitly set to its zero value
// ("" / 0), and sent as the value otherwise.
//
// monpwd is a write-only secret: it is included only when known and
// non-null (a null/unknown value means "leave the current value alone" —
// never send an empty string or nil for it).
//
// complete_identifier is server-derived and is NEVER included in the
// payload.
func (m *UPSConfigModel) updatePayload() map[string]any {
	p := map[string]any{}

	if !m.Identifier.IsNull() && !m.Identifier.IsUnknown() {
		p["identifier"] = m.Identifier.ValueString()
	}
	if !m.Mode.IsNull() && !m.Mode.IsUnknown() {
		p["mode"] = m.Mode.ValueString()
	}
	if !m.RemoteHost.IsNull() && !m.RemoteHost.IsUnknown() {
		p["remotehost"] = m.RemoteHost.ValueString()
	}
	if !m.RemotePort.IsNull() && !m.RemotePort.IsUnknown() {
		p["remoteport"] = m.RemotePort.ValueInt64()
	}
	if !m.Driver.IsNull() && !m.Driver.IsUnknown() {
		p["driver"] = m.Driver.ValueString()
	}
	if !m.Port.IsNull() && !m.Port.IsUnknown() {
		p["port"] = m.Port.ValueString()
	}
	if !m.Options.IsNull() && !m.Options.IsUnknown() {
		p["options"] = m.Options.ValueString()
	}
	if !m.OptionsUPSD.IsNull() && !m.OptionsUPSD.IsUnknown() {
		p["optionsupsd"] = m.OptionsUPSD.ValueString()
	}
	if !m.Description.IsNull() && !m.Description.IsUnknown() {
		p["description"] = m.Description.ValueString()
	}
	if !m.Shutdown.IsNull() && !m.Shutdown.IsUnknown() {
		p["shutdown"] = m.Shutdown.ValueString()
	}
	if !m.ShutdownTimer.IsNull() && !m.ShutdownTimer.IsUnknown() {
		p["shutdowntimer"] = m.ShutdownTimer.ValueInt64()
	}
	if !m.MonUser.IsNull() && !m.MonUser.IsUnknown() {
		p["monuser"] = m.MonUser.ValueString()
	}
	if !m.ExtraUsers.IsNull() && !m.ExtraUsers.IsUnknown() {
		p["extrausers"] = m.ExtraUsers.ValueString()
	}
	if !m.RMonitor.IsNull() && !m.RMonitor.IsUnknown() {
		p["rmonitor"] = m.RMonitor.ValueBool()
	}
	if !m.PowerDown.IsNull() && !m.PowerDown.IsUnknown() {
		p["powerdown"] = m.PowerDown.ValueBool()
	}
	if !m.HostSync.IsNull() && !m.HostSync.IsUnknown() {
		p["hostsync"] = m.HostSync.ValueInt64()
	}

	if !m.ShutdownCmd.IsNull() && !m.ShutdownCmd.IsUnknown() {
		if v := m.ShutdownCmd.ValueString(); v != "" {
			p["shutdowncmd"] = v
		} else {
			p["shutdowncmd"] = nil
		}
	}
	if !m.NoCommWarnTime.IsNull() && !m.NoCommWarnTime.IsUnknown() {
		if v := m.NoCommWarnTime.ValueInt64(); v != 0 {
			p["nocommwarntime"] = v
		} else {
			p["nocommwarntime"] = nil
		}
	}

	if !m.MonPwd.IsNull() && !m.MonPwd.IsUnknown() {
		p["monpwd"] = m.MonPwd.ValueString()
	}

	return p
}

// basePayloadFromConfig builds a ups.update payload containing every
// writable, non-secret field taken from a live ups.config response. It is
// the base onto which plan-known values are overlaid (see mergedPayload):
// ups.update requires several fields (at minimum port and driver) on every
// call, even when the Terraform config only sets one cosmetic field such as
// description, so the fields the plan doesn't know about still need to be
// sent with their current live value. complete_identifier is server-derived
// and is intentionally excluded (never accepted by ups.update).
func basePayloadFromConfig(api *upsConfigAPI) map[string]any {
	p := map[string]any{
		"identifier":    api.Identifier,
		"mode":          api.Mode,
		"remotehost":    api.RemoteHost,
		"remoteport":    api.RemotePort,
		"driver":        api.Driver,
		"port":          api.Port,
		"options":       api.Options,
		"optionsupsd":   api.OptionsUPSD,
		"description":   api.Description,
		"shutdown":      api.Shutdown,
		"shutdowntimer": api.ShutdownTimer,
		"monuser":       api.MonUser,
		"extrausers":    api.ExtraUsers,
		"rmonitor":      api.RMonitor,
		"powerdown":     api.PowerDown,
		"hostsync":      api.HostSync,
	}
	if api.ShutdownCmd != nil {
		p["shutdowncmd"] = *api.ShutdownCmd
	} else {
		p["shutdowncmd"] = nil
	}
	if api.NoCommWarnTime != nil {
		p["nocommwarntime"] = *api.NoCommWarnTime
	} else {
		p["nocommwarntime"] = nil
	}
	return p
}

// mergedPayload builds the full ups.update argument for Create/Update: it
// starts from the live config's writable fields (basePayloadFromConfig) and
// overlays the plan's known values on top (updatePayload), so a plan value
// always wins over the live value it's replacing. monpwd is never part of
// the base (it's a write-only secret, absent from upsConfigAPI) and is
// included only when updatePayload includes it, i.e. only when the plan
// sets it.
func mergedPayload(live *upsConfigAPI, m *UPSConfigModel) map[string]any {
	p := basePayloadFromConfig(live)
	for k, v := range m.updatePayload() {
		p[k] = v
	}
	return p
}

// deleteWarningDiagnostics builds the warning diagnostic emitted by Delete.
// Delete makes NO client calls: UPS settings are system-critical power
// management configuration, so removing this resource from Terraform state
// must never reset the box's UPS configuration. Splitting this into its own
// function keeps Delete's "no client calls" contract independently
// unit-testable.
func deleteWarningDiagnostics() diag.Diagnostics {
	var diags diag.Diagnostics
	diags.AddWarning(
		"UPS configuration left in place",
		"UPS configuration left in place; removed from Terraform state only",
	)
	return diags
}

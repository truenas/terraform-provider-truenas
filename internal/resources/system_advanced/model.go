// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package system_advanced

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

// nvidiaSupported reports whether system.advanced.update accepts the
// "nvidia" field on the given (already-probed) TrueNAS release string. Pure
// function of the version string -- no live client access -- so it stays
// independently unit-testable, matching the webshare_config/lxc_config
// versionGateDiagnostics precedent.
//
// Live-probed evidence (see resource.go's applyNvidiaSupport doc comment
// for the full trail): a 25.10.3.1 VM and a 25.10.4 HA pair member both
// reject "nvidia" as an unrecognized system.advanced.update field ("Extra
// inputs are not permitted"); a 26.0.0-BETA.2 box accepts it. The field was
// added to system.advanced.update in TrueNAS 26.0 -- it never existed on any
// probed 25.10.x release, so this is a floor, not a drop (contrast with
// docker_config's applyNvidiaSupport, which gates a field 26.0+ removed).
func nvidiaSupported(version string) bool {
	return client.VersionAtLeastString(version, 26, 0)
}

// systemAdvancedResourceID is the fixed Terraform ID for this singleton
// resource: there is exactly one system advanced configuration per TrueNAS
// system, and it is never created or deleted on TrueNAS itself.
const systemAdvancedResourceID = "system_advanced"

// SystemAdvancedModel is the Terraform state model for
// truenas_system_advanced.
//
// SedPasswd is a write-only secret: TrueNAS never returns a usable value for
// it, so it is Optional+Sensitive only (never Computed) and is never touched
// by responseToModel; it has no corresponding field in systemAdvancedAPI at
// all, so it cannot be decoded from a response even by accident.
type SystemAdvancedModel struct {
	ID                 types.String `tfsdk:"id"` // fixed: "system_advanced"
	Advancedmode       types.Bool   `tfsdk:"advancedmode"`
	Anonstats          types.Bool   `tfsdk:"anonstats"`
	Autotune           types.Bool   `tfsdk:"autotune"`
	BootScrub          types.Int64  `tfsdk:"boot_scrub"`
	Consolemenu        types.Bool   `tfsdk:"consolemenu"`
	Consolemsg         types.Bool   `tfsdk:"consolemsg"`
	Debugkernel        types.Bool   `tfsdk:"debugkernel"`
	FqdnSyslog         types.Bool   `tfsdk:"fqdn_syslog"`
	KdumpEnabled       types.Bool   `tfsdk:"kdump_enabled"`
	KernelExtraOptions types.String `tfsdk:"kernel_extra_options"`
	LoginBanner        types.String `tfsdk:"login_banner"`
	Motd               types.String `tfsdk:"motd"`
	Nvidia             types.Bool   `tfsdk:"nvidia"`
	Overprovision      types.Int64  `tfsdk:"overprovision"` // nullable, three-way
	Powerdaemon        types.Bool   `tfsdk:"powerdaemon"`
	SedPasswd          types.String `tfsdk:"sed_passwd"` // write-only, Sensitive
	SedUser            types.String `tfsdk:"sed_user"`
	Serialconsole      types.Bool   `tfsdk:"serialconsole"`
	Serialport         types.String `tfsdk:"serialport"`
	Serialspeed        types.String `tfsdk:"serialspeed"`
	SyslogAudit        types.Bool   `tfsdk:"syslog_audit"`
	Sysloglevel        types.String `tfsdk:"sysloglevel"`
	Syslogservers      types.List   `tfsdk:"syslogservers"` // List[String]
	Traceback          types.Bool   `tfsdk:"traceback"`
	Uploadcrash        types.Bool   `tfsdk:"uploadcrash"`

	// Computed-only: never sent to system.advanced.update.
	AnonstatsToken    types.String `tfsdk:"anonstats_token"`
	IsolatedGpuPciIds types.List   `tfsdk:"isolated_gpu_pci_ids"` // List[String]
}

// SystemAdvancedDataSourceModel is the read-only model for the
// truenas_system_advanced datasource. It has no sed_passwd field: the API
// never returns a usable value for it, so a datasource attribute for it
// would always read as empty/unknown.
type SystemAdvancedDataSourceModel struct {
	ID                 types.String `tfsdk:"id"`
	Advancedmode       types.Bool   `tfsdk:"advancedmode"`
	Anonstats          types.Bool   `tfsdk:"anonstats"`
	Autotune           types.Bool   `tfsdk:"autotune"`
	BootScrub          types.Int64  `tfsdk:"boot_scrub"`
	Consolemenu        types.Bool   `tfsdk:"consolemenu"`
	Consolemsg         types.Bool   `tfsdk:"consolemsg"`
	Debugkernel        types.Bool   `tfsdk:"debugkernel"`
	FqdnSyslog         types.Bool   `tfsdk:"fqdn_syslog"`
	KdumpEnabled       types.Bool   `tfsdk:"kdump_enabled"`
	KernelExtraOptions types.String `tfsdk:"kernel_extra_options"`
	LoginBanner        types.String `tfsdk:"login_banner"`
	Motd               types.String `tfsdk:"motd"`
	Nvidia             types.Bool   `tfsdk:"nvidia"`
	Overprovision      types.Int64  `tfsdk:"overprovision"`
	Powerdaemon        types.Bool   `tfsdk:"powerdaemon"`
	SedUser            types.String `tfsdk:"sed_user"`
	Serialconsole      types.Bool   `tfsdk:"serialconsole"`
	Serialport         types.String `tfsdk:"serialport"`
	Serialspeed        types.String `tfsdk:"serialspeed"`
	SyslogAudit        types.Bool   `tfsdk:"syslog_audit"`
	Sysloglevel        types.String `tfsdk:"sysloglevel"`
	Syslogservers      types.List   `tfsdk:"syslogservers"`
	Traceback          types.Bool   `tfsdk:"traceback"`
	Uploadcrash        types.Bool   `tfsdk:"uploadcrash"`
	AnonstatsToken     types.String `tfsdk:"anonstats_token"`
	IsolatedGpuPciIds  types.List   `tfsdk:"isolated_gpu_pci_ids"`
}

// systemAdvancedAPI mirrors the JSON object returned by
// system.advanced.config and accepted (as a subset) by
// system.advanced.update. overprovision is nullable on the wire, so it is
// modeled as a pointer; every other writable field is always present as a
// concrete value. anonstats_token and isolated_gpu_pci_ids are
// server-computed and are never accepted by system.advanced.update.
// sed_passwd deliberately has NO field here: TrueNAS never returns a usable
// value for it, so it must never be decodable from a response.
type systemAdvancedAPI struct {
	ID                 int64    `json:"id"`
	Advancedmode       bool     `json:"advancedmode"`
	Anonstats          bool     `json:"anonstats"`
	AnonstatsToken     string   `json:"anonstats_token"`
	Autotune           bool     `json:"autotune"`
	BootScrub          int64    `json:"boot_scrub"`
	Consolemenu        bool     `json:"consolemenu"`
	Consolemsg         bool     `json:"consolemsg"`
	Debugkernel        bool     `json:"debugkernel"`
	FqdnSyslog         bool     `json:"fqdn_syslog"`
	IsolatedGpuPciIds  []string `json:"isolated_gpu_pci_ids"`
	KdumpEnabled       bool     `json:"kdump_enabled"`
	KernelExtraOptions string   `json:"kernel_extra_options"`
	LoginBanner        string   `json:"login_banner"`
	Motd               string   `json:"motd"`
	Nvidia             bool     `json:"nvidia"`
	Overprovision      *int64   `json:"overprovision"`
	Powerdaemon        bool     `json:"powerdaemon"`
	SedUser            string   `json:"sed_user"`
	Serialconsole      bool     `json:"serialconsole"`
	Serialport         string   `json:"serialport"`
	Serialspeed        string   `json:"serialspeed"`
	SyslogAudit        bool     `json:"syslog_audit"`
	Sysloglevel        string   `json:"sysloglevel"`
	Syslogservers      []string `json:"syslogservers"`
	Traceback          bool     `json:"traceback"`
	Uploadcrash        bool     `json:"uploadcrash"`
}

// responseToModel maps an API response onto a Terraform model. The nil
// pointer field (overprovision) maps to null rather than a zero value, so
// that an update payload built from this model omits it (see updatePayload)
// instead of sending an explicit 0 that overwrites a server-side null.
// SedPasswd is NOT set here (write-only, never stored in state from a
// response; whatever value came from the plan is left untouched by the
// caller).
func responseToModel(ctx context.Context, api *systemAdvancedAPI, m *SystemAdvancedModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(systemAdvancedResourceID)
	m.Advancedmode = types.BoolValue(api.Advancedmode)
	m.Anonstats = types.BoolValue(api.Anonstats)
	m.AnonstatsToken = types.StringValue(api.AnonstatsToken)
	m.Autotune = types.BoolValue(api.Autotune)
	m.BootScrub = types.Int64Value(api.BootScrub)
	m.Consolemenu = types.BoolValue(api.Consolemenu)
	m.Consolemsg = types.BoolValue(api.Consolemsg)
	m.Debugkernel = types.BoolValue(api.Debugkernel)
	m.FqdnSyslog = types.BoolValue(api.FqdnSyslog)
	m.KdumpEnabled = types.BoolValue(api.KdumpEnabled)
	m.KernelExtraOptions = types.StringValue(api.KernelExtraOptions)
	m.LoginBanner = types.StringValue(api.LoginBanner)
	m.Motd = types.StringValue(api.Motd)
	m.Nvidia = types.BoolValue(api.Nvidia)
	m.Powerdaemon = types.BoolValue(api.Powerdaemon)
	m.SedUser = types.StringValue(api.SedUser)
	m.Serialconsole = types.BoolValue(api.Serialconsole)
	m.Serialport = types.StringValue(api.Serialport)
	m.Serialspeed = types.StringValue(api.Serialspeed)
	m.SyslogAudit = types.BoolValue(api.SyslogAudit)
	m.Sysloglevel = types.StringValue(api.Sysloglevel)
	m.Traceback = types.BoolValue(api.Traceback)
	m.Uploadcrash = types.BoolValue(api.Uploadcrash)

	// NOTE: SedPasswd is NOT set here (write-only).

	if api.Overprovision != nil {
		m.Overprovision = types.Int64Value(*api.Overprovision)
	} else {
		m.Overprovision = types.Int64Null()
	}

	isolatedGpuPciIds := api.IsolatedGpuPciIds
	if isolatedGpuPciIds == nil {
		isolatedGpuPciIds = []string{}
	}
	isolatedGpuPciIdsList, d := types.ListValueFrom(ctx, types.StringType, isolatedGpuPciIds)
	diags.Append(d...)
	m.IsolatedGpuPciIds = isolatedGpuPciIdsList

	syslogservers := api.Syslogservers
	if syslogservers == nil {
		syslogservers = []string{}
	}
	syslogserversList, d := types.ListValueFrom(ctx, types.StringType, syslogservers)
	diags.Append(d...)
	m.Syslogservers = syslogserversList

	return diags
}

// responseToDataSourceModel maps an API response onto a
// SystemAdvancedDataSourceModel using the same nil-pointer-to-null rules as
// responseToModel.
func responseToDataSourceModel(ctx context.Context, api *systemAdvancedAPI, m *SystemAdvancedDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(systemAdvancedResourceID)
	m.Advancedmode = types.BoolValue(api.Advancedmode)
	m.Anonstats = types.BoolValue(api.Anonstats)
	m.AnonstatsToken = types.StringValue(api.AnonstatsToken)
	m.Autotune = types.BoolValue(api.Autotune)
	m.BootScrub = types.Int64Value(api.BootScrub)
	m.Consolemenu = types.BoolValue(api.Consolemenu)
	m.Consolemsg = types.BoolValue(api.Consolemsg)
	m.Debugkernel = types.BoolValue(api.Debugkernel)
	m.FqdnSyslog = types.BoolValue(api.FqdnSyslog)
	m.KdumpEnabled = types.BoolValue(api.KdumpEnabled)
	m.KernelExtraOptions = types.StringValue(api.KernelExtraOptions)
	m.LoginBanner = types.StringValue(api.LoginBanner)
	m.Motd = types.StringValue(api.Motd)
	m.Nvidia = types.BoolValue(api.Nvidia)
	m.Powerdaemon = types.BoolValue(api.Powerdaemon)
	m.SedUser = types.StringValue(api.SedUser)
	m.Serialconsole = types.BoolValue(api.Serialconsole)
	m.Serialport = types.StringValue(api.Serialport)
	m.Serialspeed = types.StringValue(api.Serialspeed)
	m.SyslogAudit = types.BoolValue(api.SyslogAudit)
	m.Sysloglevel = types.StringValue(api.Sysloglevel)
	m.Traceback = types.BoolValue(api.Traceback)
	m.Uploadcrash = types.BoolValue(api.Uploadcrash)

	if api.Overprovision != nil {
		m.Overprovision = types.Int64Value(*api.Overprovision)
	} else {
		m.Overprovision = types.Int64Null()
	}

	isolatedGpuPciIds := api.IsolatedGpuPciIds
	if isolatedGpuPciIds == nil {
		isolatedGpuPciIds = []string{}
	}
	isolatedGpuPciIdsList, d := types.ListValueFrom(ctx, types.StringType, isolatedGpuPciIds)
	diags.Append(d...)
	m.IsolatedGpuPciIds = isolatedGpuPciIdsList

	syslogservers := api.Syslogservers
	if syslogservers == nil {
		syslogservers = []string{}
	}
	syslogserversList, d := types.ListValueFrom(ctx, types.StringType, syslogservers)
	diags.Append(d...)
	m.Syslogservers = syslogserversList

	return diags
}

// updatePayload builds the system.advanced.update argument. Every writable
// field is guarded: each is only included when known (Optional+Computed).
// anonstats_token and isolated_gpu_pci_ids are Computed-only and are NEVER
// included here: they are server-computed values (isolated_gpu_pci_ids is
// managed by the separate system.advanced.update_gpu_pci_ids method, out of
// scope for this resource). overprovision is nullable on the wire: it is
// omitted entirely when null/unknown, sent as JSON nil when the model holds
// an explicit 0 (clearing overprovision), and sent as its value otherwise.
// syslogservers is only included when known, with a nil ElementsAs guard so
// a null/unknown list never panics; it is normalized to an empty slice when
// nil so an explicitly-set-but-empty list clears the value on TrueNAS rather
// than being omitted. sed_passwd is a write-only secret: it is included only
// when known and non-null (a null/unknown value means "leave the current
// value alone" — never send an empty string or nil for it). nvidia is
// intentionally NOT handled here: it needs its own version-gated handling
// (resource.go's applyNvidiaSupport, driven by the practitioner's raw
// Config rather than the resolved Plan) since it does not exist on
// system.advanced.update below TrueNAS 26.0 — see nvidiaSupported's doc
// comment for the live-probed evidence.
func (m *SystemAdvancedModel) updatePayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	p := map[string]any{}

	if !m.Advancedmode.IsNull() && !m.Advancedmode.IsUnknown() {
		p["advancedmode"] = m.Advancedmode.ValueBool()
	}
	if !m.Anonstats.IsNull() && !m.Anonstats.IsUnknown() {
		p["anonstats"] = m.Anonstats.ValueBool()
	}
	if !m.Autotune.IsNull() && !m.Autotune.IsUnknown() {
		p["autotune"] = m.Autotune.ValueBool()
	}
	if !m.BootScrub.IsNull() && !m.BootScrub.IsUnknown() {
		p["boot_scrub"] = m.BootScrub.ValueInt64()
	}
	if !m.Consolemenu.IsNull() && !m.Consolemenu.IsUnknown() {
		p["consolemenu"] = m.Consolemenu.ValueBool()
	}
	if !m.Consolemsg.IsNull() && !m.Consolemsg.IsUnknown() {
		p["consolemsg"] = m.Consolemsg.ValueBool()
	}
	if !m.Debugkernel.IsNull() && !m.Debugkernel.IsUnknown() {
		p["debugkernel"] = m.Debugkernel.ValueBool()
	}
	if !m.FqdnSyslog.IsNull() && !m.FqdnSyslog.IsUnknown() {
		p["fqdn_syslog"] = m.FqdnSyslog.ValueBool()
	}
	if !m.KdumpEnabled.IsNull() && !m.KdumpEnabled.IsUnknown() {
		p["kdump_enabled"] = m.KdumpEnabled.ValueBool()
	}
	if !m.KernelExtraOptions.IsNull() && !m.KernelExtraOptions.IsUnknown() {
		p["kernel_extra_options"] = m.KernelExtraOptions.ValueString()
	}
	if !m.LoginBanner.IsNull() && !m.LoginBanner.IsUnknown() {
		p["login_banner"] = m.LoginBanner.ValueString()
	}
	if !m.Motd.IsNull() && !m.Motd.IsUnknown() {
		p["motd"] = m.Motd.ValueString()
	}
	if !m.Overprovision.IsNull() && !m.Overprovision.IsUnknown() {
		if v := m.Overprovision.ValueInt64(); v != 0 {
			p["overprovision"] = v
		} else {
			p["overprovision"] = nil
		}
	}
	if !m.Powerdaemon.IsNull() && !m.Powerdaemon.IsUnknown() {
		p["powerdaemon"] = m.Powerdaemon.ValueBool()
	}
	if !m.SedPasswd.IsNull() && !m.SedPasswd.IsUnknown() {
		p["sed_passwd"] = m.SedPasswd.ValueString()
	}
	if !m.SedUser.IsNull() && !m.SedUser.IsUnknown() {
		p["sed_user"] = m.SedUser.ValueString()
	}
	if !m.Serialconsole.IsNull() && !m.Serialconsole.IsUnknown() {
		p["serialconsole"] = m.Serialconsole.ValueBool()
	}
	if !m.Serialport.IsNull() && !m.Serialport.IsUnknown() {
		p["serialport"] = m.Serialport.ValueString()
	}
	if !m.Serialspeed.IsNull() && !m.Serialspeed.IsUnknown() {
		p["serialspeed"] = m.Serialspeed.ValueString()
	}
	if !m.SyslogAudit.IsNull() && !m.SyslogAudit.IsUnknown() {
		p["syslog_audit"] = m.SyslogAudit.ValueBool()
	}
	if !m.Sysloglevel.IsNull() && !m.Sysloglevel.IsUnknown() {
		p["sysloglevel"] = m.Sysloglevel.ValueString()
	}
	if !m.Syslogservers.IsNull() && !m.Syslogservers.IsUnknown() {
		var v []string
		diags.Append(m.Syslogservers.ElementsAs(ctx, &v, false)...)
		if v == nil {
			v = []string{}
		}
		p["syslogservers"] = v
	}
	if !m.Traceback.IsNull() && !m.Traceback.IsUnknown() {
		p["traceback"] = m.Traceback.ValueBool()
	}
	if !m.Uploadcrash.IsNull() && !m.Uploadcrash.IsUnknown() {
		p["uploadcrash"] = m.Uploadcrash.ValueBool()
	}

	return p, diags
}

// deleteWarningDiagnostics builds the warning diagnostic emitted by Delete.
// Delete makes NO client calls: the system advanced configuration controls
// syslog, console, and kernel/hardware behavior, so removing this resource
// from Terraform state must never rewrite the box's configuration.
// Splitting this into its own function keeps Delete's "no client calls"
// contract independently unit-testable.
func deleteWarningDiagnostics() diag.Diagnostics {
	var diags diag.Diagnostics
	diags.AddWarning(
		"System advanced configuration left in place",
		"System advanced configuration left in place; removed from Terraform state only",
	)
	return diags
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package smb_config

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

// post2600FieldsSupported reports whether smb.update accepts
// "stateful_failover", "minimum_protocol", and "search_protocols" on the
// given (already-probed) TrueNAS release string. Pure function of the
// version string -- no live client access -- so it stays independently
// unit-testable, matching the webshare_config/lxc_config
// versionGateDiagnostics precedent.
//
// Live-probed evidence (see resource.go's applyPost2600FieldsSupport doc
// comment for the full trail): a 25.10.3.1 VM and a 25.10.4 HA pair member
// both reject all three fields as unrecognized smb.update fields ("Extra
// inputs are not permitted"); a 26.0.0-BETA.2 box accepts all three. All
// three were added to smb.update in TrueNAS 26.0 -- none of them existed on
// any probed 25.10.x release, so this is a floor, not a drop.
func post2600FieldsSupported(version string) bool {
	return client.VersionAtLeastString(version, 26, 0)
}

// smbConfigResourceID is the fixed Terraform ID for this singleton resource:
// there is exactly one SMB service configuration per TrueNAS system, and it
// is never created or deleted on TrueNAS itself.
const smbConfigResourceID = "smb_config"

// SMBConfigModel is the Terraform state model for truenas_smb_config.
type SMBConfigModel struct {
	ID               types.String `tfsdk:"id"` // fixed: "smb_config"
	NetBIOSName      types.String `tfsdk:"netbiosname"`
	NetBIOSAlias     types.List   `tfsdk:"netbiosalias"` // List[String]
	Workgroup        types.String `tfsdk:"workgroup"`
	Description      types.String `tfsdk:"description"`
	UnixCharset      types.String `tfsdk:"unixcharset"`
	LocalMaster      types.Bool   `tfsdk:"localmaster"`
	Syslog           types.Bool   `tfsdk:"syslog"`
	AAPLExtensions   types.Bool   `tfsdk:"aapl_extensions"`
	AdminGroup       types.String `tfsdk:"admin_group"` // nullable in API
	Guest            types.String `tfsdk:"guest"`
	FileMask         types.String `tfsdk:"filemask"` // "DEFAULT" sentinel
	DirMask          types.String `tfsdk:"dirmask"`  // "DEFAULT" sentinel
	NTLMv1Auth       types.Bool   `tfsdk:"ntlmv1_auth"`
	Multichannel     types.Bool   `tfsdk:"multichannel"`
	Encryption       types.String `tfsdk:"encryption"`
	BindIP           types.List   `tfsdk:"bindip"` // List[String]
	SMBOptions       types.String `tfsdk:"smb_options"`
	Debug            types.Bool   `tfsdk:"debug"`
	StatefulFailover types.Bool   `tfsdk:"stateful_failover"`
	MinimumProtocol  types.String `tfsdk:"minimum_protocol"`
	SearchProtocols  types.List   `tfsdk:"search_protocols"` // List[String]
	ServerSID        types.String `tfsdk:"server_sid"`       // computed-only, never in payload
}

// SMBConfigDataSourceModel is the read-only model for the truenas_smb_config
// datasource.
type SMBConfigDataSourceModel struct {
	ID               types.String `tfsdk:"id"`
	NetBIOSName      types.String `tfsdk:"netbiosname"`
	NetBIOSAlias     types.List   `tfsdk:"netbiosalias"`
	Workgroup        types.String `tfsdk:"workgroup"`
	Description      types.String `tfsdk:"description"`
	UnixCharset      types.String `tfsdk:"unixcharset"`
	LocalMaster      types.Bool   `tfsdk:"localmaster"`
	Syslog           types.Bool   `tfsdk:"syslog"`
	AAPLExtensions   types.Bool   `tfsdk:"aapl_extensions"`
	AdminGroup       types.String `tfsdk:"admin_group"`
	Guest            types.String `tfsdk:"guest"`
	FileMask         types.String `tfsdk:"filemask"`
	DirMask          types.String `tfsdk:"dirmask"`
	NTLMv1Auth       types.Bool   `tfsdk:"ntlmv1_auth"`
	Multichannel     types.Bool   `tfsdk:"multichannel"`
	Encryption       types.String `tfsdk:"encryption"`
	BindIP           types.List   `tfsdk:"bindip"`
	SMBOptions       types.String `tfsdk:"smb_options"`
	Debug            types.Bool   `tfsdk:"debug"`
	StatefulFailover types.Bool   `tfsdk:"stateful_failover"`
	MinimumProtocol  types.String `tfsdk:"minimum_protocol"`
	SearchProtocols  types.List   `tfsdk:"search_protocols"`
	ServerSID        types.String `tfsdk:"server_sid"`
}

// smbConfigAPI mirrors the JSON object returned by smb.config and accepted
// (as a subset) by smb.update. admin_group is nullable on the wire (TrueNAS
// clears the SMB admin group by sending/receiving JSON null), so it is
// modeled as *string; every other field is always present as a concrete
// value.
type smbConfigAPI struct {
	ID               int64    `json:"id"`
	NetBIOSName      string   `json:"netbiosname"`
	NetBIOSAlias     []string `json:"netbiosalias"`
	Workgroup        string   `json:"workgroup"`
	Description      string   `json:"description"`
	UnixCharset      string   `json:"unixcharset"`
	LocalMaster      bool     `json:"localmaster"`
	Syslog           bool     `json:"syslog"`
	AAPLExtensions   bool     `json:"aapl_extensions"`
	AdminGroup       *string  `json:"admin_group"`
	Guest            string   `json:"guest"`
	FileMask         string   `json:"filemask"`
	DirMask          string   `json:"dirmask"`
	NTLMv1Auth       bool     `json:"ntlmv1_auth"`
	Multichannel     bool     `json:"multichannel"`
	Encryption       string   `json:"encryption"`
	BindIP           []string `json:"bindip"`
	SMBOptions       string   `json:"smb_options"`
	Debug            bool     `json:"debug"`
	StatefulFailover bool     `json:"stateful_failover"`
	MinimumProtocol  string   `json:"minimum_protocol"`
	SearchProtocols  []string `json:"search_protocols"`
	ServerSID        string   `json:"server_sid"`
}

// responseToModel maps an API response onto a Terraform model.
func responseToModel(ctx context.Context, api *smbConfigAPI, m *SMBConfigModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(smbConfigResourceID)
	m.NetBIOSName = types.StringValue(api.NetBIOSName)
	m.Workgroup = types.StringValue(api.Workgroup)
	m.Description = types.StringValue(api.Description)
	m.UnixCharset = types.StringValue(api.UnixCharset)
	m.LocalMaster = types.BoolValue(api.LocalMaster)
	m.Syslog = types.BoolValue(api.Syslog)
	m.AAPLExtensions = types.BoolValue(api.AAPLExtensions)
	m.Guest = types.StringValue(api.Guest)
	m.FileMask = types.StringValue(api.FileMask)
	m.DirMask = types.StringValue(api.DirMask)
	m.NTLMv1Auth = types.BoolValue(api.NTLMv1Auth)
	m.Multichannel = types.BoolValue(api.Multichannel)
	m.Encryption = types.StringValue(api.Encryption)
	m.SMBOptions = types.StringValue(api.SMBOptions)
	m.Debug = types.BoolValue(api.Debug)
	m.StatefulFailover = types.BoolValue(api.StatefulFailover)
	m.MinimumProtocol = types.StringValue(api.MinimumProtocol)
	m.ServerSID = types.StringValue(api.ServerSID)

	if api.AdminGroup != nil {
		m.AdminGroup = types.StringValue(*api.AdminGroup)
	} else {
		m.AdminGroup = types.StringValue("")
	}

	netbiosalias := api.NetBIOSAlias
	if netbiosalias == nil {
		netbiosalias = []string{}
	}
	netbiosaliasList, d := types.ListValueFrom(ctx, types.StringType, netbiosalias)
	diags.Append(d...)
	m.NetBIOSAlias = netbiosaliasList

	bindip := api.BindIP
	if bindip == nil {
		bindip = []string{}
	}
	bindipList, d := types.ListValueFrom(ctx, types.StringType, bindip)
	diags.Append(d...)
	m.BindIP = bindipList

	searchProtocols := api.SearchProtocols
	if searchProtocols == nil {
		searchProtocols = []string{}
	}
	searchProtocolsList, d := types.ListValueFrom(ctx, types.StringType, searchProtocols)
	diags.Append(d...)
	m.SearchProtocols = searchProtocolsList

	return diags
}

// responseToDataSourceModel maps an API response onto a
// SMBConfigDataSourceModel.
func responseToDataSourceModel(ctx context.Context, api *smbConfigAPI, m *SMBConfigDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(smbConfigResourceID)
	m.NetBIOSName = types.StringValue(api.NetBIOSName)
	m.Workgroup = types.StringValue(api.Workgroup)
	m.Description = types.StringValue(api.Description)
	m.UnixCharset = types.StringValue(api.UnixCharset)
	m.LocalMaster = types.BoolValue(api.LocalMaster)
	m.Syslog = types.BoolValue(api.Syslog)
	m.AAPLExtensions = types.BoolValue(api.AAPLExtensions)
	m.Guest = types.StringValue(api.Guest)
	m.FileMask = types.StringValue(api.FileMask)
	m.DirMask = types.StringValue(api.DirMask)
	m.NTLMv1Auth = types.BoolValue(api.NTLMv1Auth)
	m.Multichannel = types.BoolValue(api.Multichannel)
	m.Encryption = types.StringValue(api.Encryption)
	m.SMBOptions = types.StringValue(api.SMBOptions)
	m.Debug = types.BoolValue(api.Debug)
	m.StatefulFailover = types.BoolValue(api.StatefulFailover)
	m.MinimumProtocol = types.StringValue(api.MinimumProtocol)
	m.ServerSID = types.StringValue(api.ServerSID)

	if api.AdminGroup != nil {
		m.AdminGroup = types.StringValue(*api.AdminGroup)
	} else {
		m.AdminGroup = types.StringValue("")
	}

	netbiosalias := api.NetBIOSAlias
	if netbiosalias == nil {
		netbiosalias = []string{}
	}
	netbiosaliasList, d := types.ListValueFrom(ctx, types.StringType, netbiosalias)
	diags.Append(d...)
	m.NetBIOSAlias = netbiosaliasList

	bindip := api.BindIP
	if bindip == nil {
		bindip = []string{}
	}
	bindipList, d := types.ListValueFrom(ctx, types.StringType, bindip)
	diags.Append(d...)
	m.BindIP = bindipList

	searchProtocols := api.SearchProtocols
	if searchProtocols == nil {
		searchProtocols = []string{}
	}
	searchProtocolsList, d := types.ListValueFrom(ctx, types.StringType, searchProtocols)
	diags.Append(d...)
	m.SearchProtocols = searchProtocolsList

	return diags
}

// updatePayload builds the smb.update argument. Every field is guarded: each
// is only included when known (Optional+Computed). server_sid is
// Computed-only and is NEVER included here: it is a stable server-assigned
// value, not something smb.update accepts. admin_group is nullable on the
// wire: it is omitted when null/unknown, sent as JSON nil when the model
// holds an explicit empty string (clearing the SMB admin group), and sent as
// its value otherwise. The two list fields handled here (netbiosalias,
// bindip) are only included when known, with a nil ElementsAs guard so a
// null/unknown list never panics; each list is normalized to an empty slice
// when nil so an explicitly-set-but-empty list clears the corresponding
// value on TrueNAS rather than being omitted. stateful_failover,
// minimum_protocol, and search_protocols are intentionally NOT handled
// here: they need their own version-gated handling (resource.go's
// applyPost2600FieldsSupport, driven by the practitioner's raw Config
// rather than the resolved Plan) since none of them exist on smb.update
// below TrueNAS 26.0 — see post2600FieldsSupported's doc comment for the
// live-probed evidence.
func (m *SMBConfigModel) updatePayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	p := map[string]any{}

	if !m.NetBIOSName.IsNull() && !m.NetBIOSName.IsUnknown() {
		p["netbiosname"] = m.NetBIOSName.ValueString()
	}
	if !m.NetBIOSAlias.IsNull() && !m.NetBIOSAlias.IsUnknown() {
		var v []string
		diags.Append(m.NetBIOSAlias.ElementsAs(ctx, &v, false)...)
		if v == nil {
			v = []string{}
		}
		p["netbiosalias"] = v
	}
	if !m.Workgroup.IsNull() && !m.Workgroup.IsUnknown() {
		p["workgroup"] = m.Workgroup.ValueString()
	}
	if !m.Description.IsNull() && !m.Description.IsUnknown() {
		p["description"] = m.Description.ValueString()
	}
	if !m.UnixCharset.IsNull() && !m.UnixCharset.IsUnknown() {
		p["unixcharset"] = m.UnixCharset.ValueString()
	}
	if !m.LocalMaster.IsNull() && !m.LocalMaster.IsUnknown() {
		p["localmaster"] = m.LocalMaster.ValueBool()
	}
	if !m.Syslog.IsNull() && !m.Syslog.IsUnknown() {
		p["syslog"] = m.Syslog.ValueBool()
	}
	if !m.AAPLExtensions.IsNull() && !m.AAPLExtensions.IsUnknown() {
		p["aapl_extensions"] = m.AAPLExtensions.ValueBool()
	}
	if !m.AdminGroup.IsNull() && !m.AdminGroup.IsUnknown() {
		v := m.AdminGroup.ValueString()
		if v == "" {
			p["admin_group"] = nil
		} else {
			p["admin_group"] = v
		}
	}
	if !m.Guest.IsNull() && !m.Guest.IsUnknown() {
		p["guest"] = m.Guest.ValueString()
	}
	if !m.FileMask.IsNull() && !m.FileMask.IsUnknown() {
		p["filemask"] = m.FileMask.ValueString()
	}
	if !m.DirMask.IsNull() && !m.DirMask.IsUnknown() {
		p["dirmask"] = m.DirMask.ValueString()
	}
	if !m.NTLMv1Auth.IsNull() && !m.NTLMv1Auth.IsUnknown() {
		p["ntlmv1_auth"] = m.NTLMv1Auth.ValueBool()
	}
	if !m.Multichannel.IsNull() && !m.Multichannel.IsUnknown() {
		p["multichannel"] = m.Multichannel.ValueBool()
	}
	if !m.Encryption.IsNull() && !m.Encryption.IsUnknown() {
		p["encryption"] = m.Encryption.ValueString()
	}
	if !m.BindIP.IsNull() && !m.BindIP.IsUnknown() {
		var v []string
		diags.Append(m.BindIP.ElementsAs(ctx, &v, false)...)
		if v == nil {
			v = []string{}
		}
		p["bindip"] = v
	}
	if !m.SMBOptions.IsNull() && !m.SMBOptions.IsUnknown() {
		p["smb_options"] = m.SMBOptions.ValueString()
	}
	if !m.Debug.IsNull() && !m.Debug.IsUnknown() {
		p["debug"] = m.Debug.ValueBool()
	}

	return p, diags
}

// deleteWarningDiagnostics builds the warning diagnostic emitted by Delete.
// Delete makes NO client calls: the SMB service configuration is
// system-critical (shares and domain membership may depend on it), so
// removing this resource from Terraform state must never rewrite the box's
// SMB configuration. Splitting this into its own function keeps Delete's
// "no client calls" contract independently unit-testable.
func deleteWarningDiagnostics() diag.Diagnostics {
	var diags diag.Diagnostics
	diags.AddWarning(
		"SMB configuration left in place",
		"SMB configuration left in place; removed from Terraform state only",
	)
	return diags
}

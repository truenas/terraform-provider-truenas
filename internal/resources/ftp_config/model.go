// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package ftp_config

import (
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ftpConfigResourceID is the fixed Terraform ID for this singleton resource:
// there is exactly one FTP service configuration per TrueNAS system, and it
// is never created or deleted on TrueNAS itself.
const ftpConfigResourceID = "ftp_config"

// FTPConfigModel is the Terraform state model for truenas_ftp_config.
type FTPConfigModel struct {
	ID                types.String `tfsdk:"id"` // fixed: "ftp_config"
	Port              types.Int64  `tfsdk:"port"`
	Clients           types.Int64  `tfsdk:"clients"`
	IPConnections     types.Int64  `tfsdk:"ipconnections"`
	LoginAttempt      types.Int64  `tfsdk:"loginattempt"`
	Timeout           types.Int64  `tfsdk:"timeout"`
	TimeoutNoTransfer types.Int64  `tfsdk:"timeout_notransfer"`
	LocalUserBW       types.Int64  `tfsdk:"localuserbw"`
	LocalUserDLBW     types.Int64  `tfsdk:"localuserdlbw"`
	AnonUserBW        types.Int64  `tfsdk:"anonuserbw"`
	AnonUserDLBW      types.Int64  `tfsdk:"anonuserdlbw"`
	PassivePortsMin   types.Int64  `tfsdk:"passiveportsmin"`
	PassivePortsMax   types.Int64  `tfsdk:"passiveportsmax"`

	DefaultRoot                     types.Bool `tfsdk:"defaultroot"`
	OnlyAnonymous                   types.Bool `tfsdk:"onlyanonymous"`
	OnlyLocal                       types.Bool `tfsdk:"onlylocal"`
	Ident                           types.Bool `tfsdk:"ident"`
	FXP                             types.Bool `tfsdk:"fxp"`
	Resume                          types.Bool `tfsdk:"resume"`
	ReverseDNS                      types.Bool `tfsdk:"reversedns"`
	TLS                             types.Bool `tfsdk:"tls"`
	TLSOptAllowClientRenegotiations types.Bool `tfsdk:"tls_opt_allow_client_renegotiations"`
	TLSOptAllowDotLogin             types.Bool `tfsdk:"tls_opt_allow_dot_login"`
	TLSOptAllowPerUser              types.Bool `tfsdk:"tls_opt_allow_per_user"`
	TLSOptCommonNameRequired        types.Bool `tfsdk:"tls_opt_common_name_required"`
	TLSOptDNSNameRequired           types.Bool `tfsdk:"tls_opt_dns_name_required"`
	TLSOptEnableDiags               types.Bool `tfsdk:"tls_opt_enable_diags"`
	TLSOptExportCertData            types.Bool `tfsdk:"tls_opt_export_cert_data"`
	TLSOptIPAddressRequired         types.Bool `tfsdk:"tls_opt_ip_address_required"`
	TLSOptNoEmptyFragments          types.Bool `tfsdk:"tls_opt_no_empty_fragments"`
	TLSOptNoSessionReuseRequired    types.Bool `tfsdk:"tls_opt_no_session_reuse_required"`
	TLSOptStdEnvVars                types.Bool `tfsdk:"tls_opt_stdenvvars"`

	MasqAddress types.String `tfsdk:"masqaddress"`
	Banner      types.String `tfsdk:"banner"`
	Options     types.String `tfsdk:"options"`
	DirMask     types.String `tfsdk:"dirmask"`
	FileMask    types.String `tfsdk:"filemask"`
	TLSPolicy   types.String `tfsdk:"tls_policy"`

	// Nullable fields: the API accepts/returns null for these. A null API
	// value maps to the type's zero value (0 / "") in the model; an
	// explicitly-set zero value in the model is sent back to the API as
	// null (see updatePayload).
	SSLTLSCertificate types.Int64  `tfsdk:"ssltls_certificate"`
	AnonPath          types.String `tfsdk:"anonpath"`
}

// FTPConfigDataSourceModel is the read-only model for the truenas_ftp_config
// datasource.
type FTPConfigDataSourceModel struct {
	ID                types.String `tfsdk:"id"`
	Port              types.Int64  `tfsdk:"port"`
	Clients           types.Int64  `tfsdk:"clients"`
	IPConnections     types.Int64  `tfsdk:"ipconnections"`
	LoginAttempt      types.Int64  `tfsdk:"loginattempt"`
	Timeout           types.Int64  `tfsdk:"timeout"`
	TimeoutNoTransfer types.Int64  `tfsdk:"timeout_notransfer"`
	LocalUserBW       types.Int64  `tfsdk:"localuserbw"`
	LocalUserDLBW     types.Int64  `tfsdk:"localuserdlbw"`
	AnonUserBW        types.Int64  `tfsdk:"anonuserbw"`
	AnonUserDLBW      types.Int64  `tfsdk:"anonuserdlbw"`
	PassivePortsMin   types.Int64  `tfsdk:"passiveportsmin"`
	PassivePortsMax   types.Int64  `tfsdk:"passiveportsmax"`

	DefaultRoot                     types.Bool `tfsdk:"defaultroot"`
	OnlyAnonymous                   types.Bool `tfsdk:"onlyanonymous"`
	OnlyLocal                       types.Bool `tfsdk:"onlylocal"`
	Ident                           types.Bool `tfsdk:"ident"`
	FXP                             types.Bool `tfsdk:"fxp"`
	Resume                          types.Bool `tfsdk:"resume"`
	ReverseDNS                      types.Bool `tfsdk:"reversedns"`
	TLS                             types.Bool `tfsdk:"tls"`
	TLSOptAllowClientRenegotiations types.Bool `tfsdk:"tls_opt_allow_client_renegotiations"`
	TLSOptAllowDotLogin             types.Bool `tfsdk:"tls_opt_allow_dot_login"`
	TLSOptAllowPerUser              types.Bool `tfsdk:"tls_opt_allow_per_user"`
	TLSOptCommonNameRequired        types.Bool `tfsdk:"tls_opt_common_name_required"`
	TLSOptDNSNameRequired           types.Bool `tfsdk:"tls_opt_dns_name_required"`
	TLSOptEnableDiags               types.Bool `tfsdk:"tls_opt_enable_diags"`
	TLSOptExportCertData            types.Bool `tfsdk:"tls_opt_export_cert_data"`
	TLSOptIPAddressRequired         types.Bool `tfsdk:"tls_opt_ip_address_required"`
	TLSOptNoEmptyFragments          types.Bool `tfsdk:"tls_opt_no_empty_fragments"`
	TLSOptNoSessionReuseRequired    types.Bool `tfsdk:"tls_opt_no_session_reuse_required"`
	TLSOptStdEnvVars                types.Bool `tfsdk:"tls_opt_stdenvvars"`

	MasqAddress types.String `tfsdk:"masqaddress"`
	Banner      types.String `tfsdk:"banner"`
	Options     types.String `tfsdk:"options"`
	DirMask     types.String `tfsdk:"dirmask"`
	FileMask    types.String `tfsdk:"filemask"`
	TLSPolicy   types.String `tfsdk:"tls_policy"`

	SSLTLSCertificate types.Int64  `tfsdk:"ssltls_certificate"`
	AnonPath          types.String `tfsdk:"anonpath"`
}

// ftpConfigAPI mirrors the JSON object returned by ftp.config and accepted
// (as a subset) by ftp.update. SSLTLSCertificate and AnonPath are nullable
// on the API side, hence the pointer types.
type ftpConfigAPI struct {
	ID                int64 `json:"id"`
	Port              int64 `json:"port"`
	Clients           int64 `json:"clients"`
	IPConnections     int64 `json:"ipconnections"`
	LoginAttempt      int64 `json:"loginattempt"`
	Timeout           int64 `json:"timeout"`
	TimeoutNoTransfer int64 `json:"timeout_notransfer"`
	LocalUserBW       int64 `json:"localuserbw"`
	LocalUserDLBW     int64 `json:"localuserdlbw"`
	AnonUserBW        int64 `json:"anonuserbw"`
	AnonUserDLBW      int64 `json:"anonuserdlbw"`
	PassivePortsMin   int64 `json:"passiveportsmin"`
	PassivePortsMax   int64 `json:"passiveportsmax"`

	DefaultRoot                     bool `json:"defaultroot"`
	OnlyAnonymous                   bool `json:"onlyanonymous"`
	OnlyLocal                       bool `json:"onlylocal"`
	Ident                           bool `json:"ident"`
	FXP                             bool `json:"fxp"`
	Resume                          bool `json:"resume"`
	ReverseDNS                      bool `json:"reversedns"`
	TLS                             bool `json:"tls"`
	TLSOptAllowClientRenegotiations bool `json:"tls_opt_allow_client_renegotiations"`
	TLSOptAllowDotLogin             bool `json:"tls_opt_allow_dot_login"`
	TLSOptAllowPerUser              bool `json:"tls_opt_allow_per_user"`
	TLSOptCommonNameRequired        bool `json:"tls_opt_common_name_required"`
	TLSOptDNSNameRequired           bool `json:"tls_opt_dns_name_required"`
	TLSOptEnableDiags               bool `json:"tls_opt_enable_diags"`
	TLSOptExportCertData            bool `json:"tls_opt_export_cert_data"`
	TLSOptIPAddressRequired         bool `json:"tls_opt_ip_address_required"`
	TLSOptNoEmptyFragments          bool `json:"tls_opt_no_empty_fragments"`
	TLSOptNoSessionReuseRequired    bool `json:"tls_opt_no_session_reuse_required"`
	TLSOptStdEnvVars                bool `json:"tls_opt_stdenvvars"`

	MasqAddress string `json:"masqaddress"`
	Banner      string `json:"banner"`
	Options     string `json:"options"`
	DirMask     string `json:"dirmask"`
	FileMask    string `json:"filemask"`
	TLSPolicy   string `json:"tls_policy"`

	SSLTLSCertificate *int64  `json:"ssltls_certificate"`
	AnonPath          *string `json:"anonpath"`
}

// responseToModel maps an API response onto a Terraform model.
func responseToModel(api *ftpConfigAPI, m *FTPConfigModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(ftpConfigResourceID)
	m.Port = types.Int64Value(api.Port)
	m.Clients = types.Int64Value(api.Clients)
	m.IPConnections = types.Int64Value(api.IPConnections)
	m.LoginAttempt = types.Int64Value(api.LoginAttempt)
	m.Timeout = types.Int64Value(api.Timeout)
	m.TimeoutNoTransfer = types.Int64Value(api.TimeoutNoTransfer)
	m.LocalUserBW = types.Int64Value(api.LocalUserBW)
	m.LocalUserDLBW = types.Int64Value(api.LocalUserDLBW)
	m.AnonUserBW = types.Int64Value(api.AnonUserBW)
	m.AnonUserDLBW = types.Int64Value(api.AnonUserDLBW)
	m.PassivePortsMin = types.Int64Value(api.PassivePortsMin)
	m.PassivePortsMax = types.Int64Value(api.PassivePortsMax)

	m.DefaultRoot = types.BoolValue(api.DefaultRoot)
	m.OnlyAnonymous = types.BoolValue(api.OnlyAnonymous)
	m.OnlyLocal = types.BoolValue(api.OnlyLocal)
	m.Ident = types.BoolValue(api.Ident)
	m.FXP = types.BoolValue(api.FXP)
	m.Resume = types.BoolValue(api.Resume)
	m.ReverseDNS = types.BoolValue(api.ReverseDNS)
	m.TLS = types.BoolValue(api.TLS)
	m.TLSOptAllowClientRenegotiations = types.BoolValue(api.TLSOptAllowClientRenegotiations)
	m.TLSOptAllowDotLogin = types.BoolValue(api.TLSOptAllowDotLogin)
	m.TLSOptAllowPerUser = types.BoolValue(api.TLSOptAllowPerUser)
	m.TLSOptCommonNameRequired = types.BoolValue(api.TLSOptCommonNameRequired)
	m.TLSOptDNSNameRequired = types.BoolValue(api.TLSOptDNSNameRequired)
	m.TLSOptEnableDiags = types.BoolValue(api.TLSOptEnableDiags)
	m.TLSOptExportCertData = types.BoolValue(api.TLSOptExportCertData)
	m.TLSOptIPAddressRequired = types.BoolValue(api.TLSOptIPAddressRequired)
	m.TLSOptNoEmptyFragments = types.BoolValue(api.TLSOptNoEmptyFragments)
	m.TLSOptNoSessionReuseRequired = types.BoolValue(api.TLSOptNoSessionReuseRequired)
	m.TLSOptStdEnvVars = types.BoolValue(api.TLSOptStdEnvVars)

	m.MasqAddress = types.StringValue(api.MasqAddress)
	m.Banner = types.StringValue(api.Banner)
	m.Options = types.StringValue(api.Options)
	m.DirMask = types.StringValue(api.DirMask)
	m.FileMask = types.StringValue(api.FileMask)
	m.TLSPolicy = types.StringValue(api.TLSPolicy)

	if api.SSLTLSCertificate != nil {
		m.SSLTLSCertificate = types.Int64Value(*api.SSLTLSCertificate)
	} else {
		m.SSLTLSCertificate = types.Int64Value(0)
	}
	if api.AnonPath != nil {
		m.AnonPath = types.StringValue(*api.AnonPath)
	} else {
		m.AnonPath = types.StringValue("")
	}

	return diags
}

// responseToDataSourceModel maps an API response onto a
// FTPConfigDataSourceModel.
func responseToDataSourceModel(api *ftpConfigAPI, m *FTPConfigDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(ftpConfigResourceID)
	m.Port = types.Int64Value(api.Port)
	m.Clients = types.Int64Value(api.Clients)
	m.IPConnections = types.Int64Value(api.IPConnections)
	m.LoginAttempt = types.Int64Value(api.LoginAttempt)
	m.Timeout = types.Int64Value(api.Timeout)
	m.TimeoutNoTransfer = types.Int64Value(api.TimeoutNoTransfer)
	m.LocalUserBW = types.Int64Value(api.LocalUserBW)
	m.LocalUserDLBW = types.Int64Value(api.LocalUserDLBW)
	m.AnonUserBW = types.Int64Value(api.AnonUserBW)
	m.AnonUserDLBW = types.Int64Value(api.AnonUserDLBW)
	m.PassivePortsMin = types.Int64Value(api.PassivePortsMin)
	m.PassivePortsMax = types.Int64Value(api.PassivePortsMax)

	m.DefaultRoot = types.BoolValue(api.DefaultRoot)
	m.OnlyAnonymous = types.BoolValue(api.OnlyAnonymous)
	m.OnlyLocal = types.BoolValue(api.OnlyLocal)
	m.Ident = types.BoolValue(api.Ident)
	m.FXP = types.BoolValue(api.FXP)
	m.Resume = types.BoolValue(api.Resume)
	m.ReverseDNS = types.BoolValue(api.ReverseDNS)
	m.TLS = types.BoolValue(api.TLS)
	m.TLSOptAllowClientRenegotiations = types.BoolValue(api.TLSOptAllowClientRenegotiations)
	m.TLSOptAllowDotLogin = types.BoolValue(api.TLSOptAllowDotLogin)
	m.TLSOptAllowPerUser = types.BoolValue(api.TLSOptAllowPerUser)
	m.TLSOptCommonNameRequired = types.BoolValue(api.TLSOptCommonNameRequired)
	m.TLSOptDNSNameRequired = types.BoolValue(api.TLSOptDNSNameRequired)
	m.TLSOptEnableDiags = types.BoolValue(api.TLSOptEnableDiags)
	m.TLSOptExportCertData = types.BoolValue(api.TLSOptExportCertData)
	m.TLSOptIPAddressRequired = types.BoolValue(api.TLSOptIPAddressRequired)
	m.TLSOptNoEmptyFragments = types.BoolValue(api.TLSOptNoEmptyFragments)
	m.TLSOptNoSessionReuseRequired = types.BoolValue(api.TLSOptNoSessionReuseRequired)
	m.TLSOptStdEnvVars = types.BoolValue(api.TLSOptStdEnvVars)

	m.MasqAddress = types.StringValue(api.MasqAddress)
	m.Banner = types.StringValue(api.Banner)
	m.Options = types.StringValue(api.Options)
	m.DirMask = types.StringValue(api.DirMask)
	m.FileMask = types.StringValue(api.FileMask)
	m.TLSPolicy = types.StringValue(api.TLSPolicy)

	if api.SSLTLSCertificate != nil {
		m.SSLTLSCertificate = types.Int64Value(*api.SSLTLSCertificate)
	} else {
		m.SSLTLSCertificate = types.Int64Value(0)
	}
	if api.AnonPath != nil {
		m.AnonPath = types.StringValue(*api.AnonPath)
	} else {
		m.AnonPath = types.StringValue("")
	}

	return diags
}

// updatePayload builds the ftp.update argument. Every field is guarded:
// each is only included when known (Optional+Computed). The two nullable
// fields (ssltls_certificate, anonpath) get three-way handling: null/unknown
// is omitted entirely, an explicitly-set zero value (0 / "") is sent as
// nil to clear it on TrueNAS, and any other known value is sent as-is.
func (m *FTPConfigModel) updatePayload() map[string]any {
	p := map[string]any{}

	if !m.Port.IsNull() && !m.Port.IsUnknown() {
		p["port"] = m.Port.ValueInt64()
	}
	if !m.Clients.IsNull() && !m.Clients.IsUnknown() {
		p["clients"] = m.Clients.ValueInt64()
	}
	if !m.IPConnections.IsNull() && !m.IPConnections.IsUnknown() {
		p["ipconnections"] = m.IPConnections.ValueInt64()
	}
	if !m.LoginAttempt.IsNull() && !m.LoginAttempt.IsUnknown() {
		p["loginattempt"] = m.LoginAttempt.ValueInt64()
	}
	if !m.Timeout.IsNull() && !m.Timeout.IsUnknown() {
		p["timeout"] = m.Timeout.ValueInt64()
	}
	if !m.TimeoutNoTransfer.IsNull() && !m.TimeoutNoTransfer.IsUnknown() {
		p["timeout_notransfer"] = m.TimeoutNoTransfer.ValueInt64()
	}
	if !m.LocalUserBW.IsNull() && !m.LocalUserBW.IsUnknown() {
		p["localuserbw"] = m.LocalUserBW.ValueInt64()
	}
	if !m.LocalUserDLBW.IsNull() && !m.LocalUserDLBW.IsUnknown() {
		p["localuserdlbw"] = m.LocalUserDLBW.ValueInt64()
	}
	if !m.AnonUserBW.IsNull() && !m.AnonUserBW.IsUnknown() {
		p["anonuserbw"] = m.AnonUserBW.ValueInt64()
	}
	if !m.AnonUserDLBW.IsNull() && !m.AnonUserDLBW.IsUnknown() {
		p["anonuserdlbw"] = m.AnonUserDLBW.ValueInt64()
	}
	if !m.PassivePortsMin.IsNull() && !m.PassivePortsMin.IsUnknown() {
		p["passiveportsmin"] = m.PassivePortsMin.ValueInt64()
	}
	if !m.PassivePortsMax.IsNull() && !m.PassivePortsMax.IsUnknown() {
		p["passiveportsmax"] = m.PassivePortsMax.ValueInt64()
	}

	if !m.DefaultRoot.IsNull() && !m.DefaultRoot.IsUnknown() {
		p["defaultroot"] = m.DefaultRoot.ValueBool()
	}
	if !m.OnlyAnonymous.IsNull() && !m.OnlyAnonymous.IsUnknown() {
		p["onlyanonymous"] = m.OnlyAnonymous.ValueBool()
	}
	if !m.OnlyLocal.IsNull() && !m.OnlyLocal.IsUnknown() {
		p["onlylocal"] = m.OnlyLocal.ValueBool()
	}
	if !m.Ident.IsNull() && !m.Ident.IsUnknown() {
		p["ident"] = m.Ident.ValueBool()
	}
	if !m.FXP.IsNull() && !m.FXP.IsUnknown() {
		p["fxp"] = m.FXP.ValueBool()
	}
	if !m.Resume.IsNull() && !m.Resume.IsUnknown() {
		p["resume"] = m.Resume.ValueBool()
	}
	if !m.ReverseDNS.IsNull() && !m.ReverseDNS.IsUnknown() {
		p["reversedns"] = m.ReverseDNS.ValueBool()
	}
	if !m.TLS.IsNull() && !m.TLS.IsUnknown() {
		p["tls"] = m.TLS.ValueBool()
	}
	if !m.TLSOptAllowClientRenegotiations.IsNull() && !m.TLSOptAllowClientRenegotiations.IsUnknown() {
		p["tls_opt_allow_client_renegotiations"] = m.TLSOptAllowClientRenegotiations.ValueBool()
	}
	if !m.TLSOptAllowDotLogin.IsNull() && !m.TLSOptAllowDotLogin.IsUnknown() {
		p["tls_opt_allow_dot_login"] = m.TLSOptAllowDotLogin.ValueBool()
	}
	if !m.TLSOptAllowPerUser.IsNull() && !m.TLSOptAllowPerUser.IsUnknown() {
		p["tls_opt_allow_per_user"] = m.TLSOptAllowPerUser.ValueBool()
	}
	if !m.TLSOptCommonNameRequired.IsNull() && !m.TLSOptCommonNameRequired.IsUnknown() {
		p["tls_opt_common_name_required"] = m.TLSOptCommonNameRequired.ValueBool()
	}
	if !m.TLSOptDNSNameRequired.IsNull() && !m.TLSOptDNSNameRequired.IsUnknown() {
		p["tls_opt_dns_name_required"] = m.TLSOptDNSNameRequired.ValueBool()
	}
	if !m.TLSOptEnableDiags.IsNull() && !m.TLSOptEnableDiags.IsUnknown() {
		p["tls_opt_enable_diags"] = m.TLSOptEnableDiags.ValueBool()
	}
	if !m.TLSOptExportCertData.IsNull() && !m.TLSOptExportCertData.IsUnknown() {
		p["tls_opt_export_cert_data"] = m.TLSOptExportCertData.ValueBool()
	}
	if !m.TLSOptIPAddressRequired.IsNull() && !m.TLSOptIPAddressRequired.IsUnknown() {
		p["tls_opt_ip_address_required"] = m.TLSOptIPAddressRequired.ValueBool()
	}
	if !m.TLSOptNoEmptyFragments.IsNull() && !m.TLSOptNoEmptyFragments.IsUnknown() {
		p["tls_opt_no_empty_fragments"] = m.TLSOptNoEmptyFragments.ValueBool()
	}
	if !m.TLSOptNoSessionReuseRequired.IsNull() && !m.TLSOptNoSessionReuseRequired.IsUnknown() {
		p["tls_opt_no_session_reuse_required"] = m.TLSOptNoSessionReuseRequired.ValueBool()
	}
	if !m.TLSOptStdEnvVars.IsNull() && !m.TLSOptStdEnvVars.IsUnknown() {
		p["tls_opt_stdenvvars"] = m.TLSOptStdEnvVars.ValueBool()
	}

	if !m.MasqAddress.IsNull() && !m.MasqAddress.IsUnknown() {
		p["masqaddress"] = m.MasqAddress.ValueString()
	}
	if !m.Banner.IsNull() && !m.Banner.IsUnknown() {
		p["banner"] = m.Banner.ValueString()
	}
	if !m.Options.IsNull() && !m.Options.IsUnknown() {
		p["options"] = m.Options.ValueString()
	}
	if !m.DirMask.IsNull() && !m.DirMask.IsUnknown() {
		p["dirmask"] = m.DirMask.ValueString()
	}
	if !m.FileMask.IsNull() && !m.FileMask.IsUnknown() {
		p["filemask"] = m.FileMask.ValueString()
	}
	if !m.TLSPolicy.IsNull() && !m.TLSPolicy.IsUnknown() {
		p["tls_policy"] = m.TLSPolicy.ValueString()
	}

	if !m.SSLTLSCertificate.IsNull() && !m.SSLTLSCertificate.IsUnknown() {
		if v := m.SSLTLSCertificate.ValueInt64(); v != 0 {
			p["ssltls_certificate"] = v
		} else {
			p["ssltls_certificate"] = nil
		}
	}
	if !m.AnonPath.IsNull() && !m.AnonPath.IsUnknown() {
		if v := m.AnonPath.ValueString(); v != "" {
			p["anonpath"] = v
		} else {
			p["anonpath"] = nil
		}
	}

	return p
}

// deleteWarningDiagnostics builds the warning diagnostic emitted by Delete.
// Delete makes NO client calls: the FTP service configuration is left in
// place on TrueNAS, and removing this resource from Terraform state must
// never rewrite it. Splitting this into its own function keeps Delete's
// "no client calls" contract independently unit-testable.
func deleteWarningDiagnostics() diag.Diagnostics {
	var diags diag.Diagnostics
	diags.AddWarning(
		"FTP configuration left in place",
		"FTP configuration left in place; removed from Terraform state only",
	)
	return diags
}

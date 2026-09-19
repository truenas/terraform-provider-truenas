// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package ssh_config

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// sshConfigResourceID is the fixed Terraform ID for this singleton resource:
// there is exactly one SSH service configuration per TrueNAS system, and it
// is never created or deleted on TrueNAS itself.
const sshConfigResourceID = "ssh_config"

// SSHConfigModel is the Terraform state model for truenas_ssh_config.
//
// SSH host keys (host_ecdsa_key, host_key, host_dsa_key, host_rsa_key,
// host_ed25519_key, as returned by ssh.config) are intentionally NOT
// modeled: they are server-managed and are excluded entirely from this
// resource.
type SSHConfigModel struct {
	ID                  types.String `tfsdk:"id"`        // fixed: "ssh_config"
	BindIface           types.List   `tfsdk:"bindiface"` // List[String]
	Compression         types.Bool   `tfsdk:"compression"`
	KerberosAuth        types.Bool   `tfsdk:"kerberosauth"`
	Options             types.String `tfsdk:"options"`
	PasswordLoginGroups types.List   `tfsdk:"password_login_groups"` // List[String]
	PasswordAuth        types.Bool   `tfsdk:"passwordauth"`
	SFTPLogFacility     types.String `tfsdk:"sftp_log_facility"`
	SFTPLogLevel        types.String `tfsdk:"sftp_log_level"`
	TCPFwd              types.Bool   `tfsdk:"tcpfwd"`
	TCPPort             types.Int64  `tfsdk:"tcpport"`
	WeakCiphers         types.List   `tfsdk:"weak_ciphers"` // List[String]
}

// SSHConfigDataSourceModel is the read-only model for the truenas_ssh_config
// datasource.
type SSHConfigDataSourceModel struct {
	ID                  types.String `tfsdk:"id"`
	BindIface           types.List   `tfsdk:"bindiface"`
	Compression         types.Bool   `tfsdk:"compression"`
	KerberosAuth        types.Bool   `tfsdk:"kerberosauth"`
	Options             types.String `tfsdk:"options"`
	PasswordLoginGroups types.List   `tfsdk:"password_login_groups"`
	PasswordAuth        types.Bool   `tfsdk:"passwordauth"`
	SFTPLogFacility     types.String `tfsdk:"sftp_log_facility"`
	SFTPLogLevel        types.String `tfsdk:"sftp_log_level"`
	TCPFwd              types.Bool   `tfsdk:"tcpfwd"`
	TCPPort             types.Int64  `tfsdk:"tcpport"`
	WeakCiphers         types.List   `tfsdk:"weak_ciphers"`
}

// sshConfigAPI mirrors the JSON object returned by ssh.config and accepted
// (as a subset) by ssh.update. Host keys are deliberately absent: this
// struct is only used to decode the fields this resource models.
type sshConfigAPI struct {
	ID                  int64    `json:"id"`
	BindIface           []string `json:"bindiface"`
	Compression         bool     `json:"compression"`
	KerberosAuth        bool     `json:"kerberosauth"`
	Options             string   `json:"options"`
	PasswordLoginGroups []string `json:"password_login_groups"`
	PasswordAuth        bool     `json:"passwordauth"`
	SFTPLogFacility     string   `json:"sftp_log_facility"`
	SFTPLogLevel        string   `json:"sftp_log_level"`
	TCPFwd              bool     `json:"tcpfwd"`
	TCPPort             int64    `json:"tcpport"`
	WeakCiphers         []string `json:"weak_ciphers"`
}

// responseToModel maps an API response onto a Terraform model.
func responseToModel(ctx context.Context, api *sshConfigAPI, m *SSHConfigModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(sshConfigResourceID)
	m.Compression = types.BoolValue(api.Compression)
	m.KerberosAuth = types.BoolValue(api.KerberosAuth)
	m.Options = types.StringValue(api.Options)
	m.PasswordAuth = types.BoolValue(api.PasswordAuth)
	m.SFTPLogFacility = types.StringValue(api.SFTPLogFacility)
	m.SFTPLogLevel = types.StringValue(api.SFTPLogLevel)
	m.TCPFwd = types.BoolValue(api.TCPFwd)
	m.TCPPort = types.Int64Value(api.TCPPort)

	bindiface := api.BindIface
	if bindiface == nil {
		bindiface = []string{}
	}
	bindifaceList, d := types.ListValueFrom(ctx, types.StringType, bindiface)
	diags.Append(d...)
	m.BindIface = bindifaceList

	groups := api.PasswordLoginGroups
	if groups == nil {
		groups = []string{}
	}
	groupsList, d := types.ListValueFrom(ctx, types.StringType, groups)
	diags.Append(d...)
	m.PasswordLoginGroups = groupsList

	weakCiphers := api.WeakCiphers
	if weakCiphers == nil {
		weakCiphers = []string{}
	}
	weakCiphersList, d := types.ListValueFrom(ctx, types.StringType, weakCiphers)
	diags.Append(d...)
	m.WeakCiphers = weakCiphersList

	return diags
}

// responseToDataSourceModel maps an API response onto a
// SSHConfigDataSourceModel.
func responseToDataSourceModel(ctx context.Context, api *sshConfigAPI, m *SSHConfigDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(sshConfigResourceID)
	m.Compression = types.BoolValue(api.Compression)
	m.KerberosAuth = types.BoolValue(api.KerberosAuth)
	m.Options = types.StringValue(api.Options)
	m.PasswordAuth = types.BoolValue(api.PasswordAuth)
	m.SFTPLogFacility = types.StringValue(api.SFTPLogFacility)
	m.SFTPLogLevel = types.StringValue(api.SFTPLogLevel)
	m.TCPFwd = types.BoolValue(api.TCPFwd)
	m.TCPPort = types.Int64Value(api.TCPPort)

	bindiface := api.BindIface
	if bindiface == nil {
		bindiface = []string{}
	}
	bindifaceList, d := types.ListValueFrom(ctx, types.StringType, bindiface)
	diags.Append(d...)
	m.BindIface = bindifaceList

	groups := api.PasswordLoginGroups
	if groups == nil {
		groups = []string{}
	}
	groupsList, d := types.ListValueFrom(ctx, types.StringType, groups)
	diags.Append(d...)
	m.PasswordLoginGroups = groupsList

	weakCiphers := api.WeakCiphers
	if weakCiphers == nil {
		weakCiphers = []string{}
	}
	weakCiphersList, d := types.ListValueFrom(ctx, types.StringType, weakCiphers)
	diags.Append(d...)
	m.WeakCiphers = weakCiphersList

	return diags
}

// updatePayload builds the ssh.update argument. Every field is guarded:
// each is only included when known (Optional+Computed). The three list
// fields (bindiface, password_login_groups, weak_ciphers) are only included
// when known, with a nil ElementsAs guard so a null/unknown list never
// panics; each list is normalized to an empty slice when nil so an
// explicitly-set-but-empty list clears the corresponding value on TrueNAS
// rather than being omitted.
func (m *SSHConfigModel) updatePayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	p := map[string]any{}

	if !m.BindIface.IsNull() && !m.BindIface.IsUnknown() {
		var v []string
		diags.Append(m.BindIface.ElementsAs(ctx, &v, false)...)
		if v == nil {
			v = []string{}
		}
		p["bindiface"] = v
	}
	if !m.Compression.IsNull() && !m.Compression.IsUnknown() {
		p["compression"] = m.Compression.ValueBool()
	}
	if !m.KerberosAuth.IsNull() && !m.KerberosAuth.IsUnknown() {
		p["kerberosauth"] = m.KerberosAuth.ValueBool()
	}
	if !m.Options.IsNull() && !m.Options.IsUnknown() {
		p["options"] = m.Options.ValueString()
	}
	if !m.PasswordLoginGroups.IsNull() && !m.PasswordLoginGroups.IsUnknown() {
		var v []string
		diags.Append(m.PasswordLoginGroups.ElementsAs(ctx, &v, false)...)
		if v == nil {
			v = []string{}
		}
		p["password_login_groups"] = v
	}
	if !m.PasswordAuth.IsNull() && !m.PasswordAuth.IsUnknown() {
		p["passwordauth"] = m.PasswordAuth.ValueBool()
	}
	if !m.SFTPLogFacility.IsNull() && !m.SFTPLogFacility.IsUnknown() {
		p["sftp_log_facility"] = m.SFTPLogFacility.ValueString()
	}
	if !m.SFTPLogLevel.IsNull() && !m.SFTPLogLevel.IsUnknown() {
		p["sftp_log_level"] = m.SFTPLogLevel.ValueString()
	}
	if !m.TCPFwd.IsNull() && !m.TCPFwd.IsUnknown() {
		p["tcpfwd"] = m.TCPFwd.ValueBool()
	}
	if !m.TCPPort.IsNull() && !m.TCPPort.IsUnknown() {
		p["tcpport"] = m.TCPPort.ValueInt64()
	}
	if !m.WeakCiphers.IsNull() && !m.WeakCiphers.IsUnknown() {
		var v []string
		diags.Append(m.WeakCiphers.ElementsAs(ctx, &v, false)...)
		if v == nil {
			v = []string{}
		}
		p["weak_ciphers"] = v
	}

	return p, diags
}

// deleteWarningDiagnostics builds the warning diagnostic emitted by Delete.
// Delete makes NO client calls: the SSH service configuration is
// system-critical (management access to the box may depend on it), so
// removing this resource from Terraform state must never rewrite the box's
// SSH configuration. Splitting this into its own function keeps Delete's
// "no client calls" contract independently unit-testable.
func deleteWarningDiagnostics() diag.Diagnostics {
	var diags diag.Diagnostics
	diags.AddWarning(
		"SSH configuration left in place",
		"SSH configuration left in place; removed from Terraform state only",
	)
	return diags
}

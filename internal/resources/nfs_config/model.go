// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nfs_config

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// nfsConfigResourceID is the fixed Terraform ID for this singleton resource:
// there is exactly one NFS service configuration per TrueNAS system, and it
// is never created or deleted on TrueNAS itself.
const nfsConfigResourceID = "nfs_config"

// NFSConfigModel is the Terraform state model for truenas_nfs_config.
type NFSConfigModel struct {
	ID              types.String `tfsdk:"id"`      // fixed: "nfs_config"
	Servers         types.Int64  `tfsdk:"servers"` // nullable; API nil -> 0
	AllowNonroot    types.Bool   `tfsdk:"allow_nonroot"`
	Protocols       types.List   `tfsdk:"protocols"` // List[String]: NFSV3, NFSV4
	V4Domain        types.String `tfsdk:"v4_domain"`
	V4Krb           types.Bool   `tfsdk:"v4_krb"`
	BindIP          types.List   `tfsdk:"bindip"`        // List[String]
	MountdPort      types.Int64  `tfsdk:"mountd_port"`   // nullable; API nil -> 0
	RPCStatdPort    types.Int64  `tfsdk:"rpcstatd_port"` // nullable; API nil -> 0
	RPCLockdPort    types.Int64  `tfsdk:"rpclockd_port"` // nullable; API nil -> 0
	MountdLog       types.Bool   `tfsdk:"mountd_log"`
	StatdLockdLog   types.Bool   `tfsdk:"statd_lockd_log"`
	UserdManageGids types.Bool   `tfsdk:"userd_manage_gids"`
	RDMA            types.Bool   `tfsdk:"rdma"`
	ManagedNFSD     types.Bool   `tfsdk:"managed_nfsd"`       // computed-only, never in payload
	V4KrbEnabled    types.Bool   `tfsdk:"v4_krb_enabled"`     // computed-only, never in payload
	KeytabHasNFSSPN types.Bool   `tfsdk:"keytab_has_nfs_spn"` // computed-only, never in payload
}

// NFSConfigDataSourceModel is the read-only model for the truenas_nfs_config
// datasource.
type NFSConfigDataSourceModel struct {
	ID              types.String `tfsdk:"id"`
	Servers         types.Int64  `tfsdk:"servers"`
	AllowNonroot    types.Bool   `tfsdk:"allow_nonroot"`
	Protocols       types.List   `tfsdk:"protocols"`
	V4Domain        types.String `tfsdk:"v4_domain"`
	V4Krb           types.Bool   `tfsdk:"v4_krb"`
	BindIP          types.List   `tfsdk:"bindip"`
	MountdPort      types.Int64  `tfsdk:"mountd_port"`
	RPCStatdPort    types.Int64  `tfsdk:"rpcstatd_port"`
	RPCLockdPort    types.Int64  `tfsdk:"rpclockd_port"`
	MountdLog       types.Bool   `tfsdk:"mountd_log"`
	StatdLockdLog   types.Bool   `tfsdk:"statd_lockd_log"`
	UserdManageGids types.Bool   `tfsdk:"userd_manage_gids"`
	RDMA            types.Bool   `tfsdk:"rdma"`
	ManagedNFSD     types.Bool   `tfsdk:"managed_nfsd"`
	V4KrbEnabled    types.Bool   `tfsdk:"v4_krb_enabled"`
	KeytabHasNFSSPN types.Bool   `tfsdk:"keytab_has_nfs_spn"`
}

// nfsConfigAPI mirrors the JSON object returned by nfs.config and accepted
// (as a subset) by nfs.update. servers/mountd_port/rpcstatd_port/
// rpclockd_port are nullable on the wire (TrueNAS uses JSON null to mean
// "auto"/"default"), so they are modeled as *int64; every other field is
// always present as a concrete value. managed_nfsd, v4_krb_enabled, and
// keytab_has_nfs_spn are server-computed and are never accepted by
// nfs.update.
type nfsConfigAPI struct {
	ID              int64    `json:"id"`
	Servers         *int64   `json:"servers"`
	AllowNonroot    bool     `json:"allow_nonroot"`
	Protocols       []string `json:"protocols"`
	V4Domain        string   `json:"v4_domain"`
	V4Krb           bool     `json:"v4_krb"`
	BindIP          []string `json:"bindip"`
	MountdPort      *int64   `json:"mountd_port"`
	RPCStatdPort    *int64   `json:"rpcstatd_port"`
	RPCLockdPort    *int64   `json:"rpclockd_port"`
	MountdLog       bool     `json:"mountd_log"`
	StatdLockdLog   bool     `json:"statd_lockd_log"`
	UserdManageGids bool     `json:"userd_manage_gids"`
	RDMA            bool     `json:"rdma"`
	ManagedNFSD     bool     `json:"managed_nfsd"`
	V4KrbEnabled    bool     `json:"v4_krb_enabled"`
	KeytabHasNFSSPN bool     `json:"keytab_has_nfs_spn"`
}

// responseToModel maps an API response onto a Terraform model.
func responseToModel(ctx context.Context, api *nfsConfigAPI, m *NFSConfigModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(nfsConfigResourceID)
	m.AllowNonroot = types.BoolValue(api.AllowNonroot)
	m.V4Domain = types.StringValue(api.V4Domain)
	m.V4Krb = types.BoolValue(api.V4Krb)
	m.MountdLog = types.BoolValue(api.MountdLog)
	m.StatdLockdLog = types.BoolValue(api.StatdLockdLog)
	m.UserdManageGids = types.BoolValue(api.UserdManageGids)
	m.RDMA = types.BoolValue(api.RDMA)
	m.ManagedNFSD = types.BoolValue(api.ManagedNFSD)
	m.V4KrbEnabled = types.BoolValue(api.V4KrbEnabled)
	m.KeytabHasNFSSPN = types.BoolValue(api.KeytabHasNFSSPN)

	if api.Servers != nil {
		m.Servers = types.Int64Value(*api.Servers)
	} else {
		m.Servers = types.Int64Value(0)
	}
	if api.MountdPort != nil {
		m.MountdPort = types.Int64Value(*api.MountdPort)
	} else {
		m.MountdPort = types.Int64Value(0)
	}
	if api.RPCStatdPort != nil {
		m.RPCStatdPort = types.Int64Value(*api.RPCStatdPort)
	} else {
		m.RPCStatdPort = types.Int64Value(0)
	}
	if api.RPCLockdPort != nil {
		m.RPCLockdPort = types.Int64Value(*api.RPCLockdPort)
	} else {
		m.RPCLockdPort = types.Int64Value(0)
	}

	protocols := api.Protocols
	if protocols == nil {
		protocols = []string{}
	}
	protocolsList, d := types.ListValueFrom(ctx, types.StringType, protocols)
	diags.Append(d...)
	m.Protocols = protocolsList

	bindip := api.BindIP
	if bindip == nil {
		bindip = []string{}
	}
	bindipList, d := types.ListValueFrom(ctx, types.StringType, bindip)
	diags.Append(d...)
	m.BindIP = bindipList

	return diags
}

// responseToDataSourceModel maps an API response onto a
// NFSConfigDataSourceModel.
func responseToDataSourceModel(ctx context.Context, api *nfsConfigAPI, m *NFSConfigDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(nfsConfigResourceID)
	m.AllowNonroot = types.BoolValue(api.AllowNonroot)
	m.V4Domain = types.StringValue(api.V4Domain)
	m.V4Krb = types.BoolValue(api.V4Krb)
	m.MountdLog = types.BoolValue(api.MountdLog)
	m.StatdLockdLog = types.BoolValue(api.StatdLockdLog)
	m.UserdManageGids = types.BoolValue(api.UserdManageGids)
	m.RDMA = types.BoolValue(api.RDMA)
	m.ManagedNFSD = types.BoolValue(api.ManagedNFSD)
	m.V4KrbEnabled = types.BoolValue(api.V4KrbEnabled)
	m.KeytabHasNFSSPN = types.BoolValue(api.KeytabHasNFSSPN)

	if api.Servers != nil {
		m.Servers = types.Int64Value(*api.Servers)
	} else {
		m.Servers = types.Int64Value(0)
	}
	if api.MountdPort != nil {
		m.MountdPort = types.Int64Value(*api.MountdPort)
	} else {
		m.MountdPort = types.Int64Value(0)
	}
	if api.RPCStatdPort != nil {
		m.RPCStatdPort = types.Int64Value(*api.RPCStatdPort)
	} else {
		m.RPCStatdPort = types.Int64Value(0)
	}
	if api.RPCLockdPort != nil {
		m.RPCLockdPort = types.Int64Value(*api.RPCLockdPort)
	} else {
		m.RPCLockdPort = types.Int64Value(0)
	}

	protocols := api.Protocols
	if protocols == nil {
		protocols = []string{}
	}
	protocolsList, d := types.ListValueFrom(ctx, types.StringType, protocols)
	diags.Append(d...)
	m.Protocols = protocolsList

	bindip := api.BindIP
	if bindip == nil {
		bindip = []string{}
	}
	bindipList, d := types.ListValueFrom(ctx, types.StringType, bindip)
	diags.Append(d...)
	m.BindIP = bindipList

	return diags
}

// updatePayload builds the nfs.update argument. Every writable field is
// guarded: each is only included when known (Optional+Computed).
// managed_nfsd, v4_krb_enabled, and keytab_has_nfs_spn are Computed-only and
// are NEVER included here: they are server-computed values, not something
// nfs.update accepts. servers, mountd_port, rpcstatd_port, and
// rpclockd_port are nullable on the wire: each is omitted entirely when
// null/unknown, sent as JSON nil when the model holds an explicit 0
// (restoring the "auto"/"default" behavior), and sent as its value
// otherwise. protocols and bindip are only included when known, with a nil
// ElementsAs guard so a null/unknown list never panics; each list is
// normalized to an empty slice when nil so an explicitly-set-but-empty list
// clears the corresponding value on TrueNAS rather than being omitted.
func (m *NFSConfigModel) updatePayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	p := map[string]any{}

	if !m.Servers.IsNull() && !m.Servers.IsUnknown() {
		if v := m.Servers.ValueInt64(); v != 0 {
			p["servers"] = v
		} else {
			p["servers"] = nil
		}
	}
	if !m.AllowNonroot.IsNull() && !m.AllowNonroot.IsUnknown() {
		p["allow_nonroot"] = m.AllowNonroot.ValueBool()
	}
	if !m.Protocols.IsNull() && !m.Protocols.IsUnknown() {
		var v []string
		diags.Append(m.Protocols.ElementsAs(ctx, &v, false)...)
		if v == nil {
			v = []string{}
		}
		p["protocols"] = v
	}
	if !m.V4Domain.IsNull() && !m.V4Domain.IsUnknown() {
		p["v4_domain"] = m.V4Domain.ValueString()
	}
	if !m.V4Krb.IsNull() && !m.V4Krb.IsUnknown() {
		p["v4_krb"] = m.V4Krb.ValueBool()
	}
	if !m.BindIP.IsNull() && !m.BindIP.IsUnknown() {
		var v []string
		diags.Append(m.BindIP.ElementsAs(ctx, &v, false)...)
		if v == nil {
			v = []string{}
		}
		p["bindip"] = v
	}
	if !m.MountdPort.IsNull() && !m.MountdPort.IsUnknown() {
		if v := m.MountdPort.ValueInt64(); v != 0 {
			p["mountd_port"] = v
		} else {
			p["mountd_port"] = nil
		}
	}
	if !m.RPCStatdPort.IsNull() && !m.RPCStatdPort.IsUnknown() {
		if v := m.RPCStatdPort.ValueInt64(); v != 0 {
			p["rpcstatd_port"] = v
		} else {
			p["rpcstatd_port"] = nil
		}
	}
	if !m.RPCLockdPort.IsNull() && !m.RPCLockdPort.IsUnknown() {
		if v := m.RPCLockdPort.ValueInt64(); v != 0 {
			p["rpclockd_port"] = v
		} else {
			p["rpclockd_port"] = nil
		}
	}
	if !m.MountdLog.IsNull() && !m.MountdLog.IsUnknown() {
		p["mountd_log"] = m.MountdLog.ValueBool()
	}
	if !m.StatdLockdLog.IsNull() && !m.StatdLockdLog.IsUnknown() {
		p["statd_lockd_log"] = m.StatdLockdLog.ValueBool()
	}
	if !m.UserdManageGids.IsNull() && !m.UserdManageGids.IsUnknown() {
		p["userd_manage_gids"] = m.UserdManageGids.ValueBool()
	}
	if !m.RDMA.IsNull() && !m.RDMA.IsUnknown() {
		p["rdma"] = m.RDMA.ValueBool()
	}

	return p, diags
}

// deleteWarningDiagnostics builds the warning diagnostic emitted by Delete.
// Delete makes NO client calls: the NFS service configuration is
// system-critical (existing NFS shares/exports and clients may depend on
// it), so removing this resource from Terraform state must never rewrite
// the box's NFS configuration. Splitting this into its own function keeps
// Delete's "no client calls" contract independently unit-testable.
func deleteWarningDiagnostics() diag.Diagnostics {
	var diags diag.Diagnostics
	diags.AddWarning(
		"NFS configuration left in place",
		"NFS configuration left in place; removed from Terraform state only",
	)
	return diags
}

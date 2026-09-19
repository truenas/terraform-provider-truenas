// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_global

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// iscsiGlobalResourceID is the fixed Terraform ID for this singleton
// resource: there is exactly one iSCSI global configuration per TrueNAS
// system, and it is never created or deleted on TrueNAS itself.
const iscsiGlobalResourceID = "iscsi_global"

// ISCSIGlobalModel is the Terraform state model for truenas_iscsi_global.
type ISCSIGlobalModel struct {
	ID                 types.String `tfsdk:"id"` // fixed: "iscsi_global"
	Basename           types.String `tfsdk:"basename"`
	ListenPort         types.Int64  `tfsdk:"listen_port"`
	ALUA               types.Bool   `tfsdk:"alua"`
	ISER               types.Bool   `tfsdk:"iser"`
	ISNSServers        types.List   `tfsdk:"isns_servers"`         // List[String]
	PoolAvailThreshold types.Int64  `tfsdk:"pool_avail_threshold"` // nullable; API nil -> 0
}

// ISCSIGlobalDataSourceModel is the read-only model for the
// truenas_iscsi_global datasource.
type ISCSIGlobalDataSourceModel struct {
	ID                 types.String `tfsdk:"id"`
	Basename           types.String `tfsdk:"basename"`
	ListenPort         types.Int64  `tfsdk:"listen_port"`
	ALUA               types.Bool   `tfsdk:"alua"`
	ISER               types.Bool   `tfsdk:"iser"`
	ISNSServers        types.List   `tfsdk:"isns_servers"`
	PoolAvailThreshold types.Int64  `tfsdk:"pool_avail_threshold"`
}

// iscsiGlobalAPI mirrors the JSON object returned by iscsi.global.config and
// accepted (as a subset) by iscsi.global.update.
type iscsiGlobalAPI struct {
	ID                 int64    `json:"id"`
	Basename           string   `json:"basename"`
	ListenPort         int64    `json:"listen_port"`
	ALUA               bool     `json:"alua"`
	ISER               bool     `json:"iser"`
	ISNSServers        []string `json:"isns_servers"`
	PoolAvailThreshold *int64   `json:"pool_avail_threshold"`
}

// responseToModel maps an API response onto a Terraform model.
func responseToModel(ctx context.Context, api *iscsiGlobalAPI, m *ISCSIGlobalModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(iscsiGlobalResourceID)
	m.Basename = types.StringValue(api.Basename)
	m.ListenPort = types.Int64Value(api.ListenPort)
	m.ALUA = types.BoolValue(api.ALUA)
	m.ISER = types.BoolValue(api.ISER)

	servers := api.ISNSServers
	if servers == nil {
		servers = []string{}
	}
	list, d := types.ListValueFrom(ctx, types.StringType, servers)
	diags.Append(d...)
	m.ISNSServers = list

	if api.PoolAvailThreshold != nil {
		m.PoolAvailThreshold = types.Int64Value(*api.PoolAvailThreshold)
	} else {
		m.PoolAvailThreshold = types.Int64Value(0)
	}

	return diags
}

// responseToDataSourceModel maps an API response onto an
// ISCSIGlobalDataSourceModel.
func responseToDataSourceModel(ctx context.Context, api *iscsiGlobalAPI, m *ISCSIGlobalDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(iscsiGlobalResourceID)
	m.Basename = types.StringValue(api.Basename)
	m.ListenPort = types.Int64Value(api.ListenPort)
	m.ALUA = types.BoolValue(api.ALUA)
	m.ISER = types.BoolValue(api.ISER)

	servers := api.ISNSServers
	if servers == nil {
		servers = []string{}
	}
	list, d := types.ListValueFrom(ctx, types.StringType, servers)
	diags.Append(d...)
	m.ISNSServers = list

	if api.PoolAvailThreshold != nil {
		m.PoolAvailThreshold = types.Int64Value(*api.PoolAvailThreshold)
	} else {
		m.PoolAvailThreshold = types.Int64Value(0)
	}

	return diags
}

// updatePayload builds the iscsi.global.update argument. Every field is
// guarded: basename/listen_port/alua/iser are only included when known
// (Optional+Computed). isns_servers is only included when known, with a nil
// (null/unknown) ElementsAs guard so a null/unknown list never panics; the
// list is normalized to an empty slice when nil. pool_avail_threshold is
// nullable: omitted entirely when null/unknown, sent as nil when explicitly
// set to 0 (clearing the threshold), and sent as-is otherwise.
func (m *ISCSIGlobalModel) updatePayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	p := map[string]any{}

	if !m.Basename.IsNull() && !m.Basename.IsUnknown() {
		p["basename"] = m.Basename.ValueString()
	}
	if !m.ListenPort.IsNull() && !m.ListenPort.IsUnknown() {
		p["listen_port"] = m.ListenPort.ValueInt64()
	}
	if !m.ALUA.IsNull() && !m.ALUA.IsUnknown() {
		p["alua"] = m.ALUA.ValueBool()
	}
	if !m.ISER.IsNull() && !m.ISER.IsUnknown() {
		p["iser"] = m.ISER.ValueBool()
	}
	if !m.ISNSServers.IsNull() && !m.ISNSServers.IsUnknown() {
		var servers []string
		diags.Append(m.ISNSServers.ElementsAs(ctx, &servers, false)...)
		if servers == nil {
			servers = []string{}
		}
		p["isns_servers"] = servers
	}
	if !m.PoolAvailThreshold.IsNull() && !m.PoolAvailThreshold.IsUnknown() {
		if v := m.PoolAvailThreshold.ValueInt64(); v != 0 {
			p["pool_avail_threshold"] = v
		} else {
			p["pool_avail_threshold"] = nil
		}
	}

	return p, diags
}

// deleteWarningDiagnostics builds the warning diagnostic emitted by Delete.
// Delete makes NO client calls: the iSCSI global configuration serves live
// storage, so removing this resource from Terraform state must never alter
// the box's iSCSI service configuration. Splitting this into its own
// function keeps Delete's "no client calls" contract independently
// unit-testable.
func deleteWarningDiagnostics() diag.Diagnostics {
	var diags diag.Diagnostics
	diags.AddWarning(
		"iSCSI global configuration left in place",
		"iSCSI global configuration left in place; removed from Terraform state only",
	)
	return diags
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_global

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// nvmetGlobalResourceID is the fixed Terraform ID for this singleton
// resource: there is exactly one NVMe-oF global configuration per TrueNAS
// system, and it is never created or deleted on TrueNAS itself.
const nvmetGlobalResourceID = "nvmet_global"

// NVMeTGlobalModel is the Terraform state model for truenas_nvmet_global.
type NVMeTGlobalModel struct {
	ID            types.String `tfsdk:"id"` // fixed: "nvmet_global"
	Basenqn       types.String `tfsdk:"basenqn"`
	ANA           types.Bool   `tfsdk:"ana"`
	Kernel        types.Bool   `tfsdk:"kernel"`
	RDMA          types.Bool   `tfsdk:"rdma"`
	XportReferral types.Bool   `tfsdk:"xport_referral"`
}

// NVMeTGlobalDataSourceModel is the read-only model for the
// truenas_nvmet_global datasource.
type NVMeTGlobalDataSourceModel struct {
	ID            types.String `tfsdk:"id"`
	Basenqn       types.String `tfsdk:"basenqn"`
	ANA           types.Bool   `tfsdk:"ana"`
	Kernel        types.Bool   `tfsdk:"kernel"`
	RDMA          types.Bool   `tfsdk:"rdma"`
	XportReferral types.Bool   `tfsdk:"xport_referral"`
}

// nvmetGlobalAPI mirrors the JSON object returned by nvmet.global.config and
// accepted (as a subset) by nvmet.global.update.
type nvmetGlobalAPI struct {
	ID            int64  `json:"id"`
	Basenqn       string `json:"basenqn"`
	ANA           bool   `json:"ana"`
	Kernel        bool   `json:"kernel"`
	RDMA          bool   `json:"rdma"`
	XportReferral bool   `json:"xport_referral"`
}

// responseToModel maps an API response onto a Terraform model.
func responseToModel(_ context.Context, api *nvmetGlobalAPI, m *NVMeTGlobalModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(nvmetGlobalResourceID)
	m.Basenqn = types.StringValue(api.Basenqn)
	m.ANA = types.BoolValue(api.ANA)
	m.Kernel = types.BoolValue(api.Kernel)
	m.RDMA = types.BoolValue(api.RDMA)
	m.XportReferral = types.BoolValue(api.XportReferral)

	return diags
}

// responseToDataSourceModel maps an API response onto an
// NVMeTGlobalDataSourceModel.
func responseToDataSourceModel(_ context.Context, api *nvmetGlobalAPI, m *NVMeTGlobalDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(nvmetGlobalResourceID)
	m.Basenqn = types.StringValue(api.Basenqn)
	m.ANA = types.BoolValue(api.ANA)
	m.Kernel = types.BoolValue(api.Kernel)
	m.RDMA = types.BoolValue(api.RDMA)
	m.XportReferral = types.BoolValue(api.XportReferral)

	return diags
}

// updatePayload builds the nvmet.global.update argument. Every field is
// guarded: basenqn/ana/kernel/rdma/xport_referral are only included when
// known (Optional+Computed).
func (m *NVMeTGlobalModel) updatePayload(_ context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	p := map[string]any{}

	if !m.Basenqn.IsNull() && !m.Basenqn.IsUnknown() {
		p["basenqn"] = m.Basenqn.ValueString()
	}
	if !m.ANA.IsNull() && !m.ANA.IsUnknown() {
		p["ana"] = m.ANA.ValueBool()
	}
	if !m.Kernel.IsNull() && !m.Kernel.IsUnknown() {
		p["kernel"] = m.Kernel.ValueBool()
	}
	if !m.RDMA.IsNull() && !m.RDMA.IsUnknown() {
		p["rdma"] = m.RDMA.ValueBool()
	}
	if !m.XportReferral.IsNull() && !m.XportReferral.IsUnknown() {
		p["xport_referral"] = m.XportReferral.ValueBool()
	}

	return p, diags
}

// deleteWarningDiagnostics builds the warning diagnostic emitted by Delete.
// Delete makes NO client calls: the NVMe-oF global configuration serves live
// storage, so removing this resource from Terraform state must never alter
// the box's NVMe-oF service configuration. Splitting this into its own
// function keeps Delete's "no client calls" contract independently
// unit-testable.
func deleteWarningDiagnostics() diag.Diagnostics {
	var diags diag.Diagnostics
	diags.AddWarning(
		"NVMe-oF global configuration left in place",
		"NVMe-oF global configuration left in place; removed from Terraform state only",
	)
	return diags
}

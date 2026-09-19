// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package system_dataset

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// systemDatasetResourceID is the fixed Terraform ID for this singleton
// resource: there is exactly one system dataset configuration per TrueNAS
// system, and it is never created or deleted on TrueNAS itself.
const systemDatasetResourceID = "system_dataset"

// SystemDatasetModel is the Terraform state model for
// truenas_system_dataset.
//
// PoolExclude is a write-only migration argument: TrueNAS never returns it
// on systemdataset.config, so it is Optional only (never Computed) and has
// no corresponding field in systemDatasetAPI at all, so it cannot be decoded
// from a response even by accident.
type SystemDatasetModel struct {
	ID          types.String `tfsdk:"id"` // fixed: "system_dataset"
	Pool        types.String `tfsdk:"pool"`
	PoolExclude types.String `tfsdk:"pool_exclude"` // write-only, Optional only

	// Computed-only: never sent to systemdataset.update.
	Basename types.String `tfsdk:"basename"` // NO plan modifier: changes with pool
	Path     types.String `tfsdk:"path"`     // NO plan modifier
	UUID     types.String `tfsdk:"uuid"`     // +UseStateForUnknown
	PoolSet  types.Bool   `tfsdk:"pool_set"` // NO plan modifier
}

// SystemDatasetDataSourceModel is the read-only model for the
// truenas_system_dataset datasource. It has no pool_exclude field: the API
// never returns it, so a datasource attribute for it would always read as
// empty/unknown.
type SystemDatasetDataSourceModel struct {
	ID       types.String `tfsdk:"id"`
	Pool     types.String `tfsdk:"pool"`
	Basename types.String `tfsdk:"basename"`
	Path     types.String `tfsdk:"path"`
	UUID     types.String `tfsdk:"uuid"`
	PoolSet  types.Bool   `tfsdk:"pool_set"`
}

// systemDatasetAPI mirrors the JSON object returned by
// systemdataset.config and accepted (as a subset) by systemdataset.update.
// pool_exclude deliberately has NO field here: TrueNAS never returns a
// usable value for it, so it must never be decodable from a response.
type systemDatasetAPI struct {
	ID       int64  `json:"id"`
	Basename string `json:"basename"`
	Path     string `json:"path"`
	Pool     string `json:"pool"`
	PoolSet  bool   `json:"pool_set"`
	UUID     string `json:"uuid"`
}

// responseToModel maps an API response onto a Terraform model. PoolExclude
// is NOT set here (write-only, never stored in state from a response;
// whatever value came from the plan is left untouched by the caller).
func responseToModel(_ context.Context, api *systemDatasetAPI, m *SystemDatasetModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(systemDatasetResourceID)
	m.Pool = types.StringValue(api.Pool)
	m.Basename = types.StringValue(api.Basename)
	m.Path = types.StringValue(api.Path)
	m.UUID = types.StringValue(api.UUID)
	m.PoolSet = types.BoolValue(api.PoolSet)

	// NOTE: PoolExclude is NOT set here (write-only).

	return diags
}

// responseToDataSourceModel maps an API response onto a
// SystemDatasetDataSourceModel using the same rules as responseToModel.
func responseToDataSourceModel(_ context.Context, api *systemDatasetAPI, m *SystemDatasetDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(systemDatasetResourceID)
	m.Pool = types.StringValue(api.Pool)
	m.Basename = types.StringValue(api.Basename)
	m.Path = types.StringValue(api.Path)
	m.UUID = types.StringValue(api.UUID)
	m.PoolSet = types.BoolValue(api.PoolSet)

	return diags
}

// updatePayload builds the systemdataset.update argument. pool is guarded:
// it is only included when known (Optional+Computed). pool_exclude is
// guarded the same way but is Optional ONLY (never Computed): it is a
// write-only migration argument accepted by systemdataset.update to control
// which pool(s) are excluded from being chosen as the new system dataset
// pool, and is only included in the payload when the caller has explicitly
// set it. basename, path, uuid, and pool_set are Computed-only server-
// generated values and are NEVER included here: systemdataset.update does
// not accept them.
func (m *SystemDatasetModel) updatePayload(_ context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	p := map[string]any{}

	if !m.Pool.IsNull() && !m.Pool.IsUnknown() {
		p["pool"] = m.Pool.ValueString()
	}
	if !m.PoolExclude.IsNull() && !m.PoolExclude.IsUnknown() {
		p["pool_exclude"] = m.PoolExclude.ValueString()
	}

	return p, diags
}

// deleteWarningDiagnostics builds the warning diagnostic emitted by Delete.
// Delete makes NO client calls: the system dataset holds core system state
// (logs, reporting, syslog, samba4, etc.), so removing this resource from
// Terraform state must never rewrite the box's configuration or trigger a
// pool migration. Splitting this into its own function keeps Delete's "no
// client calls" contract independently unit-testable.
func deleteWarningDiagnostics() diag.Diagnostics {
	var diags diag.Diagnostics
	diags.AddWarning(
		"System dataset configuration left in place",
		"System dataset configuration left in place; removed from Terraform state only",
	)
	return diags
}

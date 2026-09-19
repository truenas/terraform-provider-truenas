// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package resilver_config

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// resilverConfigResourceID is the fixed Terraform ID for this singleton
// resource: there is exactly one pool resilver schedule per TrueNAS system
// (pool.resilver.config always returns a single record), and it is never
// created or deleted on TrueNAS itself. The API's own numeric "id" (probed:
// always 1) is an internal implementation detail and is intentionally not
// surfaced in the model, mirroring the snmp_config/network_config singleton
// pattern.
const resilverConfigResourceID = "resilver_config"

// ResilverConfigModel is the Terraform state model for
// truenas_resilver_config.
type ResilverConfigModel struct {
	ID      types.String `tfsdk:"id"` // fixed: "resilver_config"
	Begin   types.String `tfsdk:"begin"`
	End     types.String `tfsdk:"end"`
	Enabled types.Bool   `tfsdk:"enabled"`
	Weekday types.List   `tfsdk:"weekday"` // List[Int64], 1 (Mon) - 7 (Sun)
}

// ResilverConfigDataSourceModel is the read-only model for the
// truenas_resilver_config datasource.
type ResilverConfigDataSourceModel struct {
	ID      types.String `tfsdk:"id"`
	Begin   types.String `tfsdk:"begin"`
	End     types.String `tfsdk:"end"`
	Enabled types.Bool   `tfsdk:"enabled"`
	Weekday types.List   `tfsdk:"weekday"`
}

// resilverConfigAPI mirrors the JSON object returned by pool.resilver.config
// and pool.resilver.update.
type resilverConfigAPI struct {
	ID      int64   `json:"id"`
	Begin   string  `json:"begin"`
	End     string  `json:"end"`
	Enabled bool    `json:"enabled"`
	Weekday []int64 `json:"weekday"`
}

// responseToModel maps a resilverConfigAPI response onto a
// ResilverConfigModel.
func responseToModel(ctx context.Context, api *resilverConfigAPI, m *ResilverConfigModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(resilverConfigResourceID)
	m.Begin = types.StringValue(api.Begin)
	m.End = types.StringValue(api.End)
	m.Enabled = types.BoolValue(api.Enabled)

	weekday := api.Weekday
	if weekday == nil {
		weekday = []int64{}
	}
	weekdayList, d := types.ListValueFrom(ctx, types.Int64Type, weekday)
	diags.Append(d...)
	m.Weekday = weekdayList

	return diags
}

// responseToDataSourceModel maps a resilverConfigAPI response onto a
// ResilverConfigDataSourceModel.
func responseToDataSourceModel(ctx context.Context, api *resilverConfigAPI, m *ResilverConfigDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(resilverConfigResourceID)
	m.Begin = types.StringValue(api.Begin)
	m.End = types.StringValue(api.End)
	m.Enabled = types.BoolValue(api.Enabled)

	weekday := api.Weekday
	if weekday == nil {
		weekday = []int64{}
	}
	weekdayList, d := types.ListValueFrom(ctx, types.Int64Type, weekday)
	diags.Append(d...)
	m.Weekday = weekdayList

	return diags
}

// updatePayload builds the pool.resilver.update argument. Every field is
// guarded: each is only included when known (Optional+Computed), so an
// unset optional is omitted entirely and the TrueNAS-side current value is
// left unchanged rather than overwritten with an explicit zero value.
func (m *ResilverConfigModel) updatePayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	p := map[string]any{}

	if !m.Begin.IsNull() && !m.Begin.IsUnknown() {
		p["begin"] = m.Begin.ValueString()
	}
	if !m.End.IsNull() && !m.End.IsUnknown() {
		p["end"] = m.End.ValueString()
	}
	if !m.Enabled.IsNull() && !m.Enabled.IsUnknown() {
		p["enabled"] = m.Enabled.ValueBool()
	}
	if !m.Weekday.IsNull() && !m.Weekday.IsUnknown() {
		var v []int64
		diags.Append(m.Weekday.ElementsAs(ctx, &v, false)...)
		if v == nil {
			v = []int64{}
		}
		p["weekday"] = v
	}

	return p, diags
}

// deleteWarningDiagnostics builds the warning diagnostic emitted by Delete.
// Delete makes NO client calls: the resilver schedule is system-critical
// pool maintenance configuration, so removing this resource from Terraform
// state must never reset the box's resilver schedule. Splitting this into
// its own function keeps Delete's "no client calls" contract independently
// unit-testable.
func deleteWarningDiagnostics() diag.Diagnostics {
	var diags diag.Diagnostics
	diags.AddWarning(
		"Resilver configuration left in place",
		"Resilver configuration left in place; removed from Terraform state only",
	)
	return diags
}

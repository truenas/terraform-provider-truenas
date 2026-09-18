// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package scrub_task

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// scheduleAttrTypes describes the attribute types of the nested "schedule"
// object.
var scheduleAttrTypes = map[string]attr.Type{
	"minute": types.StringType,
	"hour":   types.StringType,
	"dom":    types.StringType,
	"month":  types.StringType,
	"dow":    types.StringType,
}

// ScheduleModel maps to the nested "schedule" attribute.
type ScheduleModel struct {
	Minute types.String `tfsdk:"minute"`
	Hour   types.String `tfsdk:"hour"`
	Dom    types.String `tfsdk:"dom"`
	Month  types.String `tfsdk:"month"`
	Dow    types.String `tfsdk:"dow"`
}

// ScrubTaskModel is the Terraform state/plan model for truenas_scrub_task.
type ScrubTaskModel struct {
	ID          types.Int64  `tfsdk:"id"`
	Pool        types.Int64  `tfsdk:"pool"`
	PoolName    types.String `tfsdk:"pool_name"`
	Threshold   types.Int64  `tfsdk:"threshold"`
	Description types.String `tfsdk:"description"`
	Schedule    types.Object `tfsdk:"schedule"`
	Enabled     types.Bool   `tfsdk:"enabled"`
}

// ScrubTaskDataSourceModel is the read-only model for the truenas_scrub_task
// datasource, looked up by "pool".
type ScrubTaskDataSourceModel struct {
	ID          types.Int64  `tfsdk:"id"`
	Pool        types.Int64  `tfsdk:"pool"`
	PoolName    types.String `tfsdk:"pool_name"`
	Threshold   types.Int64  `tfsdk:"threshold"`
	Description types.String `tfsdk:"description"`
	Schedule    types.Object `tfsdk:"schedule"`
	Enabled     types.Bool   `tfsdk:"enabled"`
}

// scheduleAPI is the JSON wire format of the nested "schedule" object
// returned/accepted by pool.scrub.* methods.
type scheduleAPI struct {
	Minute string `json:"minute"`
	Hour   string `json:"hour"`
	Dom    string `json:"dom"`
	Month  string `json:"month"`
	Dow    string `json:"dow"`
}

// scrubTaskAPI mirrors the JSON object returned by pool.scrub.create,
// pool.scrub.update, pool.scrub.get_instance, and pool.scrub.query. Probed
// against a live TrueNAS 25.10 box: "pool" comes back as a plain
// integer (the pool id), never an embedded object, and the response always
// includes a "pool_name" string alongside it.
type scrubTaskAPI struct {
	ID          int64       `json:"id"`
	Pool        int64       `json:"pool"`
	PoolName    string      `json:"pool_name"`
	Threshold   int64       `json:"threshold"`
	Description string      `json:"description"`
	Schedule    scheduleAPI `json:"schedule"`
	Enabled     bool        `json:"enabled"`
}

// scheduleObjectValue builds a types.Object for the nested "schedule"
// attribute from an API response.
func scheduleObjectValue(ctx context.Context, api scheduleAPI) (types.Object, diag.Diagnostics) {
	return types.ObjectValueFrom(ctx, scheduleAttrTypes, ScheduleModel{
		Minute: types.StringValue(api.Minute),
		Hour:   types.StringValue(api.Hour),
		Dom:    types.StringValue(api.Dom),
		Month:  types.StringValue(api.Month),
		Dow:    types.StringValue(api.Dow),
	})
}

// responseToModel maps a scrubTaskAPI response onto a ScrubTaskModel.
func responseToModel(ctx context.Context, api *scrubTaskAPI, m *ScrubTaskModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.Pool = types.Int64Value(api.Pool)
	m.PoolName = types.StringValue(api.PoolName)
	m.Threshold = types.Int64Value(api.Threshold)
	m.Description = types.StringValue(api.Description)
	m.Enabled = types.BoolValue(api.Enabled)

	sched, d := scheduleObjectValue(ctx, api.Schedule)
	diags.Append(d...)
	m.Schedule = sched

	return diags
}

// responseToDataSourceModel maps a scrubTaskAPI response onto a
// ScrubTaskDataSourceModel.
func responseToDataSourceModel(ctx context.Context, api *scrubTaskAPI, m *ScrubTaskDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.Pool = types.Int64Value(api.Pool)
	m.PoolName = types.StringValue(api.PoolName)
	m.Threshold = types.Int64Value(api.Threshold)
	m.Description = types.StringValue(api.Description)
	m.Enabled = types.BoolValue(api.Enabled)

	sched, d := scheduleObjectValue(ctx, api.Schedule)
	diags.Append(d...)
	m.Schedule = sched

	return diags
}

// apiPayload builds the map expected by pool.scrub.create/pool.scrub.update.
// "pool" is always included (Required). threshold, description, enabled,
// and schedule are Optional+Computed: each is included only when known and
// non-null, so an unset optional is omitted entirely and the TrueNAS-side
// default takes effect instead of an explicit zero value.
func (m *ScrubTaskModel) apiPayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	p := map[string]any{
		"pool": m.Pool.ValueInt64(),
	}

	if !m.Threshold.IsNull() && !m.Threshold.IsUnknown() {
		p["threshold"] = m.Threshold.ValueInt64()
	}
	if !m.Description.IsNull() && !m.Description.IsUnknown() {
		p["description"] = m.Description.ValueString()
	}
	if !m.Enabled.IsNull() && !m.Enabled.IsUnknown() {
		p["enabled"] = m.Enabled.ValueBool()
	}
	if !m.Schedule.IsNull() && !m.Schedule.IsUnknown() {
		var sched ScheduleModel
		diags.Append(m.Schedule.As(ctx, &sched, basetypes.ObjectAsOptions{})...)
		p["schedule"] = map[string]string{
			"minute": sched.Minute.ValueString(),
			"hour":   sched.Hour.ValueString(),
			"dom":    sched.Dom.ValueString(),
			"month":  sched.Month.ValueString(),
			"dow":    sched.Dow.ValueString(),
		}
	}

	return p, diags
}

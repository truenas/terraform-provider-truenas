// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package periodic_snapshot

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ScheduleModel maps to the nested "schedule" attribute.
type ScheduleModel struct {
	Minute types.String `tfsdk:"minute"`
	Hour   types.String `tfsdk:"hour"`
	Dom    types.String `tfsdk:"dom"`
	Month  types.String `tfsdk:"month"`
	Dow    types.String `tfsdk:"dow"`
}

// PeriodicSnapshotModel is the Terraform state/plan model for a periodic snapshot task.
type PeriodicSnapshotModel struct {
	ID            types.Int64   `tfsdk:"id"`
	Dataset       types.String  `tfsdk:"dataset"`
	Recursive     types.Bool    `tfsdk:"recursive"`
	Exclude       types.List    `tfsdk:"exclude"` // List[String]
	LifetimeValue types.Int64   `tfsdk:"lifetime_value"`
	LifetimeUnit  types.String  `tfsdk:"lifetime_unit"` // HOUR, DAY, WEEK, MONTH, YEAR
	NamingSchema  types.String  `tfsdk:"naming_schema"`
	Schedule      ScheduleModel `tfsdk:"schedule"`
	AllowEmpty    types.Bool    `tfsdk:"allow_empty"`
	Enabled       types.Bool    `tfsdk:"enabled"`
}

// taskAPI is the JSON shape returned by pool.snapshottask.* methods.
type taskAPI struct {
	ID            int64    `json:"id"`
	Dataset       string   `json:"dataset"`
	Recursive     bool     `json:"recursive"`
	Exclude       []string `json:"exclude"`
	LifetimeValue int64    `json:"lifetime_value"`
	LifetimeUnit  string   `json:"lifetime_unit"`
	NamingSchema  string   `json:"naming_schema"`
	Schedule      struct {
		Minute string `json:"minute"`
		Hour   string `json:"hour"`
		Dom    string `json:"dom"`
		Month  string `json:"month"`
		Dow    string `json:"dow"`
	} `json:"schedule"`
	AllowEmpty bool `json:"allow_empty"`
	Enabled    bool `json:"enabled"`
}

// apiPayload converts the Terraform model into the map expected by create/update.
func (m *PeriodicSnapshotModel) apiPayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	var exclude []string
	if !m.Exclude.IsNull() && !m.Exclude.IsUnknown() {
		diags.Append(m.Exclude.ElementsAs(ctx, &exclude, false)...)
	}
	if exclude == nil {
		exclude = []string{}
	}
	p := map[string]any{
		"dataset":        m.Dataset.ValueString(),
		"recursive":      m.Recursive.ValueBool(),
		"exclude":        exclude,
		"lifetime_value": m.LifetimeValue.ValueInt64(),
		"lifetime_unit":  m.LifetimeUnit.ValueString(),
		"naming_schema":  m.NamingSchema.ValueString(),
		"schedule": map[string]string{
			"minute": m.Schedule.Minute.ValueString(),
			"hour":   m.Schedule.Hour.ValueString(),
			"dom":    m.Schedule.Dom.ValueString(),
			"month":  m.Schedule.Month.ValueString(),
			"dow":    m.Schedule.Dow.ValueString(),
		},
		"allow_empty": m.AllowEmpty.ValueBool(),
		"enabled":     m.Enabled.ValueBool(),
	}
	return p, diags
}

// responseToModel maps a taskAPI response into a PeriodicSnapshotModel.
func responseToModel(ctx context.Context, api *taskAPI, m *PeriodicSnapshotModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.Int64Value(api.ID)
	m.Dataset = types.StringValue(api.Dataset)
	m.Recursive = types.BoolValue(api.Recursive)
	excludeList, d := types.ListValueFrom(ctx, types.StringType, api.Exclude)
	diags.Append(d...)
	m.Exclude = excludeList
	m.LifetimeValue = types.Int64Value(api.LifetimeValue)
	m.LifetimeUnit = types.StringValue(api.LifetimeUnit)
	m.NamingSchema = types.StringValue(api.NamingSchema)
	m.Schedule = ScheduleModel{
		Minute: types.StringValue(api.Schedule.Minute),
		Hour:   types.StringValue(api.Schedule.Hour),
		Dom:    types.StringValue(api.Schedule.Dom),
		Month:  types.StringValue(api.Schedule.Month),
		Dow:    types.StringValue(api.Schedule.Dow),
	}
	m.AllowEmpty = types.BoolValue(api.AllowEmpty)
	m.Enabled = types.BoolValue(api.Enabled)
	return diags
}

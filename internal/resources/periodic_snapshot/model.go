// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package periodic_snapshot

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// scheduleAttrTypes describes the attribute types of the nested "schedule"
// object. The model holds schedule as a types.Object, not a plain struct: the
// model is shared with the data source, whose schedule is Computed (null in
// config), and a plain struct cannot hold null, so every data source read
// failed with a Value Conversion Error.
var scheduleAttrTypes = map[string]attr.Type{
	"minute": types.StringType,
	"hour":   types.StringType,
	"dom":    types.StringType,
	"month":  types.StringType,
	"dow":    types.StringType,
	"begin":  types.StringType,
	"end":    types.StringType,
}

// ScheduleModel maps to the nested "schedule" attribute. begin/end are the
// daily window in which snapshots are taken.
type ScheduleModel struct {
	Minute types.String `tfsdk:"minute"`
	Hour   types.String `tfsdk:"hour"`
	Dom    types.String `tfsdk:"dom"`
	Month  types.String `tfsdk:"month"`
	Dow    types.String `tfsdk:"dow"`
	Begin  types.String `tfsdk:"begin"`
	End    types.String `tfsdk:"end"`
}

// PeriodicSnapshotModel is the Terraform state/plan model for a periodic snapshot task.
type PeriodicSnapshotModel struct {
	ID            types.Int64  `tfsdk:"id"`
	Dataset       types.String `tfsdk:"dataset"`
	Recursive     types.Bool   `tfsdk:"recursive"`
	Exclude       types.List   `tfsdk:"exclude"` // List[String]
	LifetimeValue types.Int64  `tfsdk:"lifetime_value"`
	LifetimeUnit  types.String `tfsdk:"lifetime_unit"` // HOUR, DAY, WEEK, MONTH, YEAR
	NamingSchema  types.String `tfsdk:"naming_schema"`
	Schedule      types.Object `tfsdk:"schedule"` // ScheduleModel
	AllowEmpty    types.Bool   `tfsdk:"allow_empty"`
	Enabled       types.Bool   `tfsdk:"enabled"`
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
		Begin  string `json:"begin"`
		End    string `json:"end"`
	} `json:"schedule"`
	AllowEmpty bool `json:"allow_empty"`
	Enabled    bool `json:"enabled"`
}

// apiPayload converts the Terraform model into the map expected by create/update.
//
// Optional+Computed fields are sent only when known. Left unset, their plan
// value is unknown on create, and ValueBool() would send false: enabled=false
// and allow_empty=false created a task that never took a snapshot. Omitted,
// the server defaults apply (enabled and allow_empty true).
//
// The server replaces the whole schedule object, filling any key that is not
// sent with its default, so begin/end must always go out with the cron fields
// once known; otherwise every update reset the window to 00:00-23:59.
func (m *PeriodicSnapshotModel) apiPayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	var sched ScheduleModel
	diags.Append(m.Schedule.As(ctx, &sched, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return nil, diags
	}
	schedule := map[string]string{
		"minute": sched.Minute.ValueString(),
		"hour":   sched.Hour.ValueString(),
		"dom":    sched.Dom.ValueString(),
		"month":  sched.Month.ValueString(),
		"dow":    sched.Dow.ValueString(),
	}
	if !sched.Begin.IsNull() && !sched.Begin.IsUnknown() {
		schedule["begin"] = sched.Begin.ValueString()
	}
	if !sched.End.IsNull() && !sched.End.IsUnknown() {
		schedule["end"] = sched.End.ValueString()
	}
	p := map[string]any{
		"dataset":        m.Dataset.ValueString(),
		"lifetime_value": m.LifetimeValue.ValueInt64(),
		"lifetime_unit":  m.LifetimeUnit.ValueString(),
		"naming_schema":  m.NamingSchema.ValueString(),
		"schedule":       schedule,
	}
	if !m.Exclude.IsNull() && !m.Exclude.IsUnknown() {
		var exclude []string
		diags.Append(m.Exclude.ElementsAs(ctx, &exclude, false)...)
		if exclude == nil {
			exclude = []string{}
		}
		p["exclude"] = exclude
	}
	if !m.Recursive.IsNull() && !m.Recursive.IsUnknown() {
		p["recursive"] = m.Recursive.ValueBool()
	}
	if !m.AllowEmpty.IsNull() && !m.AllowEmpty.IsUnknown() {
		p["allow_empty"] = m.AllowEmpty.ValueBool()
	}
	if !m.Enabled.IsNull() && !m.Enabled.IsUnknown() {
		p["enabled"] = m.Enabled.ValueBool()
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
	sched, d := types.ObjectValueFrom(ctx, scheduleAttrTypes, ScheduleModel{
		Minute: types.StringValue(api.Schedule.Minute),
		Hour:   types.StringValue(api.Schedule.Hour),
		Dom:    types.StringValue(api.Schedule.Dom),
		Month:  types.StringValue(api.Schedule.Month),
		Dow:    types.StringValue(api.Schedule.Dow),
		Begin:  types.StringValue(api.Schedule.Begin),
		End:    types.StringValue(api.Schedule.End),
	})
	diags.Append(d...)
	m.Schedule = sched
	m.AllowEmpty = types.BoolValue(api.AllowEmpty)
	m.Enabled = types.BoolValue(api.Enabled)
	return diags
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cronjob

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

// CronjobModel is the Terraform state/plan model for truenas_cronjob.
type CronjobModel struct {
	ID          types.Int64  `tfsdk:"id"`
	Command     types.String `tfsdk:"command"`
	User        types.String `tfsdk:"user"`
	Description types.String `tfsdk:"description"`
	Schedule    types.Object `tfsdk:"schedule"`
	Enabled     types.Bool   `tfsdk:"enabled"`
	Stdout      types.Bool   `tfsdk:"stdout"`
	Stderr      types.Bool   `tfsdk:"stderr"`
}

// CronjobDataSourceModel is the read-only model for the truenas_cronjob
// datasource, looked up by "description".
type CronjobDataSourceModel struct {
	ID          types.Int64  `tfsdk:"id"`
	Command     types.String `tfsdk:"command"`
	User        types.String `tfsdk:"user"`
	Description types.String `tfsdk:"description"`
	Schedule    types.Object `tfsdk:"schedule"`
	Enabled     types.Bool   `tfsdk:"enabled"`
	Stdout      types.Bool   `tfsdk:"stdout"`
	Stderr      types.Bool   `tfsdk:"stderr"`
}

// scheduleAPI is the JSON wire format of the nested "schedule" object
// returned/accepted by cronjob.* methods.
type scheduleAPI struct {
	Minute string `json:"minute"`
	Hour   string `json:"hour"`
	Dom    string `json:"dom"`
	Month  string `json:"month"`
	Dow    string `json:"dow"`
}

// cronjobAPI mirrors the JSON object returned by cronjob.create,
// cronjob.update, cronjob.get_instance, and cronjob.query. Probed against
// live TrueNAS 25.10 and 26.0 boxes (identical wire shape on both
// releases, no version gating needed): the response is exactly these eight
// fields, no nullable strings/ints and no embedded runtime-status objects
// (unlike rsynctask's "locked"/"job"). "stdout"/"stderr" are IGNORE flags
// per the API's own doc string: stdout=true (the default) means standard
// output is suppressed/not emailed; stderr=false (the default) means
// standard error IS emailed. cronjob.run is a job:true method (probed via
// core.get_methods) used to manually trigger a run; this provider does not
// expose it, matching the brief.
type cronjobAPI struct {
	ID          int64       `json:"id"`
	Command     string      `json:"command"`
	User        string      `json:"user"`
	Description string      `json:"description"`
	Schedule    scheduleAPI `json:"schedule"`
	Enabled     bool        `json:"enabled"`
	Stdout      bool        `json:"stdout"`
	Stderr      bool        `json:"stderr"`
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

// responseToModel maps a cronjobAPI response onto a CronjobModel.
func responseToModel(ctx context.Context, api *cronjobAPI, m *CronjobModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.Command = types.StringValue(api.Command)
	m.User = types.StringValue(api.User)
	m.Description = types.StringValue(api.Description)

	sched, d := scheduleObjectValue(ctx, api.Schedule)
	diags.Append(d...)
	m.Schedule = sched

	m.Enabled = types.BoolValue(api.Enabled)
	m.Stdout = types.BoolValue(api.Stdout)
	m.Stderr = types.BoolValue(api.Stderr)

	return diags
}

// responseToDataSourceModel maps a cronjobAPI response onto a
// CronjobDataSourceModel.
func responseToDataSourceModel(ctx context.Context, api *cronjobAPI, m *CronjobDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.Command = types.StringValue(api.Command)
	m.User = types.StringValue(api.User)
	m.Description = types.StringValue(api.Description)

	sched, d := scheduleObjectValue(ctx, api.Schedule)
	diags.Append(d...)
	m.Schedule = sched

	m.Enabled = types.BoolValue(api.Enabled)
	m.Stdout = types.BoolValue(api.Stdout)
	m.Stderr = types.BoolValue(api.Stderr)

	return diags
}

// apiPayload builds the map expected by cronjob.create/cronjob.update.
// "command" and "user" are always included (Required). Every other field is
// Optional+Computed: each is included only when known and non-null, so an
// unset optional is omitted entirely and the TrueNAS-side default takes
// effect instead of an explicit zero value.
func (m *CronjobModel) apiPayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	p := map[string]any{
		"command": m.Command.ValueString(),
		"user":    m.User.ValueString(),
	}

	if !m.Description.IsNull() && !m.Description.IsUnknown() {
		p["description"] = m.Description.ValueString()
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
	if !m.Enabled.IsNull() && !m.Enabled.IsUnknown() {
		p["enabled"] = m.Enabled.ValueBool()
	}
	if !m.Stdout.IsNull() && !m.Stdout.IsUnknown() {
		p["stdout"] = m.Stdout.ValueBool()
	}
	if !m.Stderr.IsNull() && !m.Stderr.IsUnknown() {
		p["stderr"] = m.Stderr.ValueBool()
	}

	return p, diags
}

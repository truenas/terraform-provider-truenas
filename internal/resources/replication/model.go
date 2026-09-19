// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package replication

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// scheduleAttrTypes describes the attribute types of the nested "schedule" object.
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

// ReplicationModel is the Terraform state/plan model for truenas_replication_task.
type ReplicationModel struct {
	ID             types.Int64  `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Direction      types.String `tfsdk:"direction"`       // PUSH, PULL
	Transport      types.String `tfsdk:"transport"`       // SSH, SSH+NETCAT, LOCAL
	SSHCredentials types.Int64  `tfsdk:"ssh_credentials"` // keychain credential id; 0 = unset (LOCAL)
	Sudo           types.Bool   `tfsdk:"sudo"`
	Compression    types.String `tfsdk:"compression"` // LZ4, PIGZ, PLZIP; null unless transport = SSH
	SpeedLimit     types.Int64  `tfsdk:"speed_limit"` // bytes/sec; null unless transport = SSH
	// netcat_* fields apply only to transport = SSH+NETCAT; null otherwise.
	NetcatActiveSide                types.String `tfsdk:"netcat_active_side"`                  // LOCAL, REMOTE
	NetcatActiveSideListenAddress   types.String `tfsdk:"netcat_active_side_listen_address"`   // IP the active side listens on
	NetcatActiveSidePortMin         types.Int64  `tfsdk:"netcat_active_side_port_min"`         // 1-65535
	NetcatActiveSidePortMax         types.Int64  `tfsdk:"netcat_active_side_port_max"`         // 1-65535
	NetcatPassiveSideConnectAddress types.String `tfsdk:"netcat_passive_side_connect_address"` // IP the passive side connects to
	SourceDatasets                  types.List   `tfsdk:"source_datasets"`                     // List[String], Required
	TargetDataset                   types.String `tfsdk:"target_dataset"`
	Recursive                       types.Bool   `tfsdk:"recursive"`
	Exclude                         types.List   `tfsdk:"exclude"`
	Properties                      types.Bool   `tfsdk:"properties"`
	Replicate                       types.Bool   `tfsdk:"replicate"`
	PeriodicSnapshotTasks           types.List   `tfsdk:"periodic_snapshot_tasks"` // List[Int64]
	NamingSchema                    types.List   `tfsdk:"naming_schema"`           // List[String]
	AlsoIncludeNamingSchema         types.List   `tfsdk:"also_include_naming_schema"`
	NameRegex                       types.String `tfsdk:"name_regex"` // "" = unset -> null
	Auto                            types.Bool   `tfsdk:"auto"`
	Schedule                        types.Object `tfsdk:"schedule"`         // optional nested; null when unset
	RetentionPolicy                 types.String `tfsdk:"retention_policy"` // SOURCE, CUSTOM, NONE
	LifetimeValue                   types.Int64  `tfsdk:"lifetime_value"`   // 0 = unset -> null
	LifetimeUnit                    types.String `tfsdk:"lifetime_unit"`    // "" = unset -> null
	Readonly                        types.String `tfsdk:"readonly"`         // SET, REQUIRE, IGNORE
	Enabled                         types.Bool   `tfsdk:"enabled"`
	Retries                         types.Int64  `tfsdk:"retries"`
}

// embeddedTask is the shape of an embedded periodic snapshot task object
// as returned by replication.query/get_instance.
type embeddedTask struct {
	ID int64 `json:"id"`
}

// replicationAPI is the JSON wire format for a TrueNAS replication task object.
type replicationAPI struct {
	ID                              int64          `json:"id"`
	Name                            string         `json:"name"`
	Direction                       string         `json:"direction"`
	Transport                       string         `json:"transport"`
	SSHCredentials                  any            `json:"ssh_credentials"` // null, int, or embedded object {id: int}
	Sudo                            bool           `json:"sudo"`
	Compression                     *string        `json:"compression"`
	SpeedLimit                      *int64         `json:"speed_limit"`
	NetcatActiveSide                *string        `json:"netcat_active_side"`
	NetcatActiveSideListenAddress   *string        `json:"netcat_active_side_listen_address"`
	NetcatActiveSidePortMin         *int64         `json:"netcat_active_side_port_min"`
	NetcatActiveSidePortMax         *int64         `json:"netcat_active_side_port_max"`
	NetcatPassiveSideConnectAddress *string        `json:"netcat_passive_side_connect_address"`
	SourceDatasets                  []string       `json:"source_datasets"`
	TargetDataset                   string         `json:"target_dataset"`
	Recursive                       bool           `json:"recursive"`
	Exclude                         []string       `json:"exclude"`
	Properties                      bool           `json:"properties"`
	Replicate                       bool           `json:"replicate"`
	PeriodicSnapshotTasks           []embeddedTask `json:"periodic_snapshot_tasks"` // query embeds task objects
	NamingSchema                    []string       `json:"naming_schema"`
	AlsoIncludeNamingSchema         []string       `json:"also_include_naming_schema"`
	NameRegex                       *string        `json:"name_regex"`
	Auto                            bool           `json:"auto"`
	Schedule                        *struct {
		Minute string `json:"minute"`
		Hour   string `json:"hour"`
		Dom    string `json:"dom"`
		Month  string `json:"month"`
		Dow    string `json:"dow"`
	} `json:"schedule"`
	RetentionPolicy string  `json:"retention_policy"`
	LifetimeValue   *int64  `json:"lifetime_value"`
	LifetimeUnit    *string `json:"lifetime_unit"`
	Readonly        string  `json:"readonly"`
	Enabled         bool    `json:"enabled"`
	Retries         int64   `json:"retries"`
}

// sshCredentialsID decodes the ssh_credentials field, which the API may
// return as null, a bare integer (float64 after JSON decode), or an
// embedded object of the form {"id": N}.
func sshCredentialsID(v any) int64 {
	switch t := v.(type) {
	case nil:
		return 0
	case float64:
		return int64(t)
	case int64:
		return t
	case int:
		return int64(t)
	case map[string]any:
		if idVal, ok := t["id"]; ok {
			switch id := idVal.(type) {
			case float64:
				return int64(id)
			case int64:
				return id
			case int:
				return int64(id)
			}
		}
		return 0
	default:
		return 0
	}
}

// stringPtrToValue maps a nullable API string to a types.String (null when nil).
func stringPtrToValue(p *string) types.String {
	if p == nil {
		return types.StringNull()
	}
	return types.StringValue(*p)
}

// int64PtrToValue maps a nullable API integer to a types.Int64 (null when nil).
func int64PtrToValue(p *int64) types.Int64 {
	if p == nil {
		return types.Int64Null()
	}
	return types.Int64Value(*p)
}

// periodicSnapshotTaskIDs extracts the IDs from a list of embedded task
// objects, as returned in replication query/get_instance responses.
func periodicSnapshotTaskIDs(tasks []embeddedTask) []int64 {
	ids := make([]int64, 0, len(tasks))
	for _, t := range tasks {
		ids = append(ids, t.ID)
	}
	return ids
}

// responseToModel maps a replicationAPI response into a ReplicationModel.
func responseToModel(ctx context.Context, api *replicationAPI, m *ReplicationModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.Name = types.StringValue(api.Name)
	m.Direction = types.StringValue(api.Direction)
	m.Transport = types.StringValue(api.Transport)
	m.SSHCredentials = types.Int64Value(sshCredentialsID(api.SSHCredentials))
	m.Sudo = types.BoolValue(api.Sudo)

	if api.Compression != nil {
		m.Compression = types.StringValue(*api.Compression)
	} else {
		m.Compression = types.StringNull()
	}

	if api.SpeedLimit != nil {
		m.SpeedLimit = types.Int64Value(*api.SpeedLimit)
	} else {
		m.SpeedLimit = types.Int64Null()
	}

	m.NetcatActiveSide = stringPtrToValue(api.NetcatActiveSide)
	m.NetcatActiveSideListenAddress = stringPtrToValue(api.NetcatActiveSideListenAddress)
	m.NetcatActiveSidePortMin = int64PtrToValue(api.NetcatActiveSidePortMin)
	m.NetcatActiveSidePortMax = int64PtrToValue(api.NetcatActiveSidePortMax)
	m.NetcatPassiveSideConnectAddress = stringPtrToValue(api.NetcatPassiveSideConnectAddress)

	sourceDatasets := api.SourceDatasets
	if sourceDatasets == nil {
		sourceDatasets = []string{}
	}
	sdList, d := types.ListValueFrom(ctx, types.StringType, sourceDatasets)
	diags.Append(d...)
	m.SourceDatasets = sdList

	m.TargetDataset = types.StringValue(api.TargetDataset)
	m.Recursive = types.BoolValue(api.Recursive)

	exclude := api.Exclude
	if exclude == nil {
		exclude = []string{}
	}
	exList, d2 := types.ListValueFrom(ctx, types.StringType, exclude)
	diags.Append(d2...)
	m.Exclude = exList

	m.Properties = types.BoolValue(api.Properties)
	m.Replicate = types.BoolValue(api.Replicate)

	taskIDs := periodicSnapshotTaskIDs(api.PeriodicSnapshotTasks)
	pstList, d3 := types.ListValueFrom(ctx, types.Int64Type, taskIDs)
	diags.Append(d3...)
	m.PeriodicSnapshotTasks = pstList

	namingSchema := api.NamingSchema
	if namingSchema == nil {
		namingSchema = []string{}
	}
	nsList, d4 := types.ListValueFrom(ctx, types.StringType, namingSchema)
	diags.Append(d4...)
	m.NamingSchema = nsList

	alsoInclude := api.AlsoIncludeNamingSchema
	if alsoInclude == nil {
		alsoInclude = []string{}
	}
	aiList, d5 := types.ListValueFrom(ctx, types.StringType, alsoInclude)
	diags.Append(d5...)
	m.AlsoIncludeNamingSchema = aiList

	if api.NameRegex != nil {
		m.NameRegex = types.StringValue(*api.NameRegex)
	} else {
		m.NameRegex = types.StringValue("")
	}

	m.Auto = types.BoolValue(api.Auto)

	if api.Schedule != nil {
		schedObj, d6 := types.ObjectValueFrom(ctx, scheduleAttrTypes, ScheduleModel{
			Minute: types.StringValue(api.Schedule.Minute),
			Hour:   types.StringValue(api.Schedule.Hour),
			Dom:    types.StringValue(api.Schedule.Dom),
			Month:  types.StringValue(api.Schedule.Month),
			Dow:    types.StringValue(api.Schedule.Dow),
		})
		diags.Append(d6...)
		m.Schedule = schedObj
	} else {
		m.Schedule = types.ObjectNull(scheduleAttrTypes)
	}

	m.RetentionPolicy = types.StringValue(api.RetentionPolicy)

	if api.LifetimeValue != nil {
		m.LifetimeValue = types.Int64Value(*api.LifetimeValue)
	} else {
		m.LifetimeValue = types.Int64Value(0)
	}

	if api.LifetimeUnit != nil {
		m.LifetimeUnit = types.StringValue(*api.LifetimeUnit)
	} else {
		m.LifetimeUnit = types.StringValue("")
	}

	m.Readonly = types.StringValue(api.Readonly)
	m.Enabled = types.BoolValue(api.Enabled)
	m.Retries = types.Int64Value(api.Retries)

	return diags
}

// apiPayload builds the map[string]any payload for replication.create /
// replication.update.
func (m *ReplicationModel) apiPayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	var sourceDatasets []string
	if !m.SourceDatasets.IsNull() && !m.SourceDatasets.IsUnknown() {
		diags.Append(m.SourceDatasets.ElementsAs(ctx, &sourceDatasets, false)...)
	}
	if sourceDatasets == nil {
		sourceDatasets = []string{}
	}

	var exclude []string
	if !m.Exclude.IsNull() && !m.Exclude.IsUnknown() {
		diags.Append(m.Exclude.ElementsAs(ctx, &exclude, false)...)
	}
	if exclude == nil {
		exclude = []string{}
	}

	var periodicSnapshotTasks []int64
	if !m.PeriodicSnapshotTasks.IsNull() && !m.PeriodicSnapshotTasks.IsUnknown() {
		diags.Append(m.PeriodicSnapshotTasks.ElementsAs(ctx, &periodicSnapshotTasks, false)...)
	}
	if periodicSnapshotTasks == nil {
		periodicSnapshotTasks = []int64{}
	}

	// ssh_credentials: send int when non-zero, else nil.
	var sshCredentials any
	if v := m.SSHCredentials.ValueInt64(); v != 0 {
		sshCredentials = v
	}

	p := map[string]any{
		"name":                    m.Name.ValueString(),
		"direction":               m.Direction.ValueString(),
		"transport":               m.Transport.ValueString(),
		"ssh_credentials":         sshCredentials,
		"source_datasets":         sourceDatasets,
		"target_dataset":          m.TargetDataset.ValueString(),
		"recursive":               m.Recursive.ValueBool(),
		"exclude":                 exclude,
		"periodic_snapshot_tasks": periodicSnapshotTasks,
		"auto":                    m.Auto.ValueBool(),
		"retention_policy":        m.RetentionPolicy.ValueString(),
	}

	// Optional+Computed scalars: send only when the user has set a value.
	// When unset, the plan value is Unknown and ValueBool()/ValueString()/
	// ValueInt64() would return zero values, silently sending wrong data
	// (e.g. "enabled": false disables the task, "readonly": "" is an
	// invalid enum, "retries": 0 overrides the API default).
	if !m.Sudo.IsNull() && !m.Sudo.IsUnknown() {
		p["sudo"] = m.Sudo.ValueBool()
	}
	if !m.Properties.IsNull() && !m.Properties.IsUnknown() {
		p["properties"] = m.Properties.ValueBool()
	}
	if !m.Replicate.IsNull() && !m.Replicate.IsUnknown() {
		p["replicate"] = m.Replicate.ValueBool()
	}
	if !m.Readonly.IsNull() && !m.Readonly.IsUnknown() {
		p["readonly"] = m.Readonly.ValueString()
	}
	if !m.Enabled.IsNull() && !m.Enabled.IsUnknown() {
		p["enabled"] = m.Enabled.ValueBool()
	}
	if !m.Retries.IsNull() && !m.Retries.IsUnknown() {
		p["retries"] = m.Retries.ValueInt64()
	}

	// compression / speed_limit: SSH-only, nullable on the wire. Always
	// send the key (value or nil) so an update can clear a previously-set
	// value, matching the lifetime_value/lifetime_unit nil-clearing pattern
	// above.
	if !m.Compression.IsNull() && !m.Compression.IsUnknown() {
		p["compression"] = m.Compression.ValueString()
	} else {
		p["compression"] = nil
	}
	if !m.SpeedLimit.IsNull() && !m.SpeedLimit.IsUnknown() {
		p["speed_limit"] = m.SpeedLimit.ValueInt64()
	} else {
		p["speed_limit"] = nil
	}

	// netcat_* fields: SSH+NETCAT-only, nullable on the wire. Always send the
	// key (value or nil) so switching transport away from SSH+NETCAT clears a
	// previously-set value, matching the compression/speed_limit pattern above.
	if !m.NetcatActiveSide.IsNull() && !m.NetcatActiveSide.IsUnknown() {
		p["netcat_active_side"] = m.NetcatActiveSide.ValueString()
	} else {
		p["netcat_active_side"] = nil
	}
	if !m.NetcatActiveSideListenAddress.IsNull() && !m.NetcatActiveSideListenAddress.IsUnknown() {
		p["netcat_active_side_listen_address"] = m.NetcatActiveSideListenAddress.ValueString()
	} else {
		p["netcat_active_side_listen_address"] = nil
	}
	if !m.NetcatActiveSidePortMin.IsNull() && !m.NetcatActiveSidePortMin.IsUnknown() {
		p["netcat_active_side_port_min"] = m.NetcatActiveSidePortMin.ValueInt64()
	} else {
		p["netcat_active_side_port_min"] = nil
	}
	if !m.NetcatActiveSidePortMax.IsNull() && !m.NetcatActiveSidePortMax.IsUnknown() {
		p["netcat_active_side_port_max"] = m.NetcatActiveSidePortMax.ValueInt64()
	} else {
		p["netcat_active_side_port_max"] = nil
	}
	if !m.NetcatPassiveSideConnectAddress.IsNull() && !m.NetcatPassiveSideConnectAddress.IsUnknown() {
		p["netcat_passive_side_connect_address"] = m.NetcatPassiveSideConnectAddress.ValueString()
	} else {
		p["netcat_passive_side_connect_address"] = nil
	}

	// name_regex: send only when non-empty. When set, naming_schema and
	// also_include_naming_schema are mutually exclusive with it, so omit
	// them entirely.
	if nr := m.NameRegex.ValueString(); nr != "" {
		p["name_regex"] = nr
	} else {
		var namingSchema []string
		if !m.NamingSchema.IsNull() && !m.NamingSchema.IsUnknown() {
			diags.Append(m.NamingSchema.ElementsAs(ctx, &namingSchema, false)...)
		}
		if namingSchema == nil {
			namingSchema = []string{}
		}

		var alsoInclude []string
		if !m.AlsoIncludeNamingSchema.IsNull() && !m.AlsoIncludeNamingSchema.IsUnknown() {
			diags.Append(m.AlsoIncludeNamingSchema.ElementsAs(ctx, &alsoInclude, false)...)
		}
		if alsoInclude == nil {
			alsoInclude = []string{}
		}

		p["naming_schema"] = namingSchema
		p["also_include_naming_schema"] = alsoInclude
	}

	// lifetime_value / lifetime_unit: 0/"" -> null.
	if v := m.LifetimeValue.ValueInt64(); v != 0 {
		p["lifetime_value"] = v
	} else {
		p["lifetime_value"] = nil
	}
	if v := m.LifetimeUnit.ValueString(); v != "" {
		p["lifetime_unit"] = v
	} else {
		p["lifetime_unit"] = nil
	}

	// schedule: send map only when the Schedule object is non-null; else
	// omit the key entirely.
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

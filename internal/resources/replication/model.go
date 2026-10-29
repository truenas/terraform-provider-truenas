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

// cronAttrTypes describes a plain cron object (the schedule of a "lifetimes"
// rule, which has no begin/end window on the API).
var cronAttrTypes = map[string]attr.Type{
	"minute": types.StringType,
	"hour":   types.StringType,
	"dom":    types.StringType,
	"month":  types.StringType,
	"dow":    types.StringType,
}

// scheduleAttrTypes describes the nested "schedule" object: cron fields plus
// the begin/end window. The server replaces the whole object on update and
// fills any key not sent with its default, so begin/end are modeled and sent
// back; otherwise every update reset the window to 00:00-23:59.
var scheduleAttrTypes = map[string]attr.Type{
	"minute": types.StringType,
	"hour":   types.StringType,
	"dom":    types.StringType,
	"month":  types.StringType,
	"dow":    types.StringType,
	"begin":  types.StringType,
	"end":    types.StringType,
}

// restrictScheduleAttrTypes is the cron object used by restrict_schedule; it
// has the same shape as schedule (cron fields plus begin/end).
var restrictScheduleAttrTypes = map[string]attr.Type{
	"minute": types.StringType,
	"hour":   types.StringType,
	"dom":    types.StringType,
	"month":  types.StringType,
	"dow":    types.StringType,
	"begin":  types.StringType,
	"end":    types.StringType,
}

// lifetimeAttrTypes is one entry of the "lifetimes" list (a per-schedule
// snapshot retention rule, used with retention_policy = CUSTOM).
var lifetimeAttrTypes = map[string]attr.Type{
	"schedule":       types.ObjectType{AttrTypes: cronAttrTypes},
	"lifetime_value": types.Int64Type,
	"lifetime_unit":  types.StringType,
}

func lifetimeObjectType() types.ObjectType { return types.ObjectType{AttrTypes: lifetimeAttrTypes} }

// LifetimeModel maps to one "lifetimes" entry.
type LifetimeModel struct {
	Schedule      types.Object `tfsdk:"schedule"`
	LifetimeValue types.Int64  `tfsdk:"lifetime_value"`
	LifetimeUnit  types.String `tfsdk:"lifetime_unit"`
}

// RestrictScheduleModel maps to the nested "restrict_schedule" object.
type RestrictScheduleModel struct {
	Minute types.String `tfsdk:"minute"`
	Hour   types.String `tfsdk:"hour"`
	Dom    types.String `tfsdk:"dom"`
	Month  types.String `tfsdk:"month"`
	Dow    types.String `tfsdk:"dow"`
	Begin  types.String `tfsdk:"begin"`
	End    types.String `tfsdk:"end"`
}

// ScheduleModel maps to the nested "schedule" attribute.
type ScheduleModel struct {
	Minute types.String `tfsdk:"minute"`
	Hour   types.String `tfsdk:"hour"`
	Dom    types.String `tfsdk:"dom"`
	Month  types.String `tfsdk:"month"`
	Dow    types.String `tfsdk:"dow"`
	Begin  types.String `tfsdk:"begin"`
	End    types.String `tfsdk:"end"`
}

// CronModel maps to the schedule of one "lifetimes" entry.
type CronModel struct {
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
	LifetimeValue                   types.Int64  `tfsdk:"lifetime_value"`   // null unless retention_policy = CUSTOM
	LifetimeUnit                    types.String `tfsdk:"lifetime_unit"`    // null unless retention_policy = CUSTOM
	Readonly                        types.String `tfsdk:"readonly"`         // SET, REQUIRE, IGNORE
	Enabled                         types.Bool   `tfsdk:"enabled"`
	Retries                         types.Int64  `tfsdk:"retries"`

	// Send-stream / behaviour options (coverage audit).
	Compressed           types.Bool   `tfsdk:"compressed"`  // zfs send -c
	Embed                types.Bool   `tfsdk:"embed"`       // zfs send -e
	LargeBlock           types.Bool   `tfsdk:"large_block"` // zfs send -L
	AllowFromScratch     types.Bool   `tfsdk:"allow_from_scratch"`
	HoldPendingSnapshots types.Bool   `tfsdk:"hold_pending_snapshots"`
	OnlyMatchingSchedule types.Bool   `tfsdk:"only_matching_schedule"`
	LoggingLevel         types.String `tfsdk:"logging_level"` // null = default
	PropertiesExclude    types.List   `tfsdk:"properties_exclude"`

	// Encryption of the replicated (target) datasets. encryption_key is a
	// write-only secret (read from req.Config, never stored/read back).
	Encryption            types.Bool   `tfsdk:"encryption"`
	EncryptionInherit     types.Bool   `tfsdk:"encryption_inherit"`
	EncryptionKey         types.String `tfsdk:"encryption_key"` // write-only
	EncryptionKeyFormat   types.String `tfsdk:"encryption_key_format"`
	EncryptionKeyLocation types.String `tfsdk:"encryption_key_location"`

	// Additional scheduling / property controls (coverage audit).
	RestrictSchedule   types.Object `tfsdk:"restrict_schedule"`   // cron + begin/end; null when unset
	PropertiesOverride types.Map    `tfsdk:"properties_override"` // map[string]string
	Lifetimes          types.List   `tfsdk:"lifetimes"`           // per-schedule retention (retention_policy=CUSTOM)
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
		Begin  string `json:"begin"`
		End    string `json:"end"`
	} `json:"schedule"`
	RetentionPolicy string  `json:"retention_policy"`
	LifetimeValue   *int64  `json:"lifetime_value"`
	LifetimeUnit    *string `json:"lifetime_unit"`
	Readonly        string  `json:"readonly"`
	Enabled         bool    `json:"enabled"`
	Retries         int64   `json:"retries"`

	// Send-stream / behaviour options (coverage audit).
	Compressed           bool     `json:"compressed"`
	Embed                bool     `json:"embed"`
	LargeBlock           bool     `json:"large_block"`
	AllowFromScratch     bool     `json:"allow_from_scratch"`
	HoldPendingSnapshots bool     `json:"hold_pending_snapshots"`
	OnlyMatchingSchedule bool     `json:"only_matching_schedule"`
	LoggingLevel         *string  `json:"logging_level"`
	PropertiesExclude    []string `json:"properties_exclude"`

	// Encryption (encryption_key is write-only, not read back).
	Encryption            bool    `json:"encryption"`
	EncryptionInherit     *bool   `json:"encryption_inherit"`
	EncryptionKeyFormat   *string `json:"encryption_key_format"`
	EncryptionKeyLocation *string `json:"encryption_key_location"`

	RestrictSchedule *struct {
		Minute string `json:"minute"`
		Hour   string `json:"hour"`
		Dom    string `json:"dom"`
		Month  string `json:"month"`
		Dow    string `json:"dow"`
		Begin  string `json:"begin"`
		End    string `json:"end"`
	} `json:"restrict_schedule"`
	PropertiesOverride map[string]string `json:"properties_override"`
	Lifetimes          []lifetimeAPI     `json:"lifetimes"`
}

// lifetimeAPI is the wire shape of one "lifetimes" entry.
type lifetimeAPI struct {
	Schedule struct {
		Minute string `json:"minute"`
		Hour   string `json:"hour"`
		Dom    string `json:"dom"`
		Month  string `json:"month"`
		Dow    string `json:"dow"`
	} `json:"schedule"`
	LifetimeValue int64  `json:"lifetime_value"`
	LifetimeUnit  string `json:"lifetime_unit"`
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
// injectEncryptionKey adds the write-only encryption_key to a create/update
// payload from the config model (the framework nulls write-only attrs in the
// plan, so the resource reads it from req.Config).
func injectEncryptionKey(payload map[string]any, cfg *ReplicationModel) {
	if !cfg.EncryptionKey.IsNull() && !cfg.EncryptionKey.IsUnknown() {
		payload["encryption_key"] = cfg.EncryptionKey.ValueString()
	}
}

func stringPtrToValue(p *string) types.String {
	if p == nil {
		return types.StringNull()
	}
	return types.StringValue(*p)
}

// nullIfEmpty maps an empty API string to null.
func nullIfEmpty(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
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
			Begin:  nullIfEmpty(api.Schedule.Begin),
			End:    nullIfEmpty(api.Schedule.End),
		})
		diags.Append(d6...)
		m.Schedule = schedObj
	} else {
		m.Schedule = types.ObjectNull(scheduleAttrTypes)
	}

	m.RetentionPolicy = types.StringValue(api.RetentionPolicy)

	// Null unless retention_policy = CUSTOM. They are Optional (not
	// Computed), so removing them from config plans null and the payload
	// clears them, which is what switching CUSTOM -> SOURCE/NONE needs.
	m.LifetimeValue = int64PtrToValue(api.LifetimeValue)
	m.LifetimeUnit = stringPtrToValue(api.LifetimeUnit)

	m.Readonly = types.StringValue(api.Readonly)
	m.Enabled = types.BoolValue(api.Enabled)
	m.Retries = types.Int64Value(api.Retries)

	// Send-stream / behaviour options (coverage audit).
	m.Compressed = types.BoolValue(api.Compressed)
	m.Embed = types.BoolValue(api.Embed)
	m.LargeBlock = types.BoolValue(api.LargeBlock)
	m.AllowFromScratch = types.BoolValue(api.AllowFromScratch)
	m.HoldPendingSnapshots = types.BoolValue(api.HoldPendingSnapshots)
	m.OnlyMatchingSchedule = types.BoolValue(api.OnlyMatchingSchedule)
	m.LoggingLevel = stringPtrToValue(api.LoggingLevel)
	propsExclude := api.PropertiesExclude
	if propsExclude == nil {
		propsExclude = []string{}
	}
	peList, dpe := types.ListValueFrom(ctx, types.StringType, propsExclude)
	diags.Append(dpe...)
	m.PropertiesExclude = peList

	// Encryption: encryption_key is write-only and never read back (left as the
	// plan/state value, i.e. null).
	m.Encryption = types.BoolValue(api.Encryption)
	if api.EncryptionInherit != nil {
		m.EncryptionInherit = types.BoolValue(*api.EncryptionInherit)
	} else {
		m.EncryptionInherit = types.BoolNull()
	}
	m.EncryptionKeyFormat = stringPtrToValue(api.EncryptionKeyFormat)
	m.EncryptionKeyLocation = stringPtrToValue(api.EncryptionKeyLocation)

	if api.RestrictSchedule != nil {
		rsObj, drs := types.ObjectValueFrom(ctx, restrictScheduleAttrTypes, RestrictScheduleModel{
			Minute: types.StringValue(api.RestrictSchedule.Minute),
			Hour:   types.StringValue(api.RestrictSchedule.Hour),
			Dom:    types.StringValue(api.RestrictSchedule.Dom),
			Month:  types.StringValue(api.RestrictSchedule.Month),
			Dow:    types.StringValue(api.RestrictSchedule.Dow),
			Begin:  nullIfEmpty(api.RestrictSchedule.Begin),
			End:    nullIfEmpty(api.RestrictSchedule.End),
		})
		diags.Append(drs...)
		m.RestrictSchedule = rsObj
	} else {
		m.RestrictSchedule = types.ObjectNull(restrictScheduleAttrTypes)
	}

	if len(api.PropertiesOverride) > 0 {
		poMap, dpo := types.MapValueFrom(ctx, types.StringType, api.PropertiesOverride)
		diags.Append(dpo...)
		m.PropertiesOverride = poMap
	} else {
		// Server returns {} when unset; keep it null so an unset config (Optional,
		// non-Computed) does not drift against an empty map.
		m.PropertiesOverride = types.MapNull(types.StringType)
	}

	lifetimes := make([]LifetimeModel, 0, len(api.Lifetimes))
	for _, lt := range api.Lifetimes {
		schedObj, dsc := types.ObjectValueFrom(ctx, cronAttrTypes, CronModel{
			Minute: types.StringValue(lt.Schedule.Minute),
			Hour:   types.StringValue(lt.Schedule.Hour),
			Dom:    types.StringValue(lt.Schedule.Dom),
			Month:  types.StringValue(lt.Schedule.Month),
			Dow:    types.StringValue(lt.Schedule.Dow),
		})
		diags.Append(dsc...)
		lifetimes = append(lifetimes, LifetimeModel{
			Schedule:      schedObj,
			LifetimeValue: types.Int64Value(lt.LifetimeValue),
			LifetimeUnit:  types.StringValue(lt.LifetimeUnit),
		})
	}
	// Keep an empty lifetimes null (Optional, non-Computed) to avoid drift
	// against an unset config. An explicit [] is rejected by the schema.
	if len(lifetimes) == 0 {
		m.Lifetimes = types.ListNull(lifetimeObjectType())
	} else {
		ltList, dlt := types.ListValueFrom(ctx, lifetimeObjectType(), lifetimes)
		diags.Append(dlt...)
		m.Lifetimes = ltList
	}

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

	// Send-stream / behaviour options (coverage audit): Optional+Computed,
	// sent only when set (same reasoning as the scalars above).
	if !m.Compressed.IsNull() && !m.Compressed.IsUnknown() {
		p["compressed"] = m.Compressed.ValueBool()
	}
	if !m.Embed.IsNull() && !m.Embed.IsUnknown() {
		p["embed"] = m.Embed.ValueBool()
	}
	if !m.LargeBlock.IsNull() && !m.LargeBlock.IsUnknown() {
		p["large_block"] = m.LargeBlock.ValueBool()
	}
	if !m.AllowFromScratch.IsNull() && !m.AllowFromScratch.IsUnknown() {
		p["allow_from_scratch"] = m.AllowFromScratch.ValueBool()
	}
	if !m.HoldPendingSnapshots.IsNull() && !m.HoldPendingSnapshots.IsUnknown() {
		p["hold_pending_snapshots"] = m.HoldPendingSnapshots.ValueBool()
	}
	if !m.OnlyMatchingSchedule.IsNull() && !m.OnlyMatchingSchedule.IsUnknown() {
		p["only_matching_schedule"] = m.OnlyMatchingSchedule.ValueBool()
	}
	if !m.LoggingLevel.IsNull() && !m.LoggingLevel.IsUnknown() {
		p["logging_level"] = m.LoggingLevel.ValueString()
	}
	if !m.PropertiesExclude.IsNull() && !m.PropertiesExclude.IsUnknown() {
		var pe []string
		diags.Append(m.PropertiesExclude.ElementsAs(ctx, &pe, false)...)
		if pe == nil {
			pe = []string{}
		}
		p["properties_exclude"] = pe
	}

	// Encryption group (encryption_key is injected from req.Config by the
	// resource, since the framework nulls write-only attrs in the plan).
	if !m.Encryption.IsNull() && !m.Encryption.IsUnknown() {
		p["encryption"] = m.Encryption.ValueBool()
	}
	if !m.EncryptionInherit.IsNull() && !m.EncryptionInherit.IsUnknown() {
		p["encryption_inherit"] = m.EncryptionInherit.ValueBool()
	}
	if !m.EncryptionKeyFormat.IsNull() && !m.EncryptionKeyFormat.IsUnknown() {
		p["encryption_key_format"] = m.EncryptionKeyFormat.ValueString()
	}
	if !m.EncryptionKeyLocation.IsNull() && !m.EncryptionKeyLocation.IsUnknown() {
		p["encryption_key_location"] = m.EncryptionKeyLocation.ValueString()
	}

	// restrict_schedule, properties_override, lifetimes and schedule are
	// Optional without Computed: a null plan means the user removed them, and
	// the server keeps a value that is omitted from an update, so null is sent
	// as the cleared value (null, {}, [], null) instead of being omitted.
	if m.RestrictSchedule.IsNull() {
		p["restrict_schedule"] = nil
	} else if !m.RestrictSchedule.IsUnknown() {
		var rs RestrictScheduleModel
		diags.Append(m.RestrictSchedule.As(ctx, &rs, basetypes.ObjectAsOptions{})...)
		p["restrict_schedule"] = cronWithWindow(rs.Minute, rs.Hour, rs.Dom, rs.Month, rs.Dow, rs.Begin, rs.End)
	}
	if m.PropertiesOverride.IsNull() {
		p["properties_override"] = map[string]string{}
	} else if !m.PropertiesOverride.IsUnknown() {
		var po map[string]string
		diags.Append(m.PropertiesOverride.ElementsAs(ctx, &po, false)...)
		if po == nil {
			po = map[string]string{}
		}
		p["properties_override"] = po
	}
	if m.Lifetimes.IsNull() {
		p["lifetimes"] = []map[string]any{}
	} else if !m.Lifetimes.IsUnknown() {
		var lts []LifetimeModel
		diags.Append(m.Lifetimes.ElementsAs(ctx, &lts, false)...)
		out := make([]map[string]any, 0, len(lts))
		for _, lt := range lts {
			var sched CronModel
			diags.Append(lt.Schedule.As(ctx, &sched, basetypes.ObjectAsOptions{})...)
			out = append(out, map[string]any{
				"schedule": map[string]string{
					"minute": sched.Minute.ValueString(),
					"hour":   sched.Hour.ValueString(),
					"dom":    sched.Dom.ValueString(),
					"month":  sched.Month.ValueString(),
					"dow":    sched.Dow.ValueString(),
				},
				"lifetime_value": lt.LifetimeValue.ValueInt64(),
				"lifetime_unit":  lt.LifetimeUnit.ValueString(),
			})
		}
		p["lifetimes"] = out
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

	// lifetime_value / lifetime_unit: null (or the legacy 0/"" from state
	// written by older versions) -> null, which clears them.
	if v := m.LifetimeValue; !v.IsNull() && !v.IsUnknown() && v.ValueInt64() != 0 {
		p["lifetime_value"] = v.ValueInt64()
	} else {
		p["lifetime_value"] = nil
	}
	if v := m.LifetimeUnit; !v.IsNull() && !v.IsUnknown() && v.ValueString() != "" {
		p["lifetime_unit"] = v.ValueString()
	} else {
		p["lifetime_unit"] = nil
	}

	// schedule: null clears it (required to turn auto-run off: the server
	// rejects a schedule on a task with auto = false). begin/end go out with
	// the cron fields whenever known (see scheduleAttrTypes).
	if m.Schedule.IsNull() {
		p["schedule"] = nil
	} else if !m.Schedule.IsUnknown() {
		var sched ScheduleModel
		diags.Append(m.Schedule.As(ctx, &sched, basetypes.ObjectAsOptions{})...)
		p["schedule"] = cronWithWindow(sched.Minute, sched.Hour, sched.Dom, sched.Month, sched.Dow, sched.Begin, sched.End)
	}

	return p, diags
}

// cronWithWindow builds a schedule/restrict_schedule wire object. begin/end
// are included only when known: unknown means unset on create, and the
// server then fills 00:00/23:59.
func cronWithWindow(minute, hour, dom, month, dow, begin, end types.String) map[string]string {
	sched := map[string]string{
		"minute": minute.ValueString(),
		"hour":   hour.ValueString(),
		"dom":    dom.ValueString(),
		"month":  month.ValueString(),
		"dow":    dow.ValueString(),
	}
	if !begin.IsNull() && !begin.IsUnknown() && begin.ValueString() != "" {
		sched["begin"] = begin.ValueString()
	}
	if !end.IsNull() && !end.IsUnknown() && end.ValueString() != "" {
		sched["end"] = end.ValueString()
	}
	return sched
}

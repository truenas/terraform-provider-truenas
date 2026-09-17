// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package rsync_task

import (
	"context"
	"encoding/json"
	"fmt"

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

// RsyncTaskModel is the Terraform state/plan model for truenas_rsync_task.
// validate_rpath and ssh_keyscan are write-only in the sense that
// rsynctask.create/update accept them but rsynctask.query/get_instance never
// return them (verified by probe); responseToModel deliberately leaves them
// untouched so the plan/state value the user configured is preserved
// (mirrors zvol's "sparse" attribute).
type RsyncTaskModel struct {
	ID             types.Int64  `tfsdk:"id"`
	Path           types.String `tfsdk:"path"`
	User           types.String `tfsdk:"user"`
	Mode           types.String `tfsdk:"mode"` // MODULE, SSH
	RemoteHost     types.String `tfsdk:"remotehost"`
	RemotePort     types.Int64  `tfsdk:"remoteport"` // nullable: SSH mode only
	RemoteModule   types.String `tfsdk:"remotemodule"`
	SSHCredentials types.Int64  `tfsdk:"ssh_credentials"` // nullable: keychain credential id; null = user's own SSH keys
	RemotePath     types.String `tfsdk:"remotepath"`
	Direction      types.String `tfsdk:"direction"` // PUSH, PULL
	Desc           types.String `tfsdk:"desc"`
	Schedule       types.Object `tfsdk:"schedule"`
	Recursive      types.Bool   `tfsdk:"recursive"`
	Times          types.Bool   `tfsdk:"times"`
	Compress       types.Bool   `tfsdk:"compress"`
	Archive        types.Bool   `tfsdk:"archive"`
	Delete         types.Bool   `tfsdk:"delete"`
	Quiet          types.Bool   `tfsdk:"quiet"`
	PreservePerm   types.Bool   `tfsdk:"preserveperm"`
	PreserveAttr   types.Bool   `tfsdk:"preserveattr"`
	DelayUpdates   types.Bool   `tfsdk:"delayupdates"`
	Extra          types.List   `tfsdk:"extra"` // List[String]
	Enabled        types.Bool   `tfsdk:"enabled"`
	ValidateRPath  types.Bool   `tfsdk:"validate_rpath"` // write-only: never returned by the API
	SSHKeyscan     types.Bool   `tfsdk:"ssh_keyscan"`    // write-only: never returned by the API
}

// RsyncTaskDataSourceModel is the read-only model for the truenas_rsync_task
// datasource, looked up by "desc". It omits validate_rpath and ssh_keyscan:
// those are create/update-only operational flags that rsynctask.query never
// returns, so there is nothing meaningful for a datasource to surface.
type RsyncTaskDataSourceModel struct {
	ID             types.Int64  `tfsdk:"id"`
	Path           types.String `tfsdk:"path"`
	User           types.String `tfsdk:"user"`
	Mode           types.String `tfsdk:"mode"`
	RemoteHost     types.String `tfsdk:"remotehost"`
	RemotePort     types.Int64  `tfsdk:"remoteport"`
	RemoteModule   types.String `tfsdk:"remotemodule"`
	SSHCredentials types.Int64  `tfsdk:"ssh_credentials"`
	RemotePath     types.String `tfsdk:"remotepath"`
	Direction      types.String `tfsdk:"direction"`
	Desc           types.String `tfsdk:"desc"`
	Schedule       types.Object `tfsdk:"schedule"`
	Recursive      types.Bool   `tfsdk:"recursive"`
	Times          types.Bool   `tfsdk:"times"`
	Compress       types.Bool   `tfsdk:"compress"`
	Archive        types.Bool   `tfsdk:"archive"`
	Delete         types.Bool   `tfsdk:"delete"`
	Quiet          types.Bool   `tfsdk:"quiet"`
	PreservePerm   types.Bool   `tfsdk:"preserveperm"`
	PreserveAttr   types.Bool   `tfsdk:"preserveattr"`
	DelayUpdates   types.Bool   `tfsdk:"delayupdates"`
	Extra          types.List   `tfsdk:"extra"`
	Enabled        types.Bool   `tfsdk:"enabled"`
}

// scheduleAPI is the JSON wire format of the nested "schedule" object
// returned/accepted by rsynctask.* methods.
type scheduleAPI struct {
	Minute string `json:"minute"`
	Hour   string `json:"hour"`
	Dom    string `json:"dom"`
	Month  string `json:"month"`
	Dow    string `json:"dow"`
}

// rsyncTaskAPI mirrors the JSON object returned by rsynctask.create,
// rsynctask.update, rsynctask.get_instance, and rsynctask.query. Probed
// against a live TrueNAS 25.10 box:
//   - remotehost, remotemodule are nullable strings (null when unset, plain
//     strings otherwise) — never omitted from the response.
//   - remoteport is a nullable integer (null when unset).
//   - ssh_credentials is nullable: null when unset, or (per the API's own
//     documented schema for get_instance/query/create-returns) an embedded
//     KeychainCredentialEntry object {id, name, type, attributes} when a
//     keychain credential is configured — never a bare integer on read,
//     even though create/update accept a bare integer ID. Decoded via
//     decodeSSHCredentialsID.
//   - "locked" and "job" are present on every response but are pure runtime
//     status (whether the task is currently executing); not modeled here,
//     the brief's field list omits them.
type rsyncTaskAPI struct {
	ID             int64           `json:"id"`
	Path           string          `json:"path"`
	User           string          `json:"user"`
	Mode           string          `json:"mode"`
	RemoteHost     *string         `json:"remotehost"`
	RemotePort     *int64          `json:"remoteport"`
	RemoteModule   *string         `json:"remotemodule"`
	SSHCredentials json.RawMessage `json:"ssh_credentials"`
	RemotePath     string          `json:"remotepath"`
	Direction      string          `json:"direction"`
	Desc           string          `json:"desc"`
	Schedule       scheduleAPI     `json:"schedule"`
	Recursive      bool            `json:"recursive"`
	Times          bool            `json:"times"`
	Compress       bool            `json:"compress"`
	Archive        bool            `json:"archive"`
	Delete         bool            `json:"delete"`
	Quiet          bool            `json:"quiet"`
	PreservePerm   bool            `json:"preserveperm"`
	PreserveAttr   bool            `json:"preserveattr"`
	DelayUpdates   bool            `json:"delayupdates"`
	Extra          []string        `json:"extra"`
	Enabled        bool            `json:"enabled"`
}

// decodeSSHCredentialsID decodes the "ssh_credentials" field of a
// rsyncTaskAPI response. It returns (nil, nil) for a JSON null (no
// credential configured — the task uses the run-as user's own SSH keys),
// and otherwise accepts either an embedded object ({"id": N, ...}, the
// shape actually observed on reads) or a bare integer (accepted defensively
// in case a future API version returns one directly, mirroring cloudsync's
// decodeCredentialsID).
func decodeSSHCredentialsID(raw json.RawMessage) (*int64, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}

	var obj struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil {
		return &obj.ID, nil
	}

	var id int64
	if err := json.Unmarshal(raw, &id); err == nil {
		return &id, nil
	}

	return nil, fmt.Errorf("cannot decode ssh_credentials field %q as null, object with id, or bare integer", string(raw))
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

// nullableStringValue converts a nullable API string pointer to a
// types.String, preserving true null rather than substituting "".
func nullableStringValue(v *string) types.String {
	if v == nil {
		return types.StringNull()
	}
	return types.StringValue(*v)
}

// nullableInt64Value converts a nullable API int64 pointer to a
// types.Int64, preserving true null.
func nullableInt64Value(v *int64) types.Int64 {
	if v == nil {
		return types.Int64Null()
	}
	return types.Int64Value(*v)
}

// extraListValue builds the "extra" types.List from an API response,
// normalizing a nil slice to an empty list (API default) rather than a
// Terraform null.
func extraListValue(ctx context.Context, extra []string) (types.List, diag.Diagnostics) {
	if extra == nil {
		extra = []string{}
	}
	return types.ListValueFrom(ctx, types.StringType, extra)
}

// responseToModel maps a rsyncTaskAPI response onto a RsyncTaskModel.
// ValidateRPath and SSHKeyscan are intentionally left untouched (see
// RsyncTaskModel's doc comment).
func responseToModel(ctx context.Context, api *rsyncTaskAPI, m *RsyncTaskModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.Path = types.StringValue(api.Path)
	m.User = types.StringValue(api.User)
	m.Mode = types.StringValue(api.Mode)
	m.RemoteHost = nullableStringValue(api.RemoteHost)
	m.RemotePort = nullableInt64Value(api.RemotePort)
	m.RemoteModule = nullableStringValue(api.RemoteModule)

	credID, err := decodeSSHCredentialsID(api.SSHCredentials)
	if err != nil {
		diags.AddError("Invalid ssh_credentials in API response", err.Error())
		return diags
	}
	m.SSHCredentials = nullableInt64Value(credID)

	m.RemotePath = types.StringValue(api.RemotePath)
	m.Direction = types.StringValue(api.Direction)
	m.Desc = types.StringValue(api.Desc)

	sched, d := scheduleObjectValue(ctx, api.Schedule)
	diags.Append(d...)
	m.Schedule = sched

	m.Recursive = types.BoolValue(api.Recursive)
	m.Times = types.BoolValue(api.Times)
	m.Compress = types.BoolValue(api.Compress)
	m.Archive = types.BoolValue(api.Archive)
	m.Delete = types.BoolValue(api.Delete)
	m.Quiet = types.BoolValue(api.Quiet)
	m.PreservePerm = types.BoolValue(api.PreservePerm)
	m.PreserveAttr = types.BoolValue(api.PreserveAttr)
	m.DelayUpdates = types.BoolValue(api.DelayUpdates)

	extra, d := extraListValue(ctx, api.Extra)
	diags.Append(d...)
	m.Extra = extra

	m.Enabled = types.BoolValue(api.Enabled)

	return diags
}

// responseToDataSourceModel maps a rsyncTaskAPI response onto a
// RsyncTaskDataSourceModel.
func responseToDataSourceModel(ctx context.Context, api *rsyncTaskAPI, m *RsyncTaskDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.Path = types.StringValue(api.Path)
	m.User = types.StringValue(api.User)
	m.Mode = types.StringValue(api.Mode)
	m.RemoteHost = nullableStringValue(api.RemoteHost)
	m.RemotePort = nullableInt64Value(api.RemotePort)
	m.RemoteModule = nullableStringValue(api.RemoteModule)

	credID, err := decodeSSHCredentialsID(api.SSHCredentials)
	if err != nil {
		diags.AddError("Invalid ssh_credentials in API response", err.Error())
		return diags
	}
	m.SSHCredentials = nullableInt64Value(credID)

	m.RemotePath = types.StringValue(api.RemotePath)
	m.Direction = types.StringValue(api.Direction)
	m.Desc = types.StringValue(api.Desc)

	sched, d := scheduleObjectValue(ctx, api.Schedule)
	diags.Append(d...)
	m.Schedule = sched

	m.Recursive = types.BoolValue(api.Recursive)
	m.Times = types.BoolValue(api.Times)
	m.Compress = types.BoolValue(api.Compress)
	m.Archive = types.BoolValue(api.Archive)
	m.Delete = types.BoolValue(api.Delete)
	m.Quiet = types.BoolValue(api.Quiet)
	m.PreservePerm = types.BoolValue(api.PreservePerm)
	m.PreserveAttr = types.BoolValue(api.PreserveAttr)
	m.DelayUpdates = types.BoolValue(api.DelayUpdates)

	extra, d := extraListValue(ctx, api.Extra)
	diags.Append(d...)
	m.Extra = extra

	m.Enabled = types.BoolValue(api.Enabled)

	return diags
}

// apiPayload builds the map expected by rsynctask.create/rsynctask.update.
// "path" and "user" are always included (Required). Every other field is
// Optional+Computed: each is included only when known and non-null, so an
// unset optional is omitted entirely and the TrueNAS-side default takes
// effect instead of an explicit zero value. remotehost, remotemodule,
// remoteport, and ssh_credentials are additionally nullable on the wire;
// they follow the same "omit unless known-and-non-null" rule (matching
// nvmet_port's inline_data_size/max_queue_size convention) rather than ever
// sending an explicit JSON null, so an update payload built from a model
// that has gone from a real value back to null does not clear the
// server-side value — it simply leaves it as last configured. validate_rpath
// and ssh_keyscan are pure create/update behavior flags (never echoed back
// by the API) and are included whenever known, on both create and update.
func (m *RsyncTaskModel) apiPayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	p := map[string]any{
		"path": m.Path.ValueString(),
		"user": m.User.ValueString(),
	}

	if !m.Mode.IsNull() && !m.Mode.IsUnknown() {
		p["mode"] = m.Mode.ValueString()
	}
	if !m.RemoteHost.IsNull() && !m.RemoteHost.IsUnknown() {
		p["remotehost"] = m.RemoteHost.ValueString()
	}
	if !m.RemotePort.IsNull() && !m.RemotePort.IsUnknown() {
		p["remoteport"] = m.RemotePort.ValueInt64()
	}
	if !m.RemoteModule.IsNull() && !m.RemoteModule.IsUnknown() {
		p["remotemodule"] = m.RemoteModule.ValueString()
	}
	if !m.SSHCredentials.IsNull() && !m.SSHCredentials.IsUnknown() {
		p["ssh_credentials"] = m.SSHCredentials.ValueInt64()
	}
	if !m.RemotePath.IsNull() && !m.RemotePath.IsUnknown() {
		p["remotepath"] = m.RemotePath.ValueString()
	}
	if !m.Direction.IsNull() && !m.Direction.IsUnknown() {
		p["direction"] = m.Direction.ValueString()
	}
	if !m.Desc.IsNull() && !m.Desc.IsUnknown() {
		p["desc"] = m.Desc.ValueString()
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
	if !m.Recursive.IsNull() && !m.Recursive.IsUnknown() {
		p["recursive"] = m.Recursive.ValueBool()
	}
	if !m.Times.IsNull() && !m.Times.IsUnknown() {
		p["times"] = m.Times.ValueBool()
	}
	if !m.Compress.IsNull() && !m.Compress.IsUnknown() {
		p["compress"] = m.Compress.ValueBool()
	}
	if !m.Archive.IsNull() && !m.Archive.IsUnknown() {
		p["archive"] = m.Archive.ValueBool()
	}
	if !m.Delete.IsNull() && !m.Delete.IsUnknown() {
		p["delete"] = m.Delete.ValueBool()
	}
	if !m.Quiet.IsNull() && !m.Quiet.IsUnknown() {
		p["quiet"] = m.Quiet.ValueBool()
	}
	if !m.PreservePerm.IsNull() && !m.PreservePerm.IsUnknown() {
		p["preserveperm"] = m.PreservePerm.ValueBool()
	}
	if !m.PreserveAttr.IsNull() && !m.PreserveAttr.IsUnknown() {
		p["preserveattr"] = m.PreserveAttr.ValueBool()
	}
	if !m.DelayUpdates.IsNull() && !m.DelayUpdates.IsUnknown() {
		p["delayupdates"] = m.DelayUpdates.ValueBool()
	}
	if !m.Extra.IsNull() && !m.Extra.IsUnknown() {
		var extra []string
		diags.Append(m.Extra.ElementsAs(ctx, &extra, false)...)
		if extra == nil {
			extra = []string{}
		}
		p["extra"] = extra
	}
	if !m.Enabled.IsNull() && !m.Enabled.IsUnknown() {
		p["enabled"] = m.Enabled.ValueBool()
	}
	if !m.ValidateRPath.IsNull() && !m.ValidateRPath.IsUnknown() {
		p["validate_rpath"] = m.ValidateRPath.ValueBool()
	}
	if !m.SSHKeyscan.IsNull() && !m.SSHKeyscan.IsUnknown() {
		p["ssh_keyscan"] = m.SSHKeyscan.ValueBool()
	}

	return p, diags
}

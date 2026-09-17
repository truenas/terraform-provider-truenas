// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cloud_backup

import (
	"context"
	"encoding/json"
	"fmt"

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

// CloudBackupModel is the Terraform state/plan model for a cloud backup
// task. It is used for both the resource and the datasource: field types
// line up with both schemas, differing only in Required/Computed markers
// declared in schema.go / datasource.go.
type CloudBackupModel struct {
	ID              types.Int64   `tfsdk:"id"`
	Description     types.String  `tfsdk:"description"`
	Path            types.String  `tfsdk:"path"`
	Credentials     types.Int64   `tfsdk:"credentials"`
	Attributes      types.String  `tfsdk:"attributes"` // JSON: {"bucket": "...", "folder": "..."}
	Schedule        ScheduleModel `tfsdk:"schedule"`
	PreScript       types.String  `tfsdk:"pre_script"`
	PostScript      types.String  `tfsdk:"post_script"`
	Snapshot        types.Bool    `tfsdk:"snapshot"`
	Include         types.List    `tfsdk:"include"` // List[String]
	Exclude         types.List    `tfsdk:"exclude"` // List[String]
	Enabled         types.Bool    `tfsdk:"enabled"`
	Password        types.String  `tfsdk:"password"`
	KeepLast        types.Int64   `tfsdk:"keep_last"`
	TransferSetting types.String  `tfsdk:"transfer_setting"` // DEFAULT, PERFORMANCE, FAST_STORAGE
	AbsolutePaths   types.Bool    `tfsdk:"absolute_paths"`
	CachePath       types.String  `tfsdk:"cache_path"` // nullable
	RateLimit       types.Int64   `tfsdk:"rate_limit"` // nullable
}

// CloudBackupDataSourceModel is the read-only lookup model for the
// truenas_cloud_backup datasource.
type CloudBackupDataSourceModel struct {
	ID              types.Int64   `tfsdk:"id"`
	Description     types.String  `tfsdk:"description"`
	Path            types.String  `tfsdk:"path"`
	Credentials     types.Int64   `tfsdk:"credentials"`
	Attributes      types.String  `tfsdk:"attributes"`
	Schedule        ScheduleModel `tfsdk:"schedule"`
	PreScript       types.String  `tfsdk:"pre_script"`
	PostScript      types.String  `tfsdk:"post_script"`
	Snapshot        types.Bool    `tfsdk:"snapshot"`
	Include         types.List    `tfsdk:"include"`
	Exclude         types.List    `tfsdk:"exclude"`
	Enabled         types.Bool    `tfsdk:"enabled"`
	Password        types.String  `tfsdk:"password"`
	KeepLast        types.Int64   `tfsdk:"keep_last"`
	TransferSetting types.String  `tfsdk:"transfer_setting"`
	AbsolutePaths   types.Bool    `tfsdk:"absolute_paths"`
	CachePath       types.String  `tfsdk:"cache_path"`
	RateLimit       types.Int64   `tfsdk:"rate_limit"`
}

// cloudBackupAPI is the JSON shape returned by cloud_backup.* methods.
// Probed against a live TrueNAS 25.10 box via core.get_methods
// (cloud_backup.create/get_instance/query/update all share this shape) and
// cross-checked against middlewared's
// middlewared/api/v25_10_2/{cloud,cloud_backup}.py source:
//
//   - "credentials" is an embedded CloudCredentialEntry object
//     ({"id", "name", "provider"}) on every response, never a bare integer
//     — decoded via decodeCredentialsID (mirrors cloudsync's identical
//     field, and defensively also accepts a bare integer).
//   - "password" is a pydantic Secret[NonEmptyString]. Source-verified
//     (middlewared/main.py dump_result + api/base/handler/remove_secrets.py):
//     it is returned byte-for-byte to a caller whose session has FULL_ADMIN
//     or the CLOUD_BACKUP_WRITE role (the normal case for this provider's
//     admin-scoped API key), and masked to "********" only for a
//     lesser-privileged user-session credential. See schema.go's
//     "password" description.
//   - "job" (nested job/progress state) and "locked" are pure runtime
//     status, not configuration — not modeled here, mirroring rsync_task's
//     treatment of its own "locked"/"job" response fields.
//   - "cache_path" and "rate_limit" are nullable (null when unset).
type cloudBackupAPI struct {
	ID              int64           `json:"id"`
	Description     string          `json:"description"`
	Path            string          `json:"path"`
	Credentials     json.RawMessage `json:"credentials"`
	Attributes      map[string]any  `json:"attributes"`
	Schedule        scheduleAPI     `json:"schedule"`
	PreScript       string          `json:"pre_script"`
	PostScript      string          `json:"post_script"`
	Snapshot        bool            `json:"snapshot"`
	Include         []string        `json:"include"`
	Exclude         []string        `json:"exclude"`
	Enabled         bool            `json:"enabled"`
	Password        string          `json:"password"`
	KeepLast        int64           `json:"keep_last"`
	TransferSetting string          `json:"transfer_setting"`
	AbsolutePaths   bool            `json:"absolute_paths"`
	CachePath       *string         `json:"cache_path"`
	RateLimit       *int64          `json:"rate_limit"`
}

// scheduleAPI is the JSON wire format of the nested "schedule" object
// returned/accepted by cloud_backup.* methods.
type scheduleAPI struct {
	Minute string `json:"minute"`
	Hour   string `json:"hour"`
	Dom    string `json:"dom"`
	Month  string `json:"month"`
	Dow    string `json:"dow"`
}

// decodeCredentialsID decodes the "credentials" field of a cloudBackupAPI
// response, which is an embedded object ({"id": N, ...}) on every probed
// response, but bare-integer decoding is accepted defensively (mirrors
// cloudsync's decodeCredentialsID).
func decodeCredentialsID(raw json.RawMessage) (int64, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, fmt.Errorf("credentials is null in API response")
	}

	var obj struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil {
		return obj.ID, nil
	}

	var id int64
	if err := json.Unmarshal(raw, &id); err == nil {
		return id, nil
	}

	return 0, fmt.Errorf("cannot decode credentials field %q as object with id or bare integer", string(raw))
}

// attributesMap parses the attributes JSON string into a map.
func (m *CloudBackupModel) attributesMap() (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	var attrs map[string]any
	if err := json.Unmarshal([]byte(m.Attributes.ValueString()), &attrs); err != nil {
		diags.AddError("Invalid attributes JSON", err.Error())
		return nil, diags
	}
	return attrs, diags
}

// attributesDrifted reports whether any key present in stateAttrs has a
// different value in apiAttrs. API-added default keys are ignored (not
// drift) since the API may normalize/expand attributes with defaults.
func attributesDrifted(stateAttrs, apiAttrs map[string]any) bool {
	for k, v := range stateAttrs {
		av, ok := apiAttrs[k]
		if !ok {
			return true
		}
		sb, _ := json.Marshal(v)
		ab, _ := json.Marshal(av)
		if string(sb) != string(ab) {
			return true
		}
	}
	return false
}

// apiAttributesJSON marshals api.Attributes to a canonical JSON string.
func apiAttributesJSON(api *cloudBackupAPI) (string, diag.Diagnostics) {
	var diags diag.Diagnostics
	b, err := json.Marshal(api.Attributes)
	if err != nil {
		diags.AddError("Failed to marshal cloud backup attributes", err.Error())
		return "", diags
	}
	return string(b), diags
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

// stringListValue builds a types.List[String] from an API response,
// normalizing a nil slice to an empty list (API default) rather than a
// Terraform null.
func stringListValue(ctx context.Context, items []string) (types.List, diag.Diagnostics) {
	if items == nil {
		items = []string{}
	}
	return types.ListValueFrom(ctx, types.StringType, items)
}

// apiPayload converts the Terraform model into the map expected by
// cloud_backup.create / cloud_backup.update. "path", "credentials",
// "attributes", "password", and "keep_last" are always included (Required).
// Every other field is Optional+Computed: each is included only when known,
// so an unset optional is omitted entirely and the TrueNAS-side default
// takes effect instead of an explicit zero value. cache_path/rate_limit are
// additionally nullable on the wire; they follow the same
// "omit-unless-known" rule (matching rsync_task's remotehost/remoteport
// convention) rather than ever sending an explicit JSON null, so an update
// payload built from a model that went from a real value back to null does
// not clear the server-side value.
func (m *CloudBackupModel) apiPayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	attrs, attrDiags := m.attributesMap()
	diags.Append(attrDiags...)
	if diags.HasError() {
		return nil, diags
	}

	p := map[string]any{
		"path":        m.Path.ValueString(),
		"credentials": m.Credentials.ValueInt64(),
		"attributes":  attrs,
		"password":    m.Password.ValueString(),
		"keep_last":   m.KeepLast.ValueInt64(),
	}

	if !m.Description.IsNull() && !m.Description.IsUnknown() {
		p["description"] = m.Description.ValueString()
	}
	if !m.Schedule.Minute.IsNull() && !m.Schedule.Minute.IsUnknown() {
		p["schedule"] = map[string]string{
			"minute": m.Schedule.Minute.ValueString(),
			"hour":   m.Schedule.Hour.ValueString(),
			"dom":    m.Schedule.Dom.ValueString(),
			"month":  m.Schedule.Month.ValueString(),
			"dow":    m.Schedule.Dow.ValueString(),
		}
	}
	if !m.PreScript.IsNull() && !m.PreScript.IsUnknown() {
		p["pre_script"] = m.PreScript.ValueString()
	}
	if !m.PostScript.IsNull() && !m.PostScript.IsUnknown() {
		p["post_script"] = m.PostScript.ValueString()
	}
	if !m.Snapshot.IsNull() && !m.Snapshot.IsUnknown() {
		p["snapshot"] = m.Snapshot.ValueBool()
	}
	if !m.Include.IsNull() && !m.Include.IsUnknown() {
		var include []string
		diags.Append(m.Include.ElementsAs(ctx, &include, false)...)
		if include == nil {
			include = []string{}
		}
		p["include"] = include
	}
	if !m.Exclude.IsNull() && !m.Exclude.IsUnknown() {
		var exclude []string
		diags.Append(m.Exclude.ElementsAs(ctx, &exclude, false)...)
		if exclude == nil {
			exclude = []string{}
		}
		p["exclude"] = exclude
	}
	if !m.Enabled.IsNull() && !m.Enabled.IsUnknown() {
		p["enabled"] = m.Enabled.ValueBool()
	}
	if !m.TransferSetting.IsNull() && !m.TransferSetting.IsUnknown() {
		p["transfer_setting"] = m.TransferSetting.ValueString()
	}
	if !m.AbsolutePaths.IsNull() && !m.AbsolutePaths.IsUnknown() {
		p["absolute_paths"] = m.AbsolutePaths.ValueBool()
	}
	if !m.CachePath.IsNull() && !m.CachePath.IsUnknown() {
		p["cache_path"] = m.CachePath.ValueString()
	}
	if !m.RateLimit.IsNull() && !m.RateLimit.IsUnknown() {
		p["rate_limit"] = m.RateLimit.ValueInt64()
	}

	return p, diags
}

// updatePayload builds the second arg of cloud_backup.update. Identical to
// apiPayload except "absolute_paths" is never sent: cloud_backup.update's
// accepted fields (probed via core.get_methods) exclude it — the API
// rejects create-only fields with additionalProperties:false — matching
// middlewared's own CloudBackupUpdate model, which excludes it explicitly.
func (m *CloudBackupModel) updatePayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	p, diags := m.apiPayload(ctx)
	if diags.HasError() {
		return nil, diags
	}
	delete(p, "absolute_paths")
	return p, diags
}

// scheduleFromAPI builds a ScheduleModel from an API response.
func scheduleFromAPI(api scheduleAPI) ScheduleModel {
	return ScheduleModel{
		Minute: types.StringValue(api.Minute),
		Hour:   types.StringValue(api.Hour),
		Dom:    types.StringValue(api.Dom),
		Month:  types.StringValue(api.Month),
		Dow:    types.StringValue(api.Dow),
	}
}

// responseToModel maps a cloudBackupAPI response into a CloudBackupModel.
// It deliberately does not touch m.Attributes so callers can implement
// write-what-you-said / drift-aware semantics for that field (mirrors
// cloudsync).
func responseToModel(ctx context.Context, api *cloudBackupAPI, m *CloudBackupModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.Description = types.StringValue(api.Description)
	m.Path = types.StringValue(api.Path)

	credID, err := decodeCredentialsID(api.Credentials)
	if err != nil {
		diags.AddError("Invalid credentials in API response", err.Error())
		return diags
	}
	m.Credentials = types.Int64Value(credID)

	m.Schedule = scheduleFromAPI(api.Schedule)
	m.PreScript = types.StringValue(api.PreScript)
	m.PostScript = types.StringValue(api.PostScript)
	m.Snapshot = types.BoolValue(api.Snapshot)

	include, d := stringListValue(ctx, api.Include)
	diags.Append(d...)
	m.Include = include

	exclude, d := stringListValue(ctx, api.Exclude)
	diags.Append(d...)
	m.Exclude = exclude

	m.Enabled = types.BoolValue(api.Enabled)
	m.Password = types.StringValue(api.Password)
	m.KeepLast = types.Int64Value(api.KeepLast)
	m.TransferSetting = types.StringValue(api.TransferSetting)
	m.AbsolutePaths = types.BoolValue(api.AbsolutePaths)
	m.CachePath = nullableStringValue(api.CachePath)
	m.RateLimit = nullableInt64Value(api.RateLimit)

	return diags
}

// responseToDataSourceModel maps a cloudBackupAPI response onto a
// CloudBackupDataSourceModel.
func responseToDataSourceModel(ctx context.Context, api *cloudBackupAPI, m *CloudBackupDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.Description = types.StringValue(api.Description)
	m.Path = types.StringValue(api.Path)

	credID, err := decodeCredentialsID(api.Credentials)
	if err != nil {
		diags.AddError("Invalid credentials in API response", err.Error())
		return diags
	}
	m.Credentials = types.Int64Value(credID)

	m.Schedule = scheduleFromAPI(api.Schedule)
	m.PreScript = types.StringValue(api.PreScript)
	m.PostScript = types.StringValue(api.PostScript)
	m.Snapshot = types.BoolValue(api.Snapshot)

	include, d := stringListValue(ctx, api.Include)
	diags.Append(d...)
	m.Include = include

	exclude, d := stringListValue(ctx, api.Exclude)
	diags.Append(d...)
	m.Exclude = exclude

	m.Enabled = types.BoolValue(api.Enabled)
	m.Password = types.StringValue(api.Password)
	m.KeepLast = types.Int64Value(api.KeepLast)
	m.TransferSetting = types.StringValue(api.TransferSetting)
	m.AbsolutePaths = types.BoolValue(api.AbsolutePaths)
	m.CachePath = nullableStringValue(api.CachePath)
	m.RateLimit = nullableInt64Value(api.RateLimit)

	attrJSON, jd := apiAttributesJSON(api)
	diags.Append(jd...)
	m.Attributes = types.StringValue(attrJSON)

	return diags
}

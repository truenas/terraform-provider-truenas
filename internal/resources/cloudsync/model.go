// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cloudsync

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

// CloudSyncModel is the Terraform state/plan model for a cloud sync task.
// It is used for both the resource and the datasource: field types line up
// with both schemas, differing only in Required/Computed markers declared
// in schema.go / datasource.go.
type CloudSyncModel struct {
	ID           types.Int64   `tfsdk:"id"`
	Description  types.String  `tfsdk:"description"`
	Path         types.String  `tfsdk:"path"`
	Credentials  types.Int64   `tfsdk:"credentials"`
	Direction    types.String  `tfsdk:"direction"`     // PUSH, PULL
	TransferMode types.String  `tfsdk:"transfer_mode"` // SYNC, COPY, MOVE
	Attributes   types.String  `tfsdk:"attributes"`    // JSON: {"bucket": "...", "folder": "..."}
	Schedule     ScheduleModel `tfsdk:"schedule"`
	Enabled      types.Bool    `tfsdk:"enabled"`
	Snapshot     types.Bool    `tfsdk:"snapshot"`
	Include      types.List    `tfsdk:"include"` // List[String]
	Exclude      types.List    `tfsdk:"exclude"` // List[String]
	PreScript    types.String  `tfsdk:"pre_script"`
	PostScript   types.String  `tfsdk:"post_script"`
}

// cloudSyncAPI is the JSON shape returned by cloudsync.* methods.
//
// Credentials is decoded as json.RawMessage because the API has been
// observed (and is documented) to return either an embedded credentials
// object ({"id": N, ...}) from query/get_instance, or a bare integer ID.
// decodeCredentialsID handles both shapes.
type cloudSyncAPI struct {
	ID           int64           `json:"id"`
	Description  string          `json:"description"`
	Path         string          `json:"path"`
	Credentials  json.RawMessage `json:"credentials"`
	Direction    string          `json:"direction"`
	TransferMode string          `json:"transfer_mode"`
	Attributes   map[string]any  `json:"attributes"`
	Schedule     struct {
		Minute string `json:"minute"`
		Hour   string `json:"hour"`
		Dom    string `json:"dom"`
		Month  string `json:"month"`
		Dow    string `json:"dow"`
	} `json:"schedule"`
	Enabled    bool     `json:"enabled"`
	Snapshot   bool     `json:"snapshot"`
	Include    []string `json:"include"`
	Exclude    []string `json:"exclude"`
	PreScript  string   `json:"pre_script"`
	PostScript string   `json:"post_script"`
}

// decodeCredentialsID decodes the "credentials" field of a cloudSyncAPI
// response, which may be an embedded object ({"id": N, ...}) or a bare
// integer ID depending on the calling method / API version.
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
func (m *CloudSyncModel) attributesMap() (map[string]any, diag.Diagnostics) {
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
func apiAttributesJSON(api *cloudSyncAPI) (string, diag.Diagnostics) {
	var diags diag.Diagnostics
	b, err := json.Marshal(api.Attributes)
	if err != nil {
		diags.AddError("Failed to marshal cloud sync attributes", err.Error())
		return "", diags
	}
	return string(b), diags
}

// apiPayload converts the Terraform model into the map expected by
// cloudsync.create / cloudsync.update.
func (m *CloudSyncModel) apiPayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	var attrs map[string]any
	if err := json.Unmarshal([]byte(m.Attributes.ValueString()), &attrs); err != nil {
		diags.AddError("Invalid attributes JSON", err.Error())
		return nil, diags
	}
	var include, exclude []string
	if !m.Include.IsNull() && !m.Include.IsUnknown() {
		diags.Append(m.Include.ElementsAs(ctx, &include, false)...)
	}
	if include == nil {
		include = []string{}
	}
	if !m.Exclude.IsNull() && !m.Exclude.IsUnknown() {
		diags.Append(m.Exclude.ElementsAs(ctx, &exclude, false)...)
	}
	if exclude == nil {
		exclude = []string{}
	}
	if diags.HasError() {
		return nil, diags
	}
	p := map[string]any{
		"description":   m.Description.ValueString(),
		"path":          m.Path.ValueString(),
		"credentials":   m.Credentials.ValueInt64(),
		"direction":     m.Direction.ValueString(),
		"transfer_mode": m.TransferMode.ValueString(),
		"attributes":    attrs,
		"schedule": map[string]string{
			"minute": m.Schedule.Minute.ValueString(),
			"hour":   m.Schedule.Hour.ValueString(),
			"dom":    m.Schedule.Dom.ValueString(),
			"month":  m.Schedule.Month.ValueString(),
			"dow":    m.Schedule.Dow.ValueString(),
		},
		"include": include,
		"exclude": exclude,
	}

	// Optional+Computed scalars: send only when the user has set a value.
	// When unset, the plan value is Unknown and ValueBool()/ValueString()
	// would return zero values, silently sending wrong data (e.g.
	// "enabled": false disables the task).
	if !m.Enabled.IsNull() && !m.Enabled.IsUnknown() {
		p["enabled"] = m.Enabled.ValueBool()
	}
	if !m.Snapshot.IsNull() && !m.Snapshot.IsUnknown() {
		p["snapshot"] = m.Snapshot.ValueBool()
	}
	if !m.PreScript.IsNull() && !m.PreScript.IsUnknown() {
		p["pre_script"] = m.PreScript.ValueString()
	}
	if !m.PostScript.IsNull() && !m.PostScript.IsUnknown() {
		p["post_script"] = m.PostScript.ValueString()
	}

	return p, diags
}

// responseToModel maps a cloudSyncAPI response into a CloudSyncModel. It
// deliberately does not touch m.Attributes so callers can implement
// write-what-you-said / drift-aware semantics for that field.
func responseToModel(ctx context.Context, api *cloudSyncAPI, m *CloudSyncModel) diag.Diagnostics {
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

	m.Direction = types.StringValue(api.Direction)
	m.TransferMode = types.StringValue(api.TransferMode)
	m.Schedule = ScheduleModel{
		Minute: types.StringValue(api.Schedule.Minute),
		Hour:   types.StringValue(api.Schedule.Hour),
		Dom:    types.StringValue(api.Schedule.Dom),
		Month:  types.StringValue(api.Schedule.Month),
		Dow:    types.StringValue(api.Schedule.Dow),
	}
	m.Enabled = types.BoolValue(api.Enabled)
	m.Snapshot = types.BoolValue(api.Snapshot)

	includeList, d := types.ListValueFrom(ctx, types.StringType, api.Include)
	diags.Append(d...)
	m.Include = includeList

	excludeList, d := types.ListValueFrom(ctx, types.StringType, api.Exclude)
	diags.Append(d...)
	m.Exclude = excludeList

	m.PreScript = types.StringValue(api.PreScript)
	m.PostScript = types.StringValue(api.PostScript)

	return diags
}

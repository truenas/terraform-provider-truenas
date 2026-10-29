// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cloudsync

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// bwlimitModel is one bandwidth-limit schedule entry.
type bwlimitModel struct {
	Time      types.String `tfsdk:"time"`
	Bandwidth types.Int64  `tfsdk:"bandwidth"`
}

// bwlimitAPI is the wire shape of a bandwidth-limit entry.
type bwlimitAPI struct {
	Time      string `json:"time"`
	Bandwidth *int64 `json:"bandwidth"`
}

var bwlimitAttrTypes = map[string]attr.Type{
	"time":      types.StringType,
	"bandwidth": types.Int64Type,
}

func bwlimitObjectType() types.ObjectType { return types.ObjectType{AttrTypes: bwlimitAttrTypes} }

// injectCryptSecrets adds the write-only encryption_password/encryption_salt to
// a create/update payload from the config model. The framework nulls WriteOnly
// attributes in the plan, so the resource reads them from req.Config and passes
// the config model here.
func injectCryptSecrets(payload map[string]any, cfg *CloudSyncModel) {
	if !cfg.EncryptionPassword.IsNull() && !cfg.EncryptionPassword.IsUnknown() {
		payload["encryption_password"] = cfg.EncryptionPassword.ValueString()
	}
	if !cfg.EncryptionSalt.IsNull() && !cfg.EncryptionSalt.IsUnknown() {
		payload["encryption_salt"] = cfg.EncryptionSalt.ValueString()
	}
}

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
}

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
	ID           types.Int64  `tfsdk:"id"`
	Description  types.String `tfsdk:"description"`
	Path         types.String `tfsdk:"path"`
	Credentials  types.Int64  `tfsdk:"credentials"`
	Direction    types.String `tfsdk:"direction"`     // PUSH, PULL
	TransferMode types.String `tfsdk:"transfer_mode"` // SYNC, COPY, MOVE
	Attributes   types.String `tfsdk:"attributes"`    // JSON: {"bucket": "...", "folder": "..."}
	Schedule     types.Object `tfsdk:"schedule"`      // ScheduleModel
	Enabled      types.Bool   `tfsdk:"enabled"`
	Snapshot     types.Bool   `tfsdk:"snapshot"`
	Include      types.List   `tfsdk:"include"` // List[String]
	Exclude      types.List   `tfsdk:"exclude"` // List[String]
	PreScript    types.String `tfsdk:"pre_script"`
	PostScript   types.String `tfsdk:"post_script"`

	// Transfer options (coverage audit).
	Transfers          types.Int64 `tfsdk:"transfers"` // null = rclone default
	FollowSymlinks     types.Bool  `tfsdk:"follow_symlinks"`
	CreateEmptySrcDirs types.Bool  `tfsdk:"create_empty_src_dirs"`

	// Client-side encryption (rclone crypt). Password/salt are write-only
	// secrets (never read back into state); read from req.Config, not Plan.
	Encryption         types.Bool   `tfsdk:"encryption"`
	FilenameEncryption types.Bool   `tfsdk:"filename_encryption"`
	EncryptionPassword types.String `tfsdk:"encryption_password"` // write-only
	EncryptionSalt     types.String `tfsdk:"encryption_salt"`     // write-only

	// Bandwidth-limit schedule: list of {time "HH:MM", bandwidth bytes/sec}.
	Bwlimit types.List `tfsdk:"bwlimit"`
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

	// Transfer options (coverage audit).
	Transfers          *int64 `json:"transfers"`
	FollowSymlinks     bool   `json:"follow_symlinks"`
	CreateEmptySrcDirs bool   `json:"create_empty_src_dirs"`

	// Client-side encryption (password/salt are write-only, not read back).
	Encryption         bool `json:"encryption"`
	FilenameEncryption bool `json:"filename_encryption"`

	Bwlimit []bwlimitAPI `json:"bwlimit"`
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
	var sched ScheduleModel
	diags.Append(m.Schedule.As(ctx, &sched, basetypes.ObjectAsOptions{})...)
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
			"minute": sched.Minute.ValueString(),
			"hour":   sched.Hour.ValueString(),
			"dom":    sched.Dom.ValueString(),
			"month":  sched.Month.ValueString(),
			"dow":    sched.Dow.ValueString(),
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
	// Transfer options (coverage audit).
	if !m.Transfers.IsNull() && !m.Transfers.IsUnknown() {
		p["transfers"] = m.Transfers.ValueInt64()
	}
	if !m.FollowSymlinks.IsNull() && !m.FollowSymlinks.IsUnknown() {
		p["follow_symlinks"] = m.FollowSymlinks.ValueBool()
	}
	if !m.CreateEmptySrcDirs.IsNull() && !m.CreateEmptySrcDirs.IsUnknown() {
		p["create_empty_src_dirs"] = m.CreateEmptySrcDirs.ValueBool()
	}
	// Client-side encryption toggles (password/salt are write-only and are
	// injected from req.Config by the resource, not from the plan).
	if !m.Encryption.IsNull() && !m.Encryption.IsUnknown() {
		p["encryption"] = m.Encryption.ValueBool()
	}
	if !m.FilenameEncryption.IsNull() && !m.FilenameEncryption.IsUnknown() {
		p["filename_encryption"] = m.FilenameEncryption.ValueBool()
	}
	// bwlimit schedule.
	if !m.Bwlimit.IsNull() && !m.Bwlimit.IsUnknown() {
		var entries []bwlimitModel
		diags.Append(m.Bwlimit.ElementsAs(ctx, &entries, false)...)
		bw := make([]map[string]any, 0, len(entries))
		for _, e := range entries {
			item := map[string]any{"time": e.Time.ValueString()}
			if !e.Bandwidth.IsNull() && !e.Bandwidth.IsUnknown() {
				item["bandwidth"] = e.Bandwidth.ValueInt64()
			} else {
				item["bandwidth"] = nil
			}
			bw = append(bw, item)
		}
		p["bwlimit"] = bw
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
	sched, d := types.ObjectValueFrom(ctx, scheduleAttrTypes, ScheduleModel{
		Minute: types.StringValue(api.Schedule.Minute),
		Hour:   types.StringValue(api.Schedule.Hour),
		Dom:    types.StringValue(api.Schedule.Dom),
		Month:  types.StringValue(api.Schedule.Month),
		Dow:    types.StringValue(api.Schedule.Dow),
	})
	diags.Append(d...)
	m.Schedule = sched
	m.Enabled = types.BoolValue(api.Enabled)
	m.Snapshot = types.BoolValue(api.Snapshot)

	// Client-side encryption toggles round-trip; password/salt are write-only
	// and are never read back (left as the plan/state value, i.e. null).
	m.Encryption = types.BoolValue(api.Encryption)
	m.FilenameEncryption = types.BoolValue(api.FilenameEncryption)
	bwEntries := make([]bwlimitModel, 0, len(api.Bwlimit))
	for _, b := range api.Bwlimit {
		e := bwlimitModel{Time: types.StringValue(b.Time)}
		if b.Bandwidth != nil {
			e.Bandwidth = types.Int64Value(*b.Bandwidth)
		} else {
			e.Bandwidth = types.Int64Null()
		}
		bwEntries = append(bwEntries, e)
	}
	bwList, dbw := types.ListValueFrom(ctx, bwlimitObjectType(), bwEntries)
	diags.Append(dbw...)
	m.Bwlimit = bwList

	// Transfer options (coverage audit).
	if api.Transfers != nil {
		m.Transfers = types.Int64Value(*api.Transfers)
	} else {
		m.Transfers = types.Int64Null()
	}
	m.FollowSymlinks = types.BoolValue(api.FollowSymlinks)
	m.CreateEmptySrcDirs = types.BoolValue(api.CreateEmptySrcDirs)

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

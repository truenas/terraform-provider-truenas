// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package webshare

import (
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

// webshareVersionFloorMajor/Minor is the minimum TrueNAS release that
// exposes the sharing.webshare.* namespace at all. Probed live: TrueNAS 25.10
// returns 0 sharing.webshare.* (and webshare.*) methods from
// core.get_methods (namespace genuinely absent); TrueNAS 26.0 supports the
// full namespace (sharing.webshare.create/update/delete/get_instance/query,
// all job:false, confirmed live this session — see resource.go's
// checkVersion for where this gates every entry point before any
// sharing.webshare.* call reaches the wire).
const (
	webshareVersionFloorMajor = 26
	webshareVersionFloorMinor = 0
)

// versionGateDiagnostics reports whether the given TrueNAS release string
// (as returned by system.version_short via client.ServerVersion) is at or
// above the TrueNAS 26.0 floor the sharing.webshare namespace requires,
// returning a single clean error diagnostic when it is not. Pure function
// of an already-probed version string (no live client access), matching
// the lxc_config/container precedent so it stays independently
// unit-testable.
func versionGateDiagnostics(version string) diag.Diagnostics {
	var diags diag.Diagnostics
	if !client.VersionAtLeastString(version, webshareVersionFloorMajor, webshareVersionFloorMinor) {
		diags.AddError(
			"TrueNAS version too old",
			"truenas_webshare requires TrueNAS 26.0 or later",
		)
	}
	return diags
}

// WebshareModel is the Terraform state/plan model for truenas_webshare.
//
// Unlike truenas_smb_share/truenas_nfs_share, sharing.webshare has no
// "comment" field at all (probed live: sharing.webshare.create rejects an
// extra "comment" key with a clean EINVAL — "data.comment: Extra inputs are
// not permitted"). Enabled is this resource's cosmetic in-place-update
// field instead (mirrors smb's "abe"/nfs's "ro" role in their own
// acceptance tests).
//
// Dataset/RelativePath/Locked are read-only server-derived fields, and are
// nullable at the API level (probed live: dataset/relative_path come back
// null immediately after create/get_instance — the middleware appears to
// resolve them lazily, populated after the first update — while locked is
// null only when lock-status wasn't requested/available; none of that is a
// bug this resource needs to work around, since the schema already allows
// null for all three, but it does mean a freshly created share may briefly
// read back with dataset/relative_path null before the next Read/Update
// resolves them).
type WebshareModel struct {
	ID           types.Int64  `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Path         types.String `tfsdk:"path"`
	Enabled      types.Bool   `tfsdk:"enabled"`
	IsHomeBase   types.Bool   `tfsdk:"is_home_base"`
	Dataset      types.String `tfsdk:"dataset"`
	RelativePath types.String `tfsdk:"relative_path"`
	Locked       types.Bool   `tfsdk:"locked"`
}

// WebshareDataSourceModel is the read-only model for the truenas_webshare
// datasource, looked up by name.
type WebshareDataSourceModel struct {
	ID           types.Int64  `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Path         types.String `tfsdk:"path"`
	Enabled      types.Bool   `tfsdk:"enabled"`
	IsHomeBase   types.Bool   `tfsdk:"is_home_base"`
	Dataset      types.String `tfsdk:"dataset"`
	RelativePath types.String `tfsdk:"relative_path"`
	Locked       types.Bool   `tfsdk:"locked"`
}

// webshareAPI mirrors the JSON object returned by sharing.webshare.create,
// sharing.webshare.update, sharing.webshare.get_instance, and
// sharing.webshare.query. Probed live against TrueNAS 26.0 (the only release
// that exposes this namespace — see versionGateDiagnostics):
//
//	{"id": 2, "name": "tf-probe-webshare", "path": "/mnt/tank/tf-probe-webshare-ds",
//	 "dataset": null, "relative_path": null, "enabled": true, "is_home_base": false, "locked": false}
//
// dataset/relative_path/locked are all nullable per the API's own schema
// (anyOf string|null, anyOf string|null, anyOf boolean|null respectively).
type webshareAPI struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	Path         string  `json:"path"`
	Dataset      *string `json:"dataset"`
	RelativePath *string `json:"relative_path"`
	Enabled      bool    `json:"enabled"`
	IsHomeBase   bool    `json:"is_home_base"`
	Locked       *bool   `json:"locked"`
}

// responseToModel maps an API response onto a Terraform model.
func responseToModel(api *webshareAPI, m *WebshareModel) {
	m.ID = types.Int64Value(api.ID)
	m.Name = types.StringValue(api.Name)
	m.Path = types.StringValue(api.Path)
	m.Enabled = types.BoolValue(api.Enabled)
	m.IsHomeBase = types.BoolValue(api.IsHomeBase)
	m.Dataset = types.StringPointerValue(api.Dataset)
	m.RelativePath = types.StringPointerValue(api.RelativePath)
	m.Locked = types.BoolPointerValue(api.Locked)
}

// responseToDataSourceModel maps an API response onto a
// WebshareDataSourceModel.
func responseToDataSourceModel(api *webshareAPI, m *WebshareDataSourceModel) {
	m.ID = types.Int64Value(api.ID)
	m.Name = types.StringValue(api.Name)
	m.Path = types.StringValue(api.Path)
	m.Enabled = types.BoolValue(api.Enabled)
	m.IsHomeBase = types.BoolValue(api.IsHomeBase)
	m.Dataset = types.StringPointerValue(api.Dataset)
	m.RelativePath = types.StringPointerValue(api.RelativePath)
	m.Locked = types.BoolPointerValue(api.Locked)
}

// createPayload builds the map[string]any payload for sharing.webshare.create.
// "name" and "path" are always included (Required in schema). "enabled" and
// "is_home_base" are Optional+Computed with schema Defaults (true/false,
// matching the API's own probed defaults) so they are always known by plan
// time — no unknown-guard is needed for them, unlike smb/nfs's plain
// optional fields that lack defaults.
func (m *WebshareModel) createPayload() map[string]any {
	return map[string]any{
		"name":         m.Name.ValueString(),
		"path":         m.Path.ValueString(),
		"enabled":      m.Enabled.ValueBool(),
		"is_home_base": m.IsHomeBase.ValueBool(),
	}
}

// updatePayload builds the map[string]any payload for sharing.webshare.update.
// Same field set as createPayload (sharing.webshare.update accepts the same
// name/path/enabled/is_home_base keys, probed live) with the same
// always-known rationale (Defaults on enabled/is_home_base). "path" is
// included even though schema.go marks it RequiresReplace (see schema.go's
// doc comment) purely for symmetry with createPayload/smb's convention —
// RequiresReplace means Update is never actually called with a changed
// path.
func (m *WebshareModel) updatePayload() map[string]any {
	return map[string]any{
		"name":         m.Name.ValueString(),
		"path":         m.Path.ValueString(),
		"enabled":      m.Enabled.ValueBool(),
		"is_home_base": m.IsHomeBase.ValueBool(),
	}
}

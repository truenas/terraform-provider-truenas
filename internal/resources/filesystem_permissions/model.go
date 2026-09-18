// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package filesystem_permissions

import (
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// FilesystemPermissionsModel is the Terraform state/plan model for a
// truenas_filesystem_permissions resource.
type FilesystemPermissionsModel struct {
	ID        types.String `tfsdk:"id"`
	Path      types.String `tfsdk:"path"`
	Mode      types.String `tfsdk:"mode"`
	UID       types.Int64  `tfsdk:"uid"`
	GID       types.Int64  `tfsdk:"gid"`
	Recursive types.Bool   `tfsdk:"recursive"`
	Traverse  types.Bool   `tfsdk:"traverse"`
}

// FilesystemPermissionsDataSourceModel is the read-only lookup model for
// the truenas_filesystem_permissions datasource (looked up by path).
type FilesystemPermissionsDataSourceModel struct {
	ID   types.String `tfsdk:"id"`
	Path types.String `tfsdk:"path"`
	Mode types.String `tfsdk:"mode"`
	UID  types.Int64  `tfsdk:"uid"`
	GID  types.Int64  `tfsdk:"gid"`
}

// modeRegexp matches a 3- or 4-digit octal mode string, each digit 0-7.
func modeRegexp() *regexp.Regexp {
	return regexp.MustCompile(`^[0-7]{3,4}$`)
}

// fsStatAPI is the subset of filesystem.stat's response this resource
// cares about (probed live via core.get_methods + a real setperm/stat
// round trip on TrueNAS 25.10). filesystem.stat is job:false
// (synchronous plain Call). "mode" is the full stat(2) st_mode integer
// (file-type bits included, e.g. a directory's mode came back as decimal
// 16872 for permission bits 0750 = 0o40750) — see permModeString.
// filesystem.stat on a missing path raises CallError(ENOENT); probed live
// it arrives as APIError{Code: 2, Message: "[ENOENT] Path ... not found"},
// which client.IsNotFound already recognizes (Code == 2).
type fsStatAPI struct {
	Mode int64 `json:"mode"`
	UID  int64 `json:"uid"`
	GID  int64 `json:"gid"`
}

// permModeString masks stMode to the low 12 bits (the actual UNIX
// permission bits: setuid/setgid/sticky + rwxrwxrwx) and formats it as a
// 4-digit, zero-padded octal string, e.g. 0o40750 (a directory) -> "0750".
// This is the inverse of what a user supplies to filesystem.setperm's
// "mode" (a bare octal permission string, no file-type bits).
func permModeString(stMode int64) string {
	return fmt.Sprintf("%04o", stMode&0o7777)
}

// setpermPayload builds the map expected by filesystem.setperm. "path" is
// always included; "mode"/"uid"/"gid" are included only when set in the
// model (filesystem.setperm treats a null/omitted value as "leave
// unchanged", probed live via core.get_methods), and "options" always
// carries recursive/traverse (both default false server-side; sending them
// explicitly is harmless and keeps the payload deterministic in tests).
func (m *FilesystemPermissionsModel) setpermPayload() map[string]any {
	p := map[string]any{
		"path": m.Path.ValueString(),
		"options": map[string]any{
			"recursive": !m.Recursive.IsNull() && !m.Recursive.IsUnknown() && m.Recursive.ValueBool(),
			"traverse":  !m.Traverse.IsNull() && !m.Traverse.IsUnknown() && m.Traverse.ValueBool(),
		},
	}
	if !m.Mode.IsNull() && !m.Mode.IsUnknown() {
		p["mode"] = m.Mode.ValueString()
	}
	if !m.UID.IsNull() && !m.UID.IsUnknown() {
		p["uid"] = m.UID.ValueInt64()
	}
	if !m.GID.IsNull() && !m.GID.IsUnknown() {
		p["gid"] = m.GID.ValueInt64()
	}
	return p
}

// hasWrite reports whether the model specifies any of mode/uid/gid, i.e.
// whether calling filesystem.setperm would actually change anything.
func (m *FilesystemPermissionsModel) hasWrite() bool {
	return (!m.Mode.IsNull() && !m.Mode.IsUnknown()) ||
		(!m.UID.IsNull() && !m.UID.IsUnknown()) ||
		(!m.GID.IsNull() && !m.GID.IsUnknown())
}

// responseToModel maps an fsStatAPI response's mode/uid/gid into the
// model, plus id/path. It deliberately never touches Recursive/Traverse:
// those are apply-time-only instructions with nothing on the wire to read
// them back from (see schema.go).
func responseToModel(api *fsStatAPI, path string, m *FilesystemPermissionsModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.StringValue(path)
	m.Path = types.StringValue(path)
	m.Mode = types.StringValue(permModeString(api.Mode))
	m.UID = types.Int64Value(api.UID)
	m.GID = types.Int64Value(api.GID)
	return diags
}

// responseToDataSourceModel maps an fsStatAPI response onto a
// FilesystemPermissionsDataSourceModel.
func responseToDataSourceModel(api *fsStatAPI, path string, m *FilesystemPermissionsDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.StringValue(path)
	m.Path = types.StringValue(path)
	m.Mode = types.StringValue(permModeString(api.Mode))
	m.UID = types.Int64Value(api.UID)
	m.GID = types.Int64Value(api.GID)
	return diags
}

// deleteWarningDiagnostics builds the warning diagnostic emitted by
// Delete. Delete makes NO client calls: unlike most resources, a
// filesystem path's mode/owner/group are not an object TrueNAS creates or
// destroys — they exist independently of Terraform, so removing this
// resource from Terraform state must never reset them. This is the
// documented Delete semantic (verified live in the acceptance test): after
// destroy, the path's permissions remain exactly as last applied.
func deleteWarningDiagnostics(path string) diag.Diagnostics {
	var diags diag.Diagnostics
	diags.AddWarning(
		"Filesystem permissions left in place",
		fmt.Sprintf(
			"truenas_filesystem_permissions for %q removed from Terraform state only; "+
				"the path's mode/uid/gid were NOT reverted and remain as last applied.",
			path,
		),
	)
	return diags
}

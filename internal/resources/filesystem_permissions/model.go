// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package filesystem_permissions

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/normalize"
)

// FilesystemPermissionsModel is the Terraform state/plan model for a
// truenas_filesystem_permissions resource.
type FilesystemPermissionsModel struct {
	ID        types.String        `tfsdk:"id"`
	Path      types.String        `tfsdk:"path"`
	Mode      normalize.OctalMode `tfsdk:"mode"`
	UID       types.Int64         `tfsdk:"uid"`
	GID       types.Int64         `tfsdk:"gid"`
	Recursive types.Bool          `tfsdk:"recursive"`
	Traverse  types.Bool          `tfsdk:"traverse"`
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
// always included and "options" always carries recursive/traverse (both
// default false server-side; sending them explicitly is harmless and keeps
// the payload deterministic in tests).
//
// "mode"/"uid"/"gid" are sent only when they CHANGE: on create when
// configured (an unconfigured one is unknown in the plan), on update when the
// planned value differs from prior state. Unconfigured and unchanged values
// are carried into every update plan by UseStateForUnknown, and sending them
// back is not a no-op: with options.recursive the middleware applies each one
// to every file under the path, so changing only the mode re-owned the whole
// tree to the root's owner, and changing only the owner gave every file the
// directory's mode (#67 class, verified on 25.10, 26.0 and 28.0). Omitted,
// filesystem.setperm leaves that property of each file alone. prior is nil on
// create.
func (m *FilesystemPermissionsModel) setpermPayload(prior *FilesystemPermissionsModel) map[string]any {
	p := map[string]any{
		"path": m.Path.ValueString(),
		"options": map[string]any{
			"recursive": !m.Recursive.IsNull() && !m.Recursive.IsUnknown() && m.Recursive.ValueBool(),
			"traverse":  !m.Traverse.IsNull() && !m.Traverse.IsUnknown() && m.Traverse.ValueBool(),
		},
	}
	if m.modeChanged(prior) {
		p["mode"] = m.Mode.ValueString()
	}
	if idChanged(m.UID, prior, func(pm *FilesystemPermissionsModel) types.Int64 { return pm.UID }) {
		p["uid"] = m.UID.ValueInt64()
	}
	if idChanged(m.GID, prior, func(pm *FilesystemPermissionsModel) types.Int64 { return pm.GID }) {
		p["gid"] = m.GID.ValueInt64()
	}
	return p
}

// modeChanged reports a known planned mode that differs from prior state as
// permission bits ("750" and "0750" are the same mode).
func (m *FilesystemPermissionsModel) modeChanged(prior *FilesystemPermissionsModel) bool {
	if m.Mode.IsNull() || m.Mode.IsUnknown() {
		return false
	}
	if prior == nil || prior.Mode.IsNull() || prior.Mode.IsUnknown() {
		return true
	}
	return strings.TrimLeft(m.Mode.ValueString(), "0") != strings.TrimLeft(prior.Mode.ValueString(), "0")
}

// idChanged reports a known planned uid/gid that differs from prior state.
func idChanged(planned types.Int64, prior *FilesystemPermissionsModel, get func(*FilesystemPermissionsModel) types.Int64) bool {
	if planned.IsNull() || planned.IsUnknown() {
		return false
	}
	if prior == nil {
		return true
	}
	pv := get(prior)
	return pv.IsNull() || pv.IsUnknown() || planned.ValueInt64() != pv.ValueInt64()
}

// writeCall picks the API call for a change. filesystem.setperm needs a mode
// ("Payload must either explicitly specify permissions or contain the stripacl
// option" on 25.10, 26.0 and 28.0), so a change of owner alone goes through
// filesystem.chown, which takes the same uid/gid/options and leaves every
// file's mode alone. Returns "" when nothing changes.
func (m *FilesystemPermissionsModel) writeCall(prior *FilesystemPermissionsModel) (method string, payload map[string]any) {
	p := m.setpermPayload(prior)
	_, mode := p["mode"]
	_, uid := p["uid"]
	_, gid := p["gid"]
	switch {
	case mode:
		return "filesystem.setperm", p
	case uid || gid:
		return "filesystem.chown", p
	}
	return "", nil
}

// hasWrite reports whether the payload would change anything, i.e. whether
// a write call is needed at all.
func (m *FilesystemPermissionsModel) hasWrite(prior *FilesystemPermissionsModel) bool {
	p := m.setpermPayload(prior)
	_, mode := p["mode"]
	_, uid := p["uid"]
	_, gid := p["gid"]
	return mode || uid || gid
}

// responseToModel maps an fsStatAPI response's mode/uid/gid into the
// model, plus id/path. It deliberately never touches Recursive/Traverse:
// those are apply-time-only instructions with nothing on the wire to read
// them back from (see schema.go).
func responseToModel(api *fsStatAPI, path string, m *FilesystemPermissionsModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.StringValue(path)
	m.Path = types.StringValue(path)
	// mode is an OctalMode: "755" in the configuration equals the "0755" read
	// back, and state keeps the configured spelling. (B-091)
	m.Mode = normalize.NewOctalModeValue(permModeString(api.Mode))
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

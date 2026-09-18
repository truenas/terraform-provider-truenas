// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package filesystem_acl

import (
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// FilesystemAclModel is the Terraform state/plan model for a
// truenas_filesystem_acl resource: a path-keyed declarative wrapper around
// the imperative filesystem.setacl action, mirroring
// truenas_filesystem_permissions' path-keyed pattern and truenas_acl_
// template's canonical-JSON "entries" modeling (Task 8's ACE union shapes,
// re-probed live here against filesystem.getacl/setacl directly rather than
// imported, since acl_template's Go types are unexported by design).
type FilesystemAclModel struct {
	ID        types.String `tfsdk:"id"`
	Path      types.String `tfsdk:"path"`
	ACLType   types.String `tfsdk:"acltype"`
	Entries   types.String `tfsdk:"entries"` // JSON array of ACEs; see schema.go
	UID       types.Int64  `tfsdk:"uid"`
	GID       types.Int64  `tfsdk:"gid"`
	Recursive types.Bool   `tfsdk:"recursive"`
	Traverse  types.Bool   `tfsdk:"traverse"`
}

// FilesystemAclDataSourceModel is the read-only lookup model for the
// truenas_filesystem_acl datasource (looked up by path).
type FilesystemAclDataSourceModel struct {
	ID      types.String `tfsdk:"id"`
	Path    types.String `tfsdk:"path"`
	ACLType types.String `tfsdk:"acltype"`
	Entries types.String `tfsdk:"entries"`
	UID     types.Int64  `tfsdk:"uid"`
	GID     types.Int64  `tfsdk:"gid"`
}

// fsGetAclAPI is the subset of filesystem.getacl's response this resource
// cares about (probed live via core.get_methods + a real getacl/setacl
// round trip against TrueNAS 25.10). filesystem.getacl is job:false
// (plain synchronous Call), accepting (path, simplified=true, resolve_ids
// =false) - defaults are used here (only "path" is sent). The full
// response also carries "user"/"group" (nullable name lookups, only
// populated when resolve_ids=true), "aclflags" (NFS4-only autoinherit/
// protected/defaulted), and "trivial" (bool) - none of those are modeled
// by this resource. filesystem.setacl's job result carries this exact same
// shape (probed live), so Create/Update parse the CallJob result directly
// instead of paying for a separate getacl round trip.
type fsGetAclAPI struct {
	Path    string          `json:"path"`
	UID     int64           `json:"uid"`
	GID     int64           `json:"gid"`
	ACLType string          `json:"acltype"`
	ACL     json.RawMessage `json:"acl"`
}

// entriesNormalized parses raw into a slice of generic ACE maps and strips
// cosmetic noise the server introduces but that never reflects a real
// configuration difference: an "id" of null or -1 (both observed live on
// filesystem.getacl for entries where tag isn't USER/GROUP - a real
// uid/gid is never -1) and a "who" of null. Normalization deliberately does
// NOT sort entries, so server-side reordering (seen with POSIX1E) surfaces as
// drift; keep-order rationale: entry order is semantically significant for
// ACL evaluation. Mirrors acl_template's aclEntriesNormalized (that package's
// types are unexported and not meant to be imported - this is the same
// normalization re-implemented against this resource's own probe of
// filesystem.getacl/setacl).
func entriesNormalized(raw json.RawMessage) ([]map[string]any, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty ACL entries JSON")
	}
	var entries []map[string]any
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, err
	}
	for _, e := range entries {
		if idv, ok := e["id"]; ok {
			if idv == nil {
				delete(e, "id")
			} else if f, ok := idv.(float64); ok && f == -1 {
				delete(e, "id")
			}
		}
		if whov, ok := e["who"]; ok && whov == nil {
			delete(e, "who")
		}
	}
	return entries, nil
}

// canonicalEntriesJSON marshals raw's normalized entries back to a JSON
// string, for use as the state value when the API's content is what should
// be stored (import, or a genuine drift on read).
func canonicalEntriesJSON(raw json.RawMessage) (string, diag.Diagnostics) {
	var diags diag.Diagnostics
	entries, err := entriesNormalized(raw)
	if err != nil {
		diags.AddError("Invalid ACL entries JSON in API response", err.Error())
		return "", diags
	}
	b, err := json.Marshal(entries)
	if err != nil {
		diags.AddError("Failed to marshal ACL entries", err.Error())
		return "", diags
	}
	return string(b), diags
}

// entriesDrifted reports whether the API's ACL content (after
// normalization) differs from the currently-stored state's entries content
// (also normalized). Used to implement write-what-you-said: the plan's
// exact "entries" JSON text is kept in state as-is unless the underlying
// entries genuinely changed.
func entriesDrifted(stateEntries string, apiACL json.RawMessage) (bool, diag.Diagnostics) {
	var diags diag.Diagnostics
	stateNorm, err := entriesNormalized(json.RawMessage(stateEntries))
	if err != nil {
		diags.AddError("Invalid ACL entries JSON in state", err.Error())
		return false, diags
	}
	apiNorm, err := entriesNormalized(apiACL)
	if err != nil {
		diags.AddError("Invalid ACL entries JSON in API response", err.Error())
		return false, diags
	}
	sb, _ := json.Marshal(stateNorm)
	ab, _ := json.Marshal(apiNorm)
	return string(sb) != string(ab), diags
}

// setaclPayload builds the map expected by filesystem.setacl for Create/
// Update. "path" and "dacl" are always included (both required per
// core.get_methods); "options.recursive"/"options.traverse" always sent
// (default false server-side, sent explicitly for deterministic tests) and
// "options.stripacl" is always false here (true is reserved for Delete,
// see deleteACLPayload). "uid"/"gid" are included only when set in the
// model (filesystem.setacl treats a null/omitted value as "leave
// unchanged" per its description text, mirroring filesystem.setperm).
func (m *FilesystemAclModel) setaclPayload() map[string]any {
	p := map[string]any{
		"path": m.Path.ValueString(),
		"dacl": json.RawMessage(m.Entries.ValueString()),
		"options": map[string]any{
			"recursive": !m.Recursive.IsNull() && !m.Recursive.IsUnknown() && m.Recursive.ValueBool(),
			"traverse":  !m.Traverse.IsNull() && !m.Traverse.IsUnknown() && m.Traverse.ValueBool(),
			"stripacl":  false,
		},
	}
	if !m.UID.IsNull() && !m.UID.IsUnknown() {
		p["uid"] = m.UID.ValueInt64()
	}
	if !m.GID.IsNull() && !m.GID.IsUnknown() {
		p["gid"] = m.GID.ValueInt64()
	}
	return p
}

// deleteACLPayload builds the filesystem.setacl payload used by Delete:
// options.stripacl=true converts the path's ACL to a trivial, mode-derived
// one (probed live and confirmed clean on both NFS4 and POSIX1E - see
// resource.go's Delete doc comment and the task report for the verbatim
// before/after evidence). "dacl" must still be present (required by the
// API) but its content is irrelevant when stripacl=true, so an empty array
// is sent. uid/gid/recursive/traverse are deliberately omitted: Delete
// only ever strips the ACL on the exact path it manages, never cascades
// ownership or recursion changes.
func deleteACLPayload(path string) map[string]any {
	return map[string]any{
		"path": path,
		"dacl": []any{},
		"options": map[string]any{
			"stripacl": true,
		},
	}
}

// responseToModel maps an fsGetAclAPI response into the model's id/path/
// acltype/uid/gid. It deliberately never touches m.Entries: callers should
// use entriesDrifted first and only replace it with
// canonicalEntriesJSON(api.ACL) when there's a genuine difference (write-
// what-you-said, mirrors acl_template's responseToModel / cloud_backup's
// Attributes handling).
func responseToModel(api *fsGetAclAPI, m *FilesystemAclModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.StringValue(api.Path)
	m.Path = types.StringValue(api.Path)
	m.ACLType = types.StringValue(api.ACLType)
	m.UID = types.Int64Value(api.UID)
	m.GID = types.Int64Value(api.GID)
	return diags
}

// responseToDataSourceModel maps an fsGetAclAPI response onto a
// FilesystemAclDataSourceModel. Unlike the resource model, the datasource
// always sets Entries directly from the API (there is no prior state to
// preserve formatting from).
func responseToDataSourceModel(api *fsGetAclAPI, m *FilesystemAclDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.StringValue(api.Path)
	m.Path = types.StringValue(api.Path)
	m.ACLType = types.StringValue(api.ACLType)
	m.UID = types.Int64Value(api.UID)
	m.GID = types.Int64Value(api.GID)

	entriesJSON, d := canonicalEntriesJSON(api.ACL)
	diags.Append(d...)
	m.Entries = types.StringValue(entriesJSON)

	return diags
}

// deleteWarningDiagnostics builds the diagnostic emitted by Delete after a
// successful strip. Unlike truenas_filesystem_permissions (which makes no
// API calls on Delete and leaves permissions exactly as last applied), this
// resource DOES revert the path's ACL: options.stripacl=true converts it
// to a trivial, mode-derived ACL equivalent to its current effective
// permissions (probed live: filesystem.getacl afterward reports
// "trivial": true and filesystem.stat's "acl" field is false). This is a
// warning (not an error) so `terraform destroy` still succeeds.
func deleteWarningDiagnostics(path string) diag.Diagnostics {
	var diags diag.Diagnostics
	diags.AddWarning(
		"Filesystem ACL stripped to a trivial, mode-derived ACL",
		fmt.Sprintf(
			"truenas_filesystem_acl for %q was destroyed by calling filesystem.setacl with "+
				"options.stripacl=true, which converts the path's ACL to a trivial one fully "+
				"expressible as a UNIX mode (verified live: filesystem.getacl reports \"trivial\": "+
				"true afterward on both NFS4 and POSIX1E). The path's mode/uid/gid are NOT reverted "+
				"or removed - only the ACL's non-trivial entries are gone.",
			path,
		),
	)
	return diags
}

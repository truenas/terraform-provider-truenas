// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acl_template

import (
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// AclTemplateModel is the Terraform state/plan model for a filesystem ACL
// template.
type AclTemplateModel struct {
	ID      types.Int64  `tfsdk:"id"`
	Name    types.String `tfsdk:"name"`
	ACLType types.String `tfsdk:"acltype"`
	ACL     types.String `tfsdk:"acl"` // JSON array of ACEs; see schema.go
	Comment types.String `tfsdk:"comment"`
	Builtin types.Bool   `tfsdk:"builtin"`
}

// AclTemplateDataSourceModel is the read-only lookup model for the
// truenas_acl_template datasource (looked up by name).
type AclTemplateDataSourceModel struct {
	ID      types.Int64  `tfsdk:"id"`
	Name    types.String `tfsdk:"name"`
	ACLType types.String `tfsdk:"acltype"`
	ACL     types.String `tfsdk:"acl"`
	Comment types.String `tfsdk:"comment"`
	Builtin types.Bool   `tfsdk:"builtin"`
}

// aclTemplateAPI is the JSON shape returned by filesystem.acltemplate.*
// (create/query/get_instance/update all share this shape; probed live
// against TrueNAS 25.10 via core.get_methods and a real create/query/
// get_instance/update/delete round trip). filesystem.acltemplate.create/
// query/get_instance/update/delete are all job:false (synchronous, plain
// Call - no CallJob needed).
//
// Wire quirk (probed live): an ACE's "id" is null for entries built without
// an id (the 9 builtin templates), but -1 for the same kind of entry
// returned from a template THIS resource creates. Both are normalized away
// on read (see aclEntriesNormalized) so neither causes spurious drift.
type aclTemplateAPI struct {
	ID      int64           `json:"id"`
	Builtin bool            `json:"builtin"`
	Name    string          `json:"name"`
	ACLType string          `json:"acltype"`
	ACL     json.RawMessage `json:"acl"`
	Comment string          `json:"comment"`
}

// aclEntriesNormalized parses raw into a slice of generic ACE maps and
// strips cosmetic noise that the server introduces but that never reflects
// a real configuration difference:
//   - "id": drops the key when its value is null or -1 (the two sentinels
//     observed live for entries where tag isn't USER/GROUP; a real
//     uid/gid is never -1)
//   - "who": drops the key when its value is null (mirrors the same
//     "unset" convention)
//
// Used both to build the canonical JSON stored in state and to compare an
// API response against the currently-stored state (aclDrifted).
func aclEntriesNormalized(raw json.RawMessage) ([]map[string]any, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty ACL JSON")
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

// canonicalACLJSON marshals raw's normalized entries back to a JSON string,
// for use as the state value when the API's content is what should be
// stored (create/import, or a genuine drift on read).
func canonicalACLJSON(raw json.RawMessage) (string, diag.Diagnostics) {
	var diags diag.Diagnostics
	entries, err := aclEntriesNormalized(raw)
	if err != nil {
		diags.AddError("Invalid acl JSON in API response", err.Error())
		return "", diags
	}
	b, err := json.Marshal(entries)
	if err != nil {
		diags.AddError("Failed to marshal acl template entries", err.Error())
		return "", diags
	}
	return string(b), diags
}

// aclDrifted reports whether the API's acl content (after normalization)
// differs from the currently-stored state's acl content (also normalized).
// Used to implement write-what-you-said: the plan's exact "acl" JSON text
// is kept in state as-is unless the underlying entries genuinely changed.
func aclDrifted(stateACL string, apiACL json.RawMessage) (bool, diag.Diagnostics) {
	var diags diag.Diagnostics
	stateEntries, err := aclEntriesNormalized(json.RawMessage(stateACL))
	if err != nil {
		diags.AddError("Invalid acl JSON in state", err.Error())
		return false, diags
	}
	apiEntries, err := aclEntriesNormalized(apiACL)
	if err != nil {
		diags.AddError("Invalid acl JSON in API response", err.Error())
		return false, diags
	}
	sb, _ := json.Marshal(stateEntries)
	ab, _ := json.Marshal(apiEntries)
	return string(sb) != string(ab), diags
}

// apiPayload converts the model into the map expected by
// filesystem.acltemplate.create / .update. "name", "acltype", and "acl" are
// always included (Required); "comment" follows the Optional+Computed
// omit-unless-known convention used throughout this provider (e.g.
// cloud_backup's "description") so an unset comment lets the server-side
// default ("") apply rather than sending an explicit empty string that
// would mask a future server-side default change.
func (m *AclTemplateModel) apiPayload() (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	p := map[string]any{
		"name":    m.Name.ValueString(),
		"acltype": m.ACLType.ValueString(),
		"acl":     json.RawMessage(m.ACL.ValueString()),
	}
	if !m.Comment.IsNull() && !m.Comment.IsUnknown() {
		p["comment"] = m.Comment.ValueString()
	}
	return p, diags
}

// responseToModel maps an aclTemplateAPI response into an AclTemplateModel.
// It deliberately does not overwrite m.ACL unconditionally: callers should
// use aclDrifted first and only replace it with canonicalACLJSON(api.ACL)
// when there's a genuine difference (write-what-you-said, mirrors
// cloud_backup's Attributes handling).
func responseToModel(api *aclTemplateAPI, m *AclTemplateModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.Name = types.StringValue(api.Name)
	m.ACLType = types.StringValue(api.ACLType)
	m.Comment = types.StringValue(api.Comment)
	m.Builtin = types.BoolValue(api.Builtin)

	return diags
}

// responseToDataSourceModel maps an aclTemplateAPI response onto an
// AclTemplateDataSourceModel. Unlike the resource model, the datasource
// always sets ACL directly from the API (there is no prior state to
// preserve formatting from).
func responseToDataSourceModel(api *aclTemplateAPI, m *AclTemplateDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.Name = types.StringValue(api.Name)
	m.ACLType = types.StringValue(api.ACLType)
	m.Comment = types.StringValue(api.Comment)
	m.Builtin = types.BoolValue(api.Builtin)

	aclJSON, d := canonicalACLJSON(api.ACL)
	diags.Append(d...)
	m.ACL = types.StringValue(aclJSON)

	return diags
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package privilege

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// PrivilegeModel is the Terraform state/plan model for truenas_privilege.
type PrivilegeModel struct {
	ID          types.Int64  `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	LocalGroups types.List   `tfsdk:"local_groups"` // List[Int64] of local group GIDs
	DSGroups    types.List   `tfsdk:"ds_groups"`    // List[Int64] of directory-service group GIDs
	Roles       types.List   `tfsdk:"roles"`        // List[String]
	WebShell    types.Bool   `tfsdk:"web_shell"`
	// Computed only
	BuiltinName types.String `tfsdk:"builtin_name"` // null for custom (non-builtin) privileges
}

// PrivilegeDataSourceModel is the read-only model for the truenas_privilege
// datasource, looked up by name.
type PrivilegeDataSourceModel struct {
	ID          types.Int64  `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	LocalGroups types.List   `tfsdk:"local_groups"`
	DSGroups    types.List   `tfsdk:"ds_groups"`
	Roles       types.List   `tfsdk:"roles"`
	WebShell    types.Bool   `tfsdk:"web_shell"`
	BuiltinName types.String `tfsdk:"builtin_name"`
}

// groupRefAPI is the shape of one element of the "local_groups"/"ds_groups"
// arrays as returned by privilege.create/update/get_instance/query. Probed
// against a live TrueNAS 25.10 box: privilege.create accepts (and the
// brief specifies) a flat list of integer GIDs on write, but every read
// shape (create's own response included — it echoes back the full created
// object, not the request payload) embeds the full group object instead:
//
//	{"id": 115, "gid": 3000, "name": "...", "builtin": false, ...}
//
// The middleware's own OpenAPI-style schema additionally documents a second,
// rarer variant for elements that can't be resolved to a live group
// (UnmappedGroupEntry: happens for a local_groups GID whose group was since
// deleted, or a ds_groups SID that doesn't resolve to any directory-service
// group): {"gid": <int-or-null>, "sid": <string-or-null>, "group": null}.
// Both variants carry a "gid" key under the same name, so a single small
// struct capturing just that field decodes either shape — the rest of each
// variant's fields (name, builtin, sid, roles, users, ...) are irrelevant to
// this resource, which only needs to map back onto the flat GID list the
// config/schema uses.
type groupRefAPI struct {
	GID *int64 `json:"gid"`
}

// groupRefsToGIDs decodes a JSON array of embedded group objects (the shape
// privilege.* always returns for local_groups/ds_groups, per groupRefAPI's
// doc comment) into a flat slice of GIDs, normalizing a JSON null/empty
// input to an empty (non-nil) slice. Entries with a null "gid" — the
// UnmappedGroupEntry case, e.g. a directory-service group identified only
// by SID — are skipped: they cannot be represented as a GID, and this
// provider never writes a SID-shaped entry into local_groups/ds_groups to
// begin with, so a missing GID here always means "not representable",
// never "the entry this provider configured."
func groupRefsToGIDs(raw json.RawMessage) ([]int64, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return []int64{}, nil
	}
	var refs []groupRefAPI
	if err := json.Unmarshal(raw, &refs); err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(refs))
	for _, r := range refs {
		if r.GID != nil {
			ids = append(ids, *r.GID)
		}
	}
	return ids, nil
}

// privilegeAPI mirrors the JSON object returned by privilege.create,
// privilege.update, privilege.get_instance, and privilege.query. Probed
// against a live TrueNAS 25.10 box (privilege.create/query/update
// with a throwaway group + privilege, then deleted):
//   - local_groups/ds_groups are always arrays of embedded group objects on
//     read (see groupRefAPI/groupRefsToGIDs), even though create/update
//     accept flat GID lists on write.
//   - builtin_name is null for every privilege this provider creates
//     (custom privileges); it is non-null only for the three server-shipped
//     privileges (builtin_name "LOCAL_ADMINISTRATOR", "READONLY_ADMINISTRATOR",
//     "SHARING_ADMINISTRATOR" — observed via an unfiltered privilege.query),
//     which this resource must never manage.
type privilegeAPI struct {
	ID          int64           `json:"id"`
	BuiltinName *string         `json:"builtin_name"`
	Name        string          `json:"name"`
	LocalGroups json.RawMessage `json:"local_groups"`
	DSGroups    json.RawMessage `json:"ds_groups"`
	Roles       []string        `json:"roles"`
	WebShell    bool            `json:"web_shell"`
}

// nullableStringValue converts a nullable API string pointer to a
// types.String, preserving true null rather than substituting "".
func nullableStringValue(v *string) types.String {
	if v == nil {
		return types.StringNull()
	}
	return types.StringValue(*v)
}

// int64ListValue builds a types.List of Int64 from a Go slice, normalizing
// a nil slice to an empty (non-null) list.
func int64ListValue(ctx context.Context, ids []int64) (types.List, diag.Diagnostics) {
	if ids == nil {
		ids = []int64{}
	}
	return types.ListValueFrom(ctx, types.Int64Type, ids)
}

// stringListValue builds a types.List of String from a Go slice,
// normalizing a nil slice to an empty (non-null) list.
func stringListValue(ctx context.Context, ss []string) (types.List, diag.Diagnostics) {
	if ss == nil {
		ss = []string{}
	}
	return types.ListValueFrom(ctx, types.StringType, ss)
}

// responseToModel maps a privilegeAPI response onto a PrivilegeModel.
func responseToModel(ctx context.Context, api *privilegeAPI, m *PrivilegeModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.Name = types.StringValue(api.Name)

	localGIDs, err := groupRefsToGIDs(api.LocalGroups)
	if err != nil {
		diags.AddError("Invalid local_groups in API response", err.Error())
		return diags
	}
	lg, d := int64ListValue(ctx, localGIDs)
	diags.Append(d...)
	m.LocalGroups = lg

	dsGIDs, err := groupRefsToGIDs(api.DSGroups)
	if err != nil {
		diags.AddError("Invalid ds_groups in API response", err.Error())
		return diags
	}
	dg, d := int64ListValue(ctx, dsGIDs)
	diags.Append(d...)
	m.DSGroups = dg

	roles, d := stringListValue(ctx, api.Roles)
	diags.Append(d...)
	m.Roles = roles

	m.WebShell = types.BoolValue(api.WebShell)
	m.BuiltinName = nullableStringValue(api.BuiltinName)

	return diags
}

// responseToDataSourceModel maps a privilegeAPI response onto a
// PrivilegeDataSourceModel.
func responseToDataSourceModel(ctx context.Context, api *privilegeAPI, m *PrivilegeDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.Name = types.StringValue(api.Name)

	localGIDs, err := groupRefsToGIDs(api.LocalGroups)
	if err != nil {
		diags.AddError("Invalid local_groups in API response", err.Error())
		return diags
	}
	lg, d := int64ListValue(ctx, localGIDs)
	diags.Append(d...)
	m.LocalGroups = lg

	dsGIDs, err := groupRefsToGIDs(api.DSGroups)
	if err != nil {
		diags.AddError("Invalid ds_groups in API response", err.Error())
		return diags
	}
	dg, d := int64ListValue(ctx, dsGIDs)
	diags.Append(d...)
	m.DSGroups = dg

	roles, d := stringListValue(ctx, api.Roles)
	diags.Append(d...)
	m.Roles = roles

	m.WebShell = types.BoolValue(api.WebShell)
	m.BuiltinName = nullableStringValue(api.BuiltinName)

	return diags
}

// apiPayload builds the map expected by privilege.create/privilege.update.
// "name" and "web_shell" are always included (Required in both the schema
// and privilege.create's own required list). local_groups, ds_groups, and
// roles are always included too — as flat GID/role lists, converting a
// null/unknown list to an empty slice — mirroring the group resource's
// basePayload convention (rather than rsync_task's omit-unless-known
// pattern) since these three are always fully known at plan time (they are
// Optional+Computed with UseStateForUnknown, not nullable API scalars) and
// privilege.update, like privilege.create, treats an omitted list the same
// as an explicit empty one per its own schema default of [].
func (m *PrivilegeModel) apiPayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	var localGroups []int64
	if !m.LocalGroups.IsNull() && !m.LocalGroups.IsUnknown() {
		diags.Append(m.LocalGroups.ElementsAs(ctx, &localGroups, false)...)
	}
	if localGroups == nil {
		localGroups = []int64{}
	}

	var dsGroups []int64
	if !m.DSGroups.IsNull() && !m.DSGroups.IsUnknown() {
		diags.Append(m.DSGroups.ElementsAs(ctx, &dsGroups, false)...)
	}
	if dsGroups == nil {
		dsGroups = []int64{}
	}

	var roles []string
	if !m.Roles.IsNull() && !m.Roles.IsUnknown() {
		diags.Append(m.Roles.ElementsAs(ctx, &roles, false)...)
	}
	if roles == nil {
		roles = []string{}
	}

	p := map[string]any{
		"name":         m.Name.ValueString(),
		"local_groups": localGroups,
		"ds_groups":    dsGroups,
		"roles":        roles,
		"web_shell":    m.WebShell.ValueBool(),
	}

	return p, diags
}

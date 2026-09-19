// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package privilege

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// baseUnsetModel returns a model with every field null/unknown except the
// two Required fields, for tests that only care about a subset.
func baseUnsetModel(name string, webShell bool) *PrivilegeModel {
	return &PrivilegeModel{
		Name:        types.StringValue(name),
		LocalGroups: types.ListNull(types.Int64Type),
		DSGroups:    types.ListNull(types.Int64Type),
		Roles:       types.ListNull(types.StringType),
		WebShell:    types.BoolValue(webShell),
	}
}

// TestApiPayload_AllFieldsSet verifies the payload built when every field is
// known: local_groups/ds_groups/roles come through as flat GID/role lists.
func TestApiPayload_AllFieldsSet(t *testing.T) {
	ctx := context.Background()
	m := baseUnsetModel("tf-acc-priv", true)

	lg, diags := types.ListValueFrom(ctx, types.Int64Type, []int64{3000, 3001})
	if diags.HasError() {
		t.Fatalf("building local_groups list: %v", diags)
	}
	m.LocalGroups = lg

	dg, diags := types.ListValueFrom(ctx, types.Int64Type, []int64{100003000})
	if diags.HasError() {
		t.Fatalf("building ds_groups list: %v", diags)
	}
	m.DSGroups = dg

	roles, diags := types.ListValueFrom(ctx, types.StringType, []string{"READONLY_ADMIN", "SHARING_READ"})
	if diags.HasError() {
		t.Fatalf("building roles list: %v", diags)
	}
	m.Roles = roles

	p, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostics errors: %v", diags)
	}

	if p["name"] != "tf-acc-priv" {
		t.Errorf("payload[name] = %v, want tf-acc-priv", p["name"])
	}
	if p["web_shell"] != true {
		t.Errorf("payload[web_shell] = %v, want true", p["web_shell"])
	}
	lgOut, ok := p["local_groups"].([]int64)
	if !ok || len(lgOut) != 2 || lgOut[0] != 3000 || lgOut[1] != 3001 {
		t.Errorf("payload[local_groups] = %v, want [3000 3001]", p["local_groups"])
	}
	dgOut, ok := p["ds_groups"].([]int64)
	if !ok || len(dgOut) != 1 || dgOut[0] != 100003000 {
		t.Errorf("payload[ds_groups] = %v, want [100003000]", p["ds_groups"])
	}
	rolesOut, ok := p["roles"].([]string)
	if !ok || len(rolesOut) != 2 || rolesOut[0] != "READONLY_ADMIN" || rolesOut[1] != "SHARING_READ" {
		t.Errorf("payload[roles] = %v, want [READONLY_ADMIN SHARING_READ]", p["roles"])
	}
}

// TestApiPayload_UnsetListsBecomeEmpty verifies that null local_groups,
// ds_groups, and roles are sent as empty lists (not omitted), matching
// privilege.create/update's own default of [] and the group resource's
// basePayload convention.
func TestApiPayload_UnsetListsBecomeEmpty(t *testing.T) {
	ctx := context.Background()
	m := baseUnsetModel("tf-acc-priv", false)

	p, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostics errors: %v", diags)
	}

	for _, key := range []string{"local_groups", "ds_groups", "roles"} {
		v, ok := p[key]
		if !ok {
			t.Fatalf("payload missing %q key", key)
		}
		switch vv := v.(type) {
		case []int64:
			if len(vv) != 0 {
				t.Errorf("payload[%s] = %v, want empty", key, vv)
			}
		case []string:
			if len(vv) != 0 {
				t.Errorf("payload[%s] = %v, want empty", key, vv)
			}
		default:
			t.Errorf("payload[%s] has unexpected type %T", key, v)
		}
	}
	if len(p) != 5 {
		t.Errorf("payload has %d keys (%v), want 5 (name, local_groups, ds_groups, roles, web_shell)", len(p), p)
	}
}

// TestGroupRefsToGIDs_EmbeddedGroupEntry verifies the shape actually
// observed on privilege.create/query/update/get_instance reads: an array of
// embedded GroupEntry objects (full group details, including "gid").
func TestGroupRefsToGIDs_EmbeddedGroupEntry(t *testing.T) {
	raw := json.RawMessage(`[{"id": 115, "gid": 3000, "name": "tf-acc-grp", "builtin": false, "sudo_commands": [], "sudo_commands_nopasswd": [], "smb": false, "userns_idmap": null, "group": "tf-acc-grp", "local": true, "sid": null, "roles": ["READONLY_ADMIN"], "users": [], "immutable": false}]`)

	ids, err := groupRefsToGIDs(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 1 || ids[0] != 3000 {
		t.Errorf("ids = %v, want [3000]", ids)
	}
}

// TestGroupRefsToGIDs_MultipleEntries verifies multiple embedded group
// objects all decode in order.
func TestGroupRefsToGIDs_MultipleEntries(t *testing.T) {
	raw := json.RawMessage(`[{"gid": 3000}, {"gid": 3001}, {"gid": 3002}]`)

	ids, err := groupRefsToGIDs(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []int64{3000, 3001, 3002}
	if len(ids) != len(want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Errorf("ids[%d] = %d, want %d", i, ids[i], want[i])
		}
	}
}

// TestGroupRefsToGIDs_Empty verifies an empty array decodes to an empty
// (non-nil) slice, matching privilege.query's own [] default for a
// privilege with no groups assigned.
func TestGroupRefsToGIDs_Empty(t *testing.T) {
	ids, err := groupRefsToGIDs(json.RawMessage(`[]`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ids == nil {
		t.Error("ids is nil, want non-nil empty slice")
	}
	if len(ids) != 0 {
		t.Errorf("ids = %v, want empty", ids)
	}
}

// TestGroupRefsToGIDs_NullOrMissing verifies a JSON null (or zero-length
// RawMessage, e.g. an omitted field) decodes to an empty slice rather than
// erroring.
func TestGroupRefsToGIDs_NullOrMissing(t *testing.T) {
	for _, raw := range []json.RawMessage{json.RawMessage(`null`), nil, json.RawMessage(``)} {
		ids, err := groupRefsToGIDs(raw)
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", raw, err)
		}
		if len(ids) != 0 {
			t.Errorf("ids = %v for %q, want empty", ids, raw)
		}
	}
}

// TestGroupRefsToGIDs_UnmappedEntrySkipped verifies the UnmappedGroupEntry
// shape (a group reference that couldn't be resolved — gid null, sid set,
// group null, per the middleware's own schema for ds_groups/local_groups)
// is skipped rather than producing a zero-value GID or an error: it cannot
// be represented as a GID, and silently emitting 0 would be worse than
// omitting it (0 could collide with a real GID in principle, and more
// importantly would misrepresent an unmapped/unknown entry as a configured
// one).
func TestGroupRefsToGIDs_UnmappedEntrySkipped(t *testing.T) {
	raw := json.RawMessage(`[{"gid": 3000}, {"gid": null, "sid": "S-1-5-21-1-2-3-1001", "group": null}, {"gid": 3002}]`)

	ids, err := groupRefsToGIDs(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []int64{3000, 3002}
	if len(ids) != len(want) {
		t.Fatalf("ids = %v, want %v (unmapped entry should be skipped)", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Errorf("ids[%d] = %d, want %d", i, ids[i], want[i])
		}
	}
}

// TestGroupRefsToGIDs_Invalid verifies malformed JSON returns an error
// rather than silently producing an empty/partial list.
func TestGroupRefsToGIDs_Invalid(t *testing.T) {
	_, err := groupRefsToGIDs(json.RawMessage(`not json`))
	if err == nil {
		t.Fatal("expected an error for malformed JSON")
	}
}

// TestResponseToModel_QueryShape verifies responseToModel against the exact
// shape observed from a live privilege.create/query call: local_groups
// holding one embedded group object, ds_groups empty, roles populated.
func TestResponseToModel_QueryShape(t *testing.T) {
	ctx := context.Background()
	api := &privilegeAPI{
		ID:          4,
		BuiltinName: nil,
		Name:        "tf-acc-priv",
		LocalGroups: json.RawMessage(`[{"id": 115, "gid": 3000, "name": "tf-acc-grp", "builtin": false, "group": "tf-acc-grp", "local": true, "sid": null, "roles": [], "immutable": false}]`),
		DSGroups:    json.RawMessage(`[]`),
		Roles:       []string{"READONLY_ADMIN", "SHARING_READ"},
		WebShell:    false,
	}

	m := &PrivilegeModel{}
	diags := responseToModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if m.ID.ValueInt64() != 4 {
		t.Errorf("ID = %v, want 4", m.ID)
	}
	if m.Name.ValueString() != "tf-acc-priv" {
		t.Errorf("Name = %q, want tf-acc-priv", m.Name.ValueString())
	}
	if !m.BuiltinName.IsNull() {
		t.Errorf("BuiltinName = %v, want null", m.BuiltinName)
	}
	if m.WebShell.ValueBool() {
		t.Error("WebShell = true, want false")
	}

	var localGroups []int64
	diags = m.LocalGroups.ElementsAs(ctx, &localGroups, false)
	if diags.HasError() {
		t.Fatalf("reading back local_groups: %v", diags)
	}
	if len(localGroups) != 1 || localGroups[0] != 3000 {
		t.Errorf("LocalGroups = %v, want [3000]", localGroups)
	}

	if m.DSGroups.IsNull() {
		t.Error("DSGroups should be an empty list, not null")
	}
	var dsGroups []int64
	diags = m.DSGroups.ElementsAs(ctx, &dsGroups, false)
	if diags.HasError() {
		t.Fatalf("reading back ds_groups: %v", diags)
	}
	if len(dsGroups) != 0 {
		t.Errorf("DSGroups = %v, want empty", dsGroups)
	}

	var roles []string
	diags = m.Roles.ElementsAs(ctx, &roles, false)
	if diags.HasError() {
		t.Fatalf("reading back roles: %v", diags)
	}
	if len(roles) != 2 || roles[0] != "READONLY_ADMIN" || roles[1] != "SHARING_READ" {
		t.Errorf("Roles = %v, want [READONLY_ADMIN SHARING_READ]", roles)
	}
}

// TestResponseToModel_BuiltinPrivilege verifies a builtin privilege (as
// returned by an unfiltered privilege.query, which always includes the
// three server-shipped privileges) maps builtin_name through correctly.
func TestResponseToModel_BuiltinPrivilege(t *testing.T) {
	ctx := context.Background()
	api := &privilegeAPI{
		ID:          1,
		BuiltinName: strPtr("LOCAL_ADMINISTRATOR"),
		Name:        "Local Administrator",
		LocalGroups: json.RawMessage(`[{"gid": 544}]`),
		DSGroups:    json.RawMessage(`[]`),
		Roles:       []string{"FULL_ADMIN"},
		WebShell:    true,
	}

	m := &PrivilegeModel{}
	diags := responseToModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.BuiltinName.ValueString() != "LOCAL_ADMINISTRATOR" {
		t.Errorf("BuiltinName = %q, want LOCAL_ADMINISTRATOR", m.BuiltinName.ValueString())
	}
}

// TestResponseToDataSourceModel_QueryShape mirrors
// TestResponseToModel_QueryShape for the datasource model.
func TestResponseToDataSourceModel_QueryShape(t *testing.T) {
	ctx := context.Background()
	api := &privilegeAPI{
		ID:          4,
		BuiltinName: nil,
		Name:        "tf-acc-priv",
		LocalGroups: json.RawMessage(`[{"gid": 3000}]`),
		DSGroups:    json.RawMessage(`[]`),
		Roles:       []string{"READONLY_ADMIN"},
		WebShell:    false,
	}

	m := &PrivilegeDataSourceModel{}
	diags := responseToDataSourceModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.Name.ValueString() != "tf-acc-priv" {
		t.Errorf("Name = %q, want tf-acc-priv", m.Name.ValueString())
	}
	var localGroups []int64
	diags = m.LocalGroups.ElementsAs(ctx, &localGroups, false)
	if diags.HasError() {
		t.Fatalf("reading back local_groups: %v", diags)
	}
	if len(localGroups) != 1 || localGroups[0] != 3000 {
		t.Errorf("LocalGroups = %v, want [3000]", localGroups)
	}
}

func strPtr(s string) *string { return &s }

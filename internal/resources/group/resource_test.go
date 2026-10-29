// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package group

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/normalize"
)

// TestGroupSchema verifies key schema attributes.
func TestGroupSchema(t *testing.T) {
	s := resourceSchema()

	// id must be Int64Attribute and Computed.
	idAttr, ok := s.Attributes["id"]
	if !ok {
		t.Fatal("schema missing 'id' attribute")
	}
	idInt64, ok := idAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'id' is %T, want schema.Int64Attribute", idAttr)
	}
	if !idInt64.IsComputed() {
		t.Error("'id' should be Computed")
	}

	// gid must be Int64Attribute, Optional, Computed, with RequiresReplace.
	gidAttr, ok := s.Attributes["gid"]
	if !ok {
		t.Fatal("schema missing 'gid' attribute")
	}
	gidInt64, ok := gidAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'gid' is %T, want schema.Int64Attribute", gidAttr)
	}
	if !gidInt64.IsOptional() {
		t.Error("'gid' should be Optional")
	}
	if !gidInt64.IsComputed() {
		t.Error("'gid' should be Computed")
	}
	if len(gidInt64.PlanModifiers) == 0 {
		t.Error("'gid' should have plan modifiers (RequiresReplace, UseStateForUnknown)")
	}

	// name must be Required with RequiresReplace.
	nameAttr, ok := s.Attributes["name"]
	if !ok {
		t.Fatal("schema missing 'name' attribute")
	}
	nameStr, ok := nameAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'name' is %T, want schema.StringAttribute", nameAttr)
	}
	if !nameStr.IsRequired() {
		t.Error("'name' should be Required")
	}
	if len(nameStr.PlanModifiers) == 0 {
		t.Error("'name' should have plan modifiers (RequiresReplace)")
	}

	// builtin, immutable, local must be Computed-only.
	for _, field := range []string{"builtin", "immutable", "local"} {
		a, ok := s.Attributes[field]
		if !ok {
			t.Fatalf("schema missing %q attribute", field)
		}
		b, ok := a.(schema.BoolAttribute)
		if !ok {
			t.Errorf("%q is %T, want schema.BoolAttribute", field, a)
			continue
		}
		if !b.IsComputed() {
			t.Errorf("%q should be Computed", field)
		}
		if b.IsOptional() || b.IsRequired() {
			t.Errorf("%q should be Computed-only (not Optional/Required)", field)
		}
	}

	// List attributes.
	for _, listField := range []string{"sudo_commands", "sudo_commands_nopasswd"} {
		attr, ok := s.Attributes[listField]
		if !ok {
			t.Fatalf("schema missing %q attribute", listField)
		}
		if _, ok := attr.(schema.ListAttribute); !ok {
			t.Errorf("%q is %T, want schema.ListAttribute", listField, attr)
		}
	}
}

// TestGroupCreatePayload verifies that createPayload includes gid and name,
// and uses the "name" key (not "group").
func TestGroupCreatePayload(t *testing.T) {
	ctx := context.Background()

	m := GroupModel{
		GID:                  types.Int64Value(3000),
		Name:                 types.StringValue("testgroup"),
		SMB:                  types.BoolValue(true),
		SudoCommands:         types.ListValueMust(types.StringType, []attr.Value{}),
		SudoCommandsNoPasswd: types.ListValueMust(types.StringType, []attr.Value{}),
		Builtin:              types.BoolValue(false),
		Immutable:            types.BoolValue(false),
		Local:                types.BoolValue(true),
	}

	payload, diags := m.createPayload(ctx)
	if diags.HasError() {
		t.Fatalf("createPayload returned errors: %v", diags)
	}

	// gid must be present in create payload.
	if v, ok := payload["gid"]; !ok {
		t.Error("createPayload missing 'gid'")
	} else if v != int64(3000) {
		t.Errorf("payload[gid] = %v, want 3000", v)
	}

	// name must be present using "name" key (not "group").
	if v, ok := payload["name"]; !ok {
		t.Error("createPayload missing 'name'")
	} else if v != "testgroup" {
		t.Errorf("payload[name] = %v, want testgroup", v)
	}

	// "group" key must NOT appear in payload.
	if _, ok := payload["group"]; ok {
		t.Error("createPayload must not contain 'group' key; use 'name'")
	}

	// builtin, immutable, local, id must NOT be in payload.
	for _, forbidden := range []string{"builtin", "immutable", "local", "id"} {
		if _, ok := payload[forbidden]; ok {
			t.Errorf("createPayload should not contain %q", forbidden)
		}
	}
}

// TestGroupCreatePayload_NullGID verifies that createPayload does NOT include gid
// when GID is null, avoiding the root group conflict (gid: 0).
func TestGroupCreatePayload_NullGID(t *testing.T) {
	ctx := context.Background()

	m := GroupModel{
		GID:                  types.Int64Null(),
		Name:                 types.StringValue("testgroup"),
		SMB:                  types.BoolValue(true),
		SudoCommands:         types.ListValueMust(types.StringType, []attr.Value{}),
		SudoCommandsNoPasswd: types.ListValueMust(types.StringType, []attr.Value{}),
		Builtin:              types.BoolValue(false),
		Immutable:            types.BoolValue(false),
		Local:                types.BoolValue(true),
	}

	payload, diags := m.createPayload(ctx)
	if diags.HasError() {
		t.Fatalf("createPayload returned errors: %v", diags)
	}

	// gid must NOT be present in create payload when null.
	if _, ok := payload["gid"]; ok {
		t.Error("createPayload should not contain 'gid' when GID is null")
	}

	// name must still be present.
	if v, ok := payload["name"]; !ok {
		t.Error("createPayload missing 'name'")
	} else if v != "testgroup" {
		t.Errorf("payload[name] = %v, want testgroup", v)
	}
}

// TestGroupUpdatePayload verifies that updatePayload omits gid and name, and
// sends only fields that are known, non-null, and changed from prior state:
// an unset (unknown) or unchanged attribute is never overwritten. (B-037)
func TestGroupUpdatePayload(t *testing.T) {
	ctx := context.Background()

	prior := GroupModel{
		GID:                  types.Int64Value(3000),
		Name:                 types.StringValue("testgroup"),
		SMB:                  types.BoolValue(true),
		Users:                mustUsers(),
		SudoCommands:         types.ListValueMust(types.StringType, []attr.Value{types.StringValue("/usr/bin/ls")}),
		SudoCommandsNoPasswd: types.ListValueMust(types.StringType, []attr.Value{}),
	}
	m := prior
	m.SMB = types.BoolUnknown()
	m.SudoCommandsNoPasswd = types.ListUnknown(types.StringType)
	m.Users = mustUsers(types.Int64Value(7))

	payload, diags := m.updatePayload(ctx, prior)
	if diags.HasError() {
		t.Fatalf("updatePayload returned errors: %v", diags)
	}
	for _, k := range []string{"gid", "name", "group", "smb", "sudo_commands", "sudo_commands_nopasswd"} {
		if _, ok := payload[k]; ok {
			t.Errorf("updatePayload must not contain %q: %v", k, payload)
		}
	}
	if got, ok := payload["users"].([]int64); !ok || len(got) != 1 || got[0] != 7 {
		t.Errorf("payload[users] = %#v, want [7]", payload["users"])
	}
}

// TestGroupCreatePayload_OmitsUnset verifies that unset (unknown) optionals are
// left out of group.create so the server defaults apply (smb=true). (B-108)
func TestGroupCreatePayload_OmitsUnset(t *testing.T) {
	m := GroupModel{
		GID:                  types.Int64Unknown(),
		Name:                 types.StringValue("g"),
		SMB:                  types.BoolUnknown(),
		Users:                normalize.NewUnorderedListUnknown(types.Int64Type),
		SudoCommands:         types.ListUnknown(types.StringType),
		SudoCommandsNoPasswd: types.ListNull(types.StringType),
	}
	payload, diags := m.createPayload(context.Background())
	if diags.HasError() {
		t.Fatalf("createPayload returned errors: %v", diags)
	}
	if len(payload) != 1 || payload["name"] != "g" {
		t.Errorf("createPayload = %v, want only name", payload)
	}
}

// TestDropPrimaryMembers covers the users read-back: primary-group members are
// dropped unless configured, and other members not in the configuration are
// kept (drift). (B-058)
func TestDropPrimaryMembers(t *testing.T) {
	primary := map[int64]bool{5: true}
	cases := []struct {
		name            string
		server, desired []int64
		want            []int64
	}{
		{"primary dropped", []int64{1, 5}, []int64{1}, []int64{1}},
		{"primary configured", []int64{5, 1}, []int64{1, 5}, []int64{5, 1}},
		{"import drops primary", []int64{5, 2}, nil, []int64{2}},
		{"drift kept", []int64{4, 1}, []int64{1}, []int64{4, 1}},
		{"nil server", nil, nil, []int64{}},
	}
	for _, c := range cases {
		got := dropPrimaryMembers(c.server, c.desired, primary)
		if fmt.Sprint(got) != fmt.Sprint(c.want) || got == nil {
			t.Errorf("%s: dropPrimaryMembers(%v, %v) = %#v, want %v", c.name, c.server, c.desired, got, c.want)
		}
	}
}

// TestGroupResponseToModel verifies responseToModel populates all fields correctly.
func TestGroupResponseToModel(t *testing.T) {
	ctx := context.Background()

	api := &groupAPI{
		ID:                   42,
		GID:                  3001,
		Group:                "mygroup",
		Name:                 "mygroup",
		SMB:                  true,
		SudoCommands:         []string{"/usr/bin/ls"},
		SudoCommandsNoPasswd: []string{},
		Builtin:              false,
		Immutable:            false,
		Local:                true,
	}

	var m GroupModel
	diags := responseToModel(ctx, api, nil, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned errors: %v", diags)
	}

	if m.ID.ValueInt64() != 42 {
		t.Errorf("ID = %v, want 42", m.ID.ValueInt64())
	}
	if m.GID.ValueInt64() != 3001 {
		t.Errorf("GID = %v, want 3001", m.GID.ValueInt64())
	}
	// Name must be read from api.Group field.
	if m.Name.ValueString() != "mygroup" {
		t.Errorf("Name = %v, want mygroup", m.Name.ValueString())
	}
	if !m.SMB.ValueBool() {
		t.Error("SMB should be true")
	}
	if !m.Local.ValueBool() {
		t.Error("Local should be true")
	}

	var sudoCmds []string
	if d := m.SudoCommands.ElementsAs(ctx, &sudoCmds, false); d.HasError() {
		t.Fatalf("SudoCommands.ElementsAs failed: %v", d)
	}
	if len(sudoCmds) != 1 || sudoCmds[0] != "/usr/bin/ls" {
		t.Errorf("SudoCommands = %v, want [/usr/bin/ls]", sudoCmds)
	}
}

// TestGroupResponseToModel_NilSlices verifies nil slices map to empty lists.
func TestGroupResponseToModel_NilSlices(t *testing.T) {
	ctx := context.Background()

	api := &groupAPI{
		ID:                   1,
		GID:                  1000,
		Group:                "nogroup",
		Name:                 "nogroup",
		SMB:                  false,
		SudoCommands:         nil,
		SudoCommandsNoPasswd: nil,
		Builtin:              false,
		Immutable:            false,
		Local:                true,
	}

	var m GroupModel
	diags := responseToModel(ctx, api, nil, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned errors: %v", diags)
	}

	var sudoCmds []string
	if d := m.SudoCommands.ElementsAs(ctx, &sudoCmds, false); d.HasError() {
		t.Fatalf("SudoCommands.ElementsAs failed: %v", d)
	}
	if sudoCmds == nil {
		t.Error("SudoCommands must not be nil when API returns nil")
	}

	var sudoCmdsNP []string
	if d := m.SudoCommandsNoPasswd.ElementsAs(ctx, &sudoCmdsNP, false); d.HasError() {
		t.Fatalf("SudoCommandsNoPasswd.ElementsAs failed: %v", d)
	}
	if sudoCmdsNP == nil {
		t.Error("SudoCommandsNoPasswd must not be nil when API returns nil")
	}
}

func mustUsers(ids ...attr.Value) normalize.UnorderedList {
	if ids == nil {
		ids = []attr.Value{}
	}
	v, d := normalize.NewUnorderedListValue(types.Int64Type, ids)
	if d.HasError() {
		panic(d)
	}
	return v
}

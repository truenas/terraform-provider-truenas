// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package group

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
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

// TestGroupUpdatePayload verifies that updatePayload omits gid and name.
func TestGroupUpdatePayload(t *testing.T) {
	ctx := context.Background()

	m := GroupModel{
		GID:                  types.Int64Value(3000),
		Name:                 types.StringValue("testgroup"),
		SMB:                  types.BoolValue(false),
		SudoCommands:         types.ListNull(types.StringType),
		SudoCommandsNoPasswd: types.ListNull(types.StringType),
		Builtin:              types.BoolValue(false),
		Immutable:            types.BoolValue(false),
		Local:                types.BoolValue(true),
	}

	payload, diags := m.updatePayload(ctx)
	if diags.HasError() {
		t.Fatalf("updatePayload returned errors: %v", diags)
	}

	// gid and name must NOT be in update payload.
	if _, ok := payload["gid"]; ok {
		t.Error("updatePayload must not contain 'gid'")
	}
	if _, ok := payload["name"]; ok {
		t.Error("updatePayload must not contain 'name'")
	}
	if _, ok := payload["group"]; ok {
		t.Error("updatePayload must not contain 'group'")
	}

	// Null lists should produce empty slices, not nil.
	sudoCmds, ok := payload["sudo_commands"].([]string)
	if !ok {
		t.Fatalf("payload[sudo_commands] is %T, want []string", payload["sudo_commands"])
	}
	if sudoCmds == nil {
		t.Error("payload[sudo_commands] must not be nil")
	}

	sudoCmdsNP, ok := payload["sudo_commands_nopasswd"].([]string)
	if !ok {
		t.Fatalf("payload[sudo_commands_nopasswd] is %T, want []string", payload["sudo_commands_nopasswd"])
	}
	if sudoCmdsNP == nil {
		t.Error("payload[sudo_commands_nopasswd] must not be nil")
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
	diags := responseToModel(ctx, api, &m)
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
	diags := responseToModel(ctx, api, &m)
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

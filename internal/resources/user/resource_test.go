// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package user

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestUserSchema verifies key schema attributes.
func TestUserSchema(t *testing.T) {
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

	// uid must be Int64Attribute, Optional, Computed, with RequiresReplace.
	uidAttr, ok := s.Attributes["uid"]
	if !ok {
		t.Fatal("schema missing 'uid' attribute")
	}
	uidInt64, ok := uidAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'uid' is %T, want schema.Int64Attribute", uidAttr)
	}
	if !uidInt64.IsOptional() {
		t.Error("'uid' should be Optional")
	}
	if !uidInt64.IsComputed() {
		t.Error("'uid' should be Computed")
	}
	if len(uidInt64.PlanModifiers) == 0 {
		t.Error("'uid' should have plan modifiers (RequiresReplace)")
	}

	// username must be Required with RequiresReplace.
	usernameAttr, ok := s.Attributes["username"]
	if !ok {
		t.Fatal("schema missing 'username' attribute")
	}
	usernameStr, ok := usernameAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'username' is %T, want schema.StringAttribute", usernameAttr)
	}
	if !usernameStr.IsRequired() {
		t.Error("'username' should be Required")
	}
	if len(usernameStr.PlanModifiers) == 0 {
		t.Error("'username' should have plan modifiers (RequiresReplace)")
	}

	// password must be Optional, Sensitive, NOT Computed.
	passwordAttr, ok := s.Attributes["password"]
	if !ok {
		t.Fatal("schema missing 'password' attribute")
	}
	passwordStr, ok := passwordAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'password' is %T, want schema.StringAttribute", passwordAttr)
	}
	if !passwordStr.IsOptional() {
		t.Error("'password' should be Optional")
	}
	if passwordStr.IsComputed() {
		t.Error("'password' must NOT be Computed (write-only)")
	}
	if !passwordStr.IsSensitive() {
		t.Error("'password' should be Sensitive")
	}
	if !passwordStr.IsWriteOnly() {
		t.Error("'password' should be WriteOnly")
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
	for _, listField := range []string{"sudo_commands", "sudo_commands_nopasswd", "groups"} {
		attr, ok := s.Attributes[listField]
		if !ok {
			t.Fatalf("schema missing %q attribute", listField)
		}
		if _, ok := attr.(schema.ListAttribute); !ok {
			t.Errorf("%q is %T, want schema.ListAttribute", listField, attr)
		}
	}
}

// TestUserCreatePayload verifies that createPayload includes uid and username.
func TestUserCreatePayload(t *testing.T) {
	ctx := context.Background()

	m := UserModel{
		UID:                  types.Int64Value(1234),
		Username:             types.StringValue("testuser"),
		FullName:             types.StringValue("Test User"),
		Email:                types.StringValue("test@example.com"),
		Home:                 types.StringValue("/home/testuser"),
		Shell:                types.StringValue("/bin/bash"),
		Locked:               types.BoolValue(false),
		PasswordDisabled:     types.BoolValue(false),
		SMB:                  types.BoolValue(true),
		SSHPasswordEnabled:   types.BoolValue(false),
		SSHPubKey:            types.StringValue(""),
		SudoCommands:         types.ListValueMust(types.StringType, []attr.Value{}),
		SudoCommandsNoPasswd: types.ListValueMust(types.StringType, []attr.Value{}),
		Groups:               types.ListValueMust(types.Int64Type, []attr.Value{}),
		Password:             types.StringValue("secret123"),
		Builtin:              types.BoolValue(false),
		Immutable:            types.BoolValue(false),
		Local:                types.BoolValue(true),
	}

	payload, diags := m.createPayload(ctx)
	if diags.HasError() {
		t.Fatalf("createPayload returned errors: %v", diags)
	}

	// uid and username must be present in create payload.
	if v, ok := payload["uid"]; !ok {
		t.Error("createPayload missing 'uid'")
	} else if v != int64(1234) {
		t.Errorf("payload[uid] = %v, want 1234", v)
	}
	if v, ok := payload["username"]; !ok {
		t.Error("createPayload missing 'username'")
	} else if v != "testuser" {
		t.Errorf("payload[username] = %v, want testuser", v)
	}

	// password must be included when set.
	if v, ok := payload["password"]; !ok {
		t.Error("createPayload missing 'password' when it is set")
	} else if v != "secret123" {
		t.Errorf("payload[password] = %v, want secret123", v)
	}

	// builtin, immutable, local must NOT be in payload.
	for _, forbidden := range []string{"builtin", "immutable", "local", "id"} {
		if _, ok := payload[forbidden]; ok {
			t.Errorf("createPayload should not contain %q", forbidden)
		}
	}
}

// TestUserUpdatePayload verifies that updatePayload omits uid and username.
func TestUserUpdatePayload(t *testing.T) {
	ctx := context.Background()

	m := UserModel{
		UID:                  types.Int64Value(1234),
		Username:             types.StringValue("testuser"),
		FullName:             types.StringValue("Updated Name"),
		Email:                types.StringValue(""),
		Home:                 types.StringValue("/home/testuser"),
		Shell:                types.StringValue("/bin/bash"),
		Locked:               types.BoolValue(false),
		PasswordDisabled:     types.BoolValue(false),
		SMB:                  types.BoolValue(false),
		SSHPasswordEnabled:   types.BoolValue(false),
		SSHPubKey:            types.StringValue(""),
		SudoCommands:         types.ListNull(types.StringType),
		SudoCommandsNoPasswd: types.ListNull(types.StringType),
		Groups:               types.ListNull(types.Int64Type),
		Password:             types.StringNull(),
		Builtin:              types.BoolValue(false),
		Immutable:            types.BoolValue(false),
		Local:                types.BoolValue(true),
	}

	payload, diags := m.updatePayload(ctx)
	if diags.HasError() {
		t.Fatalf("updatePayload returned errors: %v", diags)
	}

	// uid and username must NOT be in update payload.
	if _, ok := payload["uid"]; ok {
		t.Error("updatePayload must not contain 'uid'")
	}
	if _, ok := payload["username"]; ok {
		t.Error("updatePayload must not contain 'username'")
	}

	// password must NOT be included when null.
	if _, ok := payload["password"]; ok {
		t.Error("updatePayload must not contain 'password' when it is null")
	}

	// email should be nil when empty string.
	if v, ok := payload["email"]; !ok {
		t.Error("updatePayload missing 'email'")
	} else if v != nil {
		t.Errorf("payload[email] = %v, want nil for empty string", v)
	}

	// Null lists should produce empty slices, not nil.
	sudoCmds, ok := payload["sudo_commands"].([]string)
	if !ok {
		t.Fatalf("payload[sudo_commands] is %T, want []string", payload["sudo_commands"])
	}
	if sudoCmds == nil {
		t.Error("payload[sudo_commands] must not be nil")
	}

	groups, ok := payload["groups"].([]int64)
	if !ok {
		t.Fatalf("payload[groups] is %T, want []int64", payload["groups"])
	}
	if groups == nil {
		t.Error("payload[groups] must not be nil")
	}
}

// TestUserCreatePayload_GroupCreate verifies that createPayload includes
// group_create when set, and omits it (and 'group') when neither is set.
func TestUserCreatePayload_GroupCreate(t *testing.T) {
	ctx := context.Background()

	m := UserModel{
		Username:             types.StringValue("newuser"),
		FullName:             types.StringValue("New User"),
		Email:                types.StringValue(""),
		Home:                 types.StringValue("/home/newuser"),
		Shell:                types.StringValue("/bin/bash"),
		Locked:               types.BoolValue(false),
		PasswordDisabled:     types.BoolValue(false),
		SMB:                  types.BoolValue(false),
		SSHPasswordEnabled:   types.BoolValue(false),
		SSHPubKey:            types.StringValue(""),
		SudoCommands:         types.ListValueMust(types.StringType, []attr.Value{}),
		SudoCommandsNoPasswd: types.ListValueMust(types.StringType, []attr.Value{}),
		Groups:               types.ListValueMust(types.Int64Type, []attr.Value{}),
		Group:                types.Int64Null(),
		GroupCreate:          types.BoolValue(true),
	}

	payload, diags := m.createPayload(ctx)
	if diags.HasError() {
		t.Fatalf("createPayload returned errors: %v", diags)
	}

	if v, ok := payload["group_create"]; !ok {
		t.Error("createPayload missing 'group_create' when it is set")
	} else if v != true {
		t.Errorf("payload[group_create] = %v, want true", v)
	}
	if _, ok := payload["group"]; ok {
		t.Error("createPayload should not contain 'group' when Group is null")
	}
}

// TestUserCreatePayload_GroupID verifies that createPayload includes 'group'
// when a primary group id is set, and omits 'group_create' when it is unset.
func TestUserCreatePayload_GroupID(t *testing.T) {
	ctx := context.Background()

	m := UserModel{
		Username:             types.StringValue("newuser"),
		FullName:             types.StringValue("New User"),
		Email:                types.StringValue(""),
		Home:                 types.StringValue("/home/newuser"),
		Shell:                types.StringValue("/bin/bash"),
		Locked:               types.BoolValue(false),
		PasswordDisabled:     types.BoolValue(false),
		SMB:                  types.BoolValue(false),
		SSHPasswordEnabled:   types.BoolValue(false),
		SSHPubKey:            types.StringValue(""),
		SudoCommands:         types.ListValueMust(types.StringType, []attr.Value{}),
		SudoCommandsNoPasswd: types.ListValueMust(types.StringType, []attr.Value{}),
		Groups:               types.ListValueMust(types.Int64Type, []attr.Value{}),
		Group:                types.Int64Value(3000),
		GroupCreate:          types.BoolNull(),
	}

	payload, diags := m.createPayload(ctx)
	if diags.HasError() {
		t.Fatalf("createPayload returned errors: %v", diags)
	}

	if v, ok := payload["group"]; !ok {
		t.Error("createPayload missing 'group' when Group is set")
	} else if v != int64(3000) {
		t.Errorf("payload[group] = %v, want 3000", v)
	}
	if _, ok := payload["group_create"]; ok {
		t.Error("createPayload should not contain 'group_create' when GroupCreate is null")
	}
}

// TestUserUpdatePayload_GroupIncluded verifies that updatePayload includes
// 'group' when set (primary group is updatable, unlike uid/username), and
// never includes 'group_create' (create-only, write-only).
func TestUserUpdatePayload_GroupIncluded(t *testing.T) {
	ctx := context.Background()

	m := UserModel{
		Username:             types.StringValue("existinguser"),
		FullName:             types.StringValue("Existing User"),
		Email:                types.StringValue(""),
		Home:                 types.StringValue("/home/existinguser"),
		Shell:                types.StringValue("/bin/bash"),
		Locked:               types.BoolValue(false),
		PasswordDisabled:     types.BoolValue(false),
		SMB:                  types.BoolValue(false),
		SSHPasswordEnabled:   types.BoolValue(false),
		SSHPubKey:            types.StringValue(""),
		SudoCommands:         types.ListNull(types.StringType),
		SudoCommandsNoPasswd: types.ListNull(types.StringType),
		Groups:               types.ListNull(types.Int64Type),
		Group:                types.Int64Value(4000),
		GroupCreate:          types.BoolValue(true), // must never leak into update
	}

	payload, diags := m.updatePayload(ctx)
	if diags.HasError() {
		t.Fatalf("updatePayload returned errors: %v", diags)
	}

	if v, ok := payload["group"]; !ok {
		t.Error("updatePayload missing 'group' when Group is set")
	} else if v != int64(4000) {
		t.Errorf("payload[group] = %v, want 4000", v)
	}
	if _, ok := payload["group_create"]; ok {
		t.Error("updatePayload must never contain 'group_create' (create-only, write-only)")
	}
}

// TestResponseToModel_GroupObjectDecode verifies responseToModel decodes a
// "group" field embedded as an object ({"id": N, ...}), matching the live
// user.query/get_instance wire shape.
func TestResponseToModel_GroupObjectDecode(t *testing.T) {
	ctx := context.Background()

	api := &userAPI{
		ID:       1,
		UID:      1000,
		Username: "grouped",
		FullName: "Grouped User",
		Home:     "/home/grouped",
		Shell:    "/bin/sh",
		Group:    json.RawMessage(`{"id": 5, "bsdgrp_gid": 5}`),
	}

	var m UserModel
	diags := responseToModel(ctx, api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned errors: %v", diags)
	}
	if m.Group.IsNull() {
		t.Fatal("Group should not be null when API returns a group object")
	}
	if m.Group.ValueInt64() != 5 {
		t.Errorf("Group = %v, want 5", m.Group.ValueInt64())
	}
}

// TestResponseToModel_GroupBareInt verifies responseToModel decodes a
// "group" field returned as a bare integer id.
func TestResponseToModel_GroupBareInt(t *testing.T) {
	ctx := context.Background()

	api := &userAPI{
		ID:       1,
		UID:      1000,
		Username: "grouped",
		FullName: "Grouped User",
		Home:     "/home/grouped",
		Shell:    "/bin/sh",
		Group:    json.RawMessage(`7`),
	}

	var m UserModel
	diags := responseToModel(ctx, api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned errors: %v", diags)
	}
	if m.Group.ValueInt64() != 7 {
		t.Errorf("Group = %v, want 7", m.Group.ValueInt64())
	}
}

// TestResponseToModel_GroupNull verifies responseToModel maps a null/absent
// "group" field to Int64Null rather than erroring or defaulting to 0.
func TestResponseToModel_GroupNull(t *testing.T) {
	ctx := context.Background()

	api := &userAPI{
		ID:       1,
		UID:      1000,
		Username: "nogroupuser",
		FullName: "No Group User",
		Home:     "/home/nogroupuser",
		Shell:    "/bin/sh",
		Group:    json.RawMessage(`null`),
	}

	var m UserModel
	diags := responseToModel(ctx, api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned errors: %v", diags)
	}
	if !m.Group.IsNull() {
		t.Errorf("Group = %v, want null", m.Group)
	}
}

// TestResponseToModel verifies responseToModel populates all fields correctly.
func TestResponseToModel(t *testing.T) {
	ctx := context.Background()

	email := "user@example.com"
	pubkey := "ssh-rsa AAAAB3NzaC1"
	api := &userAPI{
		ID:                   99,
		UID:                  1001,
		Username:             "jdoe",
		FullName:             "John Doe",
		Email:                &email,
		Home:                 "/home/jdoe",
		Shell:                "/bin/zsh",
		Locked:               false,
		PasswordDisabled:     false,
		SMB:                  true,
		SSHPasswordEnabled:   true,
		SSHPubKey:            &pubkey,
		SudoCommands:         []string{"/usr/bin/ls"},
		SudoCommandsNoPasswd: []string{},
		Groups:               []int64{100, 200},
		Builtin:              false,
		Immutable:            false,
		Local:                true,
	}

	var m UserModel
	diags := responseToModel(ctx, api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned errors: %v", diags)
	}

	if m.ID.ValueInt64() != 99 {
		t.Errorf("ID = %v, want 99", m.ID.ValueInt64())
	}
	if m.UID.ValueInt64() != 1001 {
		t.Errorf("UID = %v, want 1001", m.UID.ValueInt64())
	}
	if m.Username.ValueString() != "jdoe" {
		t.Errorf("Username = %v, want jdoe", m.Username.ValueString())
	}
	if m.Email.ValueString() != "user@example.com" {
		t.Errorf("Email = %v, want user@example.com", m.Email.ValueString())
	}
	if m.SSHPubKey.ValueString() != "ssh-rsa AAAAB3NzaC1" {
		t.Errorf("SSHPubKey = %v, want ssh-rsa AAAAB3NzaC1", m.SSHPubKey.ValueString())
	}
	if m.Local.ValueBool() != true {
		t.Errorf("Local = %v, want true", m.Local.ValueBool())
	}

	// Password must NOT be set by responseToModel.
	if !m.Password.IsNull() && !m.Password.IsUnknown() && m.Password.ValueString() != "" {
		t.Errorf("Password should not be set by responseToModel, got %q", m.Password.ValueString())
	}

	var sudoCmds []string
	if d := m.SudoCommands.ElementsAs(ctx, &sudoCmds, false); d.HasError() {
		t.Fatalf("SudoCommands.ElementsAs failed: %v", d)
	}
	if len(sudoCmds) != 1 || sudoCmds[0] != "/usr/bin/ls" {
		t.Errorf("SudoCommands = %v, want [/usr/bin/ls]", sudoCmds)
	}

	var groups []int64
	if d := m.Groups.ElementsAs(ctx, &groups, false); d.HasError() {
		t.Fatalf("Groups.ElementsAs failed: %v", d)
	}
	if len(groups) != 2 || groups[0] != 100 || groups[1] != 200 {
		t.Errorf("Groups = %v, want [100 200]", groups)
	}
}

// TestResponseToModel_NilPointers verifies nil email/sshpubkey map to empty strings.
func TestResponseToModel_NilPointers(t *testing.T) {
	ctx := context.Background()

	api := &userAPI{
		ID:                   1,
		UID:                  1000,
		Username:             "nomail",
		FullName:             "No Mail",
		Email:                nil,
		Home:                 "/home/nomail",
		Shell:                "/bin/sh",
		Locked:               false,
		PasswordDisabled:     true,
		SMB:                  false,
		SSHPasswordEnabled:   false,
		SSHPubKey:            nil,
		SudoCommands:         nil,
		SudoCommandsNoPasswd: nil,
		Groups:               nil,
		Builtin:              false,
		Immutable:            false,
		Local:                true,
	}

	var m UserModel
	diags := responseToModel(ctx, api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned errors: %v", diags)
	}

	if m.Email.ValueString() != "" {
		t.Errorf("Email with nil API value = %q, want empty string", m.Email.ValueString())
	}
	if m.SSHPubKey.ValueString() != "" {
		t.Errorf("SSHPubKey with nil API value = %q, want empty string", m.SSHPubKey.ValueString())
	}

	var sudoCmds []string
	if d := m.SudoCommands.ElementsAs(ctx, &sudoCmds, false); d.HasError() {
		t.Fatalf("SudoCommands.ElementsAs failed: %v", d)
	}
	if sudoCmds == nil {
		t.Error("SudoCommands must not be nil when API returns nil")
	}

	var groups []int64
	if d := m.Groups.ElementsAs(ctx, &groups, false); d.HasError() {
		t.Fatalf("Groups.ElementsAs failed: %v", d)
	}
	if groups == nil {
		t.Error("Groups must not be nil when API returns nil")
	}
}

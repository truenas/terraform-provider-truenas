// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package user

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// UserModel is the Terraform state model for truenas_user.
type UserModel struct {
	ID                   types.Int64  `tfsdk:"id"`
	UID                  types.Int64  `tfsdk:"uid"`
	Username             types.String `tfsdk:"username"`
	FullName             types.String `tfsdk:"full_name"`
	Email                types.String `tfsdk:"email"`
	Home                 types.String `tfsdk:"home"`
	Shell                types.String `tfsdk:"shell"`
	Locked               types.Bool   `tfsdk:"locked"`
	PasswordDisabled     types.Bool   `tfsdk:"password_disabled"`
	SMB                  types.Bool   `tfsdk:"smb"`
	SSHPasswordEnabled   types.Bool   `tfsdk:"ssh_password_enabled"`
	SSHPubKey            types.String `tfsdk:"sshpubkey"`
	SudoCommands         types.List   `tfsdk:"sudo_commands"`
	SudoCommandsNoPasswd types.List   `tfsdk:"sudo_commands_nopasswd"`
	Groups               types.List   `tfsdk:"groups"`
	Group                types.Int64  `tfsdk:"group"`
	GroupCreate          types.Bool   `tfsdk:"group_create"`
	Password             types.String `tfsdk:"password"`
	// Computed only
	Builtin   types.Bool `tfsdk:"builtin"`
	Immutable types.Bool `tfsdk:"immutable"`
	Local     types.Bool `tfsdk:"local"`
}

// UserDatasourceModel is UserModel without the write-only password field.
type UserDatasourceModel struct {
	ID                   types.Int64  `tfsdk:"id"`
	UID                  types.Int64  `tfsdk:"uid"`
	Username             types.String `tfsdk:"username"`
	FullName             types.String `tfsdk:"full_name"`
	Email                types.String `tfsdk:"email"`
	Home                 types.String `tfsdk:"home"`
	Shell                types.String `tfsdk:"shell"`
	Locked               types.Bool   `tfsdk:"locked"`
	PasswordDisabled     types.Bool   `tfsdk:"password_disabled"`
	SMB                  types.Bool   `tfsdk:"smb"`
	SSHPasswordEnabled   types.Bool   `tfsdk:"ssh_password_enabled"`
	SSHPubKey            types.String `tfsdk:"sshpubkey"`
	SudoCommands         types.List   `tfsdk:"sudo_commands"`
	SudoCommandsNoPasswd types.List   `tfsdk:"sudo_commands_nopasswd"`
	Groups               types.List   `tfsdk:"groups"`
	Group                types.Int64  `tfsdk:"group"`
	Builtin              types.Bool   `tfsdk:"builtin"`
	Immutable            types.Bool   `tfsdk:"immutable"`
	Local                types.Bool   `tfsdk:"local"`
}

// responseToDataSourceModel maps an API response onto a UserDatasourceModel.
func responseToDataSourceModel(ctx context.Context, api *userAPI, m *UserDatasourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.UID = types.Int64Value(api.UID)
	m.Username = types.StringValue(api.Username)
	m.FullName = types.StringValue(api.FullName)

	if api.Email != nil {
		m.Email = types.StringValue(*api.Email)
	} else {
		m.Email = types.StringValue("")
	}

	m.Home = types.StringValue(api.Home)
	m.Shell = types.StringValue(api.Shell)
	m.Locked = types.BoolValue(api.Locked)
	m.PasswordDisabled = types.BoolValue(api.PasswordDisabled)
	m.SMB = types.BoolValue(api.SMB)
	m.SSHPasswordEnabled = types.BoolValue(api.SSHPasswordEnabled)

	if api.SSHPubKey != nil {
		m.SSHPubKey = types.StringValue(*api.SSHPubKey)
	} else {
		m.SSHPubKey = types.StringValue("")
	}

	sudoCmds := api.SudoCommands
	if sudoCmds == nil {
		sudoCmds = []string{}
	}
	sl, d := types.ListValueFrom(ctx, types.StringType, sudoCmds)
	diags.Append(d...)
	m.SudoCommands = sl

	sudoCmdsNP := api.SudoCommandsNoPasswd
	if sudoCmdsNP == nil {
		sudoCmdsNP = []string{}
	}
	snl, d2 := types.ListValueFrom(ctx, types.StringType, sudoCmdsNP)
	diags.Append(d2...)
	m.SudoCommandsNoPasswd = snl

	grps := api.Groups
	if grps == nil {
		grps = []int64{}
	}
	gl, d3 := types.ListValueFrom(ctx, types.Int64Type, grps)
	diags.Append(d3...)
	m.Groups = gl

	groupID, groupOK, gErr := decodeGroupField(api.Group)
	if gErr != nil {
		diags.AddError("Parse group field", gErr.Error())
	} else if groupOK {
		m.Group = types.Int64Value(groupID)
	} else {
		m.Group = types.Int64Null()
	}

	m.Builtin = types.BoolValue(api.Builtin)
	m.Immutable = types.BoolValue(api.Immutable)
	m.Local = types.BoolValue(api.Local)

	return diags
}

// userAPI is the JSON wire format for a TrueNAS user object.
//
// Group is decoded as json.RawMessage because user.query/get_instance
// responses embed the primary group as an object ({"id": N,
// "bsdgrp_gid": ...}), while it may also come back as a bare integer id or
// null depending on the calling method / API version. decodeGroupField
// handles all three shapes.
type userAPI struct {
	ID                   int64           `json:"id"`
	UID                  int64           `json:"uid"`
	Username             string          `json:"username"`
	FullName             string          `json:"full_name"`
	Email                *string         `json:"email"`
	Home                 string          `json:"home"`
	Shell                string          `json:"shell"`
	Locked               bool            `json:"locked"`
	PasswordDisabled     bool            `json:"password_disabled"`
	SMB                  bool            `json:"smb"`
	SSHPasswordEnabled   bool            `json:"ssh_password_enabled"`
	SSHPubKey            *string         `json:"sshpubkey"`
	SudoCommands         []string        `json:"sudo_commands"`
	SudoCommandsNoPasswd []string        `json:"sudo_commands_nopasswd"`
	Groups               []int64         `json:"groups"`
	Group                json.RawMessage `json:"group"`
	Builtin              bool            `json:"builtin"`
	Immutable            bool            `json:"immutable"`
	Local                bool            `json:"local"`
}

// decodeGroupField decodes the "group" field embedded in a
// user.query/get_instance response. It may be an object ({"id": N, ...}),
// a bare integer id, or null (e.g. absent/not yet set). ok is false when
// the field is null, absent, or has no usable id — callers should map that
// to types.Int64Null() rather than treating it as an error.
func decodeGroupField(raw json.RawMessage) (id int64, ok bool, err error) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, false, nil
	}

	var obj map[string]json.RawMessage
	if uErr := json.Unmarshal(raw, &obj); uErr == nil {
		idRaw, present := obj["id"]
		if !present || len(idRaw) == 0 || string(idRaw) == "null" {
			return 0, false, nil
		}
		if uErr := json.Unmarshal(idRaw, &id); uErr != nil {
			return 0, false, fmt.Errorf("cannot decode group.id field %q as integer: %w", string(idRaw), uErr)
		}
		return id, true, nil
	}

	if uErr := json.Unmarshal(raw, &id); uErr == nil {
		return id, true, nil
	}

	return 0, false, fmt.Errorf("cannot decode group field %q as object with id, bare integer, or null", string(raw))
}

// responseToModel maps an API response onto a Terraform model.
// Password is NOT set here (write-only, never stored in state).
func responseToModel(ctx context.Context, api *userAPI, m *UserModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.UID = types.Int64Value(api.UID)
	m.Username = types.StringValue(api.Username)
	m.FullName = types.StringValue(api.FullName)

	if api.Email != nil {
		m.Email = types.StringValue(*api.Email)
	} else {
		m.Email = types.StringValue("")
	}

	m.Home = types.StringValue(api.Home)
	m.Shell = types.StringValue(api.Shell)
	m.Locked = types.BoolValue(api.Locked)
	m.PasswordDisabled = types.BoolValue(api.PasswordDisabled)
	m.SMB = types.BoolValue(api.SMB)
	m.SSHPasswordEnabled = types.BoolValue(api.SSHPasswordEnabled)

	if api.SSHPubKey != nil {
		m.SSHPubKey = types.StringValue(*api.SSHPubKey)
	} else {
		m.SSHPubKey = types.StringValue("")
	}

	sudoCmds := api.SudoCommands
	if sudoCmds == nil {
		sudoCmds = []string{}
	}
	sl, d := types.ListValueFrom(ctx, types.StringType, sudoCmds)
	diags.Append(d...)
	m.SudoCommands = sl

	sudoCmdsNP := api.SudoCommandsNoPasswd
	if sudoCmdsNP == nil {
		sudoCmdsNP = []string{}
	}
	snl, d2 := types.ListValueFrom(ctx, types.StringType, sudoCmdsNP)
	diags.Append(d2...)
	m.SudoCommandsNoPasswd = snl

	grps := api.Groups
	if grps == nil {
		grps = []int64{}
	}
	gl, d3 := types.ListValueFrom(ctx, types.Int64Type, grps)
	diags.Append(d3...)
	m.Groups = gl

	groupID, groupOK, gErr := decodeGroupField(api.Group)
	if gErr != nil {
		diags.AddError("Parse group field", gErr.Error())
	} else if groupOK {
		m.Group = types.Int64Value(groupID)
	} else {
		m.Group = types.Int64Null()
	}

	// Computed
	m.Builtin = types.BoolValue(api.Builtin)
	m.Immutable = types.BoolValue(api.Immutable)
	m.Local = types.BoolValue(api.Local)

	// NOTE: Password is NOT set here (write-only)

	return diags
}

// decodeCreateResult decodes the result of user.create, which (being a
// sync, job:false method) may be returned by the middleware either as the
// full created user object or as a bare integer id, depending on
// middleware version. It returns either a populated *userAPI (object
// shape) or a non-zero id (bare-int shape), never both.
func decodeCreateResult(raw json.RawMessage) (*userAPI, int64, error) {
	var api userAPI
	if err := json.Unmarshal(raw, &api); err == nil && api.ID != 0 {
		return &api, 0, nil
	}

	var id int64
	if err := json.Unmarshal(raw, &id); err == nil {
		return nil, id, nil
	}

	return nil, 0, fmt.Errorf("unable to decode user.create result: %s", string(raw))
}

// createPayload includes uid and username (only for initial creation), plus
// group_create — a write-only, create-only flag never read back into state.
// Callers must set either "group" (an existing group id, handled in
// basePayload) or "group_create" (create a new group matching the
// username); the API rejects a create with neither.
func (m *UserModel) createPayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	p, diags := m.basePayload(ctx)
	if !m.UID.IsNull() && !m.UID.IsUnknown() {
		p["uid"] = m.UID.ValueInt64()
	}
	p["username"] = m.Username.ValueString()
	if !m.GroupCreate.IsNull() && !m.GroupCreate.IsUnknown() {
		p["group_create"] = m.GroupCreate.ValueBool()
	}
	return p, diags
}

// updatePayload omits uid and username (immutable after create).
func (m *UserModel) updatePayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	return m.basePayload(ctx)
}

func (m *UserModel) basePayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	var sudoCmds []string
	if !m.SudoCommands.IsNull() && !m.SudoCommands.IsUnknown() {
		diags.Append(m.SudoCommands.ElementsAs(ctx, &sudoCmds, false)...)
	}
	if sudoCmds == nil {
		sudoCmds = []string{}
	}

	var sudoCmdsNP []string
	if !m.SudoCommandsNoPasswd.IsNull() && !m.SudoCommandsNoPasswd.IsUnknown() {
		diags.Append(m.SudoCommandsNoPasswd.ElementsAs(ctx, &sudoCmdsNP, false)...)
	}
	if sudoCmdsNP == nil {
		sudoCmdsNP = []string{}
	}

	var groups []int64
	if !m.Groups.IsNull() && !m.Groups.IsUnknown() {
		diags.Append(m.Groups.ElementsAs(ctx, &groups, false)...)
	}
	if groups == nil {
		groups = []int64{}
	}

	// email: send null (untyped nil) if empty string.
	var email any
	if v := m.Email.ValueString(); v != "" {
		v2 := v
		email = &v2
	}

	// sshpubkey: send null (untyped nil) if empty string.
	var sshpubkey any
	if v := m.SSHPubKey.ValueString(); v != "" {
		v2 := v
		sshpubkey = &v2
	}

	p := map[string]any{
		"full_name":              m.FullName.ValueString(),
		"email":                  email,
		"home":                   m.Home.ValueString(),
		"shell":                  m.Shell.ValueString(),
		"locked":                 m.Locked.ValueBool(),
		"password_disabled":      m.PasswordDisabled.ValueBool(),
		"smb":                    m.SMB.ValueBool(),
		"ssh_password_enabled":   m.SSHPasswordEnabled.ValueBool(),
		"sshpubkey":              sshpubkey,
		"sudo_commands":          sudoCmds,
		"sudo_commands_nopasswd": sudoCmdsNP,
		"groups":                 groups,
	}

	// group: primary group id. Only include when known — on create it may
	// be omitted in favor of group_create (see createPayload).
	if !m.Group.IsNull() && !m.Group.IsUnknown() {
		p["group"] = m.Group.ValueInt64()
	}

	// password: only include if set (write-only, not stored in state)
	if !m.Password.IsNull() && !m.Password.IsUnknown() {
		p["password"] = m.Password.ValueString()
	}

	return p, diags
}

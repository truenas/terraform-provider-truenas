// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package user

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/normalize"
)

// UserModel is the Terraform state model for truenas_user.
type UserModel struct {
	ID                   types.Int64         `tfsdk:"id"`
	UID                  types.Int64         `tfsdk:"uid"`
	Username             types.String        `tfsdk:"username"`
	FullName             types.String        `tfsdk:"full_name"`
	Email                types.String        `tfsdk:"email"`
	Home                 types.String        `tfsdk:"home"`
	HomeMode             normalize.OctalMode `tfsdk:"home_mode"`
	Shell                types.String        `tfsdk:"shell"`
	Locked               types.Bool          `tfsdk:"locked"`
	PasswordDisabled     types.Bool          `tfsdk:"password_disabled"`
	SMB                  types.Bool          `tfsdk:"smb"`
	Webshare             types.Bool          `tfsdk:"webshare"`
	SSHPasswordEnabled   types.Bool          `tfsdk:"ssh_password_enabled"`
	SSHPubKey            normalize.Trimmed   `tfsdk:"sshpubkey"`
	SudoCommands         types.List          `tfsdk:"sudo_commands"`
	SudoCommandsNoPasswd types.List          `tfsdk:"sudo_commands_nopasswd"`
	Groups               types.List          `tfsdk:"groups"`
	Group                types.Int64         `tfsdk:"group"`
	GroupCreate          types.Bool          `tfsdk:"group_create"`
	Password             types.String        `tfsdk:"password"`
	PasswordWOVersion    types.Int64         `tfsdk:"password_wo_version"`
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
	Webshare             types.Bool   `tfsdk:"webshare"`
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
	// home_mode is write-only (accepted on create/update, never returned by
	// user.query); preserve the plan/state value rather than reading it back.
	m.Shell = types.StringValue(api.Shell)
	m.Locked = types.BoolValue(api.Locked)
	m.PasswordDisabled = types.BoolValue(api.PasswordDisabled)
	m.SMB = types.BoolValue(api.SMB)
	m.Webshare = types.BoolValue(api.Webshare)
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
	// TrueNAS auto-adds server-managed auxiliary groups the configuration did not
	// request — notably builtin_users when smb=true — which would otherwise
	// surface as "Provider produced inconsistent result after apply". When the
	// desired set is known (create/update/refresh), keep only the requested
	// groups that actually applied, in the requested order; on import or a data
	// source read (no desired set) keep what the API returns. A group the user
	// explicitly lists is kept.
	if !m.Groups.IsNull() && !m.Groups.IsUnknown() {
		var desired []int64
		diags.Append(m.Groups.ElementsAs(ctx, &desired, false)...)
		have := make(map[int64]bool, len(grps))
		for _, g := range grps {
			have[g] = true
		}
		kept := make([]int64, 0, len(desired))
		for _, g := range desired {
			if have[g] {
				kept = append(kept, g)
			}
		}
		grps = kept
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
	Webshare             bool            `json:"webshare"`
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

// primaryGroupName returns the bsdgrp_group name from the "group" object a
// user.query/get_instance response embeds, or "" when the field is a bare id,
// null, or has no name.
func primaryGroupName(raw json.RawMessage) string {
	var obj struct {
		Name string `json:"bsdgrp_group"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ""
	}
	return obj.Name
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
	// home_mode is write-only (accepted on create/update, never returned by
	// user.query); preserve the plan/state value rather than reading it back.
	m.Shell = types.StringValue(api.Shell)
	m.Locked = types.BoolValue(api.Locked)
	m.PasswordDisabled = types.BoolValue(api.PasswordDisabled)
	m.SMB = types.BoolValue(api.SMB)
	m.Webshare = types.BoolValue(api.Webshare)
	m.SSHPasswordEnabled = types.BoolValue(api.SSHPasswordEnabled)

	// The server strips surrounding whitespace (a key read with file() ends in
	// "\n"); Trimmed's semantic equality keeps the configured spelling. (B-053)
	if api.SSHPubKey != nil {
		m.SSHPubKey = normalize.NewTrimmedValue(*api.SSHPubKey)
	} else {
		m.SSHPubKey = normalize.NewTrimmedValue("")
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
	// TrueNAS auto-adds server-managed auxiliary groups the configuration did not
	// request — notably builtin_users when smb=true — which would otherwise
	// surface as "Provider produced inconsistent result after apply". When the
	// desired set is known (create/update/refresh), keep only the requested
	// groups that actually applied, in the requested order; on import or a data
	// source read (no desired set) keep what the API returns. A group the user
	// explicitly lists is kept.
	if !m.Groups.IsNull() && !m.Groups.IsUnknown() {
		var desired []int64
		diags.Append(m.Groups.ElementsAs(ctx, &desired, false)...)
		have := make(map[int64]bool, len(grps))
		for _, g := range grps {
			have[g] = true
		}
		kept := make([]int64, 0, len(desired))
		for _, g := range desired {
			if have[g] {
				kept = append(kept, g)
			}
		}
		grps = kept
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

	// group_create is a create-time directive, but its effect is visible:
	// user.create with group_create=true makes the primary group the local
	// group named after the username (creating it, or reusing one that already
	// exists). A known plan/state value is kept; an unset one (left out of
	// config on create, or null after import) is derived from that, so an
	// import followed by a plan with group_create = true is a no-op. (B-051)
	if m.GroupCreate.IsNull() || m.GroupCreate.IsUnknown() {
		m.GroupCreate = types.BoolValue(primaryGroupName(api.Group) == api.Username)
	}
	// home_mode is not returned by user.query. An unset value stays null here;
	// ImportState reads it from the home directory's mode.
	if m.HomeMode.IsUnknown() {
		m.HomeMode = normalize.NewOctalModeNull()
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
// optionalUserFields maps each Optional user attribute that basePayload emits
// (as a zero-coerced value when unset) to its plan value. Both createPayload
// (drop unknown/null) and updatePayload (drop unknown/null/unchanged) iterate
// this one list so the two paths cannot drift as fields are added. uid,
// username, group_create, password, and home_mode are handled separately by
// their callers and are intentionally absent. (#43, #53)
func (m *UserModel) optionalUserFields() map[string]attr.Value {
	return map[string]attr.Value{
		"full_name":              m.FullName,
		"email":                  m.Email,
		"home":                   m.Home,
		"shell":                  m.Shell,
		"locked":                 m.Locked,
		"password_disabled":      m.PasswordDisabled,
		"smb":                    m.SMB,
		"webshare":               m.Webshare,
		"ssh_password_enabled":   m.SSHPasswordEnabled,
		"sshpubkey":              m.SSHPubKey,
		"sudo_commands":          m.SudoCommands,
		"sudo_commands_nopasswd": m.SudoCommandsNoPasswd,
		"groups":                 m.Groups,
		"group":                  m.Group,
	}
}

func (m *UserModel) createPayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	p, diags := m.basePayload(ctx)
	// Drop every Optional field left unset (unknown) or null in the plan so
	// TrueNAS applies its own default instead of the zero value basePayload
	// coerced it to. Without this, an unset "home" was sent as "" and
	// user.create rejected it ("String should have at least 1 character"); the
	// same zero-coercion affected the other unset optionals. An explicitly
	// configured value (even false/""/[]) is known and non-null, so it is kept
	// and still sent. (#53)
	for key, v := range m.optionalUserFields() {
		if v.IsNull() || v.IsUnknown() {
			delete(p, key)
		}
	}
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
// updatePayload builds the user.update payload as the DIFF against prior state:
// only fields whose value changed are sent. TrueNAS validates several fields on
// every user.update and rejects some unchanged values on constrained/builtin
// accounts — the reserved administrator rejects both its non-/mnt `home`
// ("Path must start with /mnt/") and its immutable `group` ("cannot be
// changed") even when re-sent identical, which would otherwise block any other
// change to the account. Diffing the plan payload against the prior payload
// drops every unchanged field generically, so no hand-maintained field list can
// fall out of sync and silently reintroduce the bug for a newly-added field.
//
// uid and username are already absent (basePayload omits them). The
// write-only password is kept (by design, #48, it is re-sent on every update);
// home_mode is sent only when it changed. Read (responseToModel) refreshes every field, so
// out-of-band drift still surfaces as a plan diff and is re-sent — the diff is
// taken against the refreshed prior state, not against config. (#43)
func (m *UserModel) updatePayload(ctx context.Context, prior UserModel) (map[string]any, diag.Diagnostics) {
	p, diags := m.basePayload(ctx)
	// Drop each field whose PLAN value is unknown, null, or unchanged from prior
	// state. This is compared at the MODEL level, not on basePayload's output:
	// basePayload coerces an unknown/null field to a zero value ([] / "" / false),
	// so a payload-level diff would read an unmanaged `groups` as an explicit
	// empty-list removal and wipe the account's groups (incl builtin_administrators
	// on an imported admin — the v1.5.13 regression). An EXPLICITLY configured
	// empty value is a known, non-null value, so it is NOT dropped here and is
	// still sent as a real change. uid/username are absent (basePayload omits
	// them); the write-only password is not in this map: Update sets it on the
	// model only when passwordResendNeeded. Read refreshes every field, so out-of-band
	// drift still surfaces as a plan diff and is re-sent. (#43)
	priorFields := prior.optionalUserFields()
	for key, plan := range m.optionalUserFields() {
		if plan.IsNull() || plan.IsUnknown() || plan.Equal(priorFields[key]) {
			delete(p, key)
		}
	}
	// home_mode is applied with a chmod of the home directory on every update
	// that carries it, and an immutable (built-in) account rejects it outright,
	// so it is sent only when it changed. (B-142)
	if m.HomeMode.Equal(prior.HomeMode) {
		delete(p, "home_mode")
	}
	return p, diags
}

// passwordResendNeeded reports whether an update sends the write-only password.
// Every user.update that carries a password re-hashes it and resets
// last_password_change, so re-sending it on unrelated updates kept
// max_password_age from ever firing (B-141). It is sent only when
// password_wo_version changed, or when the server requires it: turning smb on
// for an account that was not an SMB user fails with "Password must be reset
// in order to enable SMB authentication" unless the password comes along.
func passwordResendNeeded(plan, prior UserModel) bool {
	if !plan.PasswordWOVersion.Equal(prior.PasswordWOVersion) {
		return true
	}
	return plan.SMB.ValueBool() && !prior.SMB.ValueBool()
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
		"webshare":               m.Webshare.ValueBool(),
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

	// home_mode: write-only, only include when set (never returned by the API).
	if !m.HomeMode.IsNull() && !m.HomeMode.IsUnknown() && m.HomeMode.ValueString() != "" {
		p["home_mode"] = m.HomeMode.ValueString()
	}

	return p, diags
}

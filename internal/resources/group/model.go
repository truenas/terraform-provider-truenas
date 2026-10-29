// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package group

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/normalize"
)

// GroupModel is the Terraform state model for truenas_group.
type GroupModel struct {
	ID                   types.Int64             `tfsdk:"id"`
	GID                  types.Int64             `tfsdk:"gid"`
	Name                 types.String            `tfsdk:"name"`
	SMB                  types.Bool              `tfsdk:"smb"`
	Users                normalize.UnorderedList `tfsdk:"users"` // int64 user IDs who are members
	SudoCommands         types.List              `tfsdk:"sudo_commands"`
	SudoCommandsNoPasswd types.List              `tfsdk:"sudo_commands_nopasswd"`
	// Computed only
	Builtin   types.Bool `tfsdk:"builtin"`
	Immutable types.Bool `tfsdk:"immutable"`
	Local     types.Bool `tfsdk:"local"`
}

// groupAPI is the JSON wire format for a TrueNAS group object.
type groupAPI struct {
	ID                   int64    `json:"id"`
	GID                  int64    `json:"gid"`
	Group                string   `json:"group"` // group name
	Name                 string   `json:"name"`  // same as group
	SMB                  bool     `json:"smb"`
	Users                []int64  `json:"users"`
	SudoCommands         []string `json:"sudo_commands"`
	SudoCommandsNoPasswd []string `json:"sudo_commands_nopasswd"`
	Builtin              bool     `json:"builtin"`
	Immutable            bool     `json:"immutable"`
	Local                bool     `json:"local"`
}

// responseToModel maps an API response onto a Terraform model. primary is the
// set of user ids whose PRIMARY group is this group: group.query lists them in
// "users" although they are not auxiliary members, and group.update cannot
// remove them, so they are dropped from state unless the configuration (the
// prior plan/state list) names them. (B-058) The server returns "users" in its
// own set-iteration order; the UnorderedList type compares without order.
// (B-057)
func responseToModel(ctx context.Context, api *groupAPI, primary map[int64]bool, m *GroupModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.GID = types.Int64Value(api.GID)
	m.Name = types.StringValue(api.Group) // read from "group" field
	m.SMB = types.BoolValue(api.SMB)

	var desired []int64
	if !m.Users.IsNull() && !m.Users.IsUnknown() {
		diags.Append(m.Users.ElementsAs(ctx, &desired, false)...)
	}
	ul, du := normalize.NewUnorderedListValueFrom(ctx, types.Int64Type, dropPrimaryMembers(api.Users, desired, primary))
	diags.Append(du...)
	m.Users = ul

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

	m.Builtin = types.BoolValue(api.Builtin)
	m.Immutable = types.BoolValue(api.Immutable)
	m.Local = types.BoolValue(api.Local)

	return diags
}

// dropPrimaryMembers returns server without the primary-group members that
// desired does not name. A nil server list gives an empty (non-nil) list.
func dropPrimaryMembers(server, desired []int64, primary map[int64]bool) []int64 {
	want := make(map[int64]bool, len(desired))
	for _, u := range desired {
		want[u] = true
	}
	out := make([]int64, 0, len(server))
	for _, u := range server {
		if want[u] || !primary[u] {
			out = append(out, u)
		}
	}
	return out
}

// decodeCreateResult decodes the result of group.create, which (being a
// sync, job:false method) may be returned by the middleware either as the
// full created group object or as a bare integer id, depending on
// middleware version. It returns either a populated *groupAPI (object
// shape) or a non-zero id (bare-int shape), never both.
func decodeCreateResult(raw json.RawMessage) (*groupAPI, int64, error) {
	var api groupAPI
	if err := json.Unmarshal(raw, &api); err == nil && api.ID != 0 {
		return &api, 0, nil
	}

	var id int64
	if err := json.Unmarshal(raw, &id); err == nil {
		return nil, id, nil
	}

	return nil, 0, fmt.Errorf("unable to decode group.create result: %s", string(raw))
}

// createPayload includes gid and name (only for initial creation). Optional
// fields left unset (unknown) or null are omitted so group.create applies its
// own default (smb defaults to true) instead of a zero value. (B-108)
func (m *GroupModel) createPayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	p, diags := m.basePayload(ctx)
	for key, v := range m.optionalGroupFields() {
		if v.IsNull() || v.IsUnknown() {
			delete(p, key)
		}
	}
	if !m.GID.IsNull() && !m.GID.IsUnknown() {
		p["gid"] = m.GID.ValueInt64()
	}
	p["name"] = m.Name.ValueString()
	return p, diags
}

// updatePayload omits gid and name (immutable after create) and sends only the
// fields whose plan value is known, non-null, and different from prior state,
// so an attribute left out of config is never overwritten with a zero value by
// an unrelated update. (B-037)
func (m *GroupModel) updatePayload(ctx context.Context, prior GroupModel) (map[string]any, diag.Diagnostics) {
	p, diags := m.basePayload(ctx)
	priorFields := prior.optionalGroupFields()
	for key, plan := range m.optionalGroupFields() {
		if plan.IsNull() || plan.IsUnknown() || plan.Equal(priorFields[key]) {
			delete(p, key)
		}
	}
	return p, diags
}

// optionalGroupFields maps each Optional attribute basePayload emits to its
// model value, for createPayload/updatePayload to filter on.
func (m *GroupModel) optionalGroupFields() map[string]attr.Value {
	return map[string]attr.Value{
		"smb":                    m.SMB,
		"sudo_commands":          m.SudoCommands,
		"sudo_commands_nopasswd": m.SudoCommandsNoPasswd,
		"users":                  m.Users,
	}
}

func (m *GroupModel) basePayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
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

	var users []int64
	if !m.Users.IsNull() && !m.Users.IsUnknown() {
		diags.Append(m.Users.ElementsAs(ctx, &users, false)...)
	}
	if users == nil {
		users = []int64{}
	}

	p := map[string]any{
		"smb":                    m.SMB.ValueBool(),
		"sudo_commands":          sudoCmds,
		"sudo_commands_nopasswd": sudoCmdsNP,
		"users":                  users,
	}
	return p, diags
}

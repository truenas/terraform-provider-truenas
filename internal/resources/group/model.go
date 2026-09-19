// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package group

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// GroupModel is the Terraform state model for truenas_group.
type GroupModel struct {
	ID                   types.Int64  `tfsdk:"id"`
	GID                  types.Int64  `tfsdk:"gid"`
	Name                 types.String `tfsdk:"name"`
	SMB                  types.Bool   `tfsdk:"smb"`
	SudoCommands         types.List   `tfsdk:"sudo_commands"`
	SudoCommandsNoPasswd types.List   `tfsdk:"sudo_commands_nopasswd"`
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
	SudoCommands         []string `json:"sudo_commands"`
	SudoCommandsNoPasswd []string `json:"sudo_commands_nopasswd"`
	Builtin              bool     `json:"builtin"`
	Immutable            bool     `json:"immutable"`
	Local                bool     `json:"local"`
}

// responseToModel maps an API response onto a Terraform model.
func responseToModel(ctx context.Context, api *groupAPI, m *GroupModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.GID = types.Int64Value(api.GID)
	m.Name = types.StringValue(api.Group) // read from "group" field
	m.SMB = types.BoolValue(api.SMB)

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

// createPayload includes gid and name (only for initial creation).
func (m *GroupModel) createPayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	p, diags := m.basePayload(ctx)
	if !m.GID.IsNull() && !m.GID.IsUnknown() {
		p["gid"] = m.GID.ValueInt64()
	}
	p["name"] = m.Name.ValueString()
	return p, diags
}

// updatePayload omits gid and name (immutable after create).
func (m *GroupModel) updatePayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	return m.basePayload(ctx)
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

	return map[string]any{
		"smb":                    m.SMB.ValueBool(),
		"sudo_commands":          sudoCmds,
		"sudo_commands_nopasswd": sudoCmdsNP,
	}, diags
}

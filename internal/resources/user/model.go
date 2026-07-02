package user

import (
	"context"

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

	m.Builtin = types.BoolValue(api.Builtin)
	m.Immutable = types.BoolValue(api.Immutable)
	m.Local = types.BoolValue(api.Local)

	return diags
}

// userAPI is the JSON wire format for a TrueNAS user object.
type userAPI struct {
	ID                   int64    `json:"id"`
	UID                  int64    `json:"uid"`
	Username             string   `json:"username"`
	FullName             string   `json:"full_name"`
	Email                *string  `json:"email"`
	Home                 string   `json:"home"`
	Shell                string   `json:"shell"`
	Locked               bool     `json:"locked"`
	PasswordDisabled     bool     `json:"password_disabled"`
	SMB                  bool     `json:"smb"`
	SSHPasswordEnabled   bool     `json:"ssh_password_enabled"`
	SSHPubKey            *string  `json:"sshpubkey"`
	SudoCommands         []string `json:"sudo_commands"`
	SudoCommandsNoPasswd []string `json:"sudo_commands_nopasswd"`
	Groups               []int64  `json:"groups"`
	Builtin              bool     `json:"builtin"`
	Immutable            bool     `json:"immutable"`
	Local                bool     `json:"local"`
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

	// Computed
	m.Builtin = types.BoolValue(api.Builtin)
	m.Immutable = types.BoolValue(api.Immutable)
	m.Local = types.BoolValue(api.Local)

	// NOTE: Password is NOT set here (write-only)

	return diags
}

// createPayload includes uid and username (only for initial creation).
func (m *UserModel) createPayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	p, diags := m.basePayload(ctx)
	if !m.UID.IsNull() && !m.UID.IsUnknown() {
		p["uid"] = m.UID.ValueInt64()
	}
	p["username"] = m.Username.ValueString()
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

	// password: only include if set (write-only, not stored in state)
	if !m.Password.IsNull() && !m.Password.IsUnknown() {
		p["password"] = m.Password.ValueString()
	}

	return p, diags
}

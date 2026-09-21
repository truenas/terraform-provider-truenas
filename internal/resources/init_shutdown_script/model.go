// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package init_shutdown_script

import (
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// InitShutdownScriptModel is the Terraform state/plan model for
// truenas_init_shutdown_script.
type InitShutdownScriptModel struct {
	ID      types.Int64  `tfsdk:"id"`
	Type    types.String `tfsdk:"type"` // COMMAND, SCRIPT
	Command types.String `tfsdk:"command"`
	Script  types.String `tfsdk:"script"`
	When    types.String `tfsdk:"when"` // PREINIT, POSTINIT, SHUTDOWN
	Enabled types.Bool   `tfsdk:"enabled"`
	Timeout types.Int64  `tfsdk:"timeout"`
	Comment types.String `tfsdk:"comment"`
}

// InitShutdownScriptDataSourceModel is the read-only model for the
// truenas_init_shutdown_script datasource, looked up by "comment".
type InitShutdownScriptDataSourceModel struct {
	ID      types.Int64  `tfsdk:"id"`
	Type    types.String `tfsdk:"type"`
	Command types.String `tfsdk:"command"`
	Script  types.String `tfsdk:"script"`
	When    types.String `tfsdk:"when"`
	Enabled types.Bool   `tfsdk:"enabled"`
	Timeout types.Int64  `tfsdk:"timeout"`
	Comment types.String `tfsdk:"comment"`
}

// initShutdownScriptAPI mirrors the JSON object returned by
// initshutdownscript.create, initshutdownscript.update,
// initshutdownscript.get_instance, and initshutdownscript.query. Probed
// against live TrueNAS 25.10 and 26.0 boxes (identical wire shape on
// both releases, no version gating needed): "command" and "script" are each
// declared nullable in the method schema (anyOf string|null, default ""),
// but in practice every observed create/query/get_instance response returns
// them as plain (non-null) strings — "" for whichever of the two doesn't
// apply to the configured "type" — never JSON null. Modeled here as plain
// strings accordingly; a defensive *string would add complexity the wire
// never actually exercises.
type initShutdownScriptAPI struct {
	ID      int64  `json:"id"`
	Type    string `json:"type"`
	Command string `json:"command"`
	Script  string `json:"script"`
	When    string `json:"when"`
	Enabled bool   `json:"enabled"`
	Timeout int64  `json:"timeout"`
	Comment string `json:"comment"`
}

// responseToModel maps an initShutdownScriptAPI response onto an
// InitShutdownScriptModel.
func responseToModel(api *initShutdownScriptAPI, m *InitShutdownScriptModel) {
	m.ID = types.Int64Value(api.ID)
	m.Type = types.StringValue(api.Type)
	m.Command = types.StringValue(api.Command)
	m.Script = types.StringValue(api.Script)
	m.When = types.StringValue(api.When)
	m.Enabled = types.BoolValue(api.Enabled)
	m.Timeout = types.Int64Value(api.Timeout)
	m.Comment = types.StringValue(api.Comment)
}

// responseToDataSourceModel maps an initShutdownScriptAPI response onto an
// InitShutdownScriptDataSourceModel.
func responseToDataSourceModel(api *initShutdownScriptAPI, m *InitShutdownScriptDataSourceModel) {
	m.ID = types.Int64Value(api.ID)
	m.Type = types.StringValue(api.Type)
	m.Command = types.StringValue(api.Command)
	m.Script = types.StringValue(api.Script)
	m.When = types.StringValue(api.When)
	m.Enabled = types.BoolValue(api.Enabled)
	m.Timeout = types.Int64Value(api.Timeout)
	m.Comment = types.StringValue(api.Comment)
}

// validateTypeFields enforces the conditional-required relationship probed
// live against initshutdownscript.create: "command" is required (and
// rejected as missing server-side with EINVAL) when type is "COMMAND", and
// symmetrically "script" is required when type is "SCRIPT". This runs
// BEFORE any API call, from within apiPayload, so a bad combination is
// rejected at plan/apply-preflight time with a clear provider-side message
// instead of a raw EINVAL bubbling up from the server. Only a genuinely
// non-empty, known value counts as "the user actually set this": a null,
// unknown, or empty-string value is indistinguishable from "not set" and
// left for the server-side check that already exists as a backstop.
func (m *InitShutdownScriptModel) validateTypeFields() diag.Diagnostics {
	var diags diag.Diagnostics

	switch m.Type.ValueString() {
	case "COMMAND":
		if m.Command.IsNull() || m.Command.IsUnknown() || m.Command.ValueString() == "" {
			diags.AddError(
				"Invalid init/shutdown script configuration",
				"\"command\" is required when \"type\" is \"COMMAND\".",
			)
		}
	case "SCRIPT":
		if m.Script.IsNull() || m.Script.IsUnknown() || m.Script.ValueString() == "" {
			diags.AddError(
				"Invalid init/shutdown script configuration",
				"\"script\" is required when \"type\" is \"SCRIPT\".",
			)
		}
	}

	return diags
}

// apiPayload builds the map expected by
// initshutdownscript.create/initshutdownscript.update. "type" and "when"
// are always included (Required). "command"/"script"/"enabled"/
// "timeout"/"comment" are Optional+Computed: each is included only when
// known and non-null, so an unset optional is omitted entirely and the
// TrueNAS-side default takes effect instead of an explicit zero value.
// validateTypeFields runs first so a conditional-required violation is
// reported before any network call.
func (m *InitShutdownScriptModel) apiPayload() (map[string]any, diag.Diagnostics) {
	diags := m.validateTypeFields()
	if diags.HasError() {
		return nil, diags
	}

	p := map[string]any{
		"type": m.Type.ValueString(),
		"when": m.When.ValueString(),
	}

	if !m.Command.IsNull() && !m.Command.IsUnknown() {
		p["command"] = m.Command.ValueString()
	}
	if !m.Script.IsNull() && !m.Script.IsUnknown() {
		p["script"] = m.Script.ValueString()
	}
	if !m.Enabled.IsNull() && !m.Enabled.IsUnknown() {
		p["enabled"] = m.Enabled.ValueBool()
	}
	if !m.Timeout.IsNull() && !m.Timeout.IsUnknown() {
		p["timeout"] = m.Timeout.ValueInt64()
	}
	if !m.Comment.IsNull() && !m.Comment.IsUnknown() {
		p["comment"] = m.Comment.ValueString()
	}

	return p, diags
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package twofactor_auth

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// twoFactorAuthResourceID is the fixed Terraform ID for this singleton
// resource: there is exactly one two-factor authentication configuration
// per TrueNAS system (auth.twofactor.config always returns a single
// record), and it is never created or deleted on TrueNAS itself. The API's
// own numeric "id" (probed: always 1) is an internal implementation detail
// and is intentionally not surfaced in the model, mirroring the
// resilver_config/audit_config singleton pattern.
const twoFactorAuthResourceID = "twofactor_auth"

// servicesAttrTypes describes the attribute types of the nested "services"
// object.
var servicesAttrTypes = map[string]attr.Type{
	"ssh": types.BoolType,
}

// ServicesModel maps to the nested "services" attribute: which services
// require two-factor authentication in addition to the system-wide toggle.
type ServicesModel struct {
	SSH types.Bool `tfsdk:"ssh"`
}

// TwoFactorAuthModel is the Terraform state model for
// truenas_twofactor_auth.
type TwoFactorAuthModel struct {
	ID       types.String `tfsdk:"id"` // fixed: "twofactor_auth"
	Enabled  types.Bool   `tfsdk:"enabled"`
	Window   types.Int64  `tfsdk:"window"`
	Services types.Object `tfsdk:"services"`
}

// TwoFactorAuthDataSourceModel is the read-only model for the
// truenas_twofactor_auth datasource.
type TwoFactorAuthDataSourceModel struct {
	ID       types.String `tfsdk:"id"`
	Enabled  types.Bool   `tfsdk:"enabled"`
	Window   types.Int64  `tfsdk:"window"`
	Services types.Object `tfsdk:"services"`
}

// servicesAPI mirrors the nested "services" object returned by
// auth.twofactor.config and auth.twofactor.update. Probed live: identical
// shape on TrueNAS 25.10 and 26.0 — a single "ssh" key.
type servicesAPI struct {
	SSH bool `json:"ssh"`
}

// twoFactorAuthAPI mirrors the JSON object returned by auth.twofactor.config
// and auth.twofactor.update. Probed live against TrueNAS 25.10 and 26.0:
// identical shape, no version gating needed.
type twoFactorAuthAPI struct {
	ID       int64       `json:"id"`
	Enabled  bool        `json:"enabled"`
	Window   int64       `json:"window"`
	Services servicesAPI `json:"services"`
}

// servicesObjectValue builds a types.Object for the nested "services"
// attribute from an API response.
func servicesObjectValue(ctx context.Context, api servicesAPI) (types.Object, diag.Diagnostics) {
	return types.ObjectValueFrom(ctx, servicesAttrTypes, ServicesModel{
		SSH: types.BoolValue(api.SSH),
	})
}

// responseToModel maps a twoFactorAuthAPI response onto a
// TwoFactorAuthModel.
func responseToModel(ctx context.Context, api *twoFactorAuthAPI, m *TwoFactorAuthModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(twoFactorAuthResourceID)
	m.Enabled = types.BoolValue(api.Enabled)
	m.Window = types.Int64Value(api.Window)

	services, d := servicesObjectValue(ctx, api.Services)
	diags.Append(d...)
	m.Services = services

	return diags
}

// responseToDataSourceModel maps a twoFactorAuthAPI response onto a
// TwoFactorAuthDataSourceModel.
func responseToDataSourceModel(ctx context.Context, api *twoFactorAuthAPI, m *TwoFactorAuthDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(twoFactorAuthResourceID)
	m.Enabled = types.BoolValue(api.Enabled)
	m.Window = types.Int64Value(api.Window)

	services, d := servicesObjectValue(ctx, api.Services)
	diags.Append(d...)
	m.Services = services

	return diags
}

// updatePayload builds the auth.twofactor.update argument. Every field is
// guarded: each is only included when non-null/non-unknown, so an unset
// optional is omitted entirely and the TrueNAS-side current value is left
// unchanged rather than overwritten with an explicit zero value.
//
// CALLERS MUST invoke this on a model populated from req.Config
// (req.Config.Get), never from req.Plan. "enabled", "window", and
// "services" are Optional+Computed with UseStateForUnknown plan modifiers:
// for any of them left unset by the user, the *plan* value is not
// null — the modifier copies the prior state's value into the plan so
// Terraform can show a stable diff. Building the payload from the plan
// would therefore resend a field's last-known value on every Create/Update
// even when the user never configured it, silently reasserting it against
// whatever the box's current value happens to be (a revert race if that
// value changed since the last read). req.Config, by contrast, stays null
// for anything the user did not set in HCL regardless of plan modifiers,
// so inclusion here is driven strictly by what the user explicitly
// configured. This is what makes it possible for the committed acceptance
// test to mutate "window" alone without ever touching "enabled": as long
// as the test config never sets "enabled", it stays null in req.Config, is
// omitted here, and the box's current value is left untouched by
// auth.twofactor.update's partial-update semantics.
func (m *TwoFactorAuthModel) updatePayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	p := map[string]any{}

	if !m.Enabled.IsNull() && !m.Enabled.IsUnknown() {
		p["enabled"] = m.Enabled.ValueBool()
	}
	if !m.Window.IsNull() && !m.Window.IsUnknown() {
		p["window"] = m.Window.ValueInt64()
	}
	if !m.Services.IsNull() && !m.Services.IsUnknown() {
		var svc ServicesModel
		diags.Append(m.Services.As(ctx, &svc, basetypes.ObjectAsOptions{})...)
		p["services"] = map[string]bool{
			"ssh": svc.SSH.ValueBool(),
		}
	}

	return p, diags
}

// deleteWarningDiagnostics builds the warning diagnostic emitted by Delete.
// Delete makes NO client calls: two-factor authentication is
// security-critical, system-wide configuration, so removing this resource
// from Terraform state must never disable 2FA (or otherwise change it) on
// the box. Splitting this into its own function keeps Delete's "no client
// calls" contract independently unit-testable.
func deleteWarningDiagnostics() diag.Diagnostics {
	var diags diag.Diagnostics
	diags.AddWarning(
		"Two-factor authentication configuration left in place",
		"Two-factor authentication configuration left in place; removed from Terraform state only",
	)
	return diags
}

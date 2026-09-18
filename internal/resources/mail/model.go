// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package mail

import (
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// mailResourceID is the fixed Terraform ID for this singleton resource:
// there is exactly one mail configuration per TrueNAS system, and it is
// never created or deleted on TrueNAS itself.
const mailResourceID = "mail"

// MailModel is the Terraform state model for truenas_mail.
type MailModel struct {
	ID             types.String `tfsdk:"id"` // fixed: "mail"
	FromEmail      types.String `tfsdk:"fromemail"`
	FromName       types.String `tfsdk:"fromname"`
	OutgoingServer types.String `tfsdk:"outgoingserver"`
	Port           types.Int64  `tfsdk:"port"`
	Security       types.String `tfsdk:"security"` // PLAIN, SSL, TLS
	SMTP           types.Bool   `tfsdk:"smtp"`     // SMTP auth enabled
	User           types.String `tfsdk:"user"`
	Pass           types.String `tfsdk:"pass"` // write-only, Sensitive
}

// MailDataSourceModel is the read-only model for the truenas_mail
// datasource. It has no "pass" field: the API never returns the password,
// so a datasource attribute for it would always read as empty/unknown.
type MailDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	FromEmail      types.String `tfsdk:"fromemail"`
	FromName       types.String `tfsdk:"fromname"`
	OutgoingServer types.String `tfsdk:"outgoingserver"`
	Port           types.Int64  `tfsdk:"port"`
	Security       types.String `tfsdk:"security"`
	SMTP           types.Bool   `tfsdk:"smtp"`
	User           types.String `tfsdk:"user"`
}

// mailAPI mirrors the JSON object returned by mail.config and accepted
// (as a subset) by mail.update. Pass is intentionally absent: mail.config
// never returns the password, and this struct is only used to decode API
// responses.
type mailAPI struct {
	ID             int64   `json:"id"`
	FromEmail      string  `json:"fromemail"`
	FromName       string  `json:"fromname"`
	OutgoingServer string  `json:"outgoingserver"`
	Port           int64   `json:"port"`
	Security       string  `json:"security"`
	SMTP           bool    `json:"smtp"`
	User           *string `json:"user"`
}

// responseToModel maps an API response onto a Terraform model. Pass is NOT
// set here (write-only, never stored in state).
func responseToModel(api *mailAPI, m *MailModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(mailResourceID)
	m.FromEmail = types.StringValue(api.FromEmail)
	m.FromName = types.StringValue(api.FromName)
	m.OutgoingServer = types.StringValue(api.OutgoingServer)
	m.Port = types.Int64Value(api.Port)
	m.Security = types.StringValue(api.Security)
	m.SMTP = types.BoolValue(api.SMTP)

	if api.User != nil {
		m.User = types.StringValue(*api.User)
	} else {
		m.User = types.StringValue("")
	}

	// NOTE: Pass is NOT set here (write-only).

	return diags
}

// responseToDataSourceModel maps an API response onto a
// MailDataSourceModel.
func responseToDataSourceModel(api *mailAPI, m *MailDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(mailResourceID)
	m.FromEmail = types.StringValue(api.FromEmail)
	m.FromName = types.StringValue(api.FromName)
	m.OutgoingServer = types.StringValue(api.OutgoingServer)
	m.Port = types.Int64Value(api.Port)
	m.Security = types.StringValue(api.Security)
	m.SMTP = types.BoolValue(api.SMTP)

	if api.User != nil {
		m.User = types.StringValue(*api.User)
	} else {
		m.User = types.StringValue("")
	}

	return diags
}

// updatePayload builds the mail.update argument. Every field is guarded:
// fromemail/fromname/outgoingserver/port/security/smtp are only included
// when known (Optional+Computed, so "unknown" means "let the server keep
// its current value" on Create and simply "not part of this plan" is not
// possible since Computed always resolves to a known value before apply,
// but the guard keeps this function safe to reuse for partial payloads).
// user is only included when known (non-null, non-unknown); a known empty
// string is sent as nil so that a previously set user can be cleared. pass
// is only included when set
// (write-only; a null/unknown value means "leave the current password
// alone").
func (m *MailModel) updatePayload() map[string]any {
	p := map[string]any{}

	if !m.FromEmail.IsNull() && !m.FromEmail.IsUnknown() {
		p["fromemail"] = m.FromEmail.ValueString()
	}
	if !m.FromName.IsNull() && !m.FromName.IsUnknown() {
		p["fromname"] = m.FromName.ValueString()
	}
	if !m.OutgoingServer.IsNull() && !m.OutgoingServer.IsUnknown() {
		p["outgoingserver"] = m.OutgoingServer.ValueString()
	}
	if !m.Port.IsNull() && !m.Port.IsUnknown() {
		p["port"] = m.Port.ValueInt64()
	}
	if !m.Security.IsNull() && !m.Security.IsUnknown() {
		p["security"] = m.Security.ValueString()
	}
	if !m.SMTP.IsNull() && !m.SMTP.IsUnknown() {
		p["smtp"] = m.SMTP.ValueBool()
	}
	if !m.User.IsNull() && !m.User.IsUnknown() {
		if v := m.User.ValueString(); v != "" {
			p["user"] = v
		} else {
			p["user"] = nil
		}
	}
	if !m.Pass.IsNull() && !m.Pass.IsUnknown() {
		p["pass"] = m.Pass.ValueString()
	}

	return p
}

// basePayloadFromConfig builds a mail.update payload containing every
// writable, non-secret field taken from a live mail.config response. It is
// the base onto which plan-known values are overlaid (see mergedPayload):
// mail.update requires several fields (at minimum fromemail) on every call,
// even when the Terraform config only sets one cosmetic field such as
// fromname, so the fields the plan doesn't know about still need to be sent
// with their current live value.
func basePayloadFromConfig(api *mailAPI) map[string]any {
	p := map[string]any{
		"fromemail":      api.FromEmail,
		"fromname":       api.FromName,
		"outgoingserver": api.OutgoingServer,
		"port":           api.Port,
		"security":       api.Security,
		"smtp":           api.SMTP,
	}
	if api.User != nil {
		p["user"] = *api.User
	} else {
		p["user"] = nil
	}
	return p
}

// mergedPayload builds the full mail.update argument for Create/Update: it
// starts from the live config's writable fields (basePayloadFromConfig) and
// overlays the plan's known values on top (updatePayload), so a plan value
// always wins over the live value it's replacing. pass is never part of the
// base (it's a write-only secret, absent from mailAPI) and is included only
// when updatePayload includes it, i.e. only when the plan sets it.
func mergedPayload(live *mailAPI, m *MailModel) map[string]any {
	p := basePayloadFromConfig(live)
	for k, v := range m.updatePayload() {
		p[k] = v
	}
	return p
}

// deleteWarningDiagnostics builds the warning diagnostic emitted by Delete.
// Delete makes NO client calls: mail settings are system-critical, so
// removing this resource from Terraform state must never blank out the
// box's mail configuration. Splitting this into its own function keeps
// Delete's "no client calls" contract independently unit-testable.
func deleteWarningDiagnostics() diag.Diagnostics {
	var diags diag.Diagnostics
	diags.AddWarning(
		"Mail configuration left in place",
		"mail configuration left in place; removed from Terraform state only",
	)
	return diags
}

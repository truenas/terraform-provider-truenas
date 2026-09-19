// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package truecommand_config

import (
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// trueCommandConfigResourceID is the fixed Terraform ID for this singleton
// resource: there is exactly one TrueCommand configuration per TrueNAS
// system (truecommand.config always returns a single record), and it is
// never created or deleted on TrueNAS itself. The API's own numeric "id"
// (probed live on both TrueNAS 25.10.4 HA and 26.0: always 1) is an internal
// implementation detail and is intentionally not surfaced in the model,
// mirroring the tn_connect_config/webshare_config/twofactor_auth singleton
// pattern.
const trueCommandConfigResourceID = "truecommand_config"

// No version floor exists for this resource: truecommand.config/
// truecommand.update are present, with an IDENTICAL shape, on BOTH probed
// releases:
//
//   - TrueNAS 25.10.4 HA (wss://10.220.16.188): truecommand.config = {"id": 1,
//     "api_key": null, "enabled": false, "remote_ip_address": null,
//     "remote_url": null, "status": "DISABLED", "status_reason":
//     "Truecommand service is disabled."}. truecommand.update's accepts
//     schema (core.get_methods) exposes EXACTLY "enabled" (bool) and
//     "api_key" (16-character string or null) — nothing else.
//   - TrueNAS 26.0 (wss://192.168.1.68): identical truecommand.config shape,
//     identical truecommand.update accepts schema.
//
// DECISIVE PROBE (both releases, live, 2026-07-23): unlike tn_connect.update
// (whose ONLY writable field on TrueNAS 26.0 is "enabled" — see
// tn_connect_config/model.go), truecommand.update's second writable field,
// "api_key", is genuinely usable without ever touching "enabled". With the
// box's live config already at enabled=false, sending ONLY
// {"api_key": "abcd1234abcd1234"} (a schema-valid 16-character throwaway
// string) left "enabled" at false, left "status"/"status_reason" at
// "DISABLED"/"Truecommand service is disabled." (no attempted outbound
// connection to iX Portal or a TrueCommand instance), and the new value
// round-tripped VERBATIM, UNMASKED, on the immediately following
// truecommand.config read — confirmed identically on both probed releases.
// Restoring {"api_key": null} afterward cleanly returned the box to its
// original state on both boxes. This is the basis for:
//
//  1. "api_key" is modeled Sensitive but NOT WriteOnly (see schema.go): it
//     is safe and meaningful to read back from state, unlike a
//     write-only/never-returned secret.
//  2. This resource's Tier 2 acceptance test (api_key set-and-restore,
//     enabled always explicit false) is NOT a documented skip, unlike
//     tn_connect_config's — see acceptance_test.go.
//
// SAFETY (read before setting "enabled" to true): setting "enabled" to true
// is what actually starts a real connection attempt from this system to a
// TrueCommand instance (self-hosted or iX-hosted) using "api_key" — a real,
// external side effect, not a mere local configuration change (mirrors
// tn_connect_config's enrollment risk). This provider's own committed tests
// never set "enabled" to true. Every other attribute is Computed-only,
// sourced from truecommand.config.

// truecommandConfigAPI mirrors the JSON object returned by
// truecommand.config and truecommand.update. "api_key" is nullable ("null
// if not configured", per the probed method description) — a pointer so an
// absent/cleared key round-trips as Go nil, mapped to a Terraform null in
// responseToModel/responseToDataSourceModel. "remote_url"/
// "remote_ip_address" are likewise nullable ("null if not connected").
type truecommandConfigAPI struct {
	ID              int64   `json:"id"`
	APIKey          *string `json:"api_key"`
	Enabled         bool    `json:"enabled"`
	Status          string  `json:"status"`
	StatusReason    string  `json:"status_reason"`
	RemoteURL       *string `json:"remote_url"`
	RemoteIPAddress *string `json:"remote_ip_address"`
}

// TrueCommandConfigModel is the Terraform state model for
// truenas_truecommand_config.
//
// "enabled" and "api_key" are the ONLY writable fields (both
// Optional+Computed) — see updatePayload's doc comment. Every other field
// is Computed-only, sourced entirely from truecommand.config/
// truecommand.update's response and never sent back to the API.
type TrueCommandConfigModel struct {
	ID              types.String `tfsdk:"id"`
	Enabled         types.Bool   `tfsdk:"enabled"`
	APIKey          types.String `tfsdk:"api_key"` // Sensitive, nullable; NOT WriteOnly — see model.go's doc comment
	Status          types.String `tfsdk:"status"`
	StatusReason    types.String `tfsdk:"status_reason"`
	RemoteURL       types.String `tfsdk:"remote_url"`
	RemoteIPAddress types.String `tfsdk:"remote_ip_address"`
}

// TrueCommandConfigDataSourceModel is the read-only model for the
// truenas_truecommand_config datasource. Same shape as
// TrueCommandConfigModel minus the writable distinction (every field is
// Computed in the datasource schema). "api_key" is included (Computed +
// Sensitive), matching cloud_backup's precedent of exposing a
// Sensitive-but-not-WriteOnly credential field through its datasource too.
type TrueCommandConfigDataSourceModel struct {
	ID              types.String `tfsdk:"id"`
	Enabled         types.Bool   `tfsdk:"enabled"`
	APIKey          types.String `tfsdk:"api_key"`
	Status          types.String `tfsdk:"status"`
	StatusReason    types.String `tfsdk:"status_reason"`
	RemoteURL       types.String `tfsdk:"remote_url"`
	RemoteIPAddress types.String `tfsdk:"remote_ip_address"`
}

// responseToModel maps a truecommandConfigAPI response onto a
// TrueCommandConfigModel.
func responseToModel(api *truecommandConfigAPI, m *TrueCommandConfigModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(trueCommandConfigResourceID)
	m.Enabled = types.BoolValue(api.Enabled)
	m.APIKey = types.StringPointerValue(api.APIKey)
	m.Status = types.StringValue(api.Status)
	m.StatusReason = types.StringValue(api.StatusReason)
	m.RemoteURL = types.StringPointerValue(api.RemoteURL)
	m.RemoteIPAddress = types.StringPointerValue(api.RemoteIPAddress)

	return diags
}

// responseToDataSourceModel maps a truecommandConfigAPI response onto a
// TrueCommandConfigDataSourceModel.
func responseToDataSourceModel(api *truecommandConfigAPI, m *TrueCommandConfigDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(trueCommandConfigResourceID)
	m.Enabled = types.BoolValue(api.Enabled)
	m.APIKey = types.StringPointerValue(api.APIKey)
	m.Status = types.StringValue(api.Status)
	m.StatusReason = types.StringValue(api.StatusReason)
	m.RemoteURL = types.StringPointerValue(api.RemoteURL)
	m.RemoteIPAddress = types.StringPointerValue(api.RemoteIPAddress)

	return diags
}

// updatePayload builds the map[string]any payload for truecommand.update.
//
// Only "enabled" and "api_key" are ever included, and only when the caller
// explicitly configured them — truecommand.update's own accepts schema
// (core.get_methods, probed live and IDENTICAL on both TrueNAS 25.10.4 HA and
// 26.0) exposes exactly these two properties, both optional in the payload
// (a genuine partial update: an omitted key means "no change", confirmed
// live by the api_key-only round trip documented in this file's top-level
// doc comment, which left "enabled" untouched).
//
// CALLERS MUST invoke this on a model populated from req.Config
// (req.Config.Get), never from req.Plan: both "enabled" and "api_key" are
// Optional+Computed with a UseStateForUnknown plan modifier, so on Update
// the *plan* value is NOT null for a user who left either unset in HCL —
// the modifier echoes the prior state's value into the plan. Sourcing from
// req.Plan would therefore resend both fields on every Update even when the
// user never configured them, which for "enabled" specifically would mean
// silently re-asserting whatever value happens to be in Terraform state —
// including, worst case, re-sending true. req.Config stays null for
// anything the user did not set in HCL regardless of plan modifiers, so it
// is the only correct and safe source here. Mirrors the tn_connect_config/
// twofactor_auth/webshare_config precedent.
//
// Note (shared framework limitation, not specific to this resource): HCL
// has no way to distinguish an omitted optional attribute from one
// explicitly set to null, so `api_key` cannot be driven back to an explicit
// null through this resource's own Terraform lifecycle once set to a real
// value — only ever to a different non-null value. Clearing it for real
// requires calling truecommand.update directly (see acceptance_test.go's
// use of acctest.RestoreCall), the same limitation cloud_backup's
// cache_path/rate_limit and this provider's other nullable Optional+Computed
// fields already carry.
func (m *TrueCommandConfigModel) updatePayload() map[string]any {
	p := map[string]any{}
	if !m.Enabled.IsNull() && !m.Enabled.IsUnknown() {
		p["enabled"] = m.Enabled.ValueBool()
	}
	if !m.APIKey.IsNull() && !m.APIKey.IsUnknown() {
		p["api_key"] = m.APIKey.ValueString()
	}
	return p
}

// deleteWarningDiagnostics builds the warning diagnostic emitted by Delete.
// Delete makes NO client calls: truecommand.config governs whether this
// system is connected to a TrueCommand instance, so removing this resource
// from Terraform state must NEVER disable (or otherwise change) an active
// connection or clear a stored api_key. Mirrors the tn_connect_config/
// webshare_config/twofactor_auth precedent exactly.
func deleteWarningDiagnostics() diag.Diagnostics {
	var diags diag.Diagnostics
	diags.AddWarning(
		"TrueCommand configuration left in place",
		"TrueCommand configuration left in place; removed from Terraform state only",
	)
	return diags
}

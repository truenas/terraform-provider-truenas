// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package tn_connect_config

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// tnConnectConfigResourceID is the fixed Terraform ID for this singleton
// resource: there is exactly one TrueNAS Connect configuration per TrueNAS
// system (tn_connect.config always returns a single record), and it is
// never created or deleted on TrueNAS itself. The API's own numeric "id"
// (probed live on both TrueNAS 25.10 and 26.0: always 1) is an internal
// implementation detail and is intentionally not surfaced in the model,
// mirroring the webshare_config/twofactor_auth singleton pattern.
const tnConnectConfigResourceID = "tn_connect_config"

// No version floor exists for this resource: tn_connect.config/tn_connect.update
// are present on BOTH probed releases (unlike webshare/lxc_config/
// container_device, which are absent on TrueNAS 25.10). Probed live:
//
//   - TrueNAS 25.10 (wss://192.168.1.249): tn_connect namespace present, 5
//     methods (config, generate_claim_token, get_registration_uri,
//     ip_choices, update).
//   - TrueNAS 26.0 (wss://192.168.1.68): tn_connect namespace present, 6
//     methods (adds ips_with_hostnames).
//
// What DOES differ sharply between the two releases is tn_connect.update's
// own accepts schema, which is why every field below except "enabled" is
// Computed-only rather than user-writable — see updatePayload's doc comment
// for the decisive finding this is built on.
//
// tnConnectConfigAPI mirrors the JSON object returned by tn_connect.config
// and tn_connect.update. Fields present on only one probed release use
// pointer types so an absent JSON key round-trips as Go nil (mapped to a
// Terraform null in responseToModel/responseToDataSourceModel below),
// distinguished from a present-but-empty value:
//
//	TrueNAS 25.10 (192.168.1.249, enabled=false):
//	{"id": 1, "enabled": false, "registration_details": {}, "ips": [],
//	 "interfaces": [], "interfaces_ips": [], "use_all_interfaces": true,
//	 "status": "DISABLED", "status_reason": "TrueNAS Connect is disabled",
//	 "certificate": null,
//	 "account_service_base_url": "https://account-service.tys1.truenasconnect.net/",
//	 "leca_service_base_url": "https://dns-service.tys1.truenasconnect.net/",
//	 "tnc_base_url": "https://web.truenasconnect.net/",
//	 "heartbeat_url": "https://heartbeat-service.tys1.truenasconnect.net/"}
//	 -- no "tier", no "last_heartbeat_failure_datetime" keys at all.
//
//	TrueNAS 26.0 (192.168.1.68, enabled=true — real, already-enrolled
//	production TrueNAS Connect account, FOUNDATION tier):
//	{"id": 1, "enabled": true,
//	 "registration_details": {"account_id": "...", "iat": ..., "iss": "TrueNAS",
//	  "jti": "...", "scopes": [...], "system_id": "...", "tier": 1,
//	  "token_type": "system"},
//	 "status": "CONFIGURED", "status_reason": "TrueNAS Connect is configured",
//	 "certificate": 2, "tier": "FOUNDATION", "last_heartbeat_failure_datetime": null,
//	 "account_service_base_url": "...", "leca_service_base_url": "...",
//	 "tnc_base_url": "...", "heartbeat_url": "..."}
//	 -- no "ips", "interfaces", "interfaces_ips", "use_all_interfaces" keys
//	 at all: TrueNAS 26.0 dropped the manual IP/interface-selection fields
//	 entirely in favor of automatic selection.
type tnConnectConfigAPI struct {
	ID                           int64           `json:"id"`
	Enabled                      bool            `json:"enabled"`
	Status                       string          `json:"status"`
	StatusReason                 string          `json:"status_reason"`
	Certificate                  *int64          `json:"certificate"`
	AccountServiceBaseURL        string          `json:"account_service_base_url"`
	LecaServiceBaseURL           string          `json:"leca_service_base_url"`
	TNCBaseURL                   string          `json:"tnc_base_url"`
	HeartbeatURL                 string          `json:"heartbeat_url"`
	RegistrationDetails          json.RawMessage `json:"registration_details"`
	Tier                         *string         `json:"tier"`                            // 26.0 only; absent (nil) on 25.10
	LastHeartbeatFailureDatetime *string         `json:"last_heartbeat_failure_datetime"` // 26.0 only; absent (nil) on 25.10
	IPs                          *[]string       `json:"ips"`                             // 25.10 only; absent (nil) on 26.0
	Interfaces                   *[]string       `json:"interfaces"`                      // 25.10 only; absent (nil) on 26.0
	InterfacesIPs                *[]string       `json:"interfaces_ips"`                  // 25.10 only; absent (nil) on 26.0
	UseAllInterfaces             *bool           `json:"use_all_interfaces"`              // 25.10 only; absent (nil) on 26.0
}

// TnConnectConfigModel is the Terraform state model for
// truenas_tn_connect_config.
//
// "enabled" is the ONLY writable field (Optional+Computed) — see
// updatePayload's doc comment for why. Every other field is Computed-only,
// sourced entirely from tn_connect.config/tn_connect.update's response and
// never sent back to the API.
type TnConnectConfigModel struct {
	ID                           types.String `tfsdk:"id"`
	Enabled                      types.Bool   `tfsdk:"enabled"`
	Status                       types.String `tfsdk:"status"`
	StatusReason                 types.String `tfsdk:"status_reason"`
	Certificate                  types.Int64  `tfsdk:"certificate"`
	AccountServiceBaseURL        types.String `tfsdk:"account_service_base_url"`
	LecaServiceBaseURL           types.String `tfsdk:"leca_service_base_url"`
	TNCBaseURL                   types.String `tfsdk:"tnc_base_url"`
	HeartbeatURL                 types.String `tfsdk:"heartbeat_url"`
	RegistrationDetails          types.String `tfsdk:"registration_details"` // JSON-encoded
	Tier                         types.String `tfsdk:"tier"`
	LastHeartbeatFailureDatetime types.String `tfsdk:"last_heartbeat_failure_datetime"`
	IPs                          types.List   `tfsdk:"ips"`            // List[String]
	Interfaces                   types.List   `tfsdk:"interfaces"`     // List[String]
	InterfacesIPs                types.List   `tfsdk:"interfaces_ips"` // List[String]
	UseAllInterfaces             types.Bool   `tfsdk:"use_all_interfaces"`
}

// TnConnectConfigDataSourceModel is the read-only model for the
// truenas_tn_connect_config datasource. Same shape as TnConnectConfigModel
// minus the writable distinction (every field is Computed in the datasource
// schema).
type TnConnectConfigDataSourceModel struct {
	ID                           types.String `tfsdk:"id"`
	Enabled                      types.Bool   `tfsdk:"enabled"`
	Status                       types.String `tfsdk:"status"`
	StatusReason                 types.String `tfsdk:"status_reason"`
	Certificate                  types.Int64  `tfsdk:"certificate"`
	AccountServiceBaseURL        types.String `tfsdk:"account_service_base_url"`
	LecaServiceBaseURL           types.String `tfsdk:"leca_service_base_url"`
	TNCBaseURL                   types.String `tfsdk:"tnc_base_url"`
	HeartbeatURL                 types.String `tfsdk:"heartbeat_url"`
	RegistrationDetails          types.String `tfsdk:"registration_details"`
	Tier                         types.String `tfsdk:"tier"`
	LastHeartbeatFailureDatetime types.String `tfsdk:"last_heartbeat_failure_datetime"`
	IPs                          types.List   `tfsdk:"ips"`
	Interfaces                   types.List   `tfsdk:"interfaces"`
	InterfacesIPs                types.List   `tfsdk:"interfaces_ips"`
	UseAllInterfaces             types.Bool   `tfsdk:"use_all_interfaces"`
}

// nonNilStrings returns s unchanged, or an empty (non-nil) slice, so
// types.ListValueFrom always produces a known empty list rather than a null
// one when a present-but-empty JSON array decodes to a nil Go slice.
// Mirrors webshare_config's helper of the same name.
func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// stringListOrNull maps a *[]string field to a Terraform list: nil (the
// JSON key was entirely absent from this release's response — see
// tnConnectConfigAPI's doc comment) becomes a null list ("not applicable on
// this TrueNAS release"), distinguished from a present pointer to an empty
// slice, which becomes a known empty list ("applicable, currently empty").
func stringListOrNull(ctx context.Context, s *[]string) (types.List, diag.Diagnostics) {
	if s == nil {
		return types.ListNull(types.StringType), nil
	}
	return types.ListValueFrom(ctx, types.StringType, nonNilStrings(*s))
}

// registrationDetailsString renders the raw registration_details JSON
// object as a compact string, defaulting to "{}" if the API omitted it
// entirely (never observed live, but guards json.RawMessage's zero value).
func registrationDetailsString(raw json.RawMessage) types.String {
	if len(raw) == 0 {
		return types.StringValue("{}")
	}
	return types.StringValue(string(raw))
}

// responseToModel maps a tnConnectConfigAPI response onto a
// TnConnectConfigModel.
func responseToModel(ctx context.Context, api *tnConnectConfigAPI, m *TnConnectConfigModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(tnConnectConfigResourceID)
	m.Enabled = types.BoolValue(api.Enabled)
	m.Status = types.StringValue(api.Status)
	m.StatusReason = types.StringValue(api.StatusReason)
	m.Certificate = types.Int64PointerValue(api.Certificate)
	m.AccountServiceBaseURL = types.StringValue(api.AccountServiceBaseURL)
	m.LecaServiceBaseURL = types.StringValue(api.LecaServiceBaseURL)
	m.TNCBaseURL = types.StringValue(api.TNCBaseURL)
	m.HeartbeatURL = types.StringValue(api.HeartbeatURL)
	m.RegistrationDetails = registrationDetailsString(api.RegistrationDetails)
	m.Tier = types.StringPointerValue(api.Tier)
	m.LastHeartbeatFailureDatetime = types.StringPointerValue(api.LastHeartbeatFailureDatetime)
	m.UseAllInterfaces = types.BoolPointerValue(api.UseAllInterfaces)

	ips, d := stringListOrNull(ctx, api.IPs)
	diags.Append(d...)
	m.IPs = ips

	interfaces, d := stringListOrNull(ctx, api.Interfaces)
	diags.Append(d...)
	m.Interfaces = interfaces

	interfacesIPs, d := stringListOrNull(ctx, api.InterfacesIPs)
	diags.Append(d...)
	m.InterfacesIPs = interfacesIPs

	return diags
}

// responseToDataSourceModel maps a tnConnectConfigAPI response onto a
// TnConnectConfigDataSourceModel.
func responseToDataSourceModel(ctx context.Context, api *tnConnectConfigAPI, m *TnConnectConfigDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(tnConnectConfigResourceID)
	m.Enabled = types.BoolValue(api.Enabled)
	m.Status = types.StringValue(api.Status)
	m.StatusReason = types.StringValue(api.StatusReason)
	m.Certificate = types.Int64PointerValue(api.Certificate)
	m.AccountServiceBaseURL = types.StringValue(api.AccountServiceBaseURL)
	m.LecaServiceBaseURL = types.StringValue(api.LecaServiceBaseURL)
	m.TNCBaseURL = types.StringValue(api.TNCBaseURL)
	m.HeartbeatURL = types.StringValue(api.HeartbeatURL)
	m.RegistrationDetails = registrationDetailsString(api.RegistrationDetails)
	m.Tier = types.StringPointerValue(api.Tier)
	m.LastHeartbeatFailureDatetime = types.StringPointerValue(api.LastHeartbeatFailureDatetime)
	m.UseAllInterfaces = types.BoolPointerValue(api.UseAllInterfaces)

	ips, d := stringListOrNull(ctx, api.IPs)
	diags.Append(d...)
	m.IPs = ips

	interfaces, d := stringListOrNull(ctx, api.Interfaces)
	diags.Append(d...)
	m.Interfaces = interfaces

	interfacesIPs, d := stringListOrNull(ctx, api.InterfacesIPs)
	diags.Append(d...)
	m.InterfacesIPs = interfacesIPs

	return diags
}

// updatePayload builds the map[string]any payload for tn_connect.update.
//
// SAFETY-CRITICAL, decisively probe-driven design: "enabled" is the ONLY
// key this function EVER includes, and only when the user explicitly
// configured it (see the CALLERS MUST contract below) — it is NEVER sent as
// true by any code path exercised by this provider's own tests (enforced by
// TestUpdatePayload_NeverSendsEnabledTrue below and by the acceptance test's
// own template never containing "enabled = true").
//
// No other field is ever writable through this resource, for two combined,
// probe-confirmed reasons:
//
//  1. On TrueNAS 26.0 (this resource's primary/production target),
//     tn_connect.update's own "accepts" schema (core.get_methods, probed
//     live against 192.168.1.68) exposes EXACTLY ONE property: "enabled".
//     There is no cosmetic field to safely toggle there at all — the
//     decisive probe question ("can a cosmetic field be updated with zero
//     side effects while enabled=false?") has a definitive "no such field
//     exists" answer on that release. Separately, that box's live
//     tn_connect.config already reports enabled=true (a real, already-
//     enrolled TrueNAS Connect account, FOUNDATION tier) — a second,
//     independent reason no live update probe was attempted there: even a
//     no-op-payload update call carries unacceptable risk against a
//     production enrollment, and the schema finding alone is decisive.
//  2. On TrueNAS 25.10, tn_connect.update's accepts schema is richer (also
//     exposes "ips", "interfaces", "use_all_interfaces"), and a supplementary
//     live probe against the disposable 25.10 test box (192.168.1.249,
//     enabled=false) confirmed sending {"ips": ["127.0.0.1"]} alone leaves
//     "enabled" at false, untouched. This provider deliberately does NOT
//     expose those fields as writable anyway, to keep one stable schema
//     across releases matching the narrower (and production-targeted) 26.0
//     capability rather than branching resource behavior by release.
//
// CALLERS MUST invoke this on a model populated from req.Config
// (req.Config.Get), never from req.Plan: "enabled" is Optional+Computed
// with a UseStateForUnknown plan modifier, so on Update the *plan* value is
// NOT null for a user who left it unset in HCL — the modifier echoes the
// prior state's value into the plan. Sourcing from req.Plan would therefore
// resend "enabled" on every Update even when the user never configured it,
// which for THIS field specifically would mean silently re-asserting
// whatever enrollment state happens to be in Terraform state — including,
// worst case, re-sending true. req.Config stays null for anything the user
// did not set in HCL regardless of plan modifiers, so it is the only
// correct and safe source here. Mirrors the twofactor_auth/webshare_config
// precedent.
func (m *TnConnectConfigModel) updatePayload() map[string]any {
	p := map[string]any{}
	if !m.Enabled.IsNull() && !m.Enabled.IsUnknown() {
		p["enabled"] = m.Enabled.ValueBool()
	}
	return p
}

// needsUpdateCall reports whether payload (as built by updatePayload) has
// anything in it worth sending to tn_connect.update at all. "enabled" is
// the only field updatePayload ever includes, and only when the user
// explicitly configured it (see updatePayload's doc comment) — so a
// practitioner who never sets "enabled" in HCL produces an empty payload.
// Calling tn_connect.update({}) in that case would be an entirely
// unprobed, unnecessary API call for a resource whose SAFETY-CRITICAL
// update semantics are otherwise fully probe-driven (see updatePayload's
// doc comment): Create/Update must degrade to a plain read (fetchConfig
// only) instead. Pure function of the already-built payload map, so this
// decision is independently unit-testable without a live client.
func needsUpdateCall(payload map[string]any) bool {
	return len(payload) > 0
}

// deleteWarningDiagnostics builds the warning diagnostic emitted by Delete.
// Delete makes NO client calls: tn_connect.config governs whether this
// system is enrolled with the TrueNAS Connect cloud service, so removing
// this resource from Terraform state must NEVER disable (or otherwise
// change) an active enrollment. Mirrors the webshare_config/twofactor_auth
// precedent exactly.
func deleteWarningDiagnostics() diag.Diagnostics {
	var diags diag.Diagnostics
	diags.AddWarning(
		"TrueNAS Connect configuration left in place",
		"TrueNAS Connect configuration left in place; removed from Terraform state only",
	)
	return diags
}

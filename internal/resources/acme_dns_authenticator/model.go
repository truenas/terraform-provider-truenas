// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acme_dns_authenticator

import (
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// AcmeDnsAuthenticatorModel is the Terraform state model for
// truenas_acme_dns_authenticator.
type AcmeDnsAuthenticatorModel struct {
	ID         types.Int64  `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	Attributes types.String `tfsdk:"attributes"` // JSON doc with "authenticator" key
}

// AcmeDnsAuthenticatorDataSourceModel is the read-only lookup model for the
// truenas_acme_dns_authenticator datasource.
type AcmeDnsAuthenticatorDataSourceModel struct {
	ID         types.Int64  `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	Attributes types.String `tfsdk:"attributes"`
}

// acmeDnsAuthenticatorAPI is the JSON wire format for a TrueNAS ACME DNS
// authenticator object, as returned by acme.dns.authenticator.create,
// .update, .get_instance, and .query.
//
// Modeled as a free-form JSON-string "attributes" attribute — the same
// pattern truenas_alert_service uses, not the single-typed-nested-block
// pattern truenas_reporting_exporter uses — because
// acme.dns.authenticator.authenticator_schemas returns FIVE genuinely
// different discriminated variants (probed live against TrueNAS
// 25.10, identical on 26.0 aside from a cosmetic pydantic "enum"-vs-"const"
// schema detail): cloudflare (cloudflare_email/api_key/api_token, all
// individually optional), digitalocean (digitalocean_token, required),
// OVH (application_key/application_secret/consumer_key/endpoint, all
// required), route53 (access_key_id/secret_access_key, required), and
// shell (script required; user/timeout/delay optional). Five variants with
// no field overlap beyond the "authenticator" discriminator itself is well
// past reporting_exporter's single-variant threshold for hoisting fields
// into a typed block — see reporting_exporter/model.go's own comment on
// this tradeoff.
//
// Credentials are NOT masked on read-back: a live throwaway create with a
// dummy cloudflare api_token showed create/get_instance/query all
// returning the token in cleartext, so "attributes" is marked Sensitive
// (kept out of plan/apply output and logs) like alert_service's
// "attributes", not WriteOnly.
//
// Also probed live: acme.dns.authenticator.create does NOT validate
// credentials against the actual DNS provider — a syntactically valid but
// fake cloudflare api_token was accepted without any outbound network call
// failing the create. The "shell" variant is the one exception: it does
// perform a local filesystem check ("script" must be an existing file
// under a pool mount point), which is why this provider's acceptance test
// uses the cloudflare variant with dummy credentials rather than shell.
type acmeDnsAuthenticatorAPI struct {
	ID         int64          `json:"id"`
	Name       string         `json:"name"`
	Attributes map[string]any `json:"attributes"`
}

// attributesMap parses the attributes JSON string and validates that it
// contains an "authenticator" key (the discriminator
// acme.dns.authenticator.authenticator_schemas uses to select a variant).
func (m *AcmeDnsAuthenticatorModel) attributesMap() (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	var attrs map[string]any
	if err := json.Unmarshal([]byte(m.Attributes.ValueString()), &attrs); err != nil {
		diags.AddError("Invalid attributes JSON", err.Error())
		return nil, diags
	}
	if _, ok := attrs["authenticator"]; !ok {
		diags.AddError("Invalid attributes",
			"attributes JSON must contain an \"authenticator\" key (one of: cloudflare, digitalocean, OVH, route53, shell)")
		return nil, diags
	}
	return attrs, diags
}

// attributesDrifted reports whether any key present in stateAttrs has a
// different value in apiAttrs. API-added keys (e.g. optional fields the
// server fills with an explicit null default) are ignored, not drift. This
// mirrors alert_service/model.go's attributesDrifted helper.
func attributesDrifted(stateAttrs, apiAttrs map[string]any) bool {
	for k, v := range stateAttrs {
		av, ok := apiAttrs[k]
		if !ok {
			return true
		}
		sb, _ := json.Marshal(v)
		ab, _ := json.Marshal(av)
		if string(sb) != string(ab) {
			return true
		}
	}
	return false
}

// responseToModel maps acmeDnsAuthenticatorAPI onto AcmeDnsAuthenticatorModel.
// ID and Name are always taken from the API; Attributes is left untouched
// by this function so callers can implement write-what-you-said /
// drift-aware semantics (see resource.go's Read).
func responseToModel(api *acmeDnsAuthenticatorAPI, m *AcmeDnsAuthenticatorModel) {
	m.ID = types.Int64Value(api.ID)
	m.Name = types.StringValue(api.Name)
}

// apiAttributesJSON marshals api.Attributes to a canonical JSON string.
func apiAttributesJSON(api *acmeDnsAuthenticatorAPI) (string, diag.Diagnostics) {
	var diags diag.Diagnostics
	b, err := json.Marshal(api.Attributes)
	if err != nil {
		diags.AddError("Failed to marshal attributes", err.Error())
		return "", diags
	}
	return string(b), diags
}

// createPayload builds the acme.dns.authenticator.create argument.
func (m *AcmeDnsAuthenticatorModel) createPayload() (map[string]any, diag.Diagnostics) {
	attrs, diags := m.attributesMap()
	if diags.HasError() {
		return nil, diags
	}
	return map[string]any{
		"name":       m.Name.ValueString(),
		"attributes": attrs,
	}, diags
}

// updatePayload builds the second arg of acme.dns.authenticator.update.
// Probed live: update accepts the same {name, attributes} shape as create
// (a full replace, not a partial patch).
func (m *AcmeDnsAuthenticatorModel) updatePayload() (map[string]any, diag.Diagnostics) {
	return m.createPayload()
}

// responseToDataSourceModel maps acmeDnsAuthenticatorAPI into
// AcmeDnsAuthenticatorDataSourceModel, marshaling Attributes to a JSON
// string.
func responseToDataSourceModel(api *acmeDnsAuthenticatorAPI, m *AcmeDnsAuthenticatorDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.Int64Value(api.ID)
	m.Name = types.StringValue(api.Name)
	b, err := json.Marshal(api.Attributes)
	if err != nil {
		diags.AddError("Failed to marshal attributes", err.Error())
		return diags
	}
	m.Attributes = types.StringValue(string(b))
	return diags
}

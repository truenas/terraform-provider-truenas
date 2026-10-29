// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acme_dns_authenticator

import (
	"bytes"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/normalize"
	"github.com/truenas/terraform-provider-truenas/internal/writeonly"
)

// AcmeDnsAuthenticatorModel is the Terraform state model for
// truenas_acme_dns_authenticator.
type AcmeDnsAuthenticatorModel struct {
	ID         types.Int64          `tfsdk:"id"`
	Name       types.String         `tfsdk:"name"`
	Attributes normalize.JSONSubset `tfsdk:"attributes"` // JSON doc with "authenticator" key
	// AttributesSecretsWO is a write-only JSON object (e.g. the DNS provider API
	// token) merged over attributes when sending, never stored in state;
	// AttributesSecretsWOVersion triggers re-send. (#37 secrets-out-of-state)
	AttributesSecretsWO        types.String `tfsdk:"attributes_secrets_wo"`
	AttributesSecretsWOVersion types.Int64  `tfsdk:"attributes_secrets_wo_version"`
}

// usesWriteOnlyAttributes reports whether the write-only secrets overlay is in
// use, in which case attributes is reconciled on read by projection so secret
// keys are never absorbed into state.
func (m *AcmeDnsAuthenticatorModel) usesWriteOnlyAttributes() bool {
	return !m.AttributesSecretsWOVersion.IsNull() && !m.AttributesSecretsWOVersion.IsUnknown()
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
	ID         int64         `json:"id"`
	Name       string        `json:"name"`
	Attributes apiAttributes `json:"attributes"`
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

// redactedValue is what the middleware returns in place of a secret to a
// session without the write role (a READONLY_ADMIN API key).
const redactedValue = "********"

// apiAttributes is the "attributes" object. A read-only session gets the
// whole object as the string "********" (B-031); UnmarshalJSON flags that as
// Redacted instead of failing the decode.
type apiAttributes struct {
	M        map[string]any
	Redacted bool
}

func (a *apiAttributes) UnmarshalJSON(b []byte) error {
	if t := bytes.TrimSpace(b); len(t) > 0 && t[0] == '"' {
		*a = apiAttributes{Redacted: true}
		return nil
	}
	a.Redacted = false
	return json.Unmarshal(b, &a.M)
}

func (a apiAttributes) MarshalJSON() ([]byte, error) {
	if a.Redacted {
		return json.Marshal(redactedValue)
	}
	return json.Marshal(a.M)
}

// unmaskRedacted returns a copy of api in which every top-level value that the
// server redacted to "********" is replaced by the prior (state) value of the
// same key, or dropped when prior has no such key.
func unmaskRedacted(api map[string]any, priorJSON string) map[string]any {
	var prior map[string]any
	_ = json.Unmarshal([]byte(priorJSON), &prior)
	out := make(map[string]any, len(api))
	for k, v := range api {
		if s, ok := v.(string); ok && s == redactedValue {
			if pv, ok := prior[k]; ok {
				out[k] = pv
			}
			continue
		}
		out[k] = v
	}
	return out
}

// readAttributes is the attributes value Read stores: the server document, or,
// when the write-only overlay is in use, the server document projected onto
// the keys in prior so secret keys stay out of state. normalize.JSONSubset
// keeps prior's spelling when every configured value matches (heredoc
// spelling, server-added null keys: B-061, B-120). A wholly redacted object
// (read-only session) keeps prior; there is nothing to compare it with.
func readAttributes(api apiAttributes, prior string, writeOnly bool) (normalize.JSONSubset, diag.Diagnostics) {
	var diags diag.Diagnostics
	if api.Redacted {
		return normalize.NewJSONSubsetValue(prior), diags
	}
	live := unmaskRedacted(api.M, prior)
	if writeOnly && prior != "" {
		return normalize.NewJSONSubsetValue(writeonly.ProjectOntoKeys(prior, live)), diags
	}
	b, err := json.Marshal(live)
	if err != nil {
		diags.AddError("Failed to marshal attributes", err.Error())
		return normalize.NewJSONSubsetNull(), diags
	}
	return normalize.NewJSONSubsetValue(string(b)), diags
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
	// A read-only session sees the secrets (or the whole object) as
	// "********"; report them as null, not as data (B-031).
	if api.Attributes.Redacted {
		m.Attributes = types.StringNull()
		return diags
	}
	attrs := make(map[string]any, len(api.Attributes.M))
	for k, v := range api.Attributes.M {
		if s, ok := v.(string); ok && s == redactedValue {
			v = nil
		}
		attrs[k] = v
	}
	b, err := json.Marshal(attrs)
	if err != nil {
		diags.AddError("Failed to marshal attributes", err.Error())
		return diags
	}
	m.Attributes = types.StringValue(string(b))
	return diags
}

// storedSecretsOverlay returns the write-only overlay with each key's value
// replaced by the value TrueNAS has stored for it, when it has one (not null
// and not the read-only redaction mask). acme.dns.authenticator.update replaces `attributes` as a whole, so
// every update must carry the secret keys; when attributes_secrets_wo_version did not change, the
// provider sends the stored secrets rather than the configured ones, so a
// secret changed in config without a version bump is not applied and the
// stored one is kept. A key with no stored value is sent from config. (B-141)
func storedSecretsOverlay(overlayJSON string, stored map[string]any) string {
	var overlay map[string]any
	if json.Unmarshal([]byte(overlayJSON), &overlay) != nil {
		return overlayJSON // injection reports the parse error
	}
	for k := range overlay {
		if v, ok := stored[k]; ok && v != nil && v != "********" {
			overlay[k] = v
		}
	}
	b, err := json.Marshal(overlay)
	if err != nil {
		return overlayJSON
	}
	return string(b)
}

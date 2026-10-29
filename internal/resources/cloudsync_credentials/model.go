// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cloudsync_credentials

import (
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/normalize"
	"github.com/truenas/terraform-provider-truenas/internal/writeonly"
)

// CredentialsModel is the Terraform state model for
// truenas_cloudsync_credentials.
//
// The wire/API field is named "provider", but "provider" is a reserved word
// in Terraform resource configuration, so the Terraform attribute is named
// provider_config.
type CredentialsModel struct {
	ID       types.Int64  `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	Provider types.String `tfsdk:"provider_config"` // JSON doc with "type" key
	// ProviderSecretsWO is a write-only JSON object (the provider's secret keys,
	// e.g. an S3 access key/secret) merged over provider_config when sending,
	// never stored in state; ProviderSecretsWOVersion triggers re-send. (#36)
	ProviderSecretsWO        types.String `tfsdk:"provider_secrets_wo"`
	ProviderSecretsWOVersion types.Int64  `tfsdk:"provider_secrets_wo_version"`
}

// usesWriteOnlyProvider reports whether the write-only secrets overlay is in
// use, in which case provider_config is reconciled on read by projection so
// secret keys are never absorbed into state.
func (m *CredentialsModel) usesWriteOnlyProvider() bool {
	return !m.ProviderSecretsWOVersion.IsNull() && !m.ProviderSecretsWOVersion.IsUnknown()
}

// CredentialsDataSourceModel is the read-only lookup model for the
// truenas_cloudsync_credentials datasource.
type CredentialsDataSourceModel struct {
	ID       types.Int64  `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	Provider types.String `tfsdk:"provider_config"`
}

// credentialsAPI is the JSON wire format for a TrueNAS cloudsync
// credentials object on /api/current (25.10 and 26.0): "provider" is an
// object holding the "type" discriminator with the provider-specific
// settings inline — the same shape as this resource's provider_config
// attribute. (The legacy /websocket endpoint translated this into a split
// provider-string + attributes form; that endpoint is no longer used.)
type credentialsAPI struct {
	ID       int64          `json:"id"`
	Name     string         `json:"name"`
	Provider map[string]any `json:"provider"`
}

// providerMap parses the provider_config JSON string and validates that it
// contains a "type" key.
func (m *CredentialsModel) providerMap() (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	var p map[string]any
	if err := json.Unmarshal([]byte(m.Provider.ValueString()), &p); err != nil {
		diags.AddError("Invalid provider_config JSON", err.Error())
		return nil, diags
	}
	if _, ok := p["type"]; !ok {
		diags.AddError("Invalid provider_config", "provider_config JSON must contain a \"type\" key (e.g. S3, B2, GOOGLE_CLOUD_STORAGE)")
		return nil, diags
	}
	return p, diags
}

// combinedProviderMap returns the provider object from a credentialsAPI
// response — already provider_config-shaped ({"type": ..., ...settings})
// on the /api/current wire.
func combinedProviderMap(api *credentialsAPI) map[string]any {
	return api.Provider
}

// urlKeys are the provider_config keys holding a URL. The server normalizes
// them (S3/AZUREBLOB/STORJ "endpoint" gets a trailing slash and its default
// port dropped; HTTP/WEBDAV "url" likewise), so they compare with
// normalize.EqualURL (#52, B-085).
var urlKeys = normalize.KeyEqual(map[string]func(a, b string) bool{
	"url":      normalize.EqualURL,
	"endpoint": normalize.EqualURL,
})

// toTree round-trips v through JSON into the normalize decode form (numbers
// as json.Number), so both sides of a comparison have the same Go types.
func toTree(v any) (any, bool) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, false
	}
	t, err := normalize.DecodeJSON(string(b))
	return t, err == nil
}

// providerDrifted reports whether a configured (state) provider value is NOT
// contained in the server's: every configured key must be present with an
// equal value (URLs compared with normalize.EqualURL, nested objects and
// lists recursively). Server-added keys are not drift.
func providerDrifted(stateProvider, apiProvider map[string]any) bool {
	want, ok1 := toTree(stateProvider)
	got, ok2 := toTree(apiProvider)
	if !ok1 || !ok2 {
		return true
	}
	return !normalize.EqualTreeFunc(want, got, true, urlKeys)
}

// providerJSONEquivalent is providerDrifted's negation for two JSON strings
// (config and state), used at plan time.
func providerJSONEquivalent(configured, state string) bool {
	var c, s map[string]any
	if json.Unmarshal([]byte(configured), &c) != nil || json.Unmarshal([]byte(state), &s) != nil {
		return false
	}
	return !providerDrifted(c, s)
}

// redactedValue is what the middleware returns in place of a secret to a
// session without the write role (a READONLY_ADMIN API key).
const redactedValue = "********"

// unmaskRedacted returns a copy of api in which every top-level value that the
// server redacted to "********" is replaced by the prior (state) value of the
// same key, or dropped when prior has no such key.
func unmaskRedacted(api map[string]any, prior map[string]any) map[string]any {
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

// readProvider is the provider_config value Read stores. When the prior value
// is contained in the server document (server-added keys, URL spelling, any
// JSON spelling) the prior string is kept verbatim (B-085). Otherwise it is the
// server document, or, with the write-only overlay, the server document
// projected onto the prior keys so secret keys stay out of state. A
// read-only session's "********" values are replaced by the prior values.
func readProvider(live map[string]any, prior string, writeOnly bool) (string, diag.Diagnostics) {
	var diags diag.Diagnostics
	var priorMap map[string]any
	_ = json.Unmarshal([]byte(prior), &priorMap)
	live = unmaskRedacted(live, priorMap)
	if prior != "" && priorMap != nil && !providerDrifted(priorMap, live) {
		return prior, diags
	}
	if writeOnly && prior != "" {
		return writeonly.ProjectOntoKeys(prior, live), diags
	}
	b, err := json.Marshal(live)
	if err != nil {
		diags.AddError("Failed to marshal provider config", err.Error())
		return "", diags
	}
	return string(b), diags
}

// responseToModel maps credentialsAPI onto CredentialsModel. ID and Name are
// always taken from the API; Provider is left untouched by this function so
// callers can implement write-what-you-said / drift-aware semantics.
func responseToModel(api *credentialsAPI, m *CredentialsModel) {
	m.ID = types.Int64Value(api.ID)
	m.Name = types.StringValue(api.Name)
}

// apiProviderJSON reconstructs the provider_config-shaped JSON string
// ({"type": <provider>, ...attributes}) from a credentialsAPI response.
func apiProviderJSON(api *credentialsAPI) (string, diag.Diagnostics) {
	var diags diag.Diagnostics
	b, err := json.Marshal(combinedProviderMap(api))
	if err != nil {
		diags.AddError("Failed to marshal provider config", err.Error())
		return "", diags
	}
	return string(b), diags
}

// createPayload builds the cloudsync.credentials.create argument.
func (m *CredentialsModel) createPayload() (map[string]any, diag.Diagnostics) {
	p, diags := m.providerMap()
	if diags.HasError() {
		return nil, diags
	}
	return map[string]any{
		"name":     m.Name.ValueString(),
		"provider": p,
	}, diags
}

// updatePayload builds the second arg of cloudsync.credentials.update.
func (m *CredentialsModel) updatePayload() (map[string]any, diag.Diagnostics) {
	p, diags := m.providerMap()
	if diags.HasError() {
		return nil, diags
	}
	return map[string]any{
		"name":     m.Name.ValueString(),
		"provider": p,
	}, diags
}

// responseToDataSourceModel maps credentialsAPI into
// CredentialsDataSourceModel, serializing the wire provider object back
// into the provider_config JSON string.
func responseToDataSourceModel(api *credentialsAPI, m *CredentialsDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.Int64Value(api.ID)
	m.Name = types.StringValue(api.Name)
	// A read-only session sees secrets as "********"; report them as null,
	// not as data (B-035).
	masked := make(map[string]any, len(api.Provider))
	for k, v := range api.Provider {
		if s, ok := v.(string); ok && s == redactedValue {
			v = nil
		}
		masked[k] = v
	}
	providerJSON, jsonDiags := apiProviderJSON(&credentialsAPI{ID: api.ID, Name: api.Name, Provider: masked})
	diags.Append(jsonDiags...)
	if diags.HasError() {
		return diags
	}
	m.Provider = types.StringValue(providerJSON)
	return diags
}

// storedSecretsOverlay returns the write-only overlay with each key's value
// replaced by the value TrueNAS has stored for it, when it has one (not null
// and not the read-only redaction mask). cloudsync.credentials.update replaces `provider` as a whole, so
// every update must carry the secret keys; when provider_secrets_wo_version did not change, the
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

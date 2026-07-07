package cloudsync_credentials

import (
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
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
}

// CredentialsDataSourceModel is the read-only lookup model for the
// truenas_cloudsync_credentials datasource.
type CredentialsDataSourceModel struct {
	ID       types.Int64  `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	Provider types.String `tfsdk:"provider_config"`
}

// credentialsAPI is the JSON wire format for a TrueNAS cloudsync credentials
// object on SCALE 26.0. "provider" is a bare string type code (e.g. "S3",
// "STORJ_IX"); the provider-specific settings live in a separate
// "attributes" object.
type credentialsAPI struct {
	ID         int64          `json:"id"`
	Name       string         `json:"name"`
	Provider   string         `json:"provider"`
	Attributes map[string]any `json:"attributes"`
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

// splitProviderMap separates a provider_config map (as returned by
// providerMap) into the wire-format "provider" type string and the
// remaining "attributes" map.
func splitProviderMap(p map[string]any) (string, map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	typeVal, ok := p["type"].(string)
	if !ok {
		diags.AddError("Invalid provider_config", "provider_config JSON key \"type\" must be a string")
		return "", nil, diags
	}
	attributes := make(map[string]any, len(p)-1)
	for k, v := range p {
		if k == "type" {
			continue
		}
		attributes[k] = v
	}
	return typeVal, attributes, diags
}

// combinedProviderMap reconstructs a provider_config-shaped map from a
// credentialsAPI response: {"type": <provider>, ...attributes}.
func combinedProviderMap(api *credentialsAPI) map[string]any {
	combined := make(map[string]any, len(api.Attributes)+1)
	combined["type"] = api.Provider
	for k, v := range api.Attributes {
		combined[k] = v
	}
	return combined
}

// providerDrifted reports whether any key present in stateProvider has a
// different value in apiProvider. API-added keys are ignored (not drift)
// since the API may normalize/expand the provider config with defaults.
//
// This mirrors internal/resources/vm_device/model.go's attributesDrifted
// helper.
func providerDrifted(stateProvider, apiProvider map[string]any) bool {
	for k, v := range stateProvider {
		av, ok := apiProvider[k]
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
	providerType, attributes, splitDiags := splitProviderMap(p)
	diags.Append(splitDiags...)
	if diags.HasError() {
		return nil, diags
	}
	return map[string]any{
		"name":       m.Name.ValueString(),
		"provider":   providerType,
		"attributes": attributes,
	}, diags
}

// updatePayload builds the second arg of cloudsync.credentials.update.
func (m *CredentialsModel) updatePayload() (map[string]any, diag.Diagnostics) {
	p, diags := m.providerMap()
	if diags.HasError() {
		return nil, diags
	}
	providerType, attributes, splitDiags := splitProviderMap(p)
	diags.Append(splitDiags...)
	if diags.HasError() {
		return nil, diags
	}
	return map[string]any{
		"name":       m.Name.ValueString(),
		"provider":   providerType,
		"attributes": attributes,
	}, diags
}

// responseToDataSourceModel maps credentialsAPI into
// CredentialsDataSourceModel, reconstructing the provider_config JSON
// string from the split provider/attributes wire fields.
func responseToDataSourceModel(api *credentialsAPI, m *CredentialsDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.Int64Value(api.ID)
	m.Name = types.StringValue(api.Name)
	providerJSON, jsonDiags := apiProviderJSON(api)
	diags.Append(jsonDiags...)
	if diags.HasError() {
		return diags
	}
	m.Provider = types.StringValue(providerJSON)
	return diags
}

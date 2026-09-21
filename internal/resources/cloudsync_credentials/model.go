// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cloudsync_credentials

import (
	"encoding/json"
	"strings"

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
		// TrueNAS normalizes the S3 "endpoint" by appending a trailing slash
		// on read-back (e.g. "http://host:9000" -> "http://host:9000/"). A
		// slash-only difference is server normalization, not real drift — so
		// any custom-endpoint S3 credential (MinIO, SeaweedFS, Wasabi, …)
		// would otherwise never converge.
		if k == "endpoint" {
			if sv, sok := v.(string); sok {
				if avs, aok := av.(string); aok {
					if strings.TrimRight(sv, "/") != strings.TrimRight(avs, "/") {
						return true
					}
					continue
				}
			}
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
	providerJSON, jsonDiags := apiProviderJSON(api)
	diags.Append(jsonDiags...)
	if diags.HasError() {
		return diags
	}
	m.Provider = types.StringValue(providerJSON)
	return diags
}

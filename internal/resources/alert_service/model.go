// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package alert_service

import (
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// AlertServiceModel is the Terraform state model for truenas_alert_service.
type AlertServiceModel struct {
	ID         types.Int64  `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	Level      types.String `tfsdk:"level"` // INFO, NOTICE, WARNING, ERROR, CRITICAL, ALERT, EMERGENCY
	Enabled    types.Bool   `tfsdk:"enabled"`
	Attributes types.String `tfsdk:"attributes"` // JSON doc with "type" key
}

// AlertServiceDataSourceModel is the read-only lookup model for the
// truenas_alert_service datasource.
type AlertServiceDataSourceModel struct {
	ID         types.Int64  `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	Level      types.String `tfsdk:"level"`
	Enabled    types.Bool   `tfsdk:"enabled"`
	Attributes types.String `tfsdk:"attributes"`
}

// alertServiceAPI is the JSON wire format for a TrueNAS alert service object.
type alertServiceAPI struct {
	ID         int64          `json:"id"`
	Name       string         `json:"name"`
	Level      string         `json:"level"`
	Enabled    bool           `json:"enabled"`
	Attributes map[string]any `json:"attributes"`
}

// attributesMap parses the attributes JSON string and validates that it
// contains a "type" key.
func (m *AlertServiceModel) attributesMap() (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	var attrs map[string]any
	if err := json.Unmarshal([]byte(m.Attributes.ValueString()), &attrs); err != nil {
		diags.AddError("Invalid attributes JSON", err.Error())
		return nil, diags
	}
	if _, ok := attrs["type"]; !ok {
		diags.AddError("Invalid attributes", "attributes JSON must contain a \"type\" key (e.g. Mail, SNMPTrap, Slack, PagerDuty)")
		return nil, diags
	}
	return attrs, diags
}

// attributesDrifted reports whether any key present in stateAttrs has a
// different value in apiAttrs. API-added keys are ignored (not drift) since
// the API may normalize/expand the attributes with defaults.
//
// This mirrors internal/resources/cloudsync_credentials/model.go's
// providerDrifted helper.
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

// responseToModel maps alertServiceAPI onto AlertServiceModel. ID, Name,
// Level, and Enabled are always taken from the API; Attributes is left
// untouched by this function so callers can implement write-what-you-said /
// drift-aware semantics.
func responseToModel(api *alertServiceAPI, m *AlertServiceModel) {
	m.ID = types.Int64Value(api.ID)
	m.Name = types.StringValue(api.Name)
	m.Level = types.StringValue(api.Level)
	m.Enabled = types.BoolValue(api.Enabled)
}

// apiAttributesJSON marshals api.Attributes to a canonical JSON string.
func apiAttributesJSON(api *alertServiceAPI) (string, diag.Diagnostics) {
	var diags diag.Diagnostics
	b, err := json.Marshal(api.Attributes)
	if err != nil {
		diags.AddError("Failed to marshal attributes", err.Error())
		return "", diags
	}
	return string(b), diags
}

// createPayload builds the alertservice.create argument.
func (m *AlertServiceModel) createPayload() (map[string]any, diag.Diagnostics) {
	return m.apiPayload()
}

// updatePayload builds the second arg of alertservice.update.
func (m *AlertServiceModel) updatePayload() (map[string]any, diag.Diagnostics) {
	return m.apiPayload()
}

// apiPayload builds the {name, level, attributes} payload common to create
// and update; enabled is only included when it is known and non-null.
func (m *AlertServiceModel) apiPayload() (map[string]any, diag.Diagnostics) {
	attrs, diags := m.attributesMap()
	if diags.HasError() {
		return nil, diags
	}

	payload := map[string]any{
		"name":       m.Name.ValueString(),
		"level":      m.Level.ValueString(),
		"attributes": attrs,
	}

	if !m.Enabled.IsNull() && !m.Enabled.IsUnknown() {
		payload["enabled"] = m.Enabled.ValueBool()
	}

	return payload, diags
}

// responseToDataSourceModel maps alertServiceAPI into
// AlertServiceDataSourceModel, marshaling Attributes to a JSON string.
func responseToDataSourceModel(api *alertServiceAPI, m *AlertServiceDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.Int64Value(api.ID)
	m.Name = types.StringValue(api.Name)
	m.Level = types.StringValue(api.Level)
	m.Enabled = types.BoolValue(api.Enabled)
	b, err := json.Marshal(api.Attributes)
	if err != nil {
		diags.AddError("Failed to marshal attributes", err.Error())
		return diags
	}
	m.Attributes = types.StringValue(string(b))
	return diags
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package alert_service

import (
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/normalize"
	"github.com/truenas/terraform-provider-truenas/internal/writeonly"
)

// AlertServiceModel is the Terraform state model for truenas_alert_service.
type AlertServiceModel struct {
	ID         types.Int64          `tfsdk:"id"`
	Name       types.String         `tfsdk:"name"`
	Level      types.String         `tfsdk:"level"` // INFO, NOTICE, WARNING, ERROR, CRITICAL, ALERT, EMERGENCY
	Enabled    types.Bool           `tfsdk:"enabled"`
	Attributes normalize.JSONSubset `tfsdk:"attributes"` // JSON doc with "type" key
	// AttributesSecretsWO is a write-only JSON object (e.g. Slack/PagerDuty/AWS
	// credentials) merged over attributes when sending, never stored in state;
	// AttributesSecretsWOVersion triggers re-send. (#36/#37 secrets-out-of-state)
	AttributesSecretsWO        types.String `tfsdk:"attributes_secrets_wo"`
	AttributesSecretsWOVersion types.Int64  `tfsdk:"attributes_secrets_wo_version"`
}

// usesWriteOnlyAttributes reports whether the write-only secrets overlay is in
// use, in which case attributes is reconciled on read by projection.
func (m *AlertServiceModel) usesWriteOnlyAttributes() bool {
	return !m.AttributesSecretsWOVersion.IsNull() && !m.AttributesSecretsWOVersion.IsUnknown()
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

// redactedValue is what the middleware returns in place of a secret
// attribute (e.g. SNMP v3_authkey/v3_privkey) to a session without the
// write role (a READONLY_ADMIN API key).
const redactedValue = "********"

// unmaskRedacted returns a copy of api in which every top-level value that the
// server redacted to "********" is replaced by the prior (state) value of the
// same key, or dropped when prior has no such key. A read-only session then
// neither reads the mask into state nor plans to overwrite the real secret
// with it (B-034).
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

// readAttributes is the attributes value Read stores: the server document
// (redacted secrets unmasked from prior), or, when the write-only overlay is
// in use, the server document projected onto the keys in prior so secret keys
// stay out of state. normalize.JSONSubset keeps prior's spelling when every
// configured value matches (heredoc/jsonencode spelling, server-added keys:
// B-061, B-120).
func readAttributes(api map[string]any, prior string, writeOnly bool) (normalize.JSONSubset, diag.Diagnostics) {
	var diags diag.Diagnostics
	live := unmaskRedacted(api, prior)
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
	// A read-only session sees secrets as "********"; report them as null,
	// not as data (B-034).
	attrs := make(map[string]any, len(api.Attributes))
	for k, v := range api.Attributes {
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
// and not the read-only redaction mask). alertservice.update replaces `attributes` as a whole, so
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

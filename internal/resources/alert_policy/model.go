// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package alert_policy

import (
	"encoding/json"
	"reflect"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// alertPolicyResourceID is the fixed Terraform ID for this singleton
// resource: there is exactly one alert policy per TrueNAS system, and it is
// never created or deleted on TrueNAS itself.
const alertPolicyResourceID = "alert_policy"

// AlertPolicyModel is the Terraform state model for truenas_alert_policy.
type AlertPolicyModel struct {
	ID      types.String `tfsdk:"id"`      // fixed: "alert_policy"
	Classes types.String `tfsdk:"classes"` // JSON: {"ClassName": {"level": "...", "policy": "..."}}
}

// AlertPolicyDataSourceModel is the read-only model for the
// truenas_alert_policy datasource.
type AlertPolicyDataSourceModel struct {
	ID      types.String `tfsdk:"id"`
	Classes types.String `tfsdk:"classes"`
}

// alertClassesAPI mirrors the JSON object returned by alertclasses.config
// and alertclasses.update.
type alertClassesAPI struct {
	ID      int64          `json:"id"`
	Classes map[string]any `json:"classes"`
}

// classesMap parses the classes JSON string into a map. Returns a diagnostic
// error when the string is not valid JSON.
func (m *AlertPolicyModel) classesMap() (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	var classes map[string]any
	if err := json.Unmarshal([]byte(m.Classes.ValueString()), &classes); err != nil {
		diags.AddError("Invalid classes JSON", err.Error())
		return nil, diags
	}
	return classes, diags
}

// updatePayload builds the alertclasses.update argument: {"classes": {...}}.
func (m *AlertPolicyModel) updatePayload() (map[string]any, diag.Diagnostics) {
	classes, diags := m.classesMap()
	if diags.HasError() {
		return nil, diags
	}
	return map[string]any{"classes": classes}, diags
}

// deletePayload builds the reset-to-empty payload sent on Delete: this
// resource owns the whole alertclasses.config object, so removing it from
// Terraform state resets TrueNAS back to its class-level defaults.
func deletePayload() map[string]any {
	return map[string]any{"classes": map[string]any{}}
}

// apiClassesJSON marshals api.Classes to a JSON string. Go's json.Marshal
// emits map keys in sorted order, so this string is a canonical
// representation of the classes map suitable for storing in state.
func apiClassesJSON(api *alertClassesAPI) (string, diag.Diagnostics) {
	var diags diag.Diagnostics
	b, err := json.Marshal(api.Classes)
	if err != nil {
		diags.AddError("Failed to marshal classes", err.Error())
		return "", diags
	}
	return string(b), diags
}

// classesEqual reports whether stateClasses and apiClasses are deeply equal.
//
// Unlike the subset-drift helpers used by other resources (e.g.
// cloudsync_credentials.providerDrifted / alert_service.attributesDrifted),
// truenas_alert_policy owns the WHOLE classes object: it is not a partial
// overlay on top of server-side defaults. So any difference at all — added,
// removed, or changed class entries — counts as drift, and reflect.DeepEqual
// over the parsed maps is sufficient and simpler than key-by-key comparison.
func classesEqual(stateClasses, apiClasses map[string]any) bool {
	return reflect.DeepEqual(stateClasses, apiClasses)
}

// applyAPIToModel sets m.ID to the fixed singleton ID and applies
// drift-aware handling of m.Classes based on api:
//
//   - If m.Classes is null/empty (e.g. right after ImportState), it is
//     populated directly from the API response.
//   - Otherwise, the existing state classes JSON is parsed and compared
//     against api.Classes via classesEqual. Equal maps leave the state
//     string untouched (even if key order in the JSON text differs);
//     unequal maps overwrite the state string with the API's JSON.
func applyAPIToModel(api *alertClassesAPI, m *AlertPolicyModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.StringValue(alertPolicyResourceID)

	if m.Classes.IsNull() || m.Classes.ValueString() == "" {
		classesJSON, d := apiClassesJSON(api)
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}
		m.Classes = types.StringValue(classesJSON)
		return diags
	}

	var stateClasses map[string]any
	if err := json.Unmarshal([]byte(m.Classes.ValueString()), &stateClasses); err != nil {
		diags.AddError("Parse state classes JSON", err.Error())
		return diags
	}

	if !classesEqual(stateClasses, api.Classes) {
		classesJSON, d := apiClassesJSON(api)
		diags.Append(d...)
		if diags.HasError() {
			return diags
		}
		m.Classes = types.StringValue(classesJSON)
	}
	return diags
}

// responseToDataSourceModel maps alertClassesAPI into
// AlertPolicyDataSourceModel, marshaling Classes to a JSON string.
func responseToDataSourceModel(api *alertClassesAPI, m *AlertPolicyDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.StringValue(alertPolicyResourceID)
	classesJSON, d := apiClassesJSON(api)
	diags.Append(d...)
	if diags.HasError() {
		return diags
	}
	m.Classes = types.StringValue(classesJSON)
	return diags
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package app

import (
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type AppModel struct {
	ID          types.String `tfsdk:"id"`          // app name (string ID)
	Name        types.String `tfsdk:"name"`        // app_name
	CatalogApp  types.String `tfsdk:"catalog_app"` // catalog app to install (e.g. "plex")
	Train       types.String `tfsdk:"train"`       // stable, community, ...
	Version     types.String `tfsdk:"version"`     // app version to install (default "latest" at create)
	Values      types.String `tfsdk:"values"`      // JSON document of app config values
	CustomApp   types.Bool   `tfsdk:"custom_app"`
	ComposeYAML types.String `tfsdk:"custom_compose_config_string"` // for custom apps
	Running     types.Bool   `tfsdk:"running"`                      // desired/actual state
	// Computed only
	State            types.String `tfsdk:"state"` // RUNNING, STOPPED, DEPLOYING...
	HumanVersion     types.String `tfsdk:"human_version"`
	UpgradeAvailable types.Bool   `tfsdk:"upgrade_available"`
}

// AppDatasourceModel is the read-only lookup model for the truenas_app
// datasource. It intentionally excludes write-only fields (values,
// custom_compose_config_string, catalog_app) and the desired-state field
// (running), since those are never echoed back by the API or don't apply to
// a read-only lookup.
type AppDatasourceModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	Train            types.String `tfsdk:"train"`
	Version          types.String `tfsdk:"version"`
	CustomApp        types.Bool   `tfsdk:"custom_app"`
	State            types.String `tfsdk:"state"`
	HumanVersion     types.String `tfsdk:"human_version"`
	UpgradeAvailable types.Bool   `tfsdk:"upgrade_available"`
}

type appAPI struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	State            string `json:"state"`
	CustomApp        bool   `json:"custom_app"`
	HumanVersion     string `json:"human_version"`
	Version          string `json:"version"`
	UpgradeAvailable bool   `json:"upgrade_available"`
	Metadata         struct {
		Train string `json:"train"`
	} `json:"metadata"`
}

// responseToModel maps appAPI into AppModel. Does NOT touch CatalogApp,
// Values, ComposeYAML (write-only-ish: API does not echo them back in query).
func responseToModel(api *appAPI, m *AppModel) {
	m.ID = types.StringValue(api.Name)
	m.Name = types.StringValue(api.Name)
	m.Train = types.StringValue(api.Metadata.Train)
	m.Version = types.StringValue(api.Version)
	m.CustomApp = types.BoolValue(api.CustomApp)
	m.State = types.StringValue(api.State)
	m.HumanVersion = types.StringValue(api.HumanVersion)
	m.UpgradeAvailable = types.BoolValue(api.UpgradeAvailable)
	m.Running = types.BoolValue(api.State == "RUNNING")
}

// responseToDatasourceModel maps appAPI into AppDatasourceModel.
func responseToDatasourceModel(api *appAPI, m *AppDatasourceModel) {
	m.ID = types.StringValue(api.Name)
	m.Name = types.StringValue(api.Name)
	m.Train = types.StringValue(api.Metadata.Train)
	m.Version = types.StringValue(api.Version)
	m.CustomApp = types.BoolValue(api.CustomApp)
	m.State = types.StringValue(api.State)
	m.HumanVersion = types.StringValue(api.HumanVersion)
	m.UpgradeAvailable = types.BoolValue(api.UpgradeAvailable)
}

// needsUpgrade reports whether plan carries a known, non-null version that
// differs from state's version, meaning Update must call app.upgrade before
// doing anything else (see AppResource.Update).
func needsUpgrade(plan, state *AppModel) bool {
	return !plan.Version.IsNull() && !plan.Version.IsUnknown() && !plan.Version.Equal(state.Version)
}

// createPayload builds the app.create argument.
func (m *AppModel) createPayload() (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	p := map[string]any{
		"app_name": m.Name.ValueString(),
	}
	if !m.CatalogApp.IsNull() && !m.CatalogApp.IsUnknown() {
		p["catalog_app"] = m.CatalogApp.ValueString()
	}
	if !m.Train.IsNull() && !m.Train.IsUnknown() {
		p["train"] = m.Train.ValueString()
	}
	if !m.Version.IsNull() && !m.Version.IsUnknown() {
		p["version"] = m.Version.ValueString()
	}
	if !m.CustomApp.IsNull() && !m.CustomApp.IsUnknown() {
		p["custom_app"] = m.CustomApp.ValueBool()
	}
	if !m.ComposeYAML.IsNull() && !m.ComposeYAML.IsUnknown() && m.ComposeYAML.ValueString() != "" {
		p["custom_compose_config_string"] = m.ComposeYAML.ValueString()
	}
	if vals, d := m.valuesMap(); d != nil {
		diags.Append(d)
	} else if vals != nil {
		p["values"] = vals
	}
	return p, diags
}

// updatePayload builds the second arg of app.update (app_name is passed separately).
func (m *AppModel) updatePayload() (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	p := map[string]any{}
	if !m.ComposeYAML.IsNull() && !m.ComposeYAML.IsUnknown() && m.ComposeYAML.ValueString() != "" {
		p["custom_compose_config_string"] = m.ComposeYAML.ValueString()
	}
	if vals, d := m.valuesMap(); d != nil {
		diags.Append(d)
	} else if vals != nil {
		p["values"] = vals
	}
	return p, diags
}

// valuesMap parses the values JSON string. Returns (nil, nil) when unset.
func (m *AppModel) valuesMap() (map[string]any, diag.Diagnostic) {
	if m.Values.IsNull() || m.Values.IsUnknown() || m.Values.ValueString() == "" {
		return nil, nil
	}
	var vals map[string]any
	if err := json.Unmarshal([]byte(m.Values.ValueString()), &vals); err != nil {
		return nil, diag.NewErrorDiagnostic("Invalid values JSON", err.Error())
	}
	return vals, nil
}

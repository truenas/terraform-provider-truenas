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
	// ComposeSecretsWO is a write-only overlay (JSON/YAML) deep-merged into
	// custom_compose_config_string when sending, never stored; on read the live
	// Compose is projected onto only the keys present in the base string, so the
	// overlay's secret keys (e.g. nested environment values) are not persisted to
	// state. Bump the version to re-send a rotated overlay. (#34 / secrets-in-state)
	ComposeSecretsWO        types.String `tfsdk:"custom_compose_config_string_wo"`
	ComposeSecretsWOVersion types.Int64  `tfsdk:"custom_compose_config_string_wo_version"`
	Running                 types.Bool   `tfsdk:"running"` // desired/actual state
	// Computed only
	State            types.String `tfsdk:"state"` // RUNNING, STOPPED, DEPLOYING...
	HumanVersion     types.String `tfsdk:"human_version"`
	UpgradeAvailable types.Bool   `tfsdk:"upgrade_available"`
}

// usesComposeSecretsWO reports whether the write-only Compose overlay is in use.
func (m *AppModel) usesComposeSecretsWO() bool {
	return !m.ComposeSecretsWOVersion.IsNull() && !m.ComposeSecretsWOVersion.IsUnknown()
}

// AppDatasourceModel is the read-only lookup model for the truenas_app
// datasource. It intentionally excludes the configuration documents (values,
// custom_compose_config_string) and the desired-state field (running).
// catalog_app is read from metadata.name, as in the resource (B-150).
type AppDatasourceModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	CatalogApp       types.String `tfsdk:"catalog_app"`
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
		Name  string `json:"name"` // catalog app the app was installed from ("custom-app" for custom apps)
		Train string `json:"train"`
	} `json:"metadata"`
}

// responseToModel maps appAPI into AppModel. Does NOT touch Values or
// ComposeYAML (reconciled separately against the configured keys).
// catalog_app is read from metadata.name for catalog apps, so import followed
// by plan does not replace the app (BUGS B-006); a custom app has no catalog
// app ("custom-app" in metadata), so its value is left as configured.
func responseToModel(api *appAPI, m *AppModel) {
	m.ID = types.StringValue(api.Name)
	m.Name = types.StringValue(api.Name)
	if !api.CustomApp && api.Metadata.Name != "" {
		m.CatalogApp = types.StringValue(api.Metadata.Name)
	}
	m.Train = types.StringValue(api.Metadata.Train)
	// "latest" asks app.create for the newest version; the server stores the
	// version it resolved. Keep the configured "latest" so the apply is
	// consistent and later refreshes do not plan an upgrade (B-096); the
	// installed version is still visible in human_version.
	if m.Version.ValueString() != versionLatest {
		m.Version = types.StringValue(api.Version)
	}
	m.CustomApp = types.BoolValue(api.CustomApp)
	m.State = types.StringValue(api.State)
	m.HumanVersion = types.StringValue(api.HumanVersion)
	m.UpgradeAvailable = types.BoolValue(api.UpgradeAvailable)
	m.Running = types.BoolValue(api.State == "RUNNING")
}

// versionLatest is the version sentinel app.create resolves to the newest
// catalog version at install time.
const versionLatest = "latest"

// responseToDatasourceModel maps appAPI into AppDatasourceModel.
func responseToDatasourceModel(api *appAPI, m *AppDatasourceModel) {
	m.ID = types.StringValue(api.Name)
	m.Name = types.StringValue(api.Name)
	m.CatalogApp = types.StringNull()
	if !api.CustomApp && api.Metadata.Name != "" {
		m.CatalogApp = types.StringValue(api.Metadata.Name)
	}
	m.Train = types.StringValue(api.Metadata.Train)
	m.Version = types.StringValue(api.Version)
	m.CustomApp = types.BoolValue(api.CustomApp)
	m.State = types.StringValue(api.State)
	m.HumanVersion = types.StringValue(api.HumanVersion)
	m.UpgradeAvailable = types.BoolValue(api.UpgradeAvailable)
}

// needsUpgrade reports whether plan carries a known, non-null version that
// differs from state's version, meaning Update must call app.upgrade before
// doing anything else (see AppResource.Update). "latest" is an install-time
// request, not an upgrade policy, so changing the configured version to
// "latest" (e.g. after import, where state holds the resolved version) never
// upgrades (B-096).
func needsUpgrade(plan, state *AppModel) bool {
	return !plan.Version.IsNull() && !plan.Version.IsUnknown() && !plan.Version.Equal(state.Version) &&
		plan.Version.ValueString() != versionLatest
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

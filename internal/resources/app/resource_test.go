// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package app

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestAppSchema_NameIsRequiresReplace verifies that "name" carries the
// RequiresReplace plan modifier, forcing resource recreation if the app
// name changes.
func TestAppSchema_NameIsRequiresReplace(t *testing.T) {
	s := resourceSchema()

	nameAttr, ok := s.Attributes["name"]
	if !ok {
		t.Fatal("schema missing 'name' attribute")
	}
	nameStr, ok := nameAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'name' attribute is %T, want schema.StringAttribute", nameAttr)
	}
	if !nameStr.IsRequired() {
		t.Error("'name' should be Required")
	}
	if len(nameStr.PlanModifiers) == 0 {
		t.Error("'name' should have plan modifiers (RequiresReplace)")
	}
}

// TestAppSchema_IDIsString verifies that "id" is a string attribute (the app
// name is used as the Terraform ID, not a numeric ID).
func TestAppSchema_IDIsString(t *testing.T) {
	s := resourceSchema()

	idAttr, ok := s.Attributes["id"]
	if !ok {
		t.Fatal("schema missing 'id' attribute")
	}
	idStr, ok := idAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'id' attribute is %T, want schema.StringAttribute", idAttr)
	}
	if !idStr.IsComputed() {
		t.Error("'id' should be Computed")
	}
	if idStr.IsRequired() || idStr.IsOptional() {
		t.Error("'id' should be Computed-only (not Required/Optional)")
	}
}

// TestAppSchema_StateHasNoUseStateForUnknown verifies that "state",
// "human_version" and "upgrade_available" are Computed WITHOUT
// UseStateForUnknown, since they legitimately change server-side between
// applies.
func TestAppSchema_StateHasNoUseStateForUnknown(t *testing.T) {
	s := resourceSchema()

	for _, name := range []string{"state", "human_version", "upgrade_available"} {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		switch a := attr.(type) {
		case schema.StringAttribute:
			if len(a.PlanModifiers) != 0 {
				t.Errorf("%q should have no plan modifiers, got %d", name, len(a.PlanModifiers))
			}
			if !a.IsComputed() {
				t.Errorf("%q should be Computed", name)
			}
		case schema.BoolAttribute:
			if len(a.PlanModifiers) != 0 {
				t.Errorf("%q should have no plan modifiers, got %d", name, len(a.PlanModifiers))
			}
			if !a.IsComputed() {
				t.Errorf("%q should be Computed", name)
			}
		default:
			t.Fatalf("%q attribute is unexpected type %T", name, attr)
		}
	}
}

// TestCreatePayload_OmitsUnsetOptionalFields verifies that createPayload
// omits catalog_app and values when they are unset (null) in the plan.
func TestCreatePayload_OmitsUnsetOptionalFields(t *testing.T) {
	m := &AppModel{
		Name:        types.StringValue("myapp"),
		CatalogApp:  types.StringNull(),
		Train:       types.StringNull(),
		Version:     types.StringNull(),
		CustomApp:   types.BoolNull(),
		ComposeYAML: types.StringNull(),
		Values:      types.StringNull(),
	}

	payload, diags := m.createPayload()
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if _, ok := payload["catalog_app"]; ok {
		t.Error("payload should not contain 'catalog_app' when unset")
	}
	if _, ok := payload["values"]; ok {
		t.Error("payload should not contain 'values' when unset")
	}
	if _, ok := payload["train"]; ok {
		t.Error("payload should not contain 'train' when unset")
	}
	if _, ok := payload["custom_compose_config_string"]; ok {
		t.Error("payload should not contain 'custom_compose_config_string' when unset")
	}
	if got, want := payload["app_name"], "myapp"; got != want {
		t.Errorf("app_name = %v, want %v", got, want)
	}
}

// TestCreatePayload_IncludesSetFields verifies that createPayload includes
// fields that are explicitly set in the plan.
func TestCreatePayload_IncludesSetFields(t *testing.T) {
	m := &AppModel{
		Name:        types.StringValue("myapp"),
		CatalogApp:  types.StringValue("plex"),
		Train:       types.StringValue("stable"),
		Version:     types.StringValue("1.2.3"),
		CustomApp:   types.BoolValue(false),
		ComposeYAML: types.StringNull(),
		Values:      types.StringValue(`{"foo":"bar"}`),
	}

	payload, diags := m.createPayload()
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if got, want := payload["catalog_app"], "plex"; got != want {
		t.Errorf("catalog_app = %v, want %v", got, want)
	}
	if got, want := payload["train"], "stable"; got != want {
		t.Errorf("train = %v, want %v", got, want)
	}
	if got, want := payload["version"], "1.2.3"; got != want {
		t.Errorf("version = %v, want %v", got, want)
	}
	vals, ok := payload["values"].(map[string]any)
	if !ok {
		t.Fatalf("values = %T, want map[string]any", payload["values"])
	}
	if got, want := vals["foo"], "bar"; got != want {
		t.Errorf("values[foo] = %v, want %v", got, want)
	}
}

// TestValuesMap_InvalidJSON verifies that invalid JSON in the "values"
// field produces an error diagnostic instead of panicking.
func TestValuesMap_InvalidJSON(t *testing.T) {
	m := &AppModel{
		Name:   types.StringValue("myapp"),
		Values: types.StringValue(`{not valid json`),
	}

	vals, d := m.valuesMap()
	if d == nil {
		t.Fatal("expected a diagnostic for invalid JSON, got nil")
	}
	if vals != nil {
		t.Errorf("expected nil values on error, got %v", vals)
	}
	if d.Severity() != diag.SeverityError {
		t.Errorf("Severity = %v, want SeverityError", d.Severity())
	}
}

// TestCreatePayload_InvalidValuesJSONProducesDiagnostic verifies that
// createPayload surfaces the values-JSON error as a diagnostic rather than
// silently dropping it.
func TestCreatePayload_InvalidValuesJSONProducesDiagnostic(t *testing.T) {
	m := &AppModel{
		Name:   types.StringValue("myapp"),
		Values: types.StringValue(`{not valid json`),
	}

	_, diags := m.createPayload()
	if !diags.HasError() {
		t.Fatal("expected createPayload to produce an error diagnostic for invalid values JSON")
	}
}

// TestResponseToModel_NeverTouchesWriteOnlyFields verifies that
// responseToModel does not modify Values, ComposeYAML, or CatalogApp, since
// the API never echoes these back (write-only pattern).
func TestResponseToModel_NeverTouchesWriteOnlyFields(t *testing.T) {
	m := &AppModel{
		Values:      types.StringValue(`{"kept":true}`),
		ComposeYAML: types.StringValue("version: '3'\n"),
		CatalogApp:  types.StringValue("plex"),
	}

	api := &appAPI{
		Name:      "myapp",
		State:     "RUNNING",
		CustomApp: false,
	}
	api.Metadata.Train = "stable"

	responseToModel(api, m)

	if m.Values.ValueString() != `{"kept":true}` {
		t.Errorf("Values was modified by responseToModel: %v", m.Values.ValueString())
	}
	if m.ComposeYAML.ValueString() != "version: '3'\n" {
		t.Errorf("ComposeYAML was modified by responseToModel: %v", m.ComposeYAML.ValueString())
	}
	if m.CatalogApp.ValueString() != "plex" {
		t.Errorf("CatalogApp was modified by responseToModel: %v", m.CatalogApp.ValueString())
	}
}

// TestResponseToModel_Running verifies that State=="RUNNING" maps to
// Running=true and any other state maps to Running=false.
func TestResponseToModel_Running(t *testing.T) {
	cases := []struct {
		state   string
		running bool
	}{
		{"RUNNING", true},
		{"STOPPED", false},
		{"DEPLOYING", false},
		{"", false},
	}

	for _, tc := range cases {
		api := &appAPI{Name: "myapp", State: tc.state}
		var m AppModel
		responseToModel(api, &m)

		if m.Running.ValueBool() != tc.running {
			t.Errorf("state=%q: Running=%v, want %v", tc.state, m.Running.ValueBool(), tc.running)
		}
	}
}

// TestResponseToModel_IDIsAppName verifies that the Terraform ID is set to
// the app name string.
func TestResponseToModel_IDIsAppName(t *testing.T) {
	api := &appAPI{Name: "plex", State: "RUNNING"}
	var m AppModel
	responseToModel(api, &m)

	if m.ID.ValueString() != "plex" {
		t.Errorf("ID = %q, want app name string \"plex\"", m.ID.ValueString())
	}
	if m.Name.ValueString() != "plex" {
		t.Errorf("Name = %q, want app name string \"plex\"", m.Name.ValueString())
	}
}

// TestNeedsUpgrade verifies that needsUpgrade correctly detects a
// plan-vs-state version change, which drives the app.upgrade call in Update.
// A regression here previously caused version changes to be silently
// dropped (the read-back would overwrite plan.Version with the old value,
// producing "Provider produced inconsistent result after apply").
func TestNeedsUpgrade(t *testing.T) {
	cases := []struct {
		name         string
		planVersion  types.String
		stateVersion types.String
		want         bool
	}{
		{"version changed", types.StringValue("1.2.4"), types.StringValue("1.2.3"), true},
		{"version unchanged", types.StringValue("1.2.3"), types.StringValue("1.2.3"), false},
		{"plan version null", types.StringNull(), types.StringValue("1.2.3"), false},
		{"plan version unknown", types.StringUnknown(), types.StringValue("1.2.3"), false},
		{"state version null, plan set", types.StringValue("1.2.3"), types.StringNull(), true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan := &AppModel{Version: tc.planVersion}
			state := &AppModel{Version: tc.stateVersion}
			if got := needsUpgrade(plan, state); got != tc.want {
				t.Errorf("needsUpgrade() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestResponseToDatasourceModel_AllFields verifies field mapping for the
// datasource model.
func TestResponseToDatasourceModel_AllFields(t *testing.T) {
	api := &appAPI{
		Name:             "plex",
		State:            "RUNNING",
		CustomApp:        false,
		HumanVersion:     "1.32.0_1.2.3",
		Version:          "1.2.3",
		UpgradeAvailable: true,
	}
	api.Metadata.Train = "stable"

	var m AppDatasourceModel
	responseToDatasourceModel(api, &m)

	if m.ID.ValueString() != "plex" {
		t.Errorf("ID = %q, want plex", m.ID.ValueString())
	}
	if m.Train.ValueString() != "stable" {
		t.Errorf("Train = %q, want stable", m.Train.ValueString())
	}
	if m.Version.ValueString() != "1.2.3" {
		t.Errorf("Version = %q, want 1.2.3", m.Version.ValueString())
	}
	if !m.UpgradeAvailable.ValueBool() {
		t.Error("UpgradeAvailable = false, want true")
	}
}

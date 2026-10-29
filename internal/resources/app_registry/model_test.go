// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package app_registry

import (
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/normalize"
)

// TestApiPayload_FullySet verifies apiPayload includes name, username,
// password (from the passed-in config password_wo value, not the model),
// description, and uri when all are set.
func TestApiPayload_FullySet(t *testing.T) {
	m := &AppRegistryModel{
		Name:        types.StringValue("tf-registry"),
		Description: types.StringValue("a test registry"),
		URI:         normalize.NewURLValue("https://registry.example.com"),
		Username:    types.StringValue("tfuser"),
		PasswordWO:  types.StringNull(), // write-only: never sourced from here
	}

	p := m.apiPayload(types.StringValue("tfpassword123"))

	if p["name"] != "tf-registry" {
		t.Errorf("payload[name] = %v, want tf-registry", p["name"])
	}
	if p["username"] != "tfuser" {
		t.Errorf("payload[username] = %v, want tfuser", p["username"])
	}
	if p["password"] != "tfpassword123" {
		t.Errorf("payload[password] = %v, want tfpassword123 (from cfgPasswordWO arg)", p["password"])
	}
	if p["description"] != "a test registry" {
		t.Errorf("payload[description] = %v, want 'a test registry'", p["description"])
	}
	if p["uri"] != "https://registry.example.com" {
		t.Errorf("payload[uri] = %v, want https://registry.example.com", p["uri"])
	}
}

// TestApiPayload_DescriptionExplicitNull verifies that a null Description
// is sent as an explicit "description": nil in the payload (not omitted) —
// required so app.registry.update (a partial update where omitted keys mean
// "no change") can clear a previously-set description back to null.
func TestApiPayload_DescriptionExplicitNull(t *testing.T) {
	m := &AppRegistryModel{
		Name:        types.StringValue("tf-registry"),
		Description: types.StringNull(),
		URI:         normalize.NewURLValue("https://registry.example.com"),
		Username:    types.StringValue("tfuser"),
	}

	p := m.apiPayload(types.StringValue("tfpassword123"))

	descVal, ok := p["description"]
	if !ok {
		t.Fatal("payload should include an explicit 'description' key when null, not omit it")
	}
	if descVal != nil {
		t.Errorf("payload[description] = %v, want nil", descVal)
	}
}

// TestApiPayload_URIOmittedWhenUnknown verifies that "uri" is omitted from
// the payload when unknown (the create-time case when the user doesn't set
// it: Optional+Computed with no prior state produces an Unknown planned
// value), letting the TrueNAS-side default ("https://index.docker.io/v1/")
// apply.
func TestApiPayload_URIOmittedWhenUnknown(t *testing.T) {
	m := &AppRegistryModel{
		Name:        types.StringValue("tf-registry"),
		Description: types.StringNull(),
		URI:         normalize.NewURLUnknown(),
		Username:    types.StringValue("tfuser"),
	}

	p := m.apiPayload(types.StringValue("tfpassword123"))

	if _, ok := p["uri"]; ok {
		t.Errorf("payload should omit 'uri' when unknown, got %v", p["uri"])
	}
}

// TestResponseToModel_ProbedShape verifies responseToModel against the
// field shape probed live from app.registry.create/get_instance/query/update
// (core.get_methods, both TrueNAS 25.10 and 26.0): id, name,
// description (nullable), uri, username. PasswordWO is intentionally absent
// from appRegistryAPI entirely (see model.go's doc comment) and must never
// be touched by responseToModel; PasswordWOVersion (config-only trigger, no
// API counterpart) must likewise survive unchanged.
func TestResponseToModel_ProbedShape(t *testing.T) {
	desc := "a probed registry"
	api := &appRegistryAPI{
		ID:          7,
		Name:        "tf-probe-registry",
		Description: &desc,
		URI:         "https://registry.example.com",
		Username:    "tfprobeuser",
	}

	m := &AppRegistryModel{
		PasswordWO:        types.StringValue("user-set-password"),
		PasswordWOVersion: types.Int64Value(3),
	}
	diags := responseToModel(api, m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned diagnostic errors: %v", diags)
	}

	if m.ID.ValueInt64() != 7 {
		t.Errorf("ID = %v, want 7", m.ID.ValueInt64())
	}
	if m.Name.ValueString() != "tf-probe-registry" {
		t.Errorf("Name = %q, want tf-probe-registry", m.Name.ValueString())
	}
	if m.Description.ValueString() != "a probed registry" {
		t.Errorf("Description = %q, want 'a probed registry'", m.Description.ValueString())
	}
	if m.URI.ValueString() != "https://registry.example.com" {
		t.Errorf("URI = %q, want https://registry.example.com", m.URI.ValueString())
	}
	if m.Username.ValueString() != "tfprobeuser" {
		t.Errorf("Username = %q, want tfprobeuser", m.Username.ValueString())
	}

	// DECISIVE: password_wo must survive unchanged (write-only, never read
	// back from the API), matching internal/resources/iscsi_auth's
	// Secret/PeerSecret treatment.
	if m.PasswordWO.ValueString() != "user-set-password" {
		t.Errorf("PasswordWO = %v, want unchanged 'user-set-password' (must never be filled from the API response)", m.PasswordWO)
	}
	// The rotation trigger has no API field; it must carry through untouched.
	if m.PasswordWOVersion.ValueInt64() != 3 {
		t.Errorf("PasswordWOVersion = %v, want unchanged 3 (config-only, no API counterpart)", m.PasswordWOVersion)
	}
}

// TestResponseToModel_DescriptionNull verifies a nil api.Description maps
// to a null Description, and that PasswordWO stays null (not coerced to "")
// when the model started null — mirrors iscsi_auth's equivalent test for
// its write-only secrets.
func TestResponseToModel_DescriptionNull(t *testing.T) {
	api := &appRegistryAPI{
		ID:          1,
		Name:        "tf-probe-registry",
		Description: nil,
		URI:         "https://index.docker.io/v1/",
		Username:    "tfprobeuser",
	}

	var m AppRegistryModel
	diags := responseToModel(api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned diagnostic errors: %v", diags)
	}

	if !m.Description.IsNull() {
		t.Errorf("Description = %v, want null", m.Description)
	}
	if !m.PasswordWO.IsNull() {
		t.Errorf("PasswordWO = %v, want null (must stay null, never coerced to \"\")", m.PasswordWO)
	}
}

// TestResponseToDataSourceModel_ProbedShape mirrors
// TestResponseToModel_ProbedShape for the datasource model, and verifies
// AppRegistryDataSourceModel has no "password" field at all (via
// reflection), mirroring iscsi_auth's datasource-security test.
func TestResponseToDataSourceModel_ProbedShape(t *testing.T) {
	desc := "a probed registry"
	api := &appRegistryAPI{
		ID:          1,
		Name:        "tf-probe-registry",
		Description: &desc,
		URI:         "https://registry.example.com",
		Username:    "tfprobeuser",
	}

	m := &AppRegistryDataSourceModel{}
	diags := responseToDataSourceModel(api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.Username.ValueString() != "tfprobeuser" {
		t.Errorf("Username = %q, want tfprobeuser", m.Username.ValueString())
	}
}

// TestAppRegistryDataSourceModel_NoPasswordField verifies (via reflection)
// that AppRegistryDataSourceModel has no password field of any form
// ("password" or the write-only "password_wo"), so registry credentials can
// never be exposed through the datasource.
func TestAppRegistryDataSourceModel_NoPasswordField(t *testing.T) {
	modelType := reflect.TypeOf(AppRegistryDataSourceModel{})
	for i := 0; i < modelType.NumField(); i++ {
		tag := modelType.Field(i).Tag.Get("tfsdk")
		if tag == "password" || tag == "password_wo" {
			t.Errorf("AppRegistryDataSourceModel must not have a %q field (password must never be exposed via datasource)", tag)
		}
	}
}

// TestApiPayload_NullPasswordOmitted: an update that does not rotate the
// token (password_wo_version unchanged) leaves password out of the payload so
// app.registry.update keeps the stored one. (B-141)
func TestApiPayload_NullPasswordOmitted(t *testing.T) {
	m := &AppRegistryModel{Name: types.StringValue("r"), Username: types.StringValue("u")}
	if _, ok := m.apiPayload(types.StringNull())["password"]; ok {
		t.Error("null password must be omitted from the payload")
	}
}

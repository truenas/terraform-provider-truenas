// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package alert_service

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestAttributesMap_InvalidJSON verifies that attributesMap rejects
// malformed JSON with an error diagnostic.
func TestAttributesMap_InvalidJSON(t *testing.T) {
	m := AlertServiceModel{Attributes: types.StringValue("{not valid json")}

	attrs, diags := m.attributesMap()
	if !diags.HasError() {
		t.Fatal("expected error diagnostics for invalid JSON, got none")
	}
	if attrs != nil {
		t.Errorf("expected nil attributes map on error, got %v", attrs)
	}
}

// TestAttributesMap_MissingType verifies that attributesMap rejects valid
// JSON that lacks the required "type" key.
func TestAttributesMap_MissingType(t *testing.T) {
	m := AlertServiceModel{Attributes: types.StringValue(`{"to": "admin@example.com"}`)}

	attrs, diags := m.attributesMap()
	if !diags.HasError() {
		t.Fatal("expected error diagnostics for missing type, got none")
	}
	if attrs != nil {
		t.Errorf("expected nil attributes map on error, got %v", attrs)
	}
}

// TestAttributesMap_Valid verifies that attributesMap parses valid JSON with
// a type key without error.
func TestAttributesMap_Valid(t *testing.T) {
	m := AlertServiceModel{Attributes: types.StringValue(`{"type": "Mail", "to": "admin@example.com"}`)}

	attrs, diags := m.attributesMap()
	if diags.HasError() {
		t.Fatalf("unexpected error diagnostics: %v", diags)
	}
	if attrs["type"] != "Mail" {
		t.Errorf("attrs[type] = %v, want Mail", attrs["type"])
	}
}

// TestCreatePayload_Keys verifies createPayload always includes name, level,
// and attributes, and includes enabled when it is known and non-null.
func TestCreatePayload_Keys(t *testing.T) {
	m := AlertServiceModel{
		Name:       types.StringValue("my-alert"),
		Level:      types.StringValue("WARNING"),
		Enabled:    types.BoolValue(true),
		Attributes: types.StringValue(`{"type": "Mail", "to": "admin@example.com"}`),
	}

	payload, diags := m.createPayload()
	if diags.HasError() {
		t.Fatalf("unexpected error diagnostics: %v", diags)
	}
	if payload["name"] != "my-alert" {
		t.Errorf("payload[name] = %v, want my-alert", payload["name"])
	}
	if payload["level"] != "WARNING" {
		t.Errorf("payload[level] = %v, want WARNING", payload["level"])
	}
	attrsVal, ok := payload["attributes"]
	if !ok {
		t.Fatal("create payload missing key 'attributes'")
	}
	am, ok := attrsVal.(map[string]any)
	if !ok {
		t.Fatalf("payload[attributes] is %T, want map[string]any", attrsVal)
	}
	if am["type"] != "Mail" {
		t.Errorf("payload[attributes][type] = %v, want Mail", am["type"])
	}
	enabledVal, ok := payload["enabled"]
	if !ok {
		t.Fatal("create payload missing key 'enabled' when Enabled is known and non-null")
	}
	if enabledVal != true {
		t.Errorf("payload[enabled] = %v, want true", enabledVal)
	}
}

// TestCreatePayload_EnabledOmittedWhenUnset verifies that enabled is omitted
// from the payload when it is null or unknown.
func TestCreatePayload_EnabledOmittedWhenUnset(t *testing.T) {
	m := AlertServiceModel{
		Name:       types.StringValue("my-alert"),
		Level:      types.StringValue("WARNING"),
		Enabled:    types.BoolNull(),
		Attributes: types.StringValue(`{"type": "Mail", "to": "admin@example.com"}`),
	}

	payload, diags := m.createPayload()
	if diags.HasError() {
		t.Fatalf("unexpected error diagnostics: %v", diags)
	}
	if _, ok := payload["enabled"]; ok {
		t.Error("create payload should not contain key 'enabled' when Enabled is null")
	}

	m.Enabled = types.BoolUnknown()
	payload, diags = m.createPayload()
	if diags.HasError() {
		t.Fatalf("unexpected error diagnostics: %v", diags)
	}
	if _, ok := payload["enabled"]; ok {
		t.Error("create payload should not contain key 'enabled' when Enabled is unknown")
	}
}

// TestUpdatePayload_Keys verifies updatePayload also includes name, level,
// and attributes always, with enabled guarded the same way.
func TestUpdatePayload_Keys(t *testing.T) {
	m := AlertServiceModel{
		ID:         types.Int64Value(7),
		Name:       types.StringValue("my-alert"),
		Level:      types.StringValue("CRITICAL"),
		Attributes: types.StringValue(`{"type": "Slack", "url": "https://hooks.slack.com/x"}`),
	}

	payload, diags := m.updatePayload()
	if diags.HasError() {
		t.Fatalf("unexpected error diagnostics: %v", diags)
	}
	if payload["name"] != "my-alert" {
		t.Errorf("payload[name] = %v, want my-alert", payload["name"])
	}
	if payload["level"] != "CRITICAL" {
		t.Errorf("payload[level] = %v, want CRITICAL", payload["level"])
	}
	if _, ok := payload["attributes"]; !ok {
		t.Fatal("update payload missing key 'attributes'")
	}
	if _, ok := payload["enabled"]; ok {
		t.Error("update payload should not contain key 'enabled' when Enabled is null (zero value)")
	}
}

// TestAttributesDrifted_Identical verifies that identical maps are not
// considered drifted.
func TestAttributesDrifted_Identical(t *testing.T) {
	state := map[string]any{"type": "Mail", "to": "admin@example.com"}
	api := map[string]any{"type": "Mail", "to": "admin@example.com"}

	if attributesDrifted(state, api) {
		t.Error("attributesDrifted() = true for identical maps, want false")
	}
}

// TestAttributesDrifted_ChangedValue verifies that a changed value for a
// user-set key is reported as drift.
func TestAttributesDrifted_ChangedValue(t *testing.T) {
	state := map[string]any{"type": "Mail", "to": "admin@example.com"}
	api := map[string]any{"type": "Mail", "to": "other@example.com"}

	if !attributesDrifted(state, api) {
		t.Error("attributesDrifted() = false for changed value, want true")
	}
}

// TestAttributesDrifted_APIAddedKey verifies that a key present only in the
// API response (a server-added default) is NOT considered drift.
func TestAttributesDrifted_APIAddedKey(t *testing.T) {
	state := map[string]any{"type": "Mail", "to": "admin@example.com"}
	api := map[string]any{"type": "Mail", "to": "admin@example.com", "from": "noreply@example.com"}

	if attributesDrifted(state, api) {
		t.Error("attributesDrifted() = true for API-added extra key, want false")
	}
}

// TestAttributesDrifted_MissingUserKey verifies that a user-set key absent
// from the API response is considered drift.
func TestAttributesDrifted_MissingUserKey(t *testing.T) {
	state := map[string]any{"type": "Mail", "to": "admin@example.com"}
	api := map[string]any{"type": "Mail"}

	if !attributesDrifted(state, api) {
		t.Error("attributesDrifted() = false for missing user key in API, want true")
	}
}

// TestSchema_AttributesSensitiveRequired verifies that attributes is
// Required and Sensitive.
func TestSchema_AttributesSensitiveRequired(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["attributes"]
	if !ok {
		t.Fatal("schema missing 'attributes' attribute")
	}
	strAttr, ok := attr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'attributes' attribute is %T, want schema.StringAttribute", attr)
	}
	if !strAttr.IsRequired() {
		t.Error("'attributes' should be Required")
	}
	if !strAttr.IsSensitive() {
		t.Error("'attributes' should be Sensitive")
	}
}

// TestSchema_IDComputedUseStateForUnknown verifies that the "id" attribute
// is Computed with a UseStateForUnknown plan modifier.
func TestSchema_IDComputedUseStateForUnknown(t *testing.T) {
	s := resourceSchema()

	idAttr, ok := s.Attributes["id"]
	if !ok {
		t.Fatal("schema missing 'id' attribute")
	}
	idInt64, ok := idAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'id' attribute is %T, want schema.Int64Attribute", idAttr)
	}
	if !idInt64.IsComputed() {
		t.Error("'id' should be Computed")
	}
	if idInt64.IsRequired() || idInt64.IsOptional() {
		t.Error("'id' should be Computed-only (not Required/Optional)")
	}

	foundUseState := false
	for _, pm := range idInt64.PlanModifiers {
		if pm.Description(context.Background()) == int64planmodifier.UseStateForUnknown().Description(context.Background()) {
			foundUseState = true
		}
	}
	if !foundUseState {
		t.Error("'id' should have a UseStateForUnknown plan modifier")
	}
}

// TestSchema_NameRequired verifies that "name" is a Required StringAttribute.
func TestSchema_NameRequired(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["name"]
	if !ok {
		t.Fatal("schema missing 'name' attribute")
	}
	strAttr, ok := attr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'name' attribute is %T, want schema.StringAttribute", attr)
	}
	if !strAttr.IsRequired() {
		t.Error("'name' should be Required")
	}
}

// TestSchema_LevelRequired verifies that "level" is a Required
// StringAttribute.
func TestSchema_LevelRequired(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["level"]
	if !ok {
		t.Fatal("schema missing 'level' attribute")
	}
	strAttr, ok := attr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'level' attribute is %T, want schema.StringAttribute", attr)
	}
	if !strAttr.IsRequired() {
		t.Error("'level' should be Required")
	}
}

// TestSchema_EnabledOptionalComputed verifies that "enabled" is Optional +
// Computed with a UseStateForUnknown plan modifier.
func TestSchema_EnabledOptionalComputed(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["enabled"]
	if !ok {
		t.Fatal("schema missing 'enabled' attribute")
	}
	boolAttr, ok := attr.(schema.BoolAttribute)
	if !ok {
		t.Fatalf("'enabled' attribute is %T, want schema.BoolAttribute", attr)
	}
	if !boolAttr.IsOptional() {
		t.Error("'enabled' should be Optional")
	}
	if !boolAttr.IsComputed() {
		t.Error("'enabled' should be Computed")
	}
}

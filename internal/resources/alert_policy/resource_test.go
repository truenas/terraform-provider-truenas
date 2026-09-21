// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package alert_policy

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestAlertPolicySchema_IDIsComputed verifies that "id" is a Computed-only
// StringAttribute, since it's a fixed singleton value never supplied by the
// user.
func TestAlertPolicySchema_IDIsComputed(t *testing.T) {
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
	if len(idStr.PlanModifiers) == 0 {
		t.Error("'id' should have plan modifiers (UseStateForUnknown)")
	}
}

// TestAlertPolicySchema_ClassesIsRequired verifies that "classes" is a
// Required (not Sensitive) StringAttribute.
func TestAlertPolicySchema_ClassesIsRequired(t *testing.T) {
	s := resourceSchema()

	classesAttr, ok := s.Attributes["classes"]
	if !ok {
		t.Fatal("schema missing 'classes' attribute")
	}
	classesStr, ok := classesAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'classes' attribute is %T, want schema.StringAttribute", classesAttr)
	}
	if !classesStr.IsRequired() {
		t.Error("'classes' should be Required")
	}
	if classesStr.IsSensitive() {
		t.Error("'classes' should not be Sensitive")
	}
}

// TestClassesMap_InvalidJSON verifies that invalid classes JSON produces a
// diagnostic error rather than a panic or silent failure.
func TestClassesMap_InvalidJSON(t *testing.T) {
	m := &AlertPolicyModel{Classes: types.StringValue("{not valid json")}

	_, diags := m.classesMap()
	if !diags.HasError() {
		t.Fatal("expected a diagnostic error for invalid classes JSON, got none")
	}
}

// TestClassesMap_ValidJSON verifies that valid classes JSON (including the
// empty object) parses without error.
func TestClassesMap_ValidJSON(t *testing.T) {
	cases := []string{
		"{}",
		`{"UPSBatteryLow": {"level": "CRITICAL", "policy": "IMMEDIATELY"}}`,
	}
	for _, c := range cases {
		m := &AlertPolicyModel{Classes: types.StringValue(c)}
		if _, diags := m.classesMap(); diags.HasError() {
			t.Errorf("classes=%q: unexpected error: %v", c, diags)
		}
	}
}

// TestUpdatePayload_InvalidJSON verifies that updatePayload surfaces the same
// diagnostic as classesMap for invalid JSON, and returns a nil payload.
func TestUpdatePayload_InvalidJSON(t *testing.T) {
	m := &AlertPolicyModel{Classes: types.StringValue("not json at all")}

	payload, diags := m.updatePayload()
	if !diags.HasError() {
		t.Fatal("expected a diagnostic error for invalid classes JSON, got none")
	}
	if payload != nil {
		t.Errorf("expected nil payload on error, got %v", payload)
	}
}

// TestUpdatePayload_Valid verifies that updatePayload wraps the parsed
// classes map under the "classes" key.
func TestUpdatePayload_Valid(t *testing.T) {
	m := &AlertPolicyModel{Classes: types.StringValue(`{"UPSBatteryLow": {"level": "CRITICAL", "policy": "IMMEDIATELY"}}`)}

	payload, diags := m.updatePayload()
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	classes, ok := payload["classes"].(map[string]any)
	if !ok {
		t.Fatalf("payload[\"classes\"] is %T, want map[string]any", payload["classes"])
	}
	entry, ok := classes["UPSBatteryLow"].(map[string]any)
	if !ok {
		t.Fatalf("classes[\"UPSBatteryLow\"] is %T, want map[string]any", classes["UPSBatteryLow"])
	}
	if entry["level"] != "CRITICAL" || entry["policy"] != "IMMEDIATELY" {
		t.Errorf("unexpected entry: %v", entry)
	}
}

// TestDeletePayload verifies that Delete's payload builder always sends an
// empty classes object, resetting all classes to their TrueNAS defaults.
func TestDeletePayload(t *testing.T) {
	payload := deletePayload()

	classes, ok := payload["classes"].(map[string]any)
	if !ok {
		t.Fatalf("payload[\"classes\"] is %T, want map[string]any", payload["classes"])
	}
	if len(classes) != 0 {
		t.Errorf("expected empty classes map, got %v", classes)
	}
}

// TestClassesEqual_SameContentDifferentKeyOrder verifies that classesEqual
// treats maps with identical content as equal, independent of the key order
// in the original JSON text used to build them (Go maps have no key order,
// so this really tests that DeepEqual is applied to parsed maps, not raw
// strings).
func TestClassesEqual_SameContentDifferentKeyOrder(t *testing.T) {
	a := map[string]any{
		"UPSBatteryLow": map[string]any{"level": "CRITICAL", "policy": "IMMEDIATELY"},
		"ZpoolCapacity": map[string]any{"level": "WARNING", "policy": "DAILY"},
	}
	b := map[string]any{
		"ZpoolCapacity": map[string]any{"policy": "DAILY", "level": "WARNING"},
		"UPSBatteryLow": map[string]any{"policy": "IMMEDIATELY", "level": "CRITICAL"},
	}

	if !classesEqual(a, b) {
		t.Error("expected classesEqual to report equal maps as equal regardless of key order")
	}
}

// TestClassesEqual_DifferentValues verifies that classesEqual reports drift
// when a class's value differs.
func TestClassesEqual_DifferentValues(t *testing.T) {
	a := map[string]any{
		"UPSBatteryLow": map[string]any{"level": "CRITICAL", "policy": "IMMEDIATELY"},
	}
	b := map[string]any{
		"UPSBatteryLow": map[string]any{"level": "WARNING", "policy": "IMMEDIATELY"},
	}

	if classesEqual(a, b) {
		t.Error("expected classesEqual to report differing values as not equal")
	}
}

// TestClassesEqual_DifferentKeys verifies that classesEqual reports drift
// when classes are added or removed (whole-object ownership, not a subset
// comparison).
func TestClassesEqual_DifferentKeys(t *testing.T) {
	a := map[string]any{
		"UPSBatteryLow": map[string]any{"level": "CRITICAL", "policy": "IMMEDIATELY"},
	}
	b := map[string]any{
		"UPSBatteryLow": map[string]any{"level": "CRITICAL", "policy": "IMMEDIATELY"},
		"ZpoolCapacity": map[string]any{"level": "WARNING", "policy": "DAILY"},
	}

	if classesEqual(a, b) {
		t.Error("expected classesEqual to report added/removed keys as not equal")
	}
}

// TestApplyAPIToModel_EqualKeepsStateString verifies that when the state's
// classes JSON deep-equals the API's classes (even with different key
// order/whitespace in the JSON text), Read/Create/Update keep the state's
// original string rather than overwriting it with the API's canonical form.
func TestApplyAPIToModel_EqualKeepsStateString(t *testing.T) {
	stateJSON := `{"UPSBatteryLow": {"policy": "IMMEDIATELY", "level": "CRITICAL"}}`
	m := &AlertPolicyModel{Classes: types.StringValue(stateJSON)}
	api := &alertClassesAPI{
		ID: 1,
		Classes: map[string]any{
			"UPSBatteryLow": map[string]any{"level": "CRITICAL", "policy": "IMMEDIATELY"},
		},
	}

	diags := applyAPIToModel(api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.ID.ValueString() != alertPolicyResourceID {
		t.Errorf("ID = %q, want %q", m.ID.ValueString(), alertPolicyResourceID)
	}
	if m.Classes.ValueString() != stateJSON {
		t.Errorf("Classes = %q, want unchanged state string %q", m.Classes.ValueString(), stateJSON)
	}
}

// TestApplyAPIToModel_DifferentOverwritesState verifies that when the API's
// classes differ from state, the state string is overwritten with the API's
// JSON.
func TestApplyAPIToModel_DifferentOverwritesState(t *testing.T) {
	stateJSON := `{"UPSBatteryLow": {"level": "CRITICAL", "policy": "IMMEDIATELY"}}`
	m := &AlertPolicyModel{Classes: types.StringValue(stateJSON)}
	api := &alertClassesAPI{
		ID: 1,
		Classes: map[string]any{
			"UPSBatteryLow": map[string]any{"level": "WARNING", "policy": "IMMEDIATELY"},
		},
	}

	diags := applyAPIToModel(api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.Classes.ValueString() == stateJSON {
		t.Error("expected state classes string to be overwritten, but it was unchanged")
	}

	want, _ := apiClassesJSON(api)
	if m.Classes.ValueString() != want {
		t.Errorf("Classes = %q, want %q", m.Classes.ValueString(), want)
	}
}

// TestApplyAPIToModel_NullStatePopulatesFromAPI verifies the import path:
// when state's Classes is null (as it is right after ImportState sets only
// id), applyAPIToModel populates Classes directly from the API response.
func TestApplyAPIToModel_NullStatePopulatesFromAPI(t *testing.T) {
	m := &AlertPolicyModel{Classes: types.StringNull()}
	api := &alertClassesAPI{
		ID: 1,
		Classes: map[string]any{
			"UPSBatteryLow": map[string]any{"level": "CRITICAL", "policy": "IMMEDIATELY"},
		},
	}

	diags := applyAPIToModel(api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.Classes.IsNull() {
		t.Fatal("expected Classes to be populated, still null")
	}

	want, _ := apiClassesJSON(api)
	if m.Classes.ValueString() != want {
		t.Errorf("Classes = %q, want %q", m.Classes.ValueString(), want)
	}
}

// TestApplyAPIToModel_EmptyAPIClasses verifies that an empty API classes
// object ("{}", the post-Delete/default state) round-trips correctly.
func TestApplyAPIToModel_EmptyAPIClasses(t *testing.T) {
	m := &AlertPolicyModel{Classes: types.StringNull()}
	api := &alertClassesAPI{ID: 1, Classes: map[string]any{}}

	diags := applyAPIToModel(api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.Classes.ValueString() != "{}" {
		t.Errorf("Classes = %q, want \"{}\"", m.Classes.ValueString())
	}
}

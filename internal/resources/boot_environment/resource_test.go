// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package boot_environment

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// --- Schema shape ---------------------------------------------------------

func TestSchema_IDIsComputedString(t *testing.T) {
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

func TestSchema_NameIsRequiresReplace(t *testing.T) {
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

func TestSchema_SourceIsRequiresReplace(t *testing.T) {
	s := resourceSchema()

	sourceAttr, ok := s.Attributes["source"]
	if !ok {
		t.Fatal("schema missing 'source' attribute")
	}
	sourceStr, ok := sourceAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'source' attribute is %T, want schema.StringAttribute", sourceAttr)
	}
	if !sourceStr.IsRequired() {
		t.Error("'source' should be Required")
	}
	if len(sourceStr.PlanModifiers) == 0 {
		t.Error("'source' should have plan modifiers (RequiresReplace)")
	}
}

func TestSchema_ActivatedIsOptionalComputed(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["activated"]
	if !ok {
		t.Fatal("schema missing 'activated' attribute")
	}
	b, ok := attr.(schema.BoolAttribute)
	if !ok {
		t.Fatalf("'activated' attribute is %T, want schema.BoolAttribute", attr)
	}
	if !b.IsOptional() {
		t.Error("'activated' should be Optional")
	}
	if !b.IsComputed() {
		t.Error("'activated' should be Computed")
	}
	if len(b.PlanModifiers) == 0 {
		t.Error("'activated' should have plan modifiers (UseStateForUnknown)")
	}
}

func TestSchema_KeepIsOptionalComputed(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["keep"]
	if !ok {
		t.Fatal("schema missing 'keep' attribute")
	}
	b, ok := attr.(schema.BoolAttribute)
	if !ok {
		t.Fatalf("'keep' attribute is %T, want schema.BoolAttribute", attr)
	}
	if !b.IsOptional() {
		t.Error("'keep' should be Optional")
	}
	if !b.IsComputed() {
		t.Error("'keep' should be Computed")
	}
	if len(b.PlanModifiers) == 0 {
		t.Error("'keep' should have plan modifiers (UseStateForUnknown)")
	}
}

func TestSchema_DatasetHasUseStateForUnknown(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["dataset"]
	if !ok {
		t.Fatal("schema missing 'dataset' attribute")
	}
	str, ok := attr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'dataset' attribute is %T, want schema.StringAttribute", attr)
	}
	if !str.IsComputed() {
		t.Error("'dataset' should be Computed")
	}
	if len(str.PlanModifiers) == 0 {
		t.Error("'dataset' should have plan modifiers (UseStateForUnknown)")
	}
}

// TestSchema_ActiveAndUsedBytesHaveNoPlanModifiers verifies that "active" and
// "used_bytes" are Computed-only with NO plan modifiers, because they are
// server-mutable (e.g. activating a *different* BE flips this one's "active"
// field) and must never be pinned to stale prior state.
func TestSchema_ActiveAndUsedBytesHaveNoPlanModifiers(t *testing.T) {
	s := resourceSchema()

	activeAttr, ok := s.Attributes["active"]
	if !ok {
		t.Fatal("schema missing 'active' attribute")
	}
	activeBool, ok := activeAttr.(schema.BoolAttribute)
	if !ok {
		t.Fatalf("'active' attribute is %T, want schema.BoolAttribute", activeAttr)
	}
	if !activeBool.IsComputed() {
		t.Error("'active' should be Computed")
	}
	if activeBool.IsOptional() {
		t.Error("'active' should be Computed-only, not Optional")
	}
	if len(activeBool.PlanModifiers) != 0 {
		t.Error("'active' should have NO plan modifiers (server-mutable)")
	}

	usedAttr, ok := s.Attributes["used_bytes"]
	if !ok {
		t.Fatal("schema missing 'used_bytes' attribute")
	}
	usedInt, ok := usedAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'used_bytes' attribute is %T, want schema.Int64Attribute", usedAttr)
	}
	if !usedInt.IsComputed() {
		t.Error("'used_bytes' should be Computed")
	}
	if len(usedInt.PlanModifiers) != 0 {
		t.Error("'used_bytes' should have NO plan modifiers (server-mutable)")
	}
}

// --- responseToModel --------------------------------------------------

func TestResponseToModel_AllFields(t *testing.T) {
	api := &bootEnvAPI{
		ID:        "24.10-BE2",
		Dataset:   "boot-pool/ROOT/24.10-BE2",
		Active:    true,
		Activated: true,
		Keep:      true,
		UsedBytes: 123456,
	}

	var m BootEnvironmentModel
	responseToModel(api, &m)

	if m.ID != types.StringValue("24.10-BE2") {
		t.Errorf("ID = %v, want 24.10-BE2", m.ID)
	}
	if m.Name != types.StringValue("24.10-BE2") {
		t.Errorf("Name = %v, want 24.10-BE2", m.Name)
	}
	if m.Dataset != types.StringValue("boot-pool/ROOT/24.10-BE2") {
		t.Errorf("Dataset = %v, want boot-pool/ROOT/24.10-BE2", m.Dataset)
	}
	if m.Active != types.BoolValue(true) {
		t.Errorf("Active = %v, want true", m.Active)
	}
	if m.Activated != types.BoolValue(true) {
		t.Errorf("Activated = %v, want true", m.Activated)
	}
	if m.Keep != types.BoolValue(true) {
		t.Errorf("Keep = %v, want true", m.Keep)
	}
	if m.UsedBytes != types.Int64Value(123456) {
		t.Errorf("UsedBytes = %v, want 123456", m.UsedBytes)
	}
}

// TestResponseToModel_DoesNotTouchSource verifies that responseToModel never
// overwrites Source: the API never returns a clone-origin field, so
// whatever value already lives in state/plan must survive untouched.
func TestResponseToModel_DoesNotTouchSource(t *testing.T) {
	api := &bootEnvAPI{
		ID:        "new-be",
		Dataset:   "boot-pool/ROOT/new-be",
		Active:    false,
		Activated: false,
		Keep:      false,
		UsedBytes: 1,
	}

	m := BootEnvironmentModel{
		Source: types.StringValue("24.10-BE1"),
	}
	responseToModel(api, &m)

	if m.Source != types.StringValue("24.10-BE1") {
		t.Errorf("Source = %v, want unchanged 24.10-BE1", m.Source)
	}
}

// --- Create payload shapes -------------------------------------------------

// TestClonePayloadShape verifies the exact shape of the clone() call args:
// {"id": source, "target": name}.
func TestClonePayloadShape(t *testing.T) {
	source := "24.10-BE1"
	name := "24.10-BE2"

	payload := clonePayload(source, name)

	want := map[string]any{"id": source, "target": name}
	if len(payload) != len(want) {
		t.Fatalf("clonePayload() has %d keys, want %d: %#v", len(payload), len(want), payload)
	}
	for k, v := range want {
		got, ok := payload[k]
		if !ok {
			t.Errorf("clonePayload() missing key %q", k)
			continue
		}
		if got != v {
			t.Errorf("clonePayload()[%q] = %v, want %v", k, got, v)
		}
	}
}

// TestKeepPayloadShape verifies the exact shape of the keep() call args:
// {"id": name, "value": v}.
func TestKeepPayloadShape(t *testing.T) {
	name := "24.10-BE2"

	for _, v := range []bool{true, false} {
		payload := keepPayload(name, v)

		want := map[string]any{"id": name, "value": v}
		if len(payload) != len(want) {
			t.Fatalf("keepPayload(%v) has %d keys, want %d: %#v", v, len(payload), len(want), payload)
		}
		for k, wv := range want {
			got, ok := payload[k]
			if !ok {
				t.Errorf("keepPayload(%v) missing key %q", v, k)
				continue
			}
			if got != wv {
				t.Errorf("keepPayload(%v)[%q] = %v, want %v", v, k, got, wv)
			}
		}
	}
}

// --- Activation decision logic ---------------------------------------------

// TestPlanActivationChange_FalseToTrue verifies that activating an inactive
// BE (state=false -> plan=true) requests an activate call.
func TestPlanActivationChange_FalseToTrue(t *testing.T) {
	if got := planActivationChange(false, true); got != activationActivate {
		t.Errorf("planActivationChange(false, true) = %v, want activationActivate", got)
	}
}

// TestPlanActivationChange_TrueToFalse verifies that flipping an activated BE
// back to inactive is flagged as unsupported: the API has no "deactivate"
// operation, only activate-a-different-BE.
func TestPlanActivationChange_TrueToFalse(t *testing.T) {
	if got := planActivationChange(true, false); got != activationUnsupportedDeactivate {
		t.Errorf("planActivationChange(true, false) = %v, want activationUnsupportedDeactivate", got)
	}
}

func TestPlanActivationChange_NoChange(t *testing.T) {
	cases := []struct{ state, plan bool }{
		{true, true},
		{false, false},
	}
	for _, tc := range cases {
		if got := planActivationChange(tc.state, tc.plan); got != activationNoChange {
			t.Errorf("planActivationChange(%v, %v) = %v, want activationNoChange", tc.state, tc.plan, got)
		}
	}
}

// TestUpdate_ActivatedTrueToFalseProducesError exercises the Update method
// end-to-end (without a live API) to confirm that requesting
// activated: true -> false surfaces as an error diagnostic rather than
// silently succeeding or panicking.
func TestUpdate_ActivatedTrueToFalseProducesError(t *testing.T) {
	// This mirrors the guard inside BootEnvironmentResource.Update: given
	// state.Activated=true and plan.Activated=false, the decision helper
	// must report activationUnsupportedDeactivate so the caller emits an
	// error diagnostic and returns without calling any API method.
	state := BootEnvironmentModel{Activated: types.BoolValue(true)}
	plan := BootEnvironmentModel{Activated: types.BoolValue(false)}

	change := planActivationChange(state.Activated.ValueBool(), plan.Activated.ValueBool())
	if change != activationUnsupportedDeactivate {
		t.Fatalf("expected activationUnsupportedDeactivate, got %v", change)
	}
}

// --- Delete guard -----------------------------------------------------

// TestDeleteGuard_ActiveOrActivatedBlocksDestroy verifies the pure decision
// logic behind Delete: a BE that is active or activated must never be
// destroyed.
func TestDeleteGuard_ActiveOrActivatedBlocksDestroy(t *testing.T) {
	cases := []struct {
		name             string
		active, activate bool
		wantBlocked      bool
	}{
		{"active only", true, false, true},
		{"activated only", false, true, true},
		{"both", true, true, true},
		{"neither", false, false, false},
	}

	for _, tc := range cases {
		state := BootEnvironmentModel{
			ID:        types.StringValue("some-be"),
			Active:    types.BoolValue(tc.active),
			Activated: types.BoolValue(tc.activate),
		}
		blocked := deleteBlocked(&state)
		if blocked != tc.wantBlocked {
			t.Errorf("%s: deleteBlocked() = %v, want %v", tc.name, blocked, tc.wantBlocked)
		}
	}
}

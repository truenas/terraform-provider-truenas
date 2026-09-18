// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package twofactor_auth

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// TestTwoFactorAuthSchema_IDIsComputed verifies that "id" is a
// Computed-only StringAttribute with UseStateForUnknown, since it's a fixed
// singleton value never supplied by the user.
func TestTwoFactorAuthSchema_IDIsComputed(t *testing.T) {
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

// TestTwoFactorAuthSchema_EnabledIsOptionalComputed verifies that "enabled"
// is Optional+Computed with a UseStateForUnknown plan modifier — settable,
// and (via updatePayload sourcing inclusion from req.Config rather than
// the plan) never sent unless explicitly configured, which is what keeps
// the committed acceptance test from ever touching it.
func TestTwoFactorAuthSchema_EnabledIsOptionalComputed(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["enabled"]
	if !ok {
		t.Fatal("schema missing 'enabled' attribute")
	}
	boolAttr, ok := attr.(schema.BoolAttribute)
	if !ok {
		t.Fatalf("'enabled' attribute is %T, want schema.BoolAttribute", attr)
	}
	if !boolAttr.IsOptional() || !boolAttr.IsComputed() {
		t.Error("'enabled' should be Optional+Computed")
	}
	if len(boolAttr.PlanModifiers) == 0 {
		t.Error("'enabled' should have plan modifiers (UseStateForUnknown)")
	}
}

// TestTwoFactorAuthSchema_WindowIsOptionalComputed verifies that "window"
// is Optional+Computed with a UseStateForUnknown plan modifier and an
// AtLeast(0) validator, matching the probed auth.twofactor.update schema
// (minimum 0, no explicit maximum).
func TestTwoFactorAuthSchema_WindowIsOptionalComputed(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["window"]
	if !ok {
		t.Fatal("schema missing 'window' attribute")
	}
	intAttr, ok := attr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'window' attribute is %T, want schema.Int64Attribute", attr)
	}
	if !intAttr.IsOptional() || !intAttr.IsComputed() {
		t.Error("'window' should be Optional+Computed")
	}
	if len(intAttr.PlanModifiers) == 0 {
		t.Error("'window' should have plan modifiers (UseStateForUnknown)")
	}
	if len(intAttr.Validators) == 0 {
		t.Error("'window' should have a validator (AtLeast(0))")
	}
}

// TestTwoFactorAuthSchema_ServicesIsOptionalComputed verifies that
// "services" is an Optional+Computed SingleNestedAttribute with a nested
// Required "ssh" bool, matching the probed shape.
func TestTwoFactorAuthSchema_ServicesIsOptionalComputed(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["services"]
	if !ok {
		t.Fatal("schema missing 'services' attribute")
	}
	nested, ok := attr.(schema.SingleNestedAttribute)
	if !ok {
		t.Fatalf("'services' attribute is %T, want schema.SingleNestedAttribute", attr)
	}
	if !nested.IsOptional() || !nested.IsComputed() {
		t.Error("'services' should be Optional+Computed")
	}
	if len(nested.PlanModifiers) == 0 {
		t.Error("'services' should have plan modifiers (UseStateForUnknown)")
	}

	sshAttr, ok := nested.Attributes["ssh"]
	if !ok {
		t.Fatal("'services' schema missing 'ssh' attribute")
	}
	sshBool, ok := sshAttr.(schema.BoolAttribute)
	if !ok {
		t.Fatalf("'services.ssh' attribute is %T, want schema.BoolAttribute", sshAttr)
	}
	if !sshBool.IsRequired() {
		t.Error("'services.ssh' should be Required")
	}
}

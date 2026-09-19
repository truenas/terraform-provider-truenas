// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package audit_config

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// TestAuditConfigSchema_IDIsComputed verifies that "id" is a Computed-only
// StringAttribute with UseStateForUnknown, since it's a fixed singleton
// value never supplied by the user.
func TestAuditConfigSchema_IDIsComputed(t *testing.T) {
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

// TestAuditConfigSchema_SettableFieldsAreOptionalComputed verifies that
// retention, reservation, quota, quota_fill_warning, and quota_fill_critical
// are all Optional+Computed with UseStateForUnknown plan modifiers, matching
// the fields audit.update actually accepts.
func TestAuditConfigSchema_SettableFieldsAreOptionalComputed(t *testing.T) {
	s := resourceSchema()

	for _, name := range []string{"retention", "reservation", "quota", "quota_fill_warning", "quota_fill_critical"} {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		intAttr, ok := attr.(schema.Int64Attribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.Int64Attribute", name, attr)
		}
		if !intAttr.IsOptional() || !intAttr.IsComputed() {
			t.Errorf("%q should be Optional+Computed", name)
		}
		if len(intAttr.PlanModifiers) == 0 {
			t.Errorf("%q should have plan modifiers (UseStateForUnknown)", name)
		}
	}
}

// TestAuditConfigSchema_ReadOnlyFieldsAreComputedOnly verifies that
// remote_logging_enabled, space, and enabled_services are Computed-only:
// audit.update does not accept any of them.
func TestAuditConfigSchema_ReadOnlyFieldsAreComputedOnly(t *testing.T) {
	s := resourceSchema()

	remoteAttr, ok := s.Attributes["remote_logging_enabled"]
	if !ok {
		t.Fatal("schema missing 'remote_logging_enabled' attribute")
	}
	boolAttr, ok := remoteAttr.(schema.BoolAttribute)
	if !ok {
		t.Fatalf("'remote_logging_enabled' attribute is %T, want schema.BoolAttribute", remoteAttr)
	}
	if !boolAttr.IsComputed() || boolAttr.IsOptional() || boolAttr.IsRequired() {
		t.Error("'remote_logging_enabled' should be Computed-only")
	}

	for _, name := range []string{"space", "enabled_services"} {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		nested, ok := attr.(schema.SingleNestedAttribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.SingleNestedAttribute", name, attr)
		}
		if !nested.IsComputed() || nested.IsOptional() || nested.IsRequired() {
			t.Errorf("%q should be Computed-only", name)
		}
	}
}

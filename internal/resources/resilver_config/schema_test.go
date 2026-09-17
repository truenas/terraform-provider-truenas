// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package resilver_config

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// TestResilverConfigSchema_IDIsComputed verifies that "id" is a
// Computed-only StringAttribute with UseStateForUnknown, since it's a fixed
// singleton value never supplied by the user.
func TestResilverConfigSchema_IDIsComputed(t *testing.T) {
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

// TestResilverConfigSchema_OtherFieldsAreOptionalComputed verifies that
// begin, end, enabled, and weekday are all Optional+Computed with
// UseStateForUnknown plan modifiers.
func TestResilverConfigSchema_OtherFieldsAreOptionalComputed(t *testing.T) {
	s := resourceSchema()

	for _, name := range []string{"begin", "end"} {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		strAttr, ok := attr.(schema.StringAttribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.StringAttribute", name, attr)
		}
		if !strAttr.IsOptional() || !strAttr.IsComputed() {
			t.Errorf("%q should be Optional+Computed", name)
		}
		if len(strAttr.PlanModifiers) == 0 {
			t.Errorf("%q should have plan modifiers (UseStateForUnknown)", name)
		}
	}

	enabledAttr, ok := s.Attributes["enabled"]
	if !ok {
		t.Fatal("schema missing 'enabled' attribute")
	}
	enabledBool, ok := enabledAttr.(schema.BoolAttribute)
	if !ok {
		t.Fatalf("'enabled' attribute is %T, want schema.BoolAttribute", enabledAttr)
	}
	if !enabledBool.IsOptional() || !enabledBool.IsComputed() {
		t.Error("'enabled' should be Optional+Computed")
	}
	if len(enabledBool.PlanModifiers) == 0 {
		t.Error("'enabled' should have plan modifiers (UseStateForUnknown)")
	}

	weekdayAttr, ok := s.Attributes["weekday"]
	if !ok {
		t.Fatal("schema missing 'weekday' attribute")
	}
	weekdayList, ok := weekdayAttr.(schema.ListAttribute)
	if !ok {
		t.Fatalf("'weekday' attribute is %T, want schema.ListAttribute", weekdayAttr)
	}
	if !weekdayList.IsOptional() || !weekdayList.IsComputed() {
		t.Error("'weekday' should be Optional+Computed")
	}
	if len(weekdayList.PlanModifiers) == 0 {
		t.Error("'weekday' should have plan modifiers (UseStateForUnknown)")
	}
}

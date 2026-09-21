// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cronjob

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// TestCronjobSchema_RequiredFields verifies "command" and "user" are
// Required, matching cronjob.create's own "required" list.
func TestCronjobSchema_RequiredFields(t *testing.T) {
	s := resourceSchema()

	for _, name := range []string{"command", "user"} {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		strAttr, ok := attr.(schema.StringAttribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.StringAttribute", name, attr)
		}
		if !strAttr.IsRequired() {
			t.Errorf("%q should be Required", name)
		}
	}
}

// TestCronjobSchema_IDIsComputed verifies "id" is Computed-only.
func TestCronjobSchema_IDIsComputed(t *testing.T) {
	s := resourceSchema()

	idAttr, ok := s.Attributes["id"]
	if !ok {
		t.Fatal("schema missing 'id' attribute")
	}
	idInt, ok := idAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'id' attribute is %T, want schema.Int64Attribute", idAttr)
	}
	if !idInt.IsComputed() {
		t.Error("'id' should be Computed")
	}
	if idInt.IsRequired() || idInt.IsOptional() {
		t.Error("'id' should be Computed-only (not Required/Optional)")
	}
}

// TestCronjobSchema_ScheduleShape verifies the nested "schedule" attribute
// is Optional+Computed at the top level with Required string sub-fields,
// matching rsync_task/scrub_task's convention.
func TestCronjobSchema_ScheduleShape(t *testing.T) {
	s := resourceSchema()

	schedAttr, ok := s.Attributes["schedule"]
	if !ok {
		t.Fatal("schema missing 'schedule' attribute")
	}
	nested, ok := schedAttr.(schema.SingleNestedAttribute)
	if !ok {
		t.Fatalf("'schedule' attribute is %T, want schema.SingleNestedAttribute", schedAttr)
	}
	if !nested.IsOptional() || !nested.IsComputed() {
		t.Error("'schedule' should be Optional+Computed")
	}
	for _, field := range []string{"minute", "hour", "dom", "month", "dow"} {
		a, exists := nested.Attributes[field]
		if !exists {
			t.Errorf("schedule missing field %q", field)
			continue
		}
		sa, ok := a.(schema.StringAttribute)
		if !ok {
			t.Errorf("schedule.%s is %T, want schema.StringAttribute", field, a)
			continue
		}
		if !sa.IsRequired() {
			t.Errorf("schedule.%s should be Required", field)
		}
	}
}

// TestCronjobSchema_OptionalComputedFields verifies description and the
// bool flags carrying TrueNAS-side defaults are all Optional+Computed.
func TestCronjobSchema_OptionalComputedFields(t *testing.T) {
	s := resourceSchema()

	boolFields := []string{"enabled", "stdout", "stderr"}
	for _, name := range boolFields {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		boolAttr, ok := attr.(schema.BoolAttribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.BoolAttribute", name, attr)
		}
		if !boolAttr.IsOptional() || !boolAttr.IsComputed() {
			t.Errorf("%q should be Optional+Computed", name)
		}
	}

	descAttr, ok := s.Attributes["description"]
	if !ok {
		t.Fatal("schema missing 'description' attribute")
	}
	strAttr, ok := descAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'description' attribute is %T, want schema.StringAttribute", descAttr)
	}
	if !strAttr.IsOptional() || !strAttr.IsComputed() {
		t.Error("'description' should be Optional+Computed")
	}
}

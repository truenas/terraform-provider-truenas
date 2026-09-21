// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package rsync_task

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// TestRsyncTaskSchema_RequiredFields verifies "path" and "user" are
// Required, matching rsynctask.create's own "required" list.
func TestRsyncTaskSchema_RequiredFields(t *testing.T) {
	s := resourceSchema()

	for _, name := range []string{"path", "user"} {
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

// TestRsyncTaskSchema_IDIsComputed verifies "id" is Computed-only.
func TestRsyncTaskSchema_IDIsComputed(t *testing.T) {
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

// TestRsyncTaskSchema_NullableIntsAreOptionalComputed verifies remoteport
// and ssh_credentials — the wire-nullable int fields — are Optional+Computed
// so a null API value doesn't fight the plan.
func TestRsyncTaskSchema_NullableIntsAreOptionalComputed(t *testing.T) {
	s := resourceSchema()

	for _, name := range []string{"remoteport", "ssh_credentials"} {
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
	}
}

// TestRsyncTaskSchema_WriteOnlyFlagsAreOptionalOnly verifies validate_rpath
// and ssh_keyscan are plain Optional (no Computed): the API never returns
// them, so nothing can compute a value for an unset config.
func TestRsyncTaskSchema_WriteOnlyFlagsAreOptionalOnly(t *testing.T) {
	s := resourceSchema()

	for _, name := range []string{"validate_rpath", "ssh_keyscan"} {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		boolAttr, ok := attr.(schema.BoolAttribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.BoolAttribute", name, attr)
		}
		if !boolAttr.IsOptional() {
			t.Errorf("%q should be Optional", name)
		}
		if boolAttr.IsComputed() {
			t.Errorf("%q should NOT be Computed (never returned by the API)", name)
		}
	}
}

// TestRsyncTaskSchema_ScheduleShape verifies the nested "schedule" attribute
// is Optional+Computed at the top level with Required string sub-fields,
// matching scrub_task's convention.
func TestRsyncTaskSchema_ScheduleShape(t *testing.T) {
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

// TestRsyncTaskSchema_ExtraIsList verifies "extra" is an Optional+Computed
// list of strings.
func TestRsyncTaskSchema_ExtraIsList(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["extra"]
	if !ok {
		t.Fatal("schema missing 'extra' attribute")
	}
	listAttr, ok := attr.(schema.ListAttribute)
	if !ok {
		t.Fatalf("'extra' attribute is %T, want schema.ListAttribute", attr)
	}
	if !listAttr.IsOptional() || !listAttr.IsComputed() {
		t.Error("'extra' should be Optional+Computed")
	}
}

// TestRsyncTaskSchema_OptionalComputedBoolFlags verifies the rsync bool
// flags carrying TrueNAS-side defaults are all Optional+Computed.
func TestRsyncTaskSchema_OptionalComputedBoolFlags(t *testing.T) {
	s := resourceSchema()

	for _, name := range []string{
		"recursive", "times", "compress", "archive", "delete", "quiet",
		"preserveperm", "preserveattr", "delayupdates", "enabled",
	} {
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
}

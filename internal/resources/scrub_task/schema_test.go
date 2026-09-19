// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package scrub_task

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// TestScrubTaskSchema_PoolRequiresReplace verifies that "pool" is Required
// and carries RequiresReplace, since a scrub task cannot be moved to a
// different pool in place (pool.scrub.update's "pool" is documented as
// changeable, but the provider treats it as immutable to keep the resource
// tied to a single pool for its whole lifecycle).
func TestScrubTaskSchema_PoolRequiresReplace(t *testing.T) {
	s := resourceSchema()

	poolAttr, ok := s.Attributes["pool"]
	if !ok {
		t.Fatal("schema missing 'pool' attribute")
	}
	poolInt, ok := poolAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'pool' attribute is %T, want schema.Int64Attribute", poolAttr)
	}
	if !poolInt.IsRequired() {
		t.Error("'pool' should be Required")
	}
	if len(poolInt.PlanModifiers) == 0 {
		t.Error("'pool' should have plan modifiers (RequiresReplace)")
	}
}

// TestScrubTaskSchema_IDIsComputed verifies "id" is Computed-only.
func TestScrubTaskSchema_IDIsComputed(t *testing.T) {
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

// TestScrubTaskSchema_OptionalComputedFields verifies threshold,
// description, schedule, and enabled are Optional+Computed, matching the
// TrueNAS-side defaults observed in the pool.scrub.create probe.
func TestScrubTaskSchema_OptionalComputedFields(t *testing.T) {
	s := resourceSchema()

	thresholdAttr, ok := s.Attributes["threshold"]
	if !ok {
		t.Fatal("schema missing 'threshold' attribute")
	}
	thresholdInt, ok := thresholdAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'threshold' attribute is %T, want schema.Int64Attribute", thresholdAttr)
	}
	if !thresholdInt.IsOptional() || !thresholdInt.IsComputed() {
		t.Error("'threshold' should be Optional+Computed")
	}

	descAttr, ok := s.Attributes["description"]
	if !ok {
		t.Fatal("schema missing 'description' attribute")
	}
	descStr, ok := descAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'description' attribute is %T, want schema.StringAttribute", descAttr)
	}
	if !descStr.IsOptional() || !descStr.IsComputed() {
		t.Error("'description' should be Optional+Computed")
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

// TestScrubTaskSchema_PoolNameIsComputedOnly verifies "pool_name" is a
// Computed-only extra field surfaced from the pool.scrub.query/get_instance
// response.
func TestScrubTaskSchema_PoolNameIsComputedOnly(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["pool_name"]
	if !ok {
		t.Fatal("schema missing 'pool_name' attribute")
	}
	strAttr, ok := attr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'pool_name' attribute is %T, want schema.StringAttribute", attr)
	}
	if !strAttr.IsComputed() {
		t.Error("'pool_name' should be Computed")
	}
	if strAttr.IsRequired() || strAttr.IsOptional() {
		t.Error("'pool_name' should be Computed-only (not Required/Optional)")
	}
}

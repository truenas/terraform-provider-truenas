// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package catalog_config

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// TestCatalogConfigSchema_IDIsComputed verifies that "id" is a Computed-only
// StringAttribute with UseStateForUnknown, since it's a fixed singleton
// value never supplied by the user.
func TestCatalogConfigSchema_IDIsComputed(t *testing.T) {
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

// TestCatalogConfigSchema_PreferredTrainsIsOptionalComputed verifies
// preferred_trains — the only field catalog.update actually accepts (per
// the probe) — is Optional+Computed with a plan modifier.
func TestCatalogConfigSchema_PreferredTrainsIsOptionalComputed(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["preferred_trains"]
	if !ok {
		t.Fatal("schema missing 'preferred_trains' attribute")
	}
	listAttr, ok := attr.(schema.ListAttribute)
	if !ok {
		t.Fatalf("'preferred_trains' attribute is %T, want schema.ListAttribute", attr)
	}
	if !listAttr.IsOptional() || !listAttr.IsComputed() {
		t.Error("'preferred_trains' should be Optional+Computed")
	}
	if len(listAttr.PlanModifiers) == 0 {
		t.Error("'preferred_trains' should have plan modifiers (UseStateForUnknown)")
	}
}

// TestCatalogConfigSchema_ReadOnlyFieldsAreComputedOnly verifies label and
// location are Computed-only: catalog.update does not accept either
// (probed live — accepts schema is {preferred_trains} only).
func TestCatalogConfigSchema_ReadOnlyFieldsAreComputedOnly(t *testing.T) {
	s := resourceSchema()

	for _, name := range []string{"label", "location"} {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		strAttr, ok := attr.(schema.StringAttribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.StringAttribute", name, attr)
		}
		if !strAttr.IsComputed() || strAttr.IsOptional() || strAttr.IsRequired() {
			t.Errorf("%q should be Computed-only", name)
		}
		if len(strAttr.PlanModifiers) == 0 {
			t.Errorf("%q should have plan modifiers (UseStateForUnknown), to avoid the framework forcing it "+
				"Unknown whenever preferred_trains changes in the same apply", name)
		}
	}
}

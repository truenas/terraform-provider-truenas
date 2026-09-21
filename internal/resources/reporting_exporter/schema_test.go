// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package reporting_exporter

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// TestSchema_TopLevelRequiredFields verifies "name" and "enabled" are
// Required, matching reporting.exporters.create's own "required" list (no
// server-side default for either).
func TestSchema_TopLevelRequiredFields(t *testing.T) {
	s := resourceSchema()

	nameAttr, ok := s.Attributes["name"]
	if !ok {
		t.Fatal("schema missing 'name' attribute")
	}
	strAttr, ok := nameAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'name' attribute is %T, want schema.StringAttribute", nameAttr)
	}
	if !strAttr.IsRequired() {
		t.Error("'name' should be Required")
	}

	enabledAttr, ok := s.Attributes["enabled"]
	if !ok {
		t.Fatal("schema missing 'enabled' attribute")
	}
	boolAttr, ok := enabledAttr.(schema.BoolAttribute)
	if !ok {
		t.Fatalf("'enabled' attribute is %T, want schema.BoolAttribute", enabledAttr)
	}
	if !boolAttr.IsRequired() {
		t.Error("'enabled' should be Required")
	}
}

// TestSchema_IDComputedUseStateForUnknown verifies that the "id" attribute
// is Computed-only.
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
}

// TestSchema_AttributesShape verifies the nested "attributes" block is
// Required at the top level, with destination_ip/destination_port/namespace
// Required and the remaining GRAPHITE fields Optional+Computed, matching the
// probed reporting.exporters.exporter_schemas GRAPHITE variant.
func TestSchema_AttributesShape(t *testing.T) {
	s := resourceSchema()

	attrsAttr, ok := s.Attributes["attributes"]
	if !ok {
		t.Fatal("schema missing 'attributes' attribute")
	}
	nested, ok := attrsAttr.(schema.SingleNestedAttribute)
	if !ok {
		t.Fatalf("'attributes' attribute is %T, want schema.SingleNestedAttribute", attrsAttr)
	}
	if !nested.IsRequired() {
		t.Error("'attributes' should be Required")
	}

	for _, field := range []string{"destination_ip", "destination_port", "namespace"} {
		a, exists := nested.Attributes[field]
		if !exists {
			t.Errorf("attributes missing field %q", field)
			continue
		}
		req, ok := a.(interface{ IsRequired() bool })
		if !ok || !req.IsRequired() {
			t.Errorf("attributes.%s should be Required", field)
		}
	}

	for _, field := range []string{"prefix", "update_every", "buffer_on_failures", "send_names_instead_of_ids", "matching_charts"} {
		a, exists := nested.Attributes[field]
		if !exists {
			t.Errorf("attributes missing field %q", field)
			continue
		}
		type optComp interface {
			IsOptional() bool
			IsComputed() bool
		}
		oc, ok := a.(optComp)
		if !ok || !oc.IsOptional() || !oc.IsComputed() {
			t.Errorf("attributes.%s should be Optional+Computed", field)
		}
	}
}

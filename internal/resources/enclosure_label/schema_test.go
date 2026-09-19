// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package enclosure_label

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// TestEnclosureLabelSchema_IDRequiredForceNew pins the identity contract:
// "id" identifies a pre-existing enclosure (enclosure.label.set takes it as
// a positional argument), so it must be Required and force replacement
// rather than being renamed in place.
func TestEnclosureLabelSchema_IDRequiredForceNew(t *testing.T) {
	s := resourceSchema()
	idAttr, ok := s.Attributes["id"].(schema.StringAttribute)
	if !ok {
		t.Fatalf("'id' attribute is %T, want schema.StringAttribute", s.Attributes["id"])
	}
	if !idAttr.IsRequired() {
		t.Error("'id' should be Required")
	}
	if idAttr.IsComputed() || idAttr.IsOptional() {
		t.Error("'id' should be Required-only (not Computed/Optional)")
	}
	if len(idAttr.PlanModifiers) == 0 {
		t.Error("'id' should have a RequiresReplace plan modifier")
	}
}

func TestEnclosureLabelSchema_LabelRequiredNotComputed(t *testing.T) {
	s := resourceSchema()
	labelAttr, ok := s.Attributes["label"].(schema.StringAttribute)
	if !ok {
		t.Fatalf("'label' attribute is %T, want schema.StringAttribute", s.Attributes["label"])
	}
	if !labelAttr.IsRequired() {
		t.Error("'label' should be Required")
	}
	if labelAttr.IsComputed() || labelAttr.IsOptional() {
		t.Error("'label' should be Required-only (not Computed/Optional): there is no meaningful " +
			"\"leave unmanaged\" state for the one field this resource exists to set")
	}
}

func TestEnclosureLabelSchema_NameComputedOnly(t *testing.T) {
	s := resourceSchema()
	nameAttr, ok := s.Attributes["name"].(schema.StringAttribute)
	if !ok {
		t.Fatalf("'name' attribute is %T, want schema.StringAttribute", s.Attributes["name"])
	}
	if !nameAttr.IsComputed() {
		t.Error("'name' should be Computed")
	}
	if nameAttr.IsRequired() || nameAttr.IsOptional() {
		t.Error("'name' should be Computed-only (not Required/Optional): read-only context, never settable here")
	}
}

func TestEnclosureLabelSchema_NoUnexpectedAttributes(t *testing.T) {
	s := resourceSchema()
	want := map[string]bool{"id": true, "label": true, "name": true}
	if len(s.Attributes) != len(want) {
		t.Errorf("schema has %d attributes, want %d", len(s.Attributes), len(want))
	}
	for name := range s.Attributes {
		if !want[name] {
			t.Errorf("unexpected schema attribute %q", name)
		}
	}
	for name := range want {
		if _, ok := s.Attributes[name]; !ok {
			t.Errorf("schema missing expected attribute %q", name)
		}
	}
}

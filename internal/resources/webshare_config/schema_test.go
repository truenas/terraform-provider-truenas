// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package webshare_config

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

func TestWebshareConfigSchema_IDIsComputed(t *testing.T) {
	s := resourceSchema()
	idAttr, ok := s.Attributes["id"].(schema.StringAttribute)
	if !ok {
		t.Fatalf("'id' attribute is %T, want schema.StringAttribute", s.Attributes["id"])
	}
	if !idAttr.IsComputed() {
		t.Error("'id' should be Computed")
	}
	if idAttr.IsRequired() || idAttr.IsOptional() {
		t.Error("'id' should be Computed-only (not Required/Optional)")
	}
}

func TestWebshareConfigSchema_OptionalComputedFields(t *testing.T) {
	s := resourceSchema()
	for _, name := range []string{"bindip", "search", "passkey", "groups"} {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		switch a := attr.(type) {
		case schema.ListAttribute:
			if !a.IsOptional() || !a.IsComputed() {
				t.Errorf("%q should be Optional+Computed", name)
			}
		case schema.BoolAttribute:
			if !a.IsOptional() || !a.IsComputed() {
				t.Errorf("%q should be Optional+Computed", name)
			}
		case schema.StringAttribute:
			if !a.IsOptional() || !a.IsComputed() {
				t.Errorf("%q should be Optional+Computed", name)
			}
		default:
			t.Errorf("%q has unexpected type %T", name, attr)
		}
	}
}

func TestWebshareConfigSchema_NoUnexpectedAttributes(t *testing.T) {
	s := resourceSchema()
	want := map[string]bool{"id": true, "bindip": true, "search": true, "passkey": true, "groups": true}
	if len(s.Attributes) != len(want) {
		t.Errorf("schema has %d attributes, want %d", len(s.Attributes), len(want))
	}
	for name := range s.Attributes {
		if !want[name] {
			t.Errorf("unexpected schema attribute %q", name)
		}
	}
}

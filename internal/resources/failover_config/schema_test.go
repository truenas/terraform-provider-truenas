// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package failover_config

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

func TestFailoverConfigSchema_IDIsComputed(t *testing.T) {
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

// TestFailoverConfigSchema_WritableFields pins the schema-level half of
// this resource's safety contract: "disabled", "master", and "timeout"
// must be Optional+Computed (user-writable), matching failover.update's
// own accepts schema (probed live: all three are optional on the "data"
// payload, on both TrueNAS 25.10.4 HA and 26.0). "id" is the only
// Computed-only attribute.
func TestFailoverConfigSchema_WritableFields(t *testing.T) {
	s := resourceSchema()
	type optionalComputed interface {
		IsOptional() bool
		IsComputed() bool
	}
	writable := map[string]bool{"disabled": true, "master": true, "timeout": true}
	for name, attr := range s.Attributes {
		if name == "id" {
			continue
		}
		oc, ok := attr.(optionalComputed)
		if !ok {
			t.Fatalf("%q has unexpected type %T", name, attr)
		}
		if writable[name] {
			if !oc.IsOptional() || !oc.IsComputed() {
				t.Errorf("%q should be Optional+Computed", name)
			}
			continue
		}
		t.Errorf("unexpected non-id, non-writable attribute %q", name)
	}
	for name := range writable {
		if _, ok := s.Attributes[name]; !ok {
			t.Errorf("schema missing expected writable attribute %q", name)
		}
	}
}

func TestFailoverConfigSchema_NoUnexpectedAttributes(t *testing.T) {
	s := resourceSchema()
	want := map[string]bool{"id": true, "disabled": true, "master": true, "timeout": true}
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

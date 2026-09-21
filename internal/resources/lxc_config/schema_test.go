// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package lxc_config

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// TestLXCConfigSchema_IDIsComputed verifies that "id" is a Computed-only
// StringAttribute with UseStateForUnknown, since it's a fixed singleton
// value never supplied by the user.
func TestLXCConfigSchema_IDIsComputed(t *testing.T) {
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
}

// TestLXCConfigSchema_AllSettableFieldsAreOptionalComputed verifies every
// field lxc.update actually accepts (per the probe) is Optional+Computed:
// preferred_pool, bridge (nullable three-way), v4_network, v6_network
// (plain guard).
func TestLXCConfigSchema_AllSettableFieldsAreOptionalComputed(t *testing.T) {
	s := resourceSchema()

	for _, name := range []string{"preferred_pool", "bridge", "v4_network", "v6_network"} {
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
	}
}

// TestLXCConfigSchema_NoUnexpectedAttributes verifies the schema exposes
// exactly the 5 attributes probed live (id + the 4 lxc.config/lxc.update
// fields) — nothing from the deprecated incus family (container.*) leaks
// in, since that namespace family is intentionally out of scope for this
// provider.
func TestLXCConfigSchema_NoUnexpectedAttributes(t *testing.T) {
	s := resourceSchema()
	want := map[string]bool{
		"id": true, "preferred_pool": true, "bridge": true,
		"v4_network": true, "v6_network": true,
	}
	if len(s.Attributes) != len(want) {
		t.Errorf("schema has %d attributes, want %d: %v", len(s.Attributes), len(want), s.Attributes)
	}
	for name := range s.Attributes {
		if !want[name] {
			t.Errorf("unexpected schema attribute %q", name)
		}
	}
}

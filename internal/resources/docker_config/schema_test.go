// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package docker_config

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// TestDockerConfigSchema_IDIsComputed verifies that "id" is a Computed-only
// StringAttribute with UseStateForUnknown, since it's a fixed singleton
// value never supplied by the user.
func TestDockerConfigSchema_IDIsComputed(t *testing.T) {
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

// TestDockerConfigSchema_SettableFieldsAreOptionalComputed verifies the
// fields docker.update actually accepts (per the probe) are all
// Optional+Computed.
func TestDockerConfigSchema_SettableFieldsAreOptionalComputed(t *testing.T) {
	s := resourceSchema()

	boolFields := []string{"enable_image_updates", "nvidia"}
	for _, name := range boolFields {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		b, ok := attr.(schema.BoolAttribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.BoolAttribute", name, attr)
		}
		if !b.IsOptional() || !b.IsComputed() {
			t.Errorf("%q should be Optional+Computed", name)
		}
	}

	stringFields := []string{"pool", "cidr_v6"}
	for _, name := range stringFields {
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

	listFields := []string{"address_pools", "registry_mirrors"}
	for _, name := range listFields {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		listAttr, ok := attr.(schema.ListNestedAttribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.ListNestedAttribute", name, attr)
		}
		if !listAttr.IsOptional() || !listAttr.IsComputed() {
			t.Errorf("%q should be Optional+Computed", name)
		}
	}
}

// TestDockerConfigSchema_DatasetIsComputedOnly verifies "dataset" is
// read-only: docker.update does not accept it.
func TestDockerConfigSchema_DatasetIsComputedOnly(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["dataset"]
	if !ok {
		t.Fatal("schema missing 'dataset' attribute")
	}
	strAttr, ok := attr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'dataset' attribute is %T, want schema.StringAttribute", attr)
	}
	if !strAttr.IsComputed() || strAttr.IsOptional() || strAttr.IsRequired() {
		t.Error("'dataset' should be Computed-only")
	}
}

// TestDockerConfigSchema_AddressPoolsNestedFieldsRequired verifies the
// nested address_pools entry has base/size both Required, matching
// docker.update's accepts schema (both required within each entry).
func TestDockerConfigSchema_AddressPoolsNestedFieldsRequired(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["address_pools"].(schema.ListNestedAttribute)
	if !ok {
		t.Fatal("'address_pools' is not a ListNestedAttribute")
	}
	for _, name := range []string{"base", "size"} {
		nested, ok := attr.NestedObject.Attributes[name]
		if !ok {
			t.Fatalf("address_pools nested object missing %q", name)
		}
		type requiredChecker interface{ IsRequired() bool }
		rc, ok := nested.(requiredChecker)
		if !ok || !rc.IsRequired() {
			t.Errorf("address_pools.%s should be Required", name)
		}
	}
}

// TestDockerConfigSchema_MigrateApplicationsExcluded verifies the schema
// never exposes "migrate_applications": it's an apply-time action flag,
// not persisted config (see resourceSchema's Description).
func TestDockerConfigSchema_MigrateApplicationsExcluded(t *testing.T) {
	s := resourceSchema()
	if _, ok := s.Attributes["migrate_applications"]; ok {
		t.Error("schema must not expose 'migrate_applications' (apply-time action flag, excluded by design)")
	}
}

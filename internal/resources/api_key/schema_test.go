// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package api_key

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// TestAPIKeySchema_KeyIsSensitiveAndComputed verifies "key" is both
// Sensitive and Computed-only (never Required/Optional — TrueNAS is the
// sole source of the plaintext value).
func TestAPIKeySchema_KeyIsSensitiveAndComputed(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["key"]
	if !ok {
		t.Fatal("schema missing 'key' attribute")
	}
	strAttr, ok := attr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'key' attribute is %T, want schema.StringAttribute", attr)
	}
	if !strAttr.IsSensitive() {
		t.Error("'key' should be Sensitive")
	}
	if !strAttr.IsComputed() {
		t.Error("'key' should be Computed")
	}
	if strAttr.IsRequired() || strAttr.IsOptional() {
		t.Error("'key' should be Computed-only (not Required/Optional)")
	}
}

// TestAPIKeySchema_UsernameRequiresReplace verifies "username" is Required
// and forces replacement on change (api_key.update rejects changing it).
func TestAPIKeySchema_UsernameRequiresReplace(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["username"]
	if !ok {
		t.Fatal("schema missing 'username' attribute")
	}
	strAttr, ok := attr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'username' attribute is %T, want schema.StringAttribute", attr)
	}
	if !strAttr.IsRequired() {
		t.Error("'username' should be Required")
	}
	if len(strAttr.PlanModifiers) == 0 {
		t.Fatal("'username' should have a RequiresReplace plan modifier")
	}
}

// TestAPIKeySchema_NameRequired verifies "name" is Required.
func TestAPIKeySchema_NameRequired(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["name"]
	if !ok {
		t.Fatal("schema missing 'name' attribute")
	}
	strAttr, ok := attr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'name' attribute is %T, want schema.StringAttribute", attr)
	}
	if !strAttr.IsRequired() {
		t.Error("'name' should be Required")
	}
}

// TestAPIKeySchema_ExpiresAtOptionalComputed verifies "expires_at" is
// Optional+Computed (nullable, user-settable, but also always populated
// from the API).
func TestAPIKeySchema_ExpiresAtOptionalComputed(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["expires_at"]
	if !ok {
		t.Fatal("schema missing 'expires_at' attribute")
	}
	strAttr, ok := attr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'expires_at' attribute is %T, want schema.StringAttribute", attr)
	}
	if !strAttr.IsOptional() || !strAttr.IsComputed() {
		t.Error("'expires_at' should be Optional+Computed")
	}
}

// TestAPIKeySchema_ComputedReadbacks verifies created_at, local, and
// revoked are Computed-only (server-reported, never user-set).
func TestAPIKeySchema_ComputedReadbacks(t *testing.T) {
	s := resourceSchema()

	for _, name := range []string{"created_at"} {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		strAttr, ok := attr.(schema.StringAttribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.StringAttribute", name, attr)
		}
		if !strAttr.IsComputed() || strAttr.IsRequired() || strAttr.IsOptional() {
			t.Errorf("%q should be Computed-only", name)
		}
	}

	for _, name := range []string{"local", "revoked"} {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		boolAttr, ok := attr.(schema.BoolAttribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.BoolAttribute", name, attr)
		}
		if !boolAttr.IsComputed() || boolAttr.IsRequired() || boolAttr.IsOptional() {
			t.Errorf("%q should be Computed-only", name)
		}
	}
}

// TestAPIKeySchema_IDIsComputed verifies "id" is Computed-only.
func TestAPIKeySchema_IDIsComputed(t *testing.T) {
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

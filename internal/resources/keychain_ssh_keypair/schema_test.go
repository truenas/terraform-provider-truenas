// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package keychain_ssh_keypair

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// TestSchema_IDComputedOnly verifies "id" is Computed-only.
func TestSchema_IDComputedOnly(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["id"].(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'id' attribute is %T, want schema.Int64Attribute", s.Attributes["id"])
	}
	if !attr.IsComputed() {
		t.Error("'id' should be Computed")
	}
	if attr.IsRequired() || attr.IsOptional() {
		t.Error("'id' should be Computed-only (not Required/Optional)")
	}
}

// TestSchema_NameRequired verifies "name" is Required (and not
// RequiresReplace — renaming happens in place).
func TestSchema_NameRequired(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["name"].(schema.StringAttribute)
	if !ok {
		t.Fatalf("'name' attribute is %T, want schema.StringAttribute", s.Attributes["name"])
	}
	if !attr.IsRequired() {
		t.Error("'name' should be Required")
	}
	if len(attr.PlanModifiers) != 0 {
		t.Error("'name' should not have plan modifiers — renaming is in-place")
	}
}

// TestSchema_GenerateOptionalComputedRequiresReplace verifies "generate" is
// Optional+Computed (Create resolves an unset value to a concrete
// true/false) and forces replacement on change.
func TestSchema_GenerateOptionalComputedRequiresReplace(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["generate"].(schema.BoolAttribute)
	if !ok {
		t.Fatalf("'generate' attribute is %T, want schema.BoolAttribute", s.Attributes["generate"])
	}
	if !attr.IsOptional() || !attr.IsComputed() {
		t.Error("'generate' should be Optional+Computed")
	}
	if len(attr.PlanModifiers) == 0 {
		t.Fatal("'generate' should have plan modifiers (RequiresReplace)")
	}
}

// TestSchema_PrivateKeySensitiveComputedRequiresReplace verifies
// "private_key" is Sensitive (but not WriteOnly — the probe showed
// keychaincredential.get_instance/query return it intact, unlike
// truenas_certificate's job-result masking), Optional+Computed (either
// user-supplied or server-generated, always read back), and immutable.
func TestSchema_PrivateKeySensitiveComputedRequiresReplace(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["private_key"].(schema.StringAttribute)
	if !ok {
		t.Fatalf("'private_key' attribute is %T, want schema.StringAttribute", s.Attributes["private_key"])
	}
	if !attr.IsSensitive() {
		t.Error("'private_key' should be Sensitive")
	}
	if !attr.IsOptional() || !attr.IsComputed() {
		t.Error("'private_key' should be Optional+Computed")
	}
	if len(attr.PlanModifiers) == 0 {
		t.Fatal("'private_key' should have plan modifiers (RequiresReplace)")
	}
}

// TestSchema_PublicKeyComputedOnly verifies "public_key" is Computed-only —
// probed live, TrueNAS always derives it server-side and this resource
// never accepts it as input.
func TestSchema_PublicKeyComputedOnly(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["public_key"].(schema.StringAttribute)
	if !ok {
		t.Fatalf("'public_key' attribute is %T, want schema.StringAttribute", s.Attributes["public_key"])
	}
	if !attr.IsComputed() {
		t.Error("'public_key' should be Computed")
	}
	if attr.IsRequired() || attr.IsOptional() {
		t.Error("'public_key' should be Computed-only (not Required/Optional)")
	}
	if attr.IsSensitive() {
		t.Error("'public_key' should not be Sensitive — it is public key material")
	}
}

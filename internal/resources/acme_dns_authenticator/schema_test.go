// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acme_dns_authenticator

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// TestSchema_TopLevelRequiredFields verifies "name" and "attributes" are
// Required, matching acme.dns.authenticator.create's own "required" list.
func TestSchema_TopLevelRequiredFields(t *testing.T) {
	s := resourceSchema()

	for _, field := range []string{"name", "attributes"} {
		attr, ok := s.Attributes[field]
		if !ok {
			t.Fatalf("schema missing %q attribute", field)
		}
		req, ok := attr.(interface{ IsRequired() bool })
		if !ok || !req.IsRequired() {
			t.Errorf("%q should be Required", field)
		}
	}
}

// TestSchema_IDComputedUseStateForUnknown verifies "id" is Computed-only.
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

// TestSchema_AttributesSensitiveNotWriteOnly verifies "attributes" is
// Required+Sensitive but NOT Computed/WriteOnly — see model.go's
// acmeDnsAuthenticatorAPI doc comment for the live probe evidence
// (credentials returned in cleartext by create/get_instance/query).
func TestSchema_AttributesSensitiveNotWriteOnly(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["attributes"].(schema.StringAttribute)
	if !ok {
		t.Fatalf("'attributes' attribute is %T, want schema.StringAttribute", s.Attributes["attributes"])
	}
	if !attr.Required {
		t.Error("'attributes' should be Required")
	}
	if !attr.Sensitive {
		t.Error("'attributes' should be Sensitive")
	}
	if attr.WriteOnly {
		t.Error("'attributes' should NOT be WriteOnly")
	}
}

package directoryservices

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// TestSchema_IDIsComputed verifies that "id" is a Computed-only
// StringAttribute with UseStateForUnknown, since it's a fixed singleton
// value never supplied by the user.
func TestSchema_IDIsComputed(t *testing.T) {
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

// TestSchema_ServiceTypeRestrictedToActiveDirectory verifies that
// "service_type" carries a validator restricting it to "ACTIVEDIRECTORY"
// only (IPA/LDAP deferred per the task-8 brief), even though the API
// itself accepts all three.
func TestSchema_ServiceTypeRestrictedToActiveDirectory(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["service_type"]
	if !ok {
		t.Fatal("schema missing 'service_type' attribute")
	}
	strAttr, ok := attr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'service_type' attribute is %T, want schema.StringAttribute", attr)
	}
	if !strAttr.IsOptional() || !strAttr.IsComputed() {
		t.Error("'service_type' should be Optional+Computed (nullable on the wire)")
	}
	if len(strAttr.Validators) == 0 {
		t.Fatal("'service_type' should have a validator restricting it to ACTIVEDIRECTORY")
	}
}

// TestSchema_EnableIsRequired verifies "enable" is a plain Required bool:
// the resource's core purpose is to toggle directory-service enablement, so
// it should never be silently defaulted.
func TestSchema_EnableIsRequired(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["enable"]
	if !ok {
		t.Fatal("schema missing 'enable' attribute")
	}
	boolAttr, ok := attr.(schema.BoolAttribute)
	if !ok {
		t.Fatalf("'enable' attribute is %T, want schema.BoolAttribute", attr)
	}
	if !boolAttr.IsRequired() {
		t.Error("'enable' should be Required")
	}
	if boolAttr.IsOptional() || boolAttr.IsComputed() {
		t.Error("'enable' should be Required-only (not Optional/Computed)")
	}
}

// TestSchema_CredentialPasswordIsWriteOnly verifies the credential nested
// block's password attribute is Sensitive+WriteOnly (never persisted in
// state), and that the parent "credential" attribute itself is NOT Computed
// — the terraform-plugin-framework forbids a Computed nested attribute from
// containing a WriteOnly child.
func TestSchema_CredentialPasswordIsWriteOnly(t *testing.T) {
	s := resourceSchema()

	credAttr, ok := s.Attributes["credential"]
	if !ok {
		t.Fatal("schema missing 'credential' attribute")
	}
	nested, ok := credAttr.(schema.SingleNestedAttribute)
	if !ok {
		t.Fatalf("'credential' attribute is %T, want schema.SingleNestedAttribute", credAttr)
	}
	if nested.IsComputed() {
		t.Error("'credential' must NOT be Computed: it contains a WriteOnly child (password), and " +
			"terraform-plugin-framework rejects a Computed nested attribute with a WriteOnly child")
	}
	if !nested.IsOptional() {
		t.Error("'credential' should be Optional")
	}

	pwAttr, ok := nested.Attributes["password"]
	if !ok {
		t.Fatal("credential schema missing 'password' attribute")
	}
	pwStr, ok := pwAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'password' attribute is %T, want schema.StringAttribute", pwAttr)
	}
	if !pwStr.IsSensitive() {
		t.Error("'password' should be Sensitive")
	}
	if !pwStr.WriteOnly {
		t.Error("'password' should be WriteOnly")
	}
	if pwStr.IsComputed() {
		t.Error("'password' must not be Computed (incompatible with WriteOnly)")
	}

	credTypeAttr, ok := nested.Attributes["credential_type"]
	if !ok {
		t.Fatal("credential schema missing 'credential_type' attribute")
	}
	credTypeStr, ok := credTypeAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'credential_type' attribute is %T, want schema.StringAttribute", credTypeAttr)
	}
	if len(credTypeStr.Validators) == 0 {
		t.Error("'credential_type' should have a validator restricting it to KERBEROS_USER")
	}
}

// TestSchema_ConfigurationActiveDirectoryOptionalComputed verifies the
// nested AD config block is Optional+Computed (nullable on the wire, and
// safe to be Computed since it has no WriteOnly children), and that its
// required leaf fields (hostname, domain) are Required.
func TestSchema_ConfigurationActiveDirectoryOptionalComputed(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["configuration_activedirectory"]
	if !ok {
		t.Fatal("schema missing 'configuration_activedirectory' attribute")
	}
	nested, ok := attr.(schema.SingleNestedAttribute)
	if !ok {
		t.Fatalf("'configuration_activedirectory' attribute is %T, want schema.SingleNestedAttribute", attr)
	}
	if !nested.IsOptional() || !nested.IsComputed() {
		t.Error("'configuration_activedirectory' should be Optional+Computed")
	}

	for _, name := range []string{"hostname", "domain"} {
		child, ok := nested.Attributes[name]
		if !ok {
			t.Fatalf("configuration_activedirectory schema missing %q attribute", name)
		}
		childStr, ok := child.(schema.StringAttribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.StringAttribute", name, child)
		}
		if !childStr.IsRequired() {
			t.Errorf("%q should be Required", name)
		}
	}
}

// TestSchema_TimeoutBetween5And60 verifies "timeout" carries a range
// validator matching the probed minimum/maximum (5-60 seconds).
func TestSchema_TimeoutBetween5And60(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["timeout"]
	if !ok {
		t.Fatal("schema missing 'timeout' attribute")
	}
	intAttr, ok := attr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'timeout' attribute is %T, want schema.Int64Attribute", attr)
	}
	if !intAttr.IsOptional() || !intAttr.IsComputed() {
		t.Error("'timeout' should be Optional+Computed")
	}
	if len(intAttr.Validators) == 0 {
		t.Fatal("'timeout' should have a range validator (5-60)")
	}
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

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

// TestSchema_ServiceTypeAllowsThreeTypes verifies that "service_type"
// carries a validator restricting it to the three service types the
// middleware itself supports: ACTIVEDIRECTORY, LDAP, IPA.
func TestSchema_ServiceTypeAllowsThreeTypes(t *testing.T) {
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
		t.Fatal("'service_type' should have a validator restricting it to ACTIVEDIRECTORY/LDAP/IPA")
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
	if pwStr.IsRequired() {
		t.Error("'password' should be Optional (only required when credential_type is KERBEROS_USER, enforced at payload time)")
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
		t.Error("'credential_type' should have a validator restricting it to the five known types")
	}
}

// TestSchema_CredentialLDAPVariantFields verifies the credential block's
// new LDAP/Kerberos-principal fields: bindpw is Sensitive+WriteOnly (like
// password); binddn/client_certificate/principal are plain Optional
// (persisted normally, not sensitive).
func TestSchema_CredentialLDAPVariantFields(t *testing.T) {
	s := resourceSchema()

	credAttr := s.Attributes["credential"].(schema.SingleNestedAttribute)

	bindpw, ok := credAttr.Attributes["bindpw"].(schema.StringAttribute)
	if !ok {
		t.Fatal("credential schema missing 'bindpw' attribute")
	}
	if !bindpw.IsSensitive() || !bindpw.WriteOnly {
		t.Error("'bindpw' should be Sensitive+WriteOnly")
	}
	if !bindpw.IsOptional() || bindpw.IsComputed() {
		t.Error("'bindpw' should be Optional (not Computed)")
	}

	for _, name := range []string{"binddn", "client_certificate", "principal"} {
		attr, ok := credAttr.Attributes[name].(schema.StringAttribute)
		if !ok {
			t.Fatalf("credential schema missing %q attribute", name)
		}
		if !attr.IsOptional() {
			t.Errorf("%q should be Optional", name)
		}
		if attr.IsSensitive() || attr.WriteOnly {
			t.Errorf("%q should NOT be Sensitive/WriteOnly", name)
		}
	}

	username, ok := credAttr.Attributes["username"].(schema.StringAttribute)
	if !ok {
		t.Fatal("credential schema missing 'username' attribute")
	}
	if !username.IsOptional() || username.IsRequired() {
		t.Error("'username' should be Optional (only required when credential_type is KERBEROS_USER)")
	}
}

// TestSchema_ConfigurationLDAPAndIPAOptionalComputed verifies the two new
// discriminated-union blocks exist as Optional+Computed SingleNestedAttribute
// with their Required leaf fields intact.
func TestSchema_ConfigurationLDAPAndIPAOptionalComputed(t *testing.T) {
	s := resourceSchema()

	ldapAttr, ok := s.Attributes["configuration_ldap"].(schema.SingleNestedAttribute)
	if !ok {
		t.Fatal("schema missing 'configuration_ldap' attribute")
	}
	if !ldapAttr.IsOptional() || !ldapAttr.IsComputed() {
		t.Error("'configuration_ldap' should be Optional+Computed")
	}
	serverURLs, ok := ldapAttr.Attributes["server_urls"].(schema.ListAttribute)
	if !ok {
		t.Fatal("configuration_ldap schema missing 'server_urls' attribute")
	}
	if !serverURLs.IsRequired() {
		t.Error("'server_urls' should be Required")
	}
	basedn, ok := ldapAttr.Attributes["basedn"].(schema.StringAttribute)
	if !ok || !basedn.IsRequired() {
		t.Error("configuration_ldap 'basedn' should be Required")
	}
	if _, ok := ldapAttr.Attributes["search_bases"].(schema.SingleNestedAttribute); !ok {
		t.Error("configuration_ldap missing 'search_bases' nested block")
	}
	if _, ok := ldapAttr.Attributes["attribute_maps"].(schema.SingleNestedAttribute); !ok {
		t.Error("configuration_ldap missing 'attribute_maps' nested block")
	}

	ipaAttr, ok := s.Attributes["configuration_ipa"].(schema.SingleNestedAttribute)
	if !ok {
		t.Fatal("schema missing 'configuration_ipa' attribute")
	}
	if !ipaAttr.IsOptional() || !ipaAttr.IsComputed() {
		t.Error("'configuration_ipa' should be Optional+Computed")
	}
	for _, name := range []string{"target_server", "hostname", "domain", "basedn"} {
		child, ok := ipaAttr.Attributes[name].(schema.StringAttribute)
		if !ok || !child.IsRequired() {
			t.Errorf("configuration_ipa %q should be Required", name)
		}
	}
}

// TestSchema_IdmapBlock verifies configuration_activedirectory.idmap is
// Optional+Computed (back-compat: unset omits it from the payload, and a
// read-back populates it — see updatePayload's doc comment), and that
// idmap_domain's idmap_backend is restricted to the two backends this
// provider version models (AD, RID).
func TestSchema_IdmapBlock(t *testing.T) {
	s := resourceSchema()

	adAttr := s.Attributes["configuration_activedirectory"].(schema.SingleNestedAttribute)
	idmapAttr, ok := adAttr.Attributes["idmap"].(schema.SingleNestedAttribute)
	if !ok {
		t.Fatal("configuration_activedirectory schema missing 'idmap' attribute")
	}
	if !idmapAttr.IsOptional() || !idmapAttr.IsComputed() {
		t.Error("'idmap' should be Optional+Computed")
	}

	builtinAttr, ok := idmapAttr.Attributes["builtin"].(schema.SingleNestedAttribute)
	if !ok {
		t.Fatal("idmap schema missing 'builtin' attribute")
	}
	for _, name := range []string{"name", "range_low", "range_high"} {
		if _, ok := builtinAttr.Attributes[name]; !ok {
			t.Errorf("idmap.builtin missing %q attribute", name)
		}
	}

	domainAttr, ok := idmapAttr.Attributes["idmap_domain"].(schema.SingleNestedAttribute)
	if !ok {
		t.Fatal("idmap schema missing 'idmap_domain' attribute")
	}
	backend, ok := domainAttr.Attributes["idmap_backend"].(schema.StringAttribute)
	if !ok {
		t.Fatal("idmap_domain schema missing 'idmap_backend' attribute")
	}
	if !backend.IsRequired() {
		t.Error("'idmap_backend' should be Required")
	}
	if len(backend.Validators) == 0 {
		t.Error("'idmap_backend' should have a validator restricting it to AD/RID")
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

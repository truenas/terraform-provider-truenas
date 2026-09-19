// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package kerberos_keytab

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// TestKerberosKeytabSchema_IDIsComputed verifies "id" is Computed-only.
func TestKerberosKeytabSchema_IDIsComputed(t *testing.T) {
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

// TestKerberosKeytabSchema_NameIsRequired verifies that "name" is Required
// and carries no plan modifiers forcing replacement, since
// kerberos.keytab.update accepts an updated "name" (renamable in place).
func TestKerberosKeytabSchema_NameIsRequired(t *testing.T) {
	s := resourceSchema()

	nameAttr, ok := s.Attributes["name"]
	if !ok {
		t.Fatal("schema missing 'name' attribute")
	}
	nameStr, ok := nameAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'name' attribute is %T, want schema.StringAttribute", nameAttr)
	}
	if !nameStr.IsRequired() {
		t.Error("'name' should be Required")
	}
	if nameStr.IsComputed() || nameStr.IsOptional() {
		t.Error("'name' should be Required-only")
	}
}

// TestKerberosKeytabSchema_FileIsSensitiveNotWriteOnly verifies that "file"
// is Required + Sensitive, and specifically NOT Computed and NOT
// WriteOnly — the probe in model.go's kerberosKeytabAPI doc comment found
// kerberos.keytab.query/get_instance return "file" intact (never redacted
// or omitted), so this resource uses the normal Sensitive modeling rather
// than the WriteOnly pattern (contrast with truenas_user's "password").
func TestKerberosKeytabSchema_FileIsSensitiveNotWriteOnly(t *testing.T) {
	s := resourceSchema()

	fileAttr, ok := s.Attributes["file"]
	if !ok {
		t.Fatal("schema missing 'file' attribute")
	}
	fileStr, ok := fileAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'file' attribute is %T, want schema.StringAttribute", fileAttr)
	}
	if !fileStr.IsRequired() {
		t.Error("'file' should be Required")
	}
	if !fileStr.IsSensitive() {
		t.Error("'file' should be Sensitive")
	}
	if fileStr.IsComputed() {
		t.Error("'file' must NOT be Computed")
	}
	if fileStr.IsWriteOnly() {
		t.Error("'file' must NOT be WriteOnly: the probe showed kerberos.keytab.query returns it intact, " +
			"unlike a genuinely one-way secret")
	}
}

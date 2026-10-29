// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package kerberos_realm

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// TestKerberosRealmSchema_IDIsComputed verifies "id" is Computed-only.
func TestKerberosRealmSchema_IDIsComputed(t *testing.T) {
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

// TestKerberosRealmSchema_RealmIsRequired verifies that "realm" is Required
// and carries no plan modifiers forcing replacement, since
// kerberos.realm.update accepts an updated "realm" value (renamable in
// place).
func TestKerberosRealmSchema_RealmIsRequired(t *testing.T) {
	s := resourceSchema()

	realmAttr, ok := s.Attributes["realm"]
	if !ok {
		t.Fatal("schema missing 'realm' attribute")
	}
	realmStr, ok := realmAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'realm' attribute is %T, want schema.StringAttribute", realmAttr)
	}
	if !realmStr.IsRequired() {
		t.Error("'realm' should be Required")
	}
	if realmStr.IsComputed() || realmStr.IsOptional() {
		t.Error("'realm' should be Required-only")
	}
}

// TestKerberosRealmSchema_OptionalComputedFields verifies primary_kdc is
// Optional only (removing it from config must plan null so it is cleared,
// B-112), and kdc, admin_server, and kpasswd_server are Optional+Computed with
// UseStateForUnknown plan modifiers, matching the TrueNAS-side defaults
// observed in the kerberos.realm.create probe.
func TestKerberosRealmSchema_OptionalComputedFields(t *testing.T) {
	s := resourceSchema()

	primaryKDCAttr, ok := s.Attributes["primary_kdc"]
	if !ok {
		t.Fatal("schema missing 'primary_kdc' attribute")
	}
	if !primaryKDCAttr.IsOptional() || primaryKDCAttr.IsComputed() {
		t.Error("'primary_kdc' should be Optional and not Computed")
	}

	for _, name := range []string{"kdc", "admin_server", "kpasswd_server"} {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		listAttr, ok := attr.(schema.ListAttribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.ListAttribute", name, attr)
		}
		if !listAttr.IsOptional() || !listAttr.IsComputed() {
			t.Errorf("%q should be Optional+Computed", name)
		}
		if len(listAttr.PlanModifiers) == 0 {
			t.Errorf("%q should have plan modifiers (UseStateForUnknown)", name)
		}
	}
}

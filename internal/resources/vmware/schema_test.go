// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vmware

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// TestVMwareSchema_RequiredFields verifies "datastore", "filesystem",
// "hostname", "username", "password" are Required, matching vmware.create's
// own "required" list (probed via core.get_methods, identical on both
// probed releases).
func TestVMwareSchema_RequiredFields(t *testing.T) {
	s := resourceSchema()

	for _, name := range []string{"datastore", "filesystem", "hostname", "username", "password"} {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		strAttr, ok := attr.(schema.StringAttribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.StringAttribute", name, attr)
		}
		if !strAttr.IsRequired() {
			t.Errorf("%q should be Required", name)
		}
	}
}

// TestVMwareSchema_IDIsComputed verifies "id" is Computed-only.
func TestVMwareSchema_IDIsComputed(t *testing.T) {
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

// TestVMwareSchema_PasswordIsSensitiveWriteOnly verifies "password" is
// marked both Sensitive and WriteOnly: no live vmware entry was ever
// obtainable to observe a read-back value (vmware.create validates against
// the real endpoint before persisting anything — see schema.go's
// description), so this mirrors truenas_app_registry's identically-situated
// "password" rather than truenas_cloud_backup's (which DOES have live
// read-back evidence).
func TestVMwareSchema_PasswordIsSensitiveWriteOnly(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["password"]
	if !ok {
		t.Fatal("schema missing 'password' attribute")
	}
	strAttr, ok := attr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'password' attribute is %T, want schema.StringAttribute", attr)
	}
	if !strAttr.IsSensitive() {
		t.Error("'password' should be Sensitive")
	}
	if !strAttr.IsWriteOnly() {
		t.Error("'password' should be WriteOnly: no live read-back evidence exists (create validates against the real endpoint before persisting)")
	}
	if !strAttr.IsRequired() {
		t.Error("'password' should be Required (vmware.create requires it)")
	}
}

// TestVMwareSchema_StateIsNestedComputed verifies the nested "state"
// attribute is Computed-only with its three documented sub-fields.
func TestVMwareSchema_StateIsNestedComputed(t *testing.T) {
	s := resourceSchema()

	stateAttr, ok := s.Attributes["state"]
	if !ok {
		t.Fatal("schema missing 'state' attribute")
	}
	nested, ok := stateAttr.(schema.SingleNestedAttribute)
	if !ok {
		t.Fatalf("'state' attribute is %T, want schema.SingleNestedAttribute", stateAttr)
	}
	if !nested.IsComputed() {
		t.Error("'state' should be Computed")
	}
	if nested.IsRequired() || nested.IsOptional() {
		t.Error("'state' should be Computed-only (not Required/Optional)")
	}
	for _, field := range []string{"state", "error", "datetime"} {
		a, exists := nested.Attributes[field]
		if !exists {
			t.Errorf("state missing field %q", field)
			continue
		}
		sa, ok := a.(schema.StringAttribute)
		if !ok {
			t.Errorf("state.%s is %T, want schema.StringAttribute", field, a)
			continue
		}
		if !sa.IsComputed() {
			t.Errorf("state.%s should be Computed", field)
		}
	}
}

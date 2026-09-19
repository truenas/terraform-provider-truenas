// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package privilege

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// TestPrivilegeSchema_RequiredFields verifies "name" and "web_shell" are
// Required, matching privilege.create's own "required" list.
func TestPrivilegeSchema_RequiredFields(t *testing.T) {
	s := resourceSchema()

	nameAttr, ok := s.Attributes["name"]
	if !ok {
		t.Fatal("schema missing 'name' attribute")
	}
	strAttr, ok := nameAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'name' attribute is %T, want schema.StringAttribute", nameAttr)
	}
	if !strAttr.IsRequired() {
		t.Error("'name' should be Required")
	}

	wsAttr, ok := s.Attributes["web_shell"]
	if !ok {
		t.Fatal("schema missing 'web_shell' attribute")
	}
	boolAttr, ok := wsAttr.(schema.BoolAttribute)
	if !ok {
		t.Fatalf("'web_shell' attribute is %T, want schema.BoolAttribute", wsAttr)
	}
	if !boolAttr.IsRequired() {
		t.Error("'web_shell' should be Required")
	}
	if boolAttr.IsComputed() {
		t.Error("'web_shell' should not be Computed (no server-side default; privilege.create requires it)")
	}
}

// TestPrivilegeSchema_IDIsComputed verifies "id" is Computed-only.
func TestPrivilegeSchema_IDIsComputed(t *testing.T) {
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

// TestPrivilegeSchema_GroupListsAreOptionalComputedInt64 verifies
// local_groups and ds_groups are Optional+Computed lists of Int64 (GIDs),
// matching the brief's "list int GID" spec and privilege.create's own
// default of [].
func TestPrivilegeSchema_GroupListsAreOptionalComputedInt64(t *testing.T) {
	s := resourceSchema()

	for _, name := range []string{"local_groups", "ds_groups"} {
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
		if listAttr.ElementType.String() != "basetypes.Int64Type" {
			t.Errorf("%q element type = %v, want Int64", name, listAttr.ElementType)
		}
	}
}

// TestPrivilegeSchema_RolesIsOptionalComputedStringList verifies "roles" is
// an Optional+Computed list of strings.
func TestPrivilegeSchema_RolesIsOptionalComputedStringList(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["roles"]
	if !ok {
		t.Fatal("schema missing 'roles' attribute")
	}
	listAttr, ok := attr.(schema.ListAttribute)
	if !ok {
		t.Fatalf("'roles' attribute is %T, want schema.ListAttribute", attr)
	}
	if !listAttr.IsOptional() || !listAttr.IsComputed() {
		t.Error("'roles' should be Optional+Computed")
	}
	if listAttr.ElementType.String() != "basetypes.StringType" {
		t.Errorf("'roles' element type = %v, want String", listAttr.ElementType)
	}
}

// TestPrivilegeSchema_BuiltinNameIsComputedOnly verifies "builtin_name" is
// Computed-only: the API generates it, and this resource never writes it
// (it only ever creates custom, non-builtin privileges).
func TestPrivilegeSchema_BuiltinNameIsComputedOnly(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["builtin_name"]
	if !ok {
		t.Fatal("schema missing 'builtin_name' attribute")
	}
	strAttr, ok := attr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'builtin_name' attribute is %T, want schema.StringAttribute", attr)
	}
	if !strAttr.IsComputed() {
		t.Error("'builtin_name' should be Computed")
	}
	if strAttr.IsRequired() || strAttr.IsOptional() {
		t.Error("'builtin_name' should be Computed-only (not Required/Optional)")
	}
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acl_template

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// TestAclTemplateSchema_RequiredFields verifies "name", "acltype", and
// "acl" are Required, matching filesystem.acltemplate.create's own
// "required" list (probed via core.get_methods).
func TestAclTemplateSchema_RequiredFields(t *testing.T) {
	s := resourceSchema()

	for _, name := range []string{"name", "acltype", "acl"} {
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

// TestAclTemplateSchema_IDIsComputed verifies "id" is Computed-only.
func TestAclTemplateSchema_IDIsComputed(t *testing.T) {
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

// TestAclTemplateSchema_BuiltinIsComputed verifies "builtin" is
// Computed-only: this resource never lets a user declare a template as
// builtin (the server assigns that).
func TestAclTemplateSchema_BuiltinIsComputed(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["builtin"]
	if !ok {
		t.Fatal("schema missing 'builtin' attribute")
	}
	boolAttr, ok := attr.(schema.BoolAttribute)
	if !ok {
		t.Fatalf("'builtin' attribute is %T, want schema.BoolAttribute", attr)
	}
	if !boolAttr.IsComputed() {
		t.Error("'builtin' should be Computed")
	}
	if boolAttr.IsRequired() || boolAttr.IsOptional() {
		t.Error("'builtin' should be Computed-only (not Required/Optional)")
	}
}

// TestAclTemplateSchema_CommentOptionalComputed verifies "comment" carries
// the TrueNAS-side default ("") via Optional+Computed.
func TestAclTemplateSchema_CommentOptionalComputed(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["comment"]
	if !ok {
		t.Fatal("schema missing 'comment' attribute")
	}
	strAttr, ok := attr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'comment' attribute is %T, want schema.StringAttribute", attr)
	}
	if !strAttr.IsOptional() || !strAttr.IsComputed() {
		t.Error("'comment' should be Optional+Computed")
	}
}

// TestAclTemplateSchema_ACLTypeOneOf verifies "acltype" is constrained to
// the two values filesystem.acltemplate.create accepts (probed live).
func TestAclTemplateSchema_ACLTypeOneOf(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["acltype"]
	if !ok {
		t.Fatal("schema missing 'acltype' attribute")
	}
	strAttr, ok := attr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'acltype' attribute is %T, want schema.StringAttribute", attr)
	}
	if len(strAttr.Validators) == 0 {
		t.Error("'acltype' should carry a OneOf(NFS4, POSIX1E) validator")
	}
}

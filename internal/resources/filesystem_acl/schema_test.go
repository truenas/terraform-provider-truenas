// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package filesystem_acl

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// TestFilesystemAclSchema_PathRequiredForceNew verifies "path" is Required
// and forces resource replacement on change, matching truenas_filesystem_
// permissions' own path attribute.
func TestFilesystemAclSchema_PathRequiredForceNew(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["path"]
	if !ok {
		t.Fatal("schema missing 'path' attribute")
	}
	strAttr, ok := attr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'path' attribute is %T, want schema.StringAttribute", attr)
	}
	if !strAttr.IsRequired() {
		t.Error("'path' should be Required")
	}
	if len(strAttr.PlanModifiers) == 0 {
		t.Error("'path' should carry a RequiresReplace plan modifier")
	}
}

// TestFilesystemAclSchema_EntriesRequired verifies "entries" is Required
// (matching truenas_acl_template's "acl" field precedent).
func TestFilesystemAclSchema_EntriesRequired(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["entries"]
	if !ok {
		t.Fatal("schema missing 'entries' attribute")
	}
	strAttr, ok := attr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'entries' attribute is %T, want schema.StringAttribute", attr)
	}
	if !strAttr.IsRequired() {
		t.Error("'entries' should be Required")
	}
}

// TestFilesystemAclSchema_ACLTypeIsComputed verifies "acltype" is
// Computed-only: this resource never sets it, only exposes what
// filesystem.getacl reports.
func TestFilesystemAclSchema_ACLTypeIsComputed(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["acltype"]
	if !ok {
		t.Fatal("schema missing 'acltype' attribute")
	}
	strAttr, ok := attr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'acltype' attribute is %T, want schema.StringAttribute", attr)
	}
	if !strAttr.IsComputed() {
		t.Error("'acltype' should be Computed")
	}
	if strAttr.IsRequired() || strAttr.IsOptional() {
		t.Error("'acltype' should be Computed-only (not Required/Optional)")
	}
}

// TestFilesystemAclSchema_IDIsComputed verifies "id" is Computed-only.
func TestFilesystemAclSchema_IDIsComputed(t *testing.T) {
	s := resourceSchema()

	idAttr, ok := s.Attributes["id"]
	if !ok {
		t.Fatal("schema missing 'id' attribute")
	}
	strAttr, ok := idAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'id' attribute is %T, want schema.StringAttribute", idAttr)
	}
	if !strAttr.IsComputed() {
		t.Error("'id' should be Computed")
	}
	if strAttr.IsRequired() || strAttr.IsOptional() {
		t.Error("'id' should be Computed-only (not Required/Optional)")
	}
}

// TestFilesystemAclSchema_UIDGIDOptionalComputed verifies "uid"/"gid" carry
// the setperm-style "leave unchanged when omitted, always read back"
// convention (Optional+Computed), matching truenas_filesystem_permissions.
func TestFilesystemAclSchema_UIDGIDOptionalComputed(t *testing.T) {
	s := resourceSchema()

	for _, name := range []string{"uid", "gid"} {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		intAttr, ok := attr.(schema.Int64Attribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.Int64Attribute", name, attr)
		}
		if !intAttr.IsOptional() || !intAttr.IsComputed() {
			t.Errorf("%q should be Optional+Computed", name)
		}
	}
}

// TestFilesystemAclSchema_RecursiveTraverseOptionalOnly verifies
// "recursive"/"traverse" are apply-time-only (Optional, never Computed),
// matching truenas_filesystem_permissions' own options.
func TestFilesystemAclSchema_RecursiveTraverseOptionalOnly(t *testing.T) {
	s := resourceSchema()

	for _, name := range []string{"recursive", "traverse"} {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		boolAttr, ok := attr.(schema.BoolAttribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.BoolAttribute", name, attr)
		}
		if !boolAttr.IsOptional() {
			t.Errorf("%q should be Optional", name)
		}
		if boolAttr.IsComputed() {
			t.Errorf("%q should not be Computed (apply-time-only, nothing to read back)", name)
		}
	}
}

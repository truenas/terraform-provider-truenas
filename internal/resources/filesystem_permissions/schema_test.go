// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package filesystem_permissions

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// TestSchema_PathIsRequiredForceNew verifies "path" is Required and forces
// replacement on change (this resource is keyed by path).
func TestSchema_PathIsRequiredForceNew(t *testing.T) {
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

// TestSchema_IDIsComputed verifies "id" is Computed-only.
func TestSchema_IDIsComputed(t *testing.T) {
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

// TestSchema_ModeUidGidOptionalComputed verifies mode/uid/gid are
// Optional+Computed: settable, but always read back from filesystem.stat
// (drift-checked on every refresh).
func TestSchema_ModeUidGidOptionalComputed(t *testing.T) {
	s := resourceSchema()

	modeAttr, ok := s.Attributes["mode"].(schema.StringAttribute)
	if !ok {
		t.Fatal("schema missing 'mode' string attribute")
	}
	if !modeAttr.IsOptional() || !modeAttr.IsComputed() {
		t.Error("'mode' should be Optional+Computed")
	}

	for _, name := range []string{"uid", "gid"} {
		attr, ok := s.Attributes[name].(schema.Int64Attribute)
		if !ok {
			t.Fatalf("schema missing %q int64 attribute", name)
		}
		if !attr.IsOptional() || !attr.IsComputed() {
			t.Errorf("%q should be Optional+Computed", name)
		}
	}
}

// TestSchema_RecursiveTraverseAreNotComputed verifies "recursive" and
// "traverse" are plain Optional (never echoed back into state, per the
// brief and the resource's Read implementation).
func TestSchema_RecursiveTraverseAreNotComputed(t *testing.T) {
	s := resourceSchema()

	for _, name := range []string{"recursive", "traverse"} {
		attr, ok := s.Attributes[name].(schema.BoolAttribute)
		if !ok {
			t.Fatalf("schema missing %q bool attribute", name)
		}
		if !attr.IsOptional() {
			t.Errorf("%q should be Optional", name)
		}
		if attr.IsComputed() {
			t.Errorf("%q should NOT be Computed (apply-time-only option, not echoed)", name)
		}
	}
}

// TestModeRegexp verifies the validator regexp accepts 3-4 octal digits
// and rejects everything else.
func TestModeRegexp(t *testing.T) {
	re := modeRegexp()
	valid := []string{"750", "0750", "0755", "1777", "0000"}
	invalid := []string{"", "08", "abcd", "07500", "75", "0-750"}

	for _, v := range valid {
		if !re.MatchString(v) {
			t.Errorf("modeRegexp() should accept %q", v)
		}
	}
	for _, v := range invalid {
		if re.MatchString(v) {
			t.Errorf("modeRegexp() should reject %q", v)
		}
	}
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package webshare

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

func TestWebshareSchema_IDIsComputed(t *testing.T) {
	s := resourceSchema()
	idAttr, ok := s.Attributes["id"].(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'id' attribute is %T, want schema.Int64Attribute", s.Attributes["id"])
	}
	if !idAttr.IsComputed() {
		t.Error("'id' should be Computed")
	}
	if idAttr.IsRequired() || idAttr.IsOptional() {
		t.Error("'id' should be Computed-only (not Required/Optional)")
	}
}

func TestWebshareSchema_NameIsRequiredAndMutable(t *testing.T) {
	// "name" must be Required but NOT carry a RequiresReplace plan modifier:
	// sharing.webshare.update accepts and applies a rename (probed live).
	s := resourceSchema()
	nameAttr, ok := s.Attributes["name"].(schema.StringAttribute)
	if !ok {
		t.Fatalf("'name' attribute is %T, want schema.StringAttribute", s.Attributes["name"])
	}
	if !nameAttr.IsRequired() {
		t.Error("'name' should be Required")
	}
	if len(nameAttr.PlanModifiers) != 0 {
		t.Errorf("'name' should have no plan modifiers (mutable via sharing.webshare.update), got %d", len(nameAttr.PlanModifiers))
	}
}

func TestWebshareSchema_PathIsRequiredAndForceNew(t *testing.T) {
	s := resourceSchema()
	pathAttr, ok := s.Attributes["path"].(schema.StringAttribute)
	if !ok {
		t.Fatalf("'path' attribute is %T, want schema.StringAttribute", s.Attributes["path"])
	}
	if !pathAttr.IsRequired() {
		t.Error("'path' should be Required")
	}
	if len(pathAttr.PlanModifiers) == 0 {
		t.Error("'path' should carry a RequiresReplace plan modifier (this provider's share-path convention)")
	}
}

func TestWebshareSchema_EnabledAndIsHomeBaseHaveDefaults(t *testing.T) {
	s := resourceSchema()
	for _, name := range []string{"enabled", "is_home_base"} {
		attr, ok := s.Attributes[name].(schema.BoolAttribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.BoolAttribute", name, s.Attributes[name])
		}
		if !attr.IsOptional() || !attr.IsComputed() {
			t.Errorf("%q should be Optional+Computed", name)
		}
		if attr.Default == nil {
			t.Errorf("%q should carry a Default (matches the API's own probed default)", name)
		}
	}
}

func TestWebshareSchema_NoCommentAttribute(t *testing.T) {
	// Unlike truenas_smb_share/truenas_nfs_share, sharing.webshare has no
	// "comment" field at all (probed live).
	s := resourceSchema()
	if _, present := s.Attributes["comment"]; present {
		t.Error(`schema should not have a "comment" attribute (sharing.webshare has none, probed live)`)
	}
}

func TestWebshareSchema_ComputedOnlyFields(t *testing.T) {
	s := resourceSchema()
	for _, name := range []string{"dataset", "relative_path"} {
		attr, ok := s.Attributes[name].(schema.StringAttribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.StringAttribute", name, s.Attributes[name])
		}
		if !attr.IsComputed() {
			t.Errorf("%q should be Computed", name)
		}
		if attr.IsRequired() || attr.IsOptional() {
			t.Errorf("%q should be Computed-only", name)
		}
	}
	lockedAttr, ok := s.Attributes["locked"].(schema.BoolAttribute)
	if !ok {
		t.Fatalf("'locked' attribute is %T, want schema.BoolAttribute", s.Attributes["locked"])
	}
	if !lockedAttr.IsComputed() || lockedAttr.IsRequired() || lockedAttr.IsOptional() {
		t.Error("'locked' should be Computed-only")
	}
}

func TestWebshareSchema_NoUnexpectedAttributes(t *testing.T) {
	s := resourceSchema()
	want := map[string]bool{
		"id": true, "name": true, "path": true, "enabled": true, "is_home_base": true,
		"dataset": true, "relative_path": true, "locked": true,
	}
	if len(s.Attributes) != len(want) {
		t.Errorf("schema has %d attributes, want %d", len(s.Attributes), len(want))
	}
	for name := range s.Attributes {
		if !want[name] {
			t.Errorf("unexpected schema attribute %q", name)
		}
	}
}

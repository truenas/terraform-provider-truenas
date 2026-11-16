// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nfs_config

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/normalize"
)

// TestNFSConfigSchema_UnorderedListCustomType verifies that the
// order-insensitive list attributes (protocols, bindip) carry the
// UnorderedListType custom type, so a server reorder does not plan a
// perpetual diff (pool #66 class).
func TestNFSConfigSchema_UnorderedListCustomType(t *testing.T) {
	s := resourceSchema()
	for _, name := range []string{"protocols", "bindip"} {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		listAttr, ok := attr.(schema.ListAttribute)
		if !ok {
			t.Fatalf("%q is %T, want schema.ListAttribute", name, attr)
		}
		if _, ok := listAttr.CustomType.(normalize.UnorderedListType); !ok {
			t.Errorf("%q CustomType is %T, want normalize.UnorderedListType", name, listAttr.CustomType)
		}
	}
}

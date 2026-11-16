// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nfs

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/normalize"
)

// TestNFSSchema_UnorderedListCustomType verifies that the order-insensitive
// allowlist attributes (networks, hosts) carry the UnorderedListType custom
// type, so a server reorder does not plan a perpetual diff (pool #66 class).
// security is deliberately excluded: it is an ordered preference list.
func TestNFSSchema_UnorderedListCustomType(t *testing.T) {
	s := resourceSchema()
	for _, name := range []string{"networks", "hosts"} {
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

	// security must NOT be unordered: it is an ordered preference list, so a
	// reorder is a real change the server preserves.
	secAttr, ok := s.Attributes["security"].(schema.ListAttribute)
	if !ok {
		t.Fatal("schema missing 'security' list attribute")
	}
	if _, ok := secAttr.CustomType.(normalize.UnorderedListType); ok {
		t.Error("'security' must not use UnorderedListType; order is significant")
	}
}

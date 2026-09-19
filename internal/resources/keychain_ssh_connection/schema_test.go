// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package keychain_ssh_connection

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// TestSchema_IDComputedOnly verifies "id" is Computed-only.
func TestSchema_IDComputedOnly(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["id"].(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'id' attribute is %T, want schema.Int64Attribute", s.Attributes["id"])
	}
	if !attr.IsComputed() {
		t.Error("'id' should be Computed")
	}
	if attr.IsRequired() || attr.IsOptional() {
		t.Error("'id' should be Computed-only (not Required/Optional)")
	}
}

// TestSchema_RequiredFields verifies "name", "host", "private_key_id", and
// "remote_host_key" are Required, matching SSH_CREDENTIALS' own required
// set (probed live: host/private_key/remote_host_key), plus "name" which
// every keychaincredential entry requires.
func TestSchema_RequiredFields(t *testing.T) {
	s := resourceSchema()

	for _, field := range []string{"name", "host", "private_key_id", "remote_host_key"} {
		attr, ok := s.Attributes[field]
		if !ok {
			t.Fatalf("schema missing %q attribute", field)
		}
		req, ok := attr.(interface{ IsRequired() bool })
		if !ok || !req.IsRequired() {
			t.Errorf("%q should be Required", field)
		}
	}
}

// TestSchema_OptionalComputedDefaultedFields verifies "port", "username",
// and "connect_timeout" are Optional+Computed — each has a server-side
// default (22, "root", 10 respectively, probed live) but is also
// updatable in place.
func TestSchema_OptionalComputedDefaultedFields(t *testing.T) {
	s := resourceSchema()

	for _, field := range []string{"port", "username", "connect_timeout"} {
		attr, ok := s.Attributes[field]
		if !ok {
			t.Fatalf("schema missing %q attribute", field)
		}
		oc, ok := attr.(interface {
			IsOptional() bool
			IsComputed() bool
		})
		if !ok || !oc.IsOptional() || !oc.IsComputed() {
			t.Errorf("%q should be Optional+Computed", field)
		}
	}
}

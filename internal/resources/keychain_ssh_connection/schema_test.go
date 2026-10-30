// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package keychain_ssh_connection

import (
	"github.com/truenas/terraform-provider-truenas/internal/planmod"
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

// TestSchema_DocumentedDefaultsApplied guards #56: port, connect_timeout, and
// username document defaults (22, 10, "root"), but without a schema Default an
// unset value was coerced to 0/"" in the create payload, which TrueNAS stored
// verbatim (port 0, connect_timeout 0) and broke replication. Assert the
// documented defaults are actually wired as schema Defaults.
func TestSchema_DocumentedDefaultsApplied(t *testing.T) {
	// port/username/connect_timeout have server defaults (22/root/10) but no
	// schema Default: a Default rewrote imported connections (hunt F9). They
	// use planmod.ResetWhenUnset*, which keeps the server value when the
	// attribute was never configured and resets it when it is removed.
	s := resourceSchema()
	for _, name := range []string{"port", "username", "connect_timeout"} {
		var mods []any
		switch a := s.Attributes[name].(type) {
		case schema.Int64Attribute:
			if a.Default != nil {
				t.Errorf("%q must not carry a schema Default", name)
			}
			for _, m := range a.PlanModifiers {
				mods = append(mods, m)
			}
		case schema.StringAttribute:
			if a.Default != nil {
				t.Errorf("%q must not carry a schema Default", name)
			}
			for _, m := range a.PlanModifiers {
				mods = append(mods, m)
			}
		default:
			t.Fatalf("%q: unexpected attribute type %T", name, s.Attributes[name])
		}
		found := false
		for _, m := range mods {
			if planmod.IsResetWhenUnset(m) {
				found = true
			}
		}
		if !found {
			t.Errorf("%q should use planmod.ResetWhenUnset*", name)
		}
	}
}

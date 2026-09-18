// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package ipmi_lan

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

func TestIPMILanSchema_IDIsComputed(t *testing.T) {
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

// TestIPMILanSchema_ChannelRequiredForceNew pins the identity contract from
// the task brief: "channel" identifies a pre-existing physical BMC LAN
// channel (ipmi.lan.update takes it as a positional argument), so it must
// be Required and force replacement rather than being renamed in place.
func TestIPMILanSchema_ChannelRequiredForceNew(t *testing.T) {
	s := resourceSchema()
	chAttr, ok := s.Attributes["channel"].(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'channel' attribute is %T, want schema.Int64Attribute", s.Attributes["channel"])
	}
	if !chAttr.IsRequired() {
		t.Error("'channel' should be Required")
	}
	if chAttr.IsComputed() || chAttr.IsOptional() {
		t.Error("'channel' should be Required-only (not Computed/Optional)")
	}
	if len(chAttr.PlanModifiers) == 0 {
		t.Error("'channel' should have a RequiresReplace plan modifier")
	}
}

func TestIPMILanSchema_DHCPRequired(t *testing.T) {
	s := resourceSchema()
	dhcpAttr, ok := s.Attributes["dhcp"].(schema.BoolAttribute)
	if !ok {
		t.Fatalf("'dhcp' attribute is %T, want schema.BoolAttribute", s.Attributes["dhcp"])
	}
	if !dhcpAttr.IsRequired() {
		t.Error("'dhcp' should be Required")
	}
	if dhcpAttr.IsComputed() || dhcpAttr.IsOptional() {
		t.Error("'dhcp' should be Required-only (not Computed/Optional)")
	}
}

// TestIPMILanSchema_PasswordWriteOnly pins the password evidence from the
// live probe (ipmi.lan.query never returns a "password" key under any
// name): the schema field must be Sensitive + WriteOnly + Optional, never
// Computed (a WriteOnly attribute must not be Computed per the framework).
func TestIPMILanSchema_PasswordWriteOnly(t *testing.T) {
	s := resourceSchema()
	pwAttr, ok := s.Attributes["password"].(schema.StringAttribute)
	if !ok {
		t.Fatalf("'password' attribute is %T, want schema.StringAttribute", s.Attributes["password"])
	}
	if !pwAttr.Sensitive {
		t.Error("'password' should be Sensitive")
	}
	if !pwAttr.WriteOnly {
		t.Error("'password' should be WriteOnly")
	}
	if !pwAttr.IsOptional() {
		t.Error("'password' should be Optional")
	}
	if pwAttr.IsComputed() {
		t.Error("'password' should not be Computed (incompatible with WriteOnly)")
	}
}

func TestIPMILanSchema_ApplyRemoteOptionalNotComputed(t *testing.T) {
	s := resourceSchema()
	arAttr, ok := s.Attributes["apply_remote"].(schema.BoolAttribute)
	if !ok {
		t.Fatalf("'apply_remote' attribute is %T, want schema.BoolAttribute", s.Attributes["apply_remote"])
	}
	if !arAttr.IsOptional() {
		t.Error("'apply_remote' should be Optional")
	}
	if arAttr.IsComputed() {
		t.Error("'apply_remote' should not be Computed: it is a write-time directive with no read-back state")
	}
}

// TestIPMILanSchema_OptionalComputedFields pins the remaining
// Optional+Computed fields: settable, but always readable back from
// ipmi.lan.query.
func TestIPMILanSchema_OptionalComputedFields(t *testing.T) {
	s := resourceSchema()
	type optionalComputed interface {
		IsOptional() bool
		IsComputed() bool
	}
	writable := map[string]bool{"ipaddress": true, "netmask": true, "gateway": true, "vlan": true}
	for name := range writable {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing expected attribute %q", name)
		}
		oc, ok := attr.(optionalComputed)
		if !ok {
			t.Fatalf("%q has unexpected type %T", name, attr)
		}
		if !oc.IsOptional() || !oc.IsComputed() {
			t.Errorf("%q should be Optional+Computed", name)
		}
	}
}

func TestIPMILanSchema_ComputedOnlyFields(t *testing.T) {
	s := resourceSchema()
	type optionalComputed interface {
		IsOptional() bool
		IsComputed() bool
	}
	computedOnly := []string{"vlan_priority", "mac_address"}
	for _, name := range computedOnly {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing expected attribute %q", name)
		}
		oc, ok := attr.(optionalComputed)
		if !ok {
			t.Fatalf("%q has unexpected type %T", name, attr)
		}
		if oc.IsOptional() {
			t.Errorf("%q should not be Optional", name)
		}
		if !oc.IsComputed() {
			t.Errorf("%q should be Computed", name)
		}
	}
}

func TestIPMILanSchema_NoUnexpectedAttributes(t *testing.T) {
	s := resourceSchema()
	want := map[string]bool{
		"id": true, "channel": true, "dhcp": true, "ipaddress": true, "netmask": true,
		"gateway": true, "vlan": true, "vlan_priority": true, "mac_address": true,
		"password": true, "apply_remote": true,
	}
	if len(s.Attributes) != len(want) {
		t.Errorf("schema has %d attributes, want %d", len(s.Attributes), len(want))
	}
	for name := range s.Attributes {
		if !want[name] {
			t.Errorf("unexpected schema attribute %q", name)
		}
	}
	for name := range want {
		if _, ok := s.Attributes[name]; !ok {
			t.Errorf("schema missing expected attribute %q", name)
		}
	}
}

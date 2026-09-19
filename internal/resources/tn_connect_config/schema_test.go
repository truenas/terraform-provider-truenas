// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package tn_connect_config

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

func TestTnConnectConfigSchema_IDIsComputed(t *testing.T) {
	s := resourceSchema()
	idAttr, ok := s.Attributes["id"].(schema.StringAttribute)
	if !ok {
		t.Fatalf("'id' attribute is %T, want schema.StringAttribute", s.Attributes["id"])
	}
	if !idAttr.IsComputed() {
		t.Error("'id' should be Computed")
	}
	if idAttr.IsRequired() || idAttr.IsOptional() {
		t.Error("'id' should be Computed-only (not Required/Optional)")
	}
}

// TestTnConnectConfigSchema_OnlyEnabledIsWritable is the schema-level half
// of this resource's safety contract: "enabled" must be the ONLY
// Optional+Computed (i.e. user-writable) attribute. Every other attribute
// must be Computed-only, matching the probe-confirmed fact that TrueNAS
// 26.0's tn_connect.update accepts nothing besides "enabled" (see model.go's
// updatePayload doc comment).
func TestTnConnectConfigSchema_OnlyEnabledIsWritable(t *testing.T) {
	s := resourceSchema()
	for name, attr := range s.Attributes {
		if name == "id" {
			continue
		}
		type optionalComputed interface {
			IsOptional() bool
			IsComputed() bool
		}
		oc, ok := attr.(optionalComputed)
		if !ok {
			t.Fatalf("%q has unexpected type %T", name, attr)
		}
		if name == "enabled" {
			if !oc.IsOptional() || !oc.IsComputed() {
				t.Errorf("%q should be Optional+Computed (the resource's only writable field)", name)
			}
			continue
		}
		if oc.IsOptional() {
			t.Errorf("%q is Optional; only \"enabled\" may be user-writable on this resource", name)
		}
		if !oc.IsComputed() {
			t.Errorf("%q should be Computed", name)
		}
	}
}

func TestTnConnectConfigSchema_NoUnexpectedAttributes(t *testing.T) {
	s := resourceSchema()
	want := map[string]bool{
		"id": true, "enabled": true, "status": true, "status_reason": true,
		"certificate": true, "account_service_base_url": true, "leca_service_base_url": true,
		"tnc_base_url": true, "heartbeat_url": true, "registration_details": true,
		"tier": true, "last_heartbeat_failure_datetime": true,
		"ips": true, "interfaces": true, "interfaces_ips": true, "use_all_interfaces": true,
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

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package truecommand_config

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// TestTrueCommandConfigSchema_IDIsComputed verifies "id" is Computed-only.
func TestTrueCommandConfigSchema_IDIsComputed(t *testing.T) {
	s := resourceSchema()

	idAttr, ok := s.Attributes["id"]
	if !ok {
		t.Fatal("schema missing 'id' attribute")
	}
	idStr, ok := idAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'id' attribute is %T, want schema.StringAttribute", idAttr)
	}
	if !idStr.IsComputed() {
		t.Error("'id' should be Computed")
	}
	if idStr.IsRequired() || idStr.IsOptional() {
		t.Error("'id' should be Computed-only (not Required/Optional)")
	}
}

// TestTrueCommandConfigSchema_EnabledIsOptionalComputed verifies "enabled"
// is the safety-critical Optional+Computed field, matching the
// tn_connect_config precedent.
func TestTrueCommandConfigSchema_EnabledIsOptionalComputed(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["enabled"]
	if !ok {
		t.Fatal("schema missing 'enabled' attribute")
	}
	boolAttr, ok := attr.(schema.BoolAttribute)
	if !ok {
		t.Fatalf("'enabled' attribute is %T, want schema.BoolAttribute", attr)
	}
	if !boolAttr.IsOptional() || !boolAttr.IsComputed() {
		t.Error("'enabled' should be Optional+Computed")
	}
}

// TestTrueCommandConfigSchema_APIKeyIsSensitiveNotWriteOnly verifies
// "api_key" is Sensitive but NOT WriteOnly: decisive live probe evidence
// (model.go's doc comment) confirmed truecommand.config returns it verbatim
// on read-back, unlike a genuinely masked/write-only credential.
func TestTrueCommandConfigSchema_APIKeyIsSensitiveNotWriteOnly(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["api_key"]
	if !ok {
		t.Fatal("schema missing 'api_key' attribute")
	}
	strAttr, ok := attr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'api_key' attribute is %T, want schema.StringAttribute", attr)
	}
	if !strAttr.IsSensitive() {
		t.Error("'api_key' should be Sensitive")
	}
	if strAttr.IsWriteOnly() {
		t.Error("'api_key' should NOT be WriteOnly: truecommand.config returns it unmasked (decisive live probe)")
	}
	if !strAttr.IsOptional() || !strAttr.IsComputed() {
		t.Error("'api_key' should be Optional+Computed")
	}
	if len(strAttr.Validators) == 0 {
		t.Error("'api_key' should carry a length validator matching the probed 16-character server-side constraint")
	}
}

// TestTrueCommandConfigSchema_ComputedOnlyFields verifies the read-only
// status/connection fields are Computed-only.
func TestTrueCommandConfigSchema_ComputedOnlyFields(t *testing.T) {
	s := resourceSchema()

	for _, name := range []string{"status", "status_reason", "remote_url", "remote_ip_address"} {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		strAttr, ok := attr.(schema.StringAttribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.StringAttribute", name, attr)
		}
		if !strAttr.IsComputed() {
			t.Errorf("%q should be Computed", name)
		}
		if strAttr.IsRequired() || strAttr.IsOptional() {
			t.Errorf("%q should be Computed-only (not Required/Optional)", name)
		}
	}
}

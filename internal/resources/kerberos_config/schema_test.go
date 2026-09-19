// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package kerberos_config

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// TestKerberosConfigSchema_IDIsComputed verifies that "id" is a
// Computed-only StringAttribute with UseStateForUnknown, since it's a fixed
// singleton value never supplied by the user.
func TestKerberosConfigSchema_IDIsComputed(t *testing.T) {
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
	if len(idStr.PlanModifiers) == 0 {
		t.Error("'id' should have plan modifiers (UseStateForUnknown)")
	}
}

// TestKerberosConfigSchema_AuxFieldsAreOptionalComputed verifies that
// appdefaults_aux and libdefaults_aux are both Optional+Computed with
// UseStateForUnknown plan modifiers.
func TestKerberosConfigSchema_AuxFieldsAreOptionalComputed(t *testing.T) {
	s := resourceSchema()

	for _, name := range []string{"appdefaults_aux", "libdefaults_aux"} {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		strAttr, ok := attr.(schema.StringAttribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.StringAttribute", name, attr)
		}
		if !strAttr.IsOptional() || !strAttr.IsComputed() {
			t.Errorf("%q should be Optional+Computed", name)
		}
		if len(strAttr.PlanModifiers) == 0 {
			t.Errorf("%q should have plan modifiers (UseStateForUnknown)", name)
		}
	}
}

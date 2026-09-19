// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package init_shutdown_script

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// TestInitShutdownScriptSchema_RequiredFields verifies "type" and "when"
// are Required, matching initshutdownscript.create's own "required" list.
func TestInitShutdownScriptSchema_RequiredFields(t *testing.T) {
	s := resourceSchema()

	for _, name := range []string{"type", "when"} {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		strAttr, ok := attr.(schema.StringAttribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.StringAttribute", name, attr)
		}
		if !strAttr.IsRequired() {
			t.Errorf("%q should be Required", name)
		}
	}
}

// TestInitShutdownScriptSchema_IDIsComputed verifies "id" is Computed-only.
func TestInitShutdownScriptSchema_IDIsComputed(t *testing.T) {
	s := resourceSchema()

	idAttr, ok := s.Attributes["id"]
	if !ok {
		t.Fatal("schema missing 'id' attribute")
	}
	idInt, ok := idAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'id' attribute is %T, want schema.Int64Attribute", idAttr)
	}
	if !idInt.IsComputed() {
		t.Error("'id' should be Computed")
	}
	if idInt.IsRequired() || idInt.IsOptional() {
		t.Error("'id' should be Computed-only (not Required/Optional)")
	}
}

// TestInitShutdownScriptSchema_CommandScriptAreOptionalComputed verifies
// "command" and "script" are Optional+Computed, not Required — the
// conditional requiredness is enforced in apiPayload, not the schema,
// since which one is required depends on "type".
func TestInitShutdownScriptSchema_CommandScriptAreOptionalComputed(t *testing.T) {
	s := resourceSchema()

	for _, name := range []string{"command", "script"} {
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
		if strAttr.IsRequired() {
			t.Errorf("%q should not be Required (conditionally required, enforced in apiPayload)", name)
		}
	}
}

// TestInitShutdownScriptSchema_TypeWhenHaveEnumValidators verifies "type"
// and "when" are constrained to the enum values probed live.
func TestInitShutdownScriptSchema_TypeWhenHaveEnumValidators(t *testing.T) {
	s := resourceSchema()

	for _, name := range []string{"type", "when"} {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		strAttr, ok := attr.(schema.StringAttribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.StringAttribute", name, attr)
		}
		if len(strAttr.Validators) == 0 {
			t.Errorf("%q should have at least one validator (OneOf enum)", name)
		}
	}
}

// TestInitShutdownScriptSchema_OptionalComputedFields verifies enabled,
// timeout, and comment are all Optional+Computed.
func TestInitShutdownScriptSchema_OptionalComputedFields(t *testing.T) {
	s := resourceSchema()

	enabledAttr, ok := s.Attributes["enabled"].(schema.BoolAttribute)
	if !ok {
		t.Fatal("schema missing 'enabled' bool attribute")
	}
	if !enabledAttr.IsOptional() || !enabledAttr.IsComputed() {
		t.Error("'enabled' should be Optional+Computed")
	}

	timeoutAttr, ok := s.Attributes["timeout"].(schema.Int64Attribute)
	if !ok {
		t.Fatal("schema missing 'timeout' int64 attribute")
	}
	if !timeoutAttr.IsOptional() || !timeoutAttr.IsComputed() {
		t.Error("'timeout' should be Optional+Computed")
	}

	commentAttr, ok := s.Attributes["comment"].(schema.StringAttribute)
	if !ok {
		t.Fatal("schema missing 'comment' string attribute")
	}
	if !commentAttr.IsOptional() || !commentAttr.IsComputed() {
		t.Error("'comment' should be Optional+Computed")
	}
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package service

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestServiceSchema_IDIsString verifies that the "id" attribute is a
// StringAttribute (not Int64), because services use the service name as their
// Terraform ID.
func TestServiceSchema_IDIsString(t *testing.T) {
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

// TestServiceSchema_NameIsRequiresReplace verifies that "name" carries the
// RequiresReplace plan modifier, forcing resource recreation if the service
// name changes.
func TestServiceSchema_NameIsRequiresReplace(t *testing.T) {
	s := resourceSchema()

	nameAttr, ok := s.Attributes["name"]
	if !ok {
		t.Fatal("schema missing 'name' attribute")
	}
	nameStr, ok := nameAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'name' attribute is %T, want schema.StringAttribute", nameAttr)
	}
	if !nameStr.IsRequired() {
		t.Error("'name' should be Required")
	}
	if len(nameStr.PlanModifiers) == 0 {
		t.Error("'name' should have plan modifiers (RequiresReplace)")
	}
}

// TestServiceSchema_RunningIsOptionalComputed verifies that "running" is both
// Optional (users may set it) and Computed (TrueNAS provides it when unset).
func TestServiceSchema_RunningIsOptionalComputed(t *testing.T) {
	s := resourceSchema()

	runningAttr, ok := s.Attributes["running"]
	if !ok {
		t.Fatal("schema missing 'running' attribute")
	}
	runningBool, ok := runningAttr.(schema.BoolAttribute)
	if !ok {
		t.Fatalf("'running' attribute is %T, want schema.BoolAttribute", runningAttr)
	}
	if !runningBool.IsOptional() {
		t.Error("'running' should be Optional")
	}
	if !runningBool.IsComputed() {
		t.Error("'running' should be Computed")
	}
}

// TestServiceSchema_EnabledIsOptionalComputed verifies that "enabled" is both
// Optional and Computed.
func TestServiceSchema_EnabledIsOptionalComputed(t *testing.T) {
	s := resourceSchema()

	enabledAttr, ok := s.Attributes["enabled"]
	if !ok {
		t.Fatal("schema missing 'enabled' attribute")
	}
	enabledBool, ok := enabledAttr.(schema.BoolAttribute)
	if !ok {
		t.Fatalf("'enabled' attribute is %T, want schema.BoolAttribute", enabledAttr)
	}
	if !enabledBool.IsOptional() {
		t.Error("'enabled' should be Optional")
	}
	if !enabledBool.IsComputed() {
		t.Error("'enabled' should be Computed")
	}
}

// TestResponseToModel_Running verifies that State=="RUNNING" maps to
// Running=true and any other state maps to Running=false.
func TestResponseToModel_Running(t *testing.T) {
	cases := []struct {
		state   string
		running bool
	}{
		{"RUNNING", true},
		{"STOPPED", false},
		{"", false},
	}

	for _, tc := range cases {
		api := &serviceAPI{
			ID:      1,
			Service: "nfs",
			State:   tc.state,
			Enable:  true,
		}
		var m ServiceModel
		responseToModel(api, &m)

		if m.Running.ValueBool() != tc.running {
			t.Errorf("state=%q: Running=%v, want %v", tc.state, m.Running.ValueBool(), tc.running)
		}
	}
}

// TestResponseToModel_AllFields verifies that all fields are mapped correctly.
func TestResponseToModel_AllFields(t *testing.T) {
	api := &serviceAPI{
		ID:      42,
		Service: "ssh",
		State:   "RUNNING",
		Enable:  true,
	}

	var m ServiceModel
	responseToModel(api, &m)

	if m.ID != types.StringValue("ssh") {
		t.Errorf("ID = %v, want ssh", m.ID)
	}
	if m.Name != types.StringValue("ssh") {
		t.Errorf("Name = %v, want ssh", m.Name)
	}
	if m.Enabled != types.BoolValue(true) {
		t.Errorf("Enabled = %v, want true", m.Enabled)
	}
	if m.Running != types.BoolValue(true) {
		t.Errorf("Running = %v, want true", m.Running)
	}
}

// TestResponseToModel_IDIsServiceName verifies that the Terraform ID is set to
// the service name string (not a numeric ID).
func TestResponseToModel_IDIsServiceName(t *testing.T) {
	api := &serviceAPI{
		ID:      99,
		Service: "cifs",
		State:   "STOPPED",
		Enable:  false,
	}

	var m ServiceModel
	responseToModel(api, &m)

	if m.ID.ValueString() != "cifs" {
		t.Errorf("ID = %q, want service name string \"cifs\"", m.ID.ValueString())
	}
}

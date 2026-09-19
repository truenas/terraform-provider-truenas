// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm_device

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestAttributesMap_InvalidJSON verifies that attributesMap rejects
// malformed JSON with an error diagnostic.
func TestAttributesMap_InvalidJSON(t *testing.T) {
	m := VMDeviceModel{Attributes: types.StringValue("{not valid json")}

	attrs, diags := m.attributesMap()
	if !diags.HasError() {
		t.Fatal("expected error diagnostics for invalid JSON, got none")
	}
	if attrs != nil {
		t.Errorf("expected nil attrs on error, got %v", attrs)
	}
}

// TestAttributesMap_MissingDtype verifies that attributesMap rejects valid
// JSON that lacks the required "dtype" key.
func TestAttributesMap_MissingDtype(t *testing.T) {
	m := VMDeviceModel{Attributes: types.StringValue(`{"path": "/dev/zvol/tank/vm0"}`)}

	attrs, diags := m.attributesMap()
	if !diags.HasError() {
		t.Fatal("expected error diagnostics for missing dtype, got none")
	}
	if attrs != nil {
		t.Errorf("expected nil attrs on error, got %v", attrs)
	}
}

// TestAttributesMap_Valid verifies that attributesMap parses valid JSON
// with a dtype key without error.
func TestAttributesMap_Valid(t *testing.T) {
	m := VMDeviceModel{Attributes: types.StringValue(`{"dtype": "DISK", "path": "/dev/zvol/tank/vm0"}`)}

	attrs, diags := m.attributesMap()
	if diags.HasError() {
		t.Fatalf("unexpected error diagnostics: %v", diags)
	}
	if attrs["dtype"] != "DISK" {
		t.Errorf("attrs[dtype] = %v, want DISK", attrs["dtype"])
	}
}

// TestAttributesDrifted_Identical verifies that identical maps are not
// considered drifted.
func TestAttributesDrifted_Identical(t *testing.T) {
	state := map[string]any{"dtype": "DISK", "path": "/dev/zvol/tank/vm0"}
	api := map[string]any{"dtype": "DISK", "path": "/dev/zvol/tank/vm0"}

	if attributesDrifted(state, api) {
		t.Error("attributesDrifted() = true for identical maps, want false")
	}
}

// TestAttributesDrifted_ChangedValue verifies that a changed value for a
// user-set key is reported as drift.
func TestAttributesDrifted_ChangedValue(t *testing.T) {
	state := map[string]any{"dtype": "DISK", "path": "/dev/zvol/tank/vm0"}
	api := map[string]any{"dtype": "DISK", "path": "/dev/zvol/tank/vm1"}

	if !attributesDrifted(state, api) {
		t.Error("attributesDrifted() = false for changed value, want true")
	}
}

// TestAttributesDrifted_APIAddedKey verifies that a key present only in the
// API response (a server-added default) is NOT considered drift.
func TestAttributesDrifted_APIAddedKey(t *testing.T) {
	state := map[string]any{"dtype": "DISK", "path": "/dev/zvol/tank/vm0"}
	api := map[string]any{"dtype": "DISK", "path": "/dev/zvol/tank/vm0", "type": "AHCI"}

	if attributesDrifted(state, api) {
		t.Error("attributesDrifted() = true for API-added extra key, want false")
	}
}

// TestAttributesDrifted_MissingUserKey verifies that a user-set key absent
// from the API response is considered drift.
func TestAttributesDrifted_MissingUserKey(t *testing.T) {
	state := map[string]any{"dtype": "DISK", "path": "/dev/zvol/tank/vm0"}
	api := map[string]any{"dtype": "DISK"}

	if !attributesDrifted(state, api) {
		t.Error("attributesDrifted() = false for missing user key in API, want true")
	}
}

// TestUpdatePayload_OmitsVM verifies that updatePayload contains attributes
// (and order when set) but never the "vm" key, since a device cannot be
// moved between VMs.
func TestUpdatePayload_OmitsVM(t *testing.T) {
	m := VMDeviceModel{
		ID:         types.Int64Value(5),
		VM:         types.Int64Value(1),
		Attributes: types.StringValue(`{"dtype": "DISK", "path": "/dev/zvol/tank/vm0"}`),
		Order:      types.Int64Value(1002),
	}

	payload, diags := m.updatePayload()
	if diags.HasError() {
		t.Fatalf("unexpected error diagnostics: %v", diags)
	}

	if _, ok := payload["vm"]; ok {
		t.Error("update payload should not contain key 'vm'")
	}
	if _, ok := payload["attributes"]; !ok {
		t.Error("update payload missing key 'attributes'")
	}
	if payload["order"] != int64(1002) {
		t.Errorf("payload[order] = %v, want 1002", payload["order"])
	}
}

// TestUpdatePayload_OmitsOrderWhenUnset verifies order is left out when
// null/unknown.
func TestUpdatePayload_OmitsOrderWhenUnset(t *testing.T) {
	m := VMDeviceModel{
		ID:         types.Int64Value(5),
		VM:         types.Int64Value(1),
		Attributes: types.StringValue(`{"dtype": "DISK"}`),
		Order:      types.Int64Null(),
	}

	payload, diags := m.updatePayload()
	if diags.HasError() {
		t.Fatalf("unexpected error diagnostics: %v", diags)
	}
	if _, ok := payload["order"]; ok {
		t.Error("update payload should not contain key 'order' when unset")
	}
	if _, ok := payload["vm"]; ok {
		t.Error("update payload should not contain key 'vm'")
	}
}

// TestCreatePayload_IncludesVM verifies createPayload includes vm and
// attributes.
func TestCreatePayload_IncludesVM(t *testing.T) {
	m := VMDeviceModel{
		VM:         types.Int64Value(3),
		Attributes: types.StringValue(`{"dtype": "NIC"}`),
		Order:      types.Int64Null(),
	}

	payload, diags := m.createPayload()
	if diags.HasError() {
		t.Fatalf("unexpected error diagnostics: %v", diags)
	}
	if payload["vm"] != int64(3) {
		t.Errorf("payload[vm] = %v, want 3", payload["vm"])
	}
	if _, ok := payload["attributes"]; !ok {
		t.Error("create payload missing key 'attributes'")
	}
}

// TestSchema_VMRequiresReplace verifies that the "vm" attribute is Required
// and has a RequiresReplace plan modifier.
func TestSchema_VMRequiresReplace(t *testing.T) {
	s := resourceSchema()

	vmAttr, ok := s.Attributes["vm"]
	if !ok {
		t.Fatal("schema missing 'vm' attribute")
	}
	vmInt64, ok := vmAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'vm' attribute is %T, want schema.Int64Attribute", vmAttr)
	}
	if !vmInt64.IsRequired() {
		t.Error("'vm' should be Required")
	}

	foundReplace := false
	for _, pm := range vmInt64.PlanModifiers {
		// Compare against the well-known RequiresReplace modifier by its
		// description, since int64planmodifier.RequiresReplace() is not a
		// named type we can type-assert directly.
		if pm.Description(context.Background()) == int64planmodifier.RequiresReplace().Description(context.Background()) {
			foundReplace = true
		}
	}
	if !foundReplace {
		t.Error("'vm' should have a RequiresReplace plan modifier")
	}
}

// TestSchema_IDComputedUseStateForUnknown verifies that the "id" attribute
// is Computed with a UseStateForUnknown plan modifier.
func TestSchema_IDComputedUseStateForUnknown(t *testing.T) {
	s := resourceSchema()

	idAttr, ok := s.Attributes["id"]
	if !ok {
		t.Fatal("schema missing 'id' attribute")
	}
	idInt64, ok := idAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'id' attribute is %T, want schema.Int64Attribute", idAttr)
	}
	if !idInt64.IsComputed() {
		t.Error("'id' should be Computed")
	}
	if idInt64.IsRequired() || idInt64.IsOptional() {
		t.Error("'id' should be Computed-only (not Required/Optional)")
	}

	foundUseState := false
	for _, pm := range idInt64.PlanModifiers {
		if pm.Description(context.Background()) == int64planmodifier.UseStateForUnknown().Description(context.Background()) {
			foundUseState = true
		}
	}
	if !foundUseState {
		t.Error("'id' should have a UseStateForUnknown plan modifier")
	}
}

// TestSchema_AttributesRequired verifies that "attributes" is a Required
// StringAttribute, and that it is Sensitive: the blob can carry secrets for
// some device types (e.g. the DISPLAY device's VNC password), so the whole
// attribute is masked in plan output and state as a pragmatic fix.
func TestSchema_AttributesRequired(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["attributes"]
	if !ok {
		t.Fatal("schema missing 'attributes' attribute")
	}
	strAttr, ok := attr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'attributes' attribute is %T, want schema.StringAttribute", attr)
	}
	if !strAttr.IsRequired() {
		t.Error("'attributes' should be Required")
	}
	if !strAttr.IsSensitive() {
		t.Error("'attributes' should be Sensitive")
	}
}

// TestSchema_OrderOptionalComputed verifies that "order" is
// Optional+Computed with a UseStateForUnknown plan modifier.
func TestSchema_OrderOptionalComputed(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["order"]
	if !ok {
		t.Fatal("schema missing 'order' attribute")
	}
	intAttr, ok := attr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'order' attribute is %T, want schema.Int64Attribute", attr)
	}
	if !intAttr.IsOptional() {
		t.Error("'order' should be Optional")
	}
	if !intAttr.IsComputed() {
		t.Error("'order' should be Computed")
	}
}

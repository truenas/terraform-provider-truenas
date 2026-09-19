// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_port_subsys

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestPortSubsysSchema verifies that the resource schema has the expected
// attributes with correct types and plan modifiers.
func TestPortSubsysSchema(t *testing.T) {
	s := resourceSchema()

	// id must be Int64Attribute, Computed-only, with UseStateForUnknown.
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
	if !hasUseStateForUnknown(idInt64.PlanModifiers) {
		t.Error("'id' should have UseStateForUnknown plan modifier")
	}

	// port_id must be Required Int64 with RequiresReplace.
	portIDAttr, ok := s.Attributes["port_id"]
	if !ok {
		t.Fatal("schema missing 'port_id' attribute")
	}
	portIDInt64, ok := portIDAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'port_id' attribute is %T, want schema.Int64Attribute", portIDAttr)
	}
	if !portIDInt64.IsRequired() {
		t.Error("'port_id' should be Required")
	}
	if !hasRequiresReplace(portIDInt64.PlanModifiers) {
		t.Error("'port_id' should have RequiresReplace plan modifier")
	}

	// subsys_id must be Required Int64 with RequiresReplace.
	subsysIDAttr, ok := s.Attributes["subsys_id"]
	if !ok {
		t.Fatal("schema missing 'subsys_id' attribute")
	}
	subsysIDInt64, ok := subsysIDAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'subsys_id' attribute is %T, want schema.Int64Attribute", subsysIDAttr)
	}
	if !subsysIDInt64.IsRequired() {
		t.Error("'subsys_id' should be Required")
	}
	if !hasRequiresReplace(subsysIDInt64.PlanModifiers) {
		t.Error("'subsys_id' should have RequiresReplace plan modifier")
	}
}

// hasRequiresReplace and hasUseStateForUnknown identify plan modifiers by
// description, since the concrete types returned by int64planmodifier are
// unexported.
func hasRequiresReplace(modifiers []planmodifier.Int64) bool {
	want := int64planmodifier.RequiresReplace().Description(context.Background())
	for _, m := range modifiers {
		if m.Description(context.Background()) == want {
			return true
		}
	}
	return false
}

func hasUseStateForUnknown(modifiers []planmodifier.Int64) bool {
	want := int64planmodifier.UseStateForUnknown().Description(context.Background())
	for _, m := range modifiers {
		if m.Description(context.Background()) == want {
			return true
		}
	}
	return false
}

// TestPortSubsysApiPayload verifies that apiPayload includes exactly
// port_id and subsys_id.
func TestPortSubsysApiPayload(t *testing.T) {
	ctx := context.Background()

	m := PortSubsysModel{
		PortID:   types.Int64Value(2),
		SubsysID: types.Int64Value(9),
	}

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostic errors: %v", diags)
	}

	if len(payload) != 2 {
		t.Fatalf("payload has %d keys, want 2: %v", len(payload), payload)
	}
	if payload["port_id"] != int64(2) {
		t.Errorf("payload[port_id] = %v, want 2", payload["port_id"])
	}
	if payload["subsys_id"] != int64(9) {
		t.Errorf("payload[subsys_id] = %v, want 9", payload["subsys_id"])
	}
}

// TestDecodeEmbeddedID_Object verifies decoding an embedded object shape
// ({"id": N, ...}), as returned by query/get_instance.
func TestDecodeEmbeddedID_Object(t *testing.T) {
	raw := json.RawMessage(`{"id": 1, "addr_traddr": "0.0.0.0"}`)
	id, err := decodeEmbeddedID(raw, "port")
	if err != nil {
		t.Fatalf("decodeEmbeddedID returned error: %v", err)
	}
	if id != 1 {
		t.Errorf("id = %v, want 1", id)
	}
}

// TestDecodeEmbeddedID_BareInt verifies decoding a bare integer shape, as
// used by create/update payload echoes.
func TestDecodeEmbeddedID_BareInt(t *testing.T) {
	raw := json.RawMessage(`9`)
	id, err := decodeEmbeddedID(raw, "subsys")
	if err != nil {
		t.Fatalf("decodeEmbeddedID returned error: %v", err)
	}
	if id != 9 {
		t.Errorf("id = %v, want 9", id)
	}
}

// TestDecodeEmbeddedID_Null verifies that a null field returns an error
// rather than silently defaulting to 0.
func TestDecodeEmbeddedID_Null(t *testing.T) {
	raw := json.RawMessage(`null`)
	if _, err := decodeEmbeddedID(raw, "port"); err == nil {
		t.Error("expected error for null field, got nil")
	}

	// Also verify the empty/absent case (zero-length RawMessage).
	if _, err := decodeEmbeddedID(json.RawMessage{}, "subsys"); err == nil {
		t.Error("expected error for empty field, got nil")
	}
}

// TestDecodeEmbeddedID_ObjectWithoutID verifies that an object shape missing
// the "id" key returns an error rather than silently defaulting to 0.
func TestDecodeEmbeddedID_ObjectWithoutID(t *testing.T) {
	if _, err := decodeEmbeddedID(json.RawMessage(`{}`), "port"); err == nil {
		t.Error("expected error for object without id, got nil")
	}
	if _, err := decodeEmbeddedID(json.RawMessage(`{"name":"x"}`), "port"); err == nil {
		t.Error("expected error for object without id, got nil")
	}
}

// TestDecodeEmbeddedID_ObjectNullID verifies that an object shape whose "id"
// key is explicitly null returns an error rather than silently defaulting
// to 0.
func TestDecodeEmbeddedID_ObjectNullID(t *testing.T) {
	if _, err := decodeEmbeddedID(json.RawMessage(`{"id":null}`), "port"); err == nil {
		t.Error("expected error for object with null id, got nil")
	}
}

// TestPortSubsysResponseToModel verifies that responseToModel decodes both
// embedded port and subsys objects and populates the model correctly.
func TestPortSubsysResponseToModel(t *testing.T) {
	ctx := context.Background()

	api := &portSubsysAPI{
		ID:     4,
		Port:   json.RawMessage(`{"id": 2, "addr_traddr": "0.0.0.0"}`),
		Subsys: json.RawMessage(`{"id": 9, "name": "subsys0"}`),
	}

	var m PortSubsysModel
	diags := responseToModel(ctx, api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned diagnostic errors: %v", diags)
	}

	if m.ID.ValueInt64() != 4 {
		t.Errorf("ID = %v, want 4", m.ID.ValueInt64())
	}
	if m.PortID.ValueInt64() != 2 {
		t.Errorf("PortID = %v, want 2", m.PortID.ValueInt64())
	}
	if m.SubsysID.ValueInt64() != 9 {
		t.Errorf("SubsysID = %v, want 9", m.SubsysID.ValueInt64())
	}
}

// TestPortSubsysResponseToModel_BareIntShapes verifies responseToModel also
// handles bare-integer port/subsys shapes.
func TestPortSubsysResponseToModel_BareIntShapes(t *testing.T) {
	ctx := context.Background()

	api := &portSubsysAPI{
		ID:     4,
		Port:   json.RawMessage(`2`),
		Subsys: json.RawMessage(`9`),
	}

	var m PortSubsysModel
	diags := responseToModel(ctx, api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned diagnostic errors: %v", diags)
	}
	if m.PortID.ValueInt64() != 2 {
		t.Errorf("PortID = %v, want 2", m.PortID.ValueInt64())
	}
	if m.SubsysID.ValueInt64() != 9 {
		t.Errorf("SubsysID = %v, want 9", m.SubsysID.ValueInt64())
	}
}

// TestPortSubsysResponseToModel_NullPort verifies that a null port field
// surfaces as a diagnostic error rather than silently zeroing PortID.
func TestPortSubsysResponseToModel_NullPort(t *testing.T) {
	ctx := context.Background()

	api := &portSubsysAPI{
		ID:     4,
		Port:   json.RawMessage(`null`),
		Subsys: json.RawMessage(`{"id": 9}`),
	}

	var m PortSubsysModel
	diags := responseToModel(ctx, api, &m)
	if !diags.HasError() {
		t.Fatal("expected diagnostic error for null port field, got none")
	}
}

// TestPortSubsysResponseToModel_NullSubsys verifies that a null subsys
// field surfaces as a diagnostic error rather than silently zeroing
// SubsysID.
func TestPortSubsysResponseToModel_NullSubsys(t *testing.T) {
	ctx := context.Background()

	api := &portSubsysAPI{
		ID:     4,
		Port:   json.RawMessage(`{"id": 2}`),
		Subsys: json.RawMessage(`null`),
	}

	var m PortSubsysModel
	diags := responseToModel(ctx, api, &m)
	if !diags.HasError() {
		t.Fatal("expected diagnostic error for null subsys field, got none")
	}
}

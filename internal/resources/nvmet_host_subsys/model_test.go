// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_host_subsys

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestHostSubsysSchema verifies that the resource schema has the expected
// attributes with correct types and plan modifiers.
func TestHostSubsysSchema(t *testing.T) {
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

	// host_id must be Required Int64 with RequiresReplace.
	hostIDAttr, ok := s.Attributes["host_id"]
	if !ok {
		t.Fatal("schema missing 'host_id' attribute")
	}
	hostIDInt64, ok := hostIDAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'host_id' attribute is %T, want schema.Int64Attribute", hostIDAttr)
	}
	if !hostIDInt64.IsRequired() {
		t.Error("'host_id' should be Required")
	}
	if !hasRequiresReplace(hostIDInt64.PlanModifiers) {
		t.Error("'host_id' should have RequiresReplace plan modifier")
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

// TestHostSubsysApiPayload verifies that apiPayload includes exactly
// host_id and subsys_id.
func TestHostSubsysApiPayload(t *testing.T) {
	ctx := context.Background()

	m := HostSubsysModel{
		HostID:   types.Int64Value(3),
		SubsysID: types.Int64Value(7),
	}

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostic errors: %v", diags)
	}

	if len(payload) != 2 {
		t.Fatalf("payload has %d keys, want 2: %v", len(payload), payload)
	}
	if payload["host_id"] != int64(3) {
		t.Errorf("payload[host_id] = %v, want 3", payload["host_id"])
	}
	if payload["subsys_id"] != int64(7) {
		t.Errorf("payload[subsys_id] = %v, want 7", payload["subsys_id"])
	}
}

// TestDecodeEmbeddedID_Object verifies decoding an embedded object shape
// ({"id": N, ...}), as returned by query/get_instance.
func TestDecodeEmbeddedID_Object(t *testing.T) {
	raw := json.RawMessage(`{"id": 5, "name": "host0"}`)
	id, err := decodeEmbeddedID(raw, "host")
	if err != nil {
		t.Fatalf("decodeEmbeddedID returned error: %v", err)
	}
	if id != 5 {
		t.Errorf("id = %v, want 5", id)
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
	if _, err := decodeEmbeddedID(raw, "host"); err == nil {
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
	if _, err := decodeEmbeddedID(json.RawMessage(`{}`), "host"); err == nil {
		t.Error("expected error for object without id, got nil")
	}
	if _, err := decodeEmbeddedID(json.RawMessage(`{"name":"x"}`), "host"); err == nil {
		t.Error("expected error for object without id, got nil")
	}
}

// TestDecodeEmbeddedID_ObjectNullID verifies that an object shape whose "id"
// key is explicitly null returns an error rather than silently defaulting
// to 0.
func TestDecodeEmbeddedID_ObjectNullID(t *testing.T) {
	if _, err := decodeEmbeddedID(json.RawMessage(`{"id":null}`), "host"); err == nil {
		t.Error("expected error for object with null id, got nil")
	}
}

// TestHostSubsysResponseToModel verifies that responseToModel decodes both
// embedded host and subsys objects and populates the model correctly.
func TestHostSubsysResponseToModel(t *testing.T) {
	ctx := context.Background()

	api := &hostSubsysAPI{
		ID:     11,
		Host:   json.RawMessage(`{"id": 3, "name": "host0"}`),
		Subsys: json.RawMessage(`{"id": 7, "name": "subsys0"}`),
	}

	var m HostSubsysModel
	diags := responseToModel(ctx, api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned diagnostic errors: %v", diags)
	}

	if m.ID.ValueInt64() != 11 {
		t.Errorf("ID = %v, want 11", m.ID.ValueInt64())
	}
	if m.HostID.ValueInt64() != 3 {
		t.Errorf("HostID = %v, want 3", m.HostID.ValueInt64())
	}
	if m.SubsysID.ValueInt64() != 7 {
		t.Errorf("SubsysID = %v, want 7", m.SubsysID.ValueInt64())
	}
}

// TestHostSubsysResponseToModel_BareIntShapes verifies responseToModel also
// handles bare-integer host/subsys shapes.
func TestHostSubsysResponseToModel_BareIntShapes(t *testing.T) {
	ctx := context.Background()

	api := &hostSubsysAPI{
		ID:     11,
		Host:   json.RawMessage(`3`),
		Subsys: json.RawMessage(`7`),
	}

	var m HostSubsysModel
	diags := responseToModel(ctx, api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned diagnostic errors: %v", diags)
	}
	if m.HostID.ValueInt64() != 3 {
		t.Errorf("HostID = %v, want 3", m.HostID.ValueInt64())
	}
	if m.SubsysID.ValueInt64() != 7 {
		t.Errorf("SubsysID = %v, want 7", m.SubsysID.ValueInt64())
	}
}

// TestHostSubsysResponseToModel_NullHost verifies that a null host field
// surfaces as a diagnostic error rather than silently zeroing HostID.
func TestHostSubsysResponseToModel_NullHost(t *testing.T) {
	ctx := context.Background()

	api := &hostSubsysAPI{
		ID:     11,
		Host:   json.RawMessage(`null`),
		Subsys: json.RawMessage(`{"id": 7}`),
	}

	var m HostSubsysModel
	diags := responseToModel(ctx, api, &m)
	if !diags.HasError() {
		t.Fatal("expected diagnostic error for null host field, got none")
	}
}

// TestHostSubsysResponseToModel_NullSubsys verifies that a null subsys
// field surfaces as a diagnostic error rather than silently zeroing
// SubsysID.
func TestHostSubsysResponseToModel_NullSubsys(t *testing.T) {
	ctx := context.Background()

	api := &hostSubsysAPI{
		ID:     11,
		Host:   json.RawMessage(`{"id": 3}`),
		Subsys: json.RawMessage(`null`),
	}

	var m HostSubsysModel
	diags := responseToModel(ctx, api, &m)
	if !diags.HasError() {
		t.Fatal("expected diagnostic error for null subsys field, got none")
	}
}

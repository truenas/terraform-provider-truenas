// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package static_route

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestStaticRouteSchema verifies that the resource schema has the expected
// attributes with correct types.
func TestStaticRouteSchema(t *testing.T) {
	s := resourceSchema()

	// id must be Int64Attribute and Computed.
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
	if len(idInt64.PlanModifiers) == 0 {
		t.Error("'id' should have a plan modifier (UseStateForUnknown)")
	}

	// destination must be StringAttribute, Required.
	destAttr, ok := s.Attributes["destination"]
	if !ok {
		t.Fatal("schema missing 'destination' attribute")
	}
	destStr, ok := destAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'destination' attribute is %T, want schema.StringAttribute", destAttr)
	}
	if !destStr.IsRequired() {
		t.Error("'destination' should be Required")
	}
	if destStr.IsOptional() || destStr.IsComputed() {
		t.Error("'destination' should be Required-only")
	}

	// gateway must be StringAttribute, Required.
	gwAttr, ok := s.Attributes["gateway"]
	if !ok {
		t.Fatal("schema missing 'gateway' attribute")
	}
	gwStr, ok := gwAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'gateway' attribute is %T, want schema.StringAttribute", gwAttr)
	}
	if !gwStr.IsRequired() {
		t.Error("'gateway' should be Required")
	}
	if gwStr.IsOptional() || gwStr.IsComputed() {
		t.Error("'gateway' should be Required-only")
	}

	// description must be StringAttribute, Optional and Computed, with a plan modifier.
	descAttr, ok := s.Attributes["description"]
	if !ok {
		t.Fatal("schema missing 'description' attribute")
	}
	descStr, ok := descAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'description' attribute is %T, want schema.StringAttribute", descAttr)
	}
	if !descStr.IsOptional() {
		t.Error("'description' should be Optional")
	}
	if !descStr.IsComputed() {
		t.Error("'description' should be Computed")
	}
	if len(descStr.PlanModifiers) == 0 {
		t.Error("'description' should have a plan modifier (UseStateForUnknown)")
	}
}

// TestStaticRouteApiPayload verifies that apiPayload produces a map with
// exactly the expected keys and values.
func TestStaticRouteApiPayload(t *testing.T) {
	ctx := context.Background()

	m := StaticRouteModel{
		Destination: types.StringValue("10.20.0.0/16"),
		Gateway:     types.StringValue("10.20.0.1"),
		Description: types.StringValue("test route"),
	}

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostic errors: %v", diags)
	}

	expectedKeys := []string{"destination", "gateway", "description"}
	if len(payload) != len(expectedKeys) {
		t.Errorf("payload has %d keys, want %d", len(payload), len(expectedKeys))
	}
	for _, k := range expectedKeys {
		if _, ok := payload[k]; !ok {
			t.Errorf("payload missing key %q", k)
		}
	}

	// id must NOT be in the payload.
	if _, ok := payload["id"]; ok {
		t.Error("payload should not contain key 'id'")
	}

	if payload["destination"] != "10.20.0.0/16" {
		t.Errorf("payload[destination] = %v, want '10.20.0.0/16'", payload["destination"])
	}
	if payload["gateway"] != "10.20.0.1" {
		t.Errorf("payload[gateway] = %v, want '10.20.0.1'", payload["gateway"])
	}
	if payload["description"] != "test route" {
		t.Errorf("payload[description] = %v, want 'test route'", payload["description"])
	}
}

// TestStaticRouteResponseToModel verifies that responseToModel populates all
// fields from the staticRouteAPI struct correctly.
func TestStaticRouteResponseToModel(t *testing.T) {
	ctx := context.Background()

	api := &staticRouteAPI{
		ID:          7,
		Destination: "10.20.0.0/16",
		Gateway:     "10.20.0.1",
		Description: "myroute",
	}

	var m StaticRouteModel
	diags := responseToModel(ctx, api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned diagnostic errors: %v", diags)
	}

	if m.ID.ValueInt64() != 7 {
		t.Errorf("ID = %v, want 7", m.ID.ValueInt64())
	}
	if m.Destination.ValueString() != "10.20.0.0/16" {
		t.Errorf("Destination = %v, want '10.20.0.0/16'", m.Destination.ValueString())
	}
	if m.Gateway.ValueString() != "10.20.0.1" {
		t.Errorf("Gateway = %v, want '10.20.0.1'", m.Gateway.ValueString())
	}
	if m.Description.ValueString() != "myroute" {
		t.Errorf("Description = %v, want 'myroute'", m.Description.ValueString())
	}
}

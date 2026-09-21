// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_initiator

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestISCSIInitiatorSchema verifies that the resource schema has the expected
// attributes with correct types.
func TestISCSIInitiatorSchema(t *testing.T) {
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

	// comment must be StringAttribute, Optional and Computed.
	commentAttr, ok := s.Attributes["comment"]
	if !ok {
		t.Fatal("schema missing 'comment' attribute")
	}
	commentStr, ok := commentAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'comment' attribute is %T, want schema.StringAttribute", commentAttr)
	}
	if !commentStr.IsOptional() {
		t.Error("'comment' should be Optional")
	}
	if !commentStr.IsComputed() {
		t.Error("'comment' should be Computed")
	}

	// initiators must be ListAttribute.
	initAttr, ok := s.Attributes["initiators"]
	if !ok {
		t.Fatal("schema missing 'initiators' attribute")
	}
	if _, ok := initAttr.(schema.ListAttribute); !ok {
		t.Errorf("'initiators' attribute is %T, want schema.ListAttribute", initAttr)
	}
}

// TestISCSIInitiatorApiPayload verifies that apiPayload produces a map with the
// expected keys and values.
func TestISCSIInitiatorApiPayload(t *testing.T) {
	ctx := context.Background()

	m := ISCSIInitiatorModel{
		Comment: types.StringValue("test comment"),
		Initiators: types.ListValueMust(types.StringType, []attr.Value{
			types.StringValue("iqn.2023-01.com.example:initiator1"),
		}),
	}

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostic errors: %v", diags)
	}

	expectedKeys := []string{"comment", "initiators"}
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

	if payload["comment"] != "test comment" {
		t.Errorf("payload[comment] = %v, want 'test comment'", payload["comment"])
	}

	initiators, ok := payload["initiators"].([]string)
	if !ok {
		t.Fatalf("payload[initiators] is %T, want []string", payload["initiators"])
	}
	if len(initiators) != 1 || initiators[0] != "iqn.2023-01.com.example:initiator1" {
		t.Errorf("payload[initiators] = %v, want [iqn.2023-01.com.example:initiator1]", initiators)
	}
}

// TestISCSIInitiatorApiPayload_NullInitiators verifies that a null initiators
// list defaults to an empty (non-nil) slice in the payload.
func TestISCSIInitiatorApiPayload_NullInitiators(t *testing.T) {
	ctx := context.Background()

	m := ISCSIInitiatorModel{
		Comment:    types.StringValue(""),
		Initiators: types.ListNull(types.StringType),
	}

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostic errors: %v", diags)
	}

	initiators, ok := payload["initiators"].([]string)
	if !ok {
		t.Fatalf("payload[initiators] is %T, want []string", payload["initiators"])
	}
	if initiators == nil {
		t.Error("payload[initiators] must not be nil, want empty slice")
	}
	if len(initiators) != 0 {
		t.Errorf("payload[initiators] = %v, want []", initiators)
	}
}

// TestISCSIInitiatorResponseToModel verifies that responseToModel populates all
// fields from the initiatorAPI struct correctly.
func TestISCSIInitiatorResponseToModel(t *testing.T) {
	ctx := context.Background()

	api := &initiatorAPI{
		ID:         7,
		Comment:    "myinitiator",
		Initiators: []string{"iqn.2023-01.com.example:host1", "iqn.2023-01.com.example:host2"},
	}

	var m ISCSIInitiatorModel
	diags := responseToModel(ctx, api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned diagnostic errors: %v", diags)
	}

	if m.ID.ValueInt64() != 7 {
		t.Errorf("ID = %v, want 7", m.ID.ValueInt64())
	}
	if m.Comment.ValueString() != "myinitiator" {
		t.Errorf("Comment = %v, want 'myinitiator'", m.Comment.ValueString())
	}

	var inits []string
	if diags := m.Initiators.ElementsAs(ctx, &inits, false); diags.HasError() {
		t.Fatalf("Initiators.ElementsAs failed: %v", diags)
	}
	if len(inits) != 2 {
		t.Fatalf("Initiators len = %d, want 2", len(inits))
	}
	if inits[0] != "iqn.2023-01.com.example:host1" {
		t.Errorf("Initiators[0] = %v, want iqn.2023-01.com.example:host1", inits[0])
	}
	if inits[1] != "iqn.2023-01.com.example:host2" {
		t.Errorf("Initiators[1] = %v, want iqn.2023-01.com.example:host2", inits[1])
	}
}

// TestISCSIInitiatorResponseToModel_NilInitiators verifies that a nil
// Initiators slice in the API response maps to an empty (non-nil) list.
func TestISCSIInitiatorResponseToModel_NilInitiators(t *testing.T) {
	ctx := context.Background()

	api := &initiatorAPI{
		ID:         1,
		Comment:    "",
		Initiators: nil,
	}

	var m ISCSIInitiatorModel
	diags := responseToModel(ctx, api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned diagnostic errors: %v", diags)
	}

	if m.Initiators.IsNull() {
		t.Error("Initiators should not be null when API returns nil slice")
	}

	var inits []string
	if diags := m.Initiators.ElementsAs(ctx, &inits, false); diags.HasError() {
		t.Fatalf("Initiators.ElementsAs failed: %v", diags)
	}
	if len(inits) != 0 {
		t.Errorf("Initiators = %v, want []", inits)
	}
}

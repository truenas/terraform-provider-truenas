// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_targetextent

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestTargetExtentSchema verifies that the resource schema has the expected
// attributes with correct types and plan modifiers.
func TestTargetExtentSchema(t *testing.T) {
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

	// target must be Required Int64 with RequiresReplace.
	targetAttr, ok := s.Attributes["target"]
	if !ok {
		t.Fatal("schema missing 'target' attribute")
	}
	targetInt64, ok := targetAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'target' attribute is %T, want schema.Int64Attribute", targetAttr)
	}
	if !targetInt64.IsRequired() {
		t.Error("'target' should be Required")
	}
	if !hasRequiresReplace(targetInt64.PlanModifiers) {
		t.Error("'target' should have RequiresReplace plan modifier")
	}

	// extent must be Required Int64 with RequiresReplace.
	extentAttr, ok := s.Attributes["extent"]
	if !ok {
		t.Fatal("schema missing 'extent' attribute")
	}
	extentInt64, ok := extentAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'extent' attribute is %T, want schema.Int64Attribute", extentAttr)
	}
	if !extentInt64.IsRequired() {
		t.Error("'extent' should be Required")
	}
	if !hasRequiresReplace(extentInt64.PlanModifiers) {
		t.Error("'extent' should have RequiresReplace plan modifier")
	}

	// lunid must be Optional+Computed Int64 with UseStateForUnknown.
	lunidAttr, ok := s.Attributes["lunid"]
	if !ok {
		t.Fatal("schema missing 'lunid' attribute")
	}
	lunidInt64, ok := lunidAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'lunid' attribute is %T, want schema.Int64Attribute", lunidAttr)
	}
	if !lunidInt64.IsOptional() {
		t.Error("'lunid' should be Optional")
	}
	if !lunidInt64.IsComputed() {
		t.Error("'lunid' should be Computed")
	}
	if !hasUseStateForUnknown(lunidInt64.PlanModifiers) {
		t.Error("'lunid' should have UseStateForUnknown plan modifier")
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

// TestTargetExtentApiPayload_WithLunID verifies that apiPayload includes
// lunid when it is known.
func TestTargetExtentApiPayload_WithLunID(t *testing.T) {
	ctx := context.Background()

	m := TargetExtentModel{
		Target: types.Int64Value(1),
		Extent: types.Int64Value(2),
		LunID:  types.Int64Value(5),
	}

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostic errors: %v", diags)
	}

	if len(payload) != 3 {
		t.Fatalf("payload has %d keys, want 3: %v", len(payload), payload)
	}
	for _, k := range []string{"target", "extent", "lunid"} {
		if _, ok := payload[k]; !ok {
			t.Errorf("payload missing key %q", k)
		}
	}
	if payload["target"] != int64(1) {
		t.Errorf("payload[target] = %v, want 1", payload["target"])
	}
	if payload["extent"] != int64(2) {
		t.Errorf("payload[extent] = %v, want 2", payload["extent"])
	}
	if payload["lunid"] != int64(5) {
		t.Errorf("payload[lunid] = %v, want 5", payload["lunid"])
	}
}

// TestTargetExtentApiPayload_NullLunID verifies that apiPayload omits lunid
// when it is null, so the API can auto-assign.
func TestTargetExtentApiPayload_NullLunID(t *testing.T) {
	ctx := context.Background()

	m := TargetExtentModel{
		Target: types.Int64Value(1),
		Extent: types.Int64Value(2),
		LunID:  types.Int64Null(),
	}

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostic errors: %v", diags)
	}

	if len(payload) != 2 {
		t.Fatalf("payload has %d keys, want 2: %v", len(payload), payload)
	}
	if _, ok := payload["lunid"]; ok {
		t.Error("payload should not contain key 'lunid' when LunID is null")
	}
}

// TestTargetExtentApiPayload_UnknownLunID verifies that apiPayload omits
// lunid when it is unknown (e.g. during plan before Create has run).
func TestTargetExtentApiPayload_UnknownLunID(t *testing.T) {
	ctx := context.Background()

	m := TargetExtentModel{
		Target: types.Int64Value(1),
		Extent: types.Int64Value(2),
		LunID:  types.Int64Unknown(),
	}

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostic errors: %v", diags)
	}

	if len(payload) != 2 {
		t.Fatalf("payload has %d keys, want 2: %v", len(payload), payload)
	}
	if _, ok := payload["lunid"]; ok {
		t.Error("payload should not contain key 'lunid' when LunID is unknown")
	}
}

// TestTargetExtentResponseToModel verifies that responseToModel populates
// all fields from the targetExtentAPI struct correctly.
func TestTargetExtentResponseToModel(t *testing.T) {
	ctx := context.Background()

	api := &targetExtentAPI{
		ID:     2,
		Target: 1,
		Extent: 2,
		LunID:  0,
	}

	var m TargetExtentModel
	diags := responseToModel(ctx, api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned diagnostic errors: %v", diags)
	}

	if m.ID.ValueInt64() != 2 {
		t.Errorf("ID = %v, want 2", m.ID.ValueInt64())
	}
	if m.Target.ValueInt64() != 1 {
		t.Errorf("Target = %v, want 1", m.Target.ValueInt64())
	}
	if m.Extent.ValueInt64() != 2 {
		t.Errorf("Extent = %v, want 2", m.Extent.ValueInt64())
	}
	if m.LunID.ValueInt64() != 0 {
		t.Errorf("LunID = %v, want 0", m.LunID.ValueInt64())
	}
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_port

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestNVMetPortSchema verifies key schema attribute shapes.
func TestNVMetPortSchema(t *testing.T) {
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

	trtypeAttr, ok := s.Attributes["addr_trtype"]
	if !ok {
		t.Fatal("schema missing 'addr_trtype' attribute")
	}
	trtypeStr, ok := trtypeAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'addr_trtype' attribute is %T, want schema.StringAttribute", trtypeAttr)
	}
	if !trtypeStr.IsRequired() {
		t.Error("'addr_trtype' should be Required")
	}
	if len(trtypeStr.PlanModifiers) == 0 {
		t.Error("'addr_trtype' should have plan modifiers (RequiresReplace)")
	}

	traddrAttr, ok := s.Attributes["addr_traddr"]
	if !ok {
		t.Fatal("schema missing 'addr_traddr' attribute")
	}
	traddrStr, ok := traddrAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'addr_traddr' attribute is %T, want schema.StringAttribute", traddrAttr)
	}
	if !traddrStr.IsRequired() {
		t.Error("'addr_traddr' should be Required")
	}

	optionalComputedInt := []string{"addr_trsvcid", "inline_data_size", "max_queue_size"}
	for _, field := range optionalComputedInt {
		attr, ok := s.Attributes[field]
		if !ok {
			t.Fatalf("schema missing %q attribute", field)
		}
		intAttr, ok := attr.(schema.Int64Attribute)
		if !ok {
			t.Errorf("%q attribute is %T, want schema.Int64Attribute", field, attr)
			continue
		}
		if !intAttr.IsOptional() || !intAttr.IsComputed() {
			t.Errorf("%q should be Optional and Computed", field)
		}
	}

	optionalComputedBool := []string{"enabled", "pi_enable"}
	for _, field := range optionalComputedBool {
		attr, ok := s.Attributes[field]
		if !ok {
			t.Fatalf("schema missing %q attribute", field)
		}
		boolAttr, ok := attr.(schema.BoolAttribute)
		if !ok {
			t.Errorf("%q attribute is %T, want schema.BoolAttribute", field, attr)
			continue
		}
		if !boolAttr.IsOptional() || !boolAttr.IsComputed() {
			t.Errorf("%q should be Optional and Computed", field)
		}
	}

	indexAttr, ok := s.Attributes["index"]
	if !ok {
		t.Fatal("schema missing 'index' attribute")
	}
	indexInt64, ok := indexAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'index' attribute is %T, want schema.Int64Attribute", indexAttr)
	}
	if !indexInt64.IsComputed() {
		t.Error("'index' should be Computed")
	}
	if indexInt64.IsOptional() || indexInt64.IsRequired() {
		t.Error("'index' should be Computed-only (not Optional/Required)")
	}

	adrfamAttr, ok := s.Attributes["addr_adrfam"]
	if !ok {
		t.Fatal("schema missing 'addr_adrfam' attribute")
	}
	adrfamStr, ok := adrfamAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'addr_adrfam' attribute is %T, want schema.StringAttribute", adrfamAttr)
	}
	if !adrfamStr.IsComputed() {
		t.Error("'addr_adrfam' should be Computed")
	}
	if adrfamStr.IsOptional() || adrfamStr.IsRequired() {
		t.Error("'addr_adrfam' should be Computed-only (not Optional/Required)")
	}
}

// TestCreatePayload_TrtypeAndTraddrAlwaysPresent verifies that "addr_trtype"
// and "addr_traddr" are always sent on create, even when every optional
// field is null/unknown.
func TestCreatePayload_TrtypeAndTraddrAlwaysPresent(t *testing.T) {
	ctx := context.Background()

	m := NVMetPortModel{
		AddrTrtype:     types.StringValue("TCP"),
		AddrTraddr:     types.StringValue("192.0.2.10"),
		AddrTrsvcid:    types.Int64Null(),
		Enabled:        types.BoolNull(),
		InlineDataSize: types.Int64Null(),
		MaxQueueSize:   types.Int64Null(),
		PIEnable:       types.BoolNull(),
	}

	payload, diags := m.createPayload(ctx)
	if diags.HasError() {
		t.Fatalf("createPayload returned diagnostic errors: %v", diags)
	}

	if payload["addr_trtype"] != "TCP" {
		t.Errorf("payload[addr_trtype] = %v, want TCP", payload["addr_trtype"])
	}
	if payload["addr_traddr"] != "192.0.2.10" {
		t.Errorf("payload[addr_traddr] = %v, want 192.0.2.10", payload["addr_traddr"])
	}
	for _, key := range []string{"addr_trsvcid", "enabled", "inline_data_size", "max_queue_size", "pi_enable"} {
		if _, ok := payload[key]; ok {
			t.Errorf("expected %q to be omitted (null), got %v", key, payload[key])
		}
	}
	if len(payload) != 2 {
		t.Errorf("payload has %d keys (%v), want 2", len(payload), payload)
	}
}

// TestCreatePayload_AllFieldsKnown verifies that every guarded field is
// present when all model values are known.
func TestCreatePayload_AllFieldsKnown(t *testing.T) {
	ctx := context.Background()

	m := NVMetPortModel{
		AddrTrtype:     types.StringValue("TCP"),
		AddrTraddr:     types.StringValue("192.0.2.10"),
		AddrTrsvcid:    types.Int64Value(14420),
		Enabled:        types.BoolValue(true),
		InlineDataSize: types.Int64Value(8192),
		MaxQueueSize:   types.Int64Value(32),
		PIEnable:       types.BoolValue(true),
	}

	payload, diags := m.createPayload(ctx)
	if diags.HasError() {
		t.Fatalf("createPayload returned diagnostic errors: %v", diags)
	}

	want := map[string]any{
		"addr_trtype":      "TCP",
		"addr_traddr":      "192.0.2.10",
		"addr_trsvcid":     int64(14420),
		"enabled":          true,
		"inline_data_size": int64(8192),
		"max_queue_size":   int64(32),
		"pi_enable":        true,
	}
	for k, v := range want {
		if payload[k] != v {
			t.Errorf("payload[%q] = %v, want %v", k, payload[k], v)
		}
	}
	if len(payload) != len(want) {
		t.Errorf("payload has %d keys (%v), want %d", len(payload), payload, len(want))
	}
}

// TestUpdatePayload_NoAddrTrtypeKey verifies that "addr_trtype" is never
// included in the update payload, even when the model's AddrTrtype field is
// known.
func TestUpdatePayload_NoAddrTrtypeKey(t *testing.T) {
	ctx := context.Background()

	m := NVMetPortModel{
		AddrTrtype:     types.StringValue("TCP"),
		AddrTraddr:     types.StringValue("192.0.2.10"),
		AddrTrsvcid:    types.Int64Value(14420),
		Enabled:        types.BoolValue(true),
		InlineDataSize: types.Int64Null(),
		MaxQueueSize:   types.Int64Null(),
		PIEnable:       types.BoolNull(),
	}

	payload, diags := m.updatePayload(ctx)
	if diags.HasError() {
		t.Fatalf("updatePayload returned diagnostic errors: %v", diags)
	}

	if _, ok := payload["addr_trtype"]; ok {
		t.Error("update payload must never contain 'addr_trtype' key")
	}
	if payload["addr_traddr"] != "192.0.2.10" {
		t.Errorf("payload[addr_traddr] = %v, want 192.0.2.10", payload["addr_traddr"])
	}
	if payload["addr_trsvcid"] != int64(14420) {
		t.Errorf("payload[addr_trsvcid] = %v, want 14420", payload["addr_trsvcid"])
	}
	if payload["enabled"] != true {
		t.Errorf("payload[enabled] = %v, want true", payload["enabled"])
	}
	for _, key := range []string{"inline_data_size", "max_queue_size", "pi_enable"} {
		if _, ok := payload[key]; ok {
			t.Errorf("expected %q to be omitted (null), got %v", key, payload[key])
		}
	}
}

// TestResponseToModel_NilPointers verifies that nil inline_data_size/
// max_queue_size/pi_enable fields from the API map to null rather than a
// zero value, so that a subsequent update payload built from this model
// omits them (see guardedFields) instead of sending an explicit 0/false
// that would overwrite a server-side null.
func TestResponseToModel_NilPointers(t *testing.T) {
	ctx := context.Background()

	api := &nvmetPortAPI{
		ID:             1,
		AddrTrtype:     "TCP",
		AddrTraddr:     "192.0.2.10",
		AddrTrsvcid:    4420,
		Enabled:        true,
		InlineDataSize: nil,
		MaxQueueSize:   nil,
		PIEnable:       nil,
		Index:          1,
		AddrAdrfam:     "IPV4",
	}

	var m NVMetPortModel
	diags := responseToModel(ctx, api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned diagnostic errors: %v", diags)
	}

	if !m.InlineDataSize.IsNull() {
		t.Errorf("InlineDataSize = %v, want null when API returns nil", m.InlineDataSize)
	}
	if !m.MaxQueueSize.IsNull() {
		t.Errorf("MaxQueueSize = %v, want null when API returns nil", m.MaxQueueSize)
	}
	if !m.PIEnable.IsNull() {
		t.Errorf("PIEnable = %v, want null when API returns nil", m.PIEnable)
	}
}

// TestResponseToModel_AllFields verifies that non-nil fields are correctly
// mapped from the API response, matching the live query shape from the box.
func TestResponseToModel_AllFields(t *testing.T) {
	ctx := context.Background()

	inlineDataSize := int64(8192)
	maxQueueSize := int64(32)
	piEnable := true

	api := &nvmetPortAPI{
		ID:             1,
		AddrTrtype:     "TCP",
		AddrTraddr:     "192.0.2.10",
		AddrTrsvcid:    4420,
		Enabled:        true,
		InlineDataSize: &inlineDataSize,
		MaxQueueSize:   &maxQueueSize,
		PIEnable:       &piEnable,
		Index:          1,
		AddrAdrfam:     "IPV4",
	}

	var m NVMetPortModel
	diags := responseToModel(ctx, api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned diagnostic errors: %v", diags)
	}

	if m.ID.ValueInt64() != 1 {
		t.Errorf("ID = %v, want 1", m.ID.ValueInt64())
	}
	if m.AddrTrtype.ValueString() != "TCP" {
		t.Errorf("AddrTrtype = %v, want TCP", m.AddrTrtype.ValueString())
	}
	if m.AddrTraddr.ValueString() != "192.0.2.10" {
		t.Errorf("AddrTraddr = %v, want 192.0.2.10", m.AddrTraddr.ValueString())
	}
	if m.AddrTrsvcid.ValueInt64() != 4420 {
		t.Errorf("AddrTrsvcid = %v, want 4420", m.AddrTrsvcid.ValueInt64())
	}
	if !m.Enabled.ValueBool() {
		t.Error("Enabled should be true")
	}
	if m.InlineDataSize.ValueInt64() != 8192 {
		t.Errorf("InlineDataSize = %v, want 8192", m.InlineDataSize.ValueInt64())
	}
	if m.MaxQueueSize.ValueInt64() != 32 {
		t.Errorf("MaxQueueSize = %v, want 32", m.MaxQueueSize.ValueInt64())
	}
	if !m.PIEnable.ValueBool() {
		t.Error("PIEnable should be true")
	}
	if m.Index.ValueInt64() != 1 {
		t.Errorf("Index = %v, want 1", m.Index.ValueInt64())
	}
	if m.AddrAdrfam.ValueString() != "IPV4" {
		t.Errorf("AddrAdrfam = %v, want IPV4", m.AddrAdrfam.ValueString())
	}
}

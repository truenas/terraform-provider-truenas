// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package tunable

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestTunableSchema verifies key schema attributes.
func TestTunableSchema(t *testing.T) {
	s := resourceSchema()

	// id must be Int64Attribute and Computed.
	idAttr, ok := s.Attributes["id"]
	if !ok {
		t.Fatal("schema missing 'id' attribute")
	}
	idInt64, ok := idAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'id' is %T, want schema.Int64Attribute", idAttr)
	}
	if !idInt64.IsComputed() {
		t.Error("'id' should be Computed")
	}

	// var must be Required with RequiresReplace.
	varAttr, ok := s.Attributes["var"]
	if !ok {
		t.Fatal("schema missing 'var' attribute")
	}
	varStr, ok := varAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'var' is %T, want schema.StringAttribute", varAttr)
	}
	if !varStr.IsRequired() {
		t.Error("'var' should be Required")
	}
	if len(varStr.PlanModifiers) == 0 {
		t.Error("'var' should have plan modifiers (RequiresReplace)")
	}

	// value must be Required, no Computed.
	valueAttr, ok := s.Attributes["value"]
	if !ok {
		t.Fatal("schema missing 'value' attribute")
	}
	valueStr, ok := valueAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'value' is %T, want schema.StringAttribute", valueAttr)
	}
	if !valueStr.IsRequired() {
		t.Error("'value' should be Required")
	}
	if valueStr.IsComputed() {
		t.Error("'value' should not be Computed")
	}

	// type must be Optional+Computed with RequiresReplace plan modifier.
	typeAttr, ok := s.Attributes["type"]
	if !ok {
		t.Fatal("schema missing 'type' attribute")
	}
	typeStr, ok := typeAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'type' is %T, want schema.StringAttribute", typeAttr)
	}
	if !typeStr.IsOptional() {
		t.Error("'type' should be Optional")
	}
	if !typeStr.IsComputed() {
		t.Error("'type' should be Computed")
	}
	if len(typeStr.PlanModifiers) < 2 {
		t.Error("'type' should have plan modifiers (RequiresReplace + UseStateForUnknown)")
	}

	// update_initramfs must be Optional Bool ONLY: never Computed.
	uiAttr, ok := s.Attributes["update_initramfs"]
	if !ok {
		t.Fatal("schema missing 'update_initramfs' attribute")
	}
	uiBool, ok := uiAttr.(schema.BoolAttribute)
	if !ok {
		t.Fatalf("'update_initramfs' is %T, want schema.BoolAttribute", uiAttr)
	}
	if !uiBool.IsOptional() {
		t.Error("'update_initramfs' should be Optional")
	}
	if uiBool.IsComputed() {
		t.Error("'update_initramfs' must NOT be Computed (write-only)")
	}
	if uiBool.IsRequired() {
		t.Error("'update_initramfs' must not be Required")
	}

	// orig_value must be Computed with NO plan modifiers.
	origAttr, ok := s.Attributes["orig_value"]
	if !ok {
		t.Fatal("schema missing 'orig_value' attribute")
	}
	origStr, ok := origAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'orig_value' is %T, want schema.StringAttribute", origAttr)
	}
	if !origStr.IsComputed() {
		t.Error("'orig_value' should be Computed")
	}
	if origStr.IsOptional() || origStr.IsRequired() {
		t.Error("'orig_value' should be Computed-only")
	}
	if len(origStr.PlanModifiers) != 0 {
		t.Error("'orig_value' must NOT have plan modifiers")
	}

	// comment and enabled must be Optional+Computed with UseStateForUnknown.
	commentAttr, ok := s.Attributes["comment"]
	if !ok {
		t.Fatal("schema missing 'comment' attribute")
	}
	commentStr, ok := commentAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'comment' is %T, want schema.StringAttribute", commentAttr)
	}
	if !commentStr.IsOptional() || !commentStr.IsComputed() {
		t.Error("'comment' should be Optional+Computed")
	}
	if len(commentStr.PlanModifiers) == 0 {
		t.Error("'comment' should have plan modifiers (UseStateForUnknown)")
	}

	enabledAttr, ok := s.Attributes["enabled"]
	if !ok {
		t.Fatal("schema missing 'enabled' attribute")
	}
	enabledBool, ok := enabledAttr.(schema.BoolAttribute)
	if !ok {
		t.Fatalf("'enabled' is %T, want schema.BoolAttribute", enabledAttr)
	}
	if !enabledBool.IsOptional() || !enabledBool.IsComputed() {
		t.Error("'enabled' should be Optional+Computed")
	}
	if len(enabledBool.PlanModifiers) == 0 {
		t.Error("'enabled' should have plan modifiers (UseStateForUnknown)")
	}
}

// TestTunableCreatePayload verifies createPayload always includes var/value
// and omits optionals that are unset.
func TestTunableCreatePayload(t *testing.T) {
	ctx := context.Background()

	m := TunableModel{
		Var:             types.StringValue("kernel.threads-max"),
		Value:           types.StringValue("100000"),
		Type:            types.StringNull(),
		Comment:         types.StringNull(),
		Enabled:         types.BoolNull(),
		UpdateInitramfs: types.BoolNull(),
	}

	payload, diags := m.createPayload(ctx)
	if diags.HasError() {
		t.Fatalf("createPayload returned errors: %v", diags)
	}

	if v, ok := payload["var"]; !ok {
		t.Error("createPayload missing 'var'")
	} else if v != "kernel.threads-max" {
		t.Errorf("payload[var] = %v, want kernel.threads-max", v)
	}
	if v, ok := payload["value"]; !ok {
		t.Error("createPayload missing 'value'")
	} else if v != "100000" {
		t.Errorf("payload[value] = %v, want 100000", v)
	}

	for _, unset := range []string{"type", "comment", "enabled", "update_initramfs"} {
		if _, ok := payload[unset]; ok {
			t.Errorf("createPayload should omit unset field %q", unset)
		}
	}

	// id must never be in payload.
	if _, ok := payload["id"]; ok {
		t.Error("createPayload should not contain 'id'")
	}
}

// TestTunableCreatePayload_AllSet verifies optionals are included when known.
func TestTunableCreatePayload_AllSet(t *testing.T) {
	ctx := context.Background()

	m := TunableModel{
		Var:             types.StringValue("kernel.threads-max"),
		Value:           types.StringValue("100000"),
		Type:            types.StringValue("SYSCTL"),
		Comment:         types.StringValue("bump thread limit"),
		Enabled:         types.BoolValue(true),
		UpdateInitramfs: types.BoolValue(true),
	}

	payload, diags := m.createPayload(ctx)
	if diags.HasError() {
		t.Fatalf("createPayload returned errors: %v", diags)
	}

	if v, ok := payload["type"]; !ok || v != "SYSCTL" {
		t.Errorf("payload[type] = %v, want SYSCTL", v)
	}
	if v, ok := payload["comment"]; !ok || v != "bump thread limit" {
		t.Errorf("payload[comment] = %v, want 'bump thread limit'", v)
	}
	if v, ok := payload["enabled"]; !ok || v != true {
		t.Errorf("payload[enabled] = %v, want true", v)
	}
	if v, ok := payload["update_initramfs"]; !ok || v != true {
		t.Errorf("payload[update_initramfs] = %v, want true", v)
	}
}

// TestTunableUpdatePayload verifies updatePayload always includes value and
// never includes var/type (immutable after create).
func TestTunableUpdatePayload(t *testing.T) {
	ctx := context.Background()

	m := TunableModel{
		Var:             types.StringValue("kernel.threads-max"),
		Value:           types.StringValue("200000"),
		Type:            types.StringValue("SYSCTL"),
		Comment:         types.StringValue("updated"),
		Enabled:         types.BoolValue(false),
		UpdateInitramfs: types.BoolNull(),
	}

	payload, diags := m.updatePayload(ctx)
	if diags.HasError() {
		t.Fatalf("updatePayload returned errors: %v", diags)
	}

	if v, ok := payload["value"]; !ok || v != "200000" {
		t.Errorf("payload[value] = %v, want 200000", v)
	}

	if _, ok := payload["var"]; ok {
		t.Error("updatePayload must not contain 'var'")
	}
	if _, ok := payload["type"]; ok {
		t.Error("updatePayload must not contain 'type'")
	}
	if _, ok := payload["update_initramfs"]; ok {
		t.Error("updatePayload must not contain 'update_initramfs' when null")
	}

	if v, ok := payload["comment"]; !ok || v != "updated" {
		t.Errorf("payload[comment] = %v, want updated", v)
	}
	if v, ok := payload["enabled"]; !ok || v != false {
		t.Errorf("payload[enabled] = %v, want false", v)
	}
}

// TestTunableUpdatePayload_UpdateInitramfsGuarded verifies update_initramfs
// is included in updatePayload only when set.
func TestTunableUpdatePayload_UpdateInitramfsGuarded(t *testing.T) {
	ctx := context.Background()

	m := TunableModel{
		Var:             types.StringValue("kernel.threads-max"),
		Value:           types.StringValue("200000"),
		UpdateInitramfs: types.BoolValue(true),
	}

	payload, diags := m.updatePayload(ctx)
	if diags.HasError() {
		t.Fatalf("updatePayload returned errors: %v", diags)
	}
	if v, ok := payload["update_initramfs"]; !ok || v != true {
		t.Errorf("payload[update_initramfs] = %v, want true", v)
	}
}

// TestResponseToModel verifies responseToModel populates all fields and
// never touches UpdateInitramfs (write-only).
func TestResponseToModel(t *testing.T) {
	ctx := context.Background()

	api := &tunableAPI{
		ID:        42,
		Var:       "kernel.threads-max",
		Value:     "100000",
		Type:      "SYSCTL",
		Comment:   "a comment",
		Enabled:   true,
		OrigValue: "50000",
	}

	// Pre-populate UpdateInitramfs to prove responseToModel leaves it alone.
	m := TunableModel{UpdateInitramfs: types.BoolValue(true)}
	diags := responseToModel(ctx, api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned errors: %v", diags)
	}

	if m.ID.ValueInt64() != 42 {
		t.Errorf("ID = %v, want 42", m.ID.ValueInt64())
	}
	if m.Var.ValueString() != "kernel.threads-max" {
		t.Errorf("Var = %v, want kernel.threads-max", m.Var.ValueString())
	}
	if m.Value.ValueString() != "100000" {
		t.Errorf("Value = %v, want 100000", m.Value.ValueString())
	}
	if m.Type.ValueString() != "SYSCTL" {
		t.Errorf("Type = %v, want SYSCTL", m.Type.ValueString())
	}
	if m.Comment.ValueString() != "a comment" {
		t.Errorf("Comment = %v, want 'a comment'", m.Comment.ValueString())
	}
	if !m.Enabled.ValueBool() {
		t.Error("Enabled = false, want true")
	}
	if m.OrigValue.ValueString() != "50000" {
		t.Errorf("OrigValue = %v, want 50000", m.OrigValue.ValueString())
	}

	// UpdateInitramfs must be untouched by responseToModel.
	if !m.UpdateInitramfs.ValueBool() {
		t.Error("responseToModel must not modify UpdateInitramfs")
	}
}

// TestDecodeCreateResult_ObjectShape verifies decodeCreateResult handles a
// job result that is the full created tunable object.
func TestDecodeCreateResult_ObjectShape(t *testing.T) {
	raw := json.RawMessage(`{"id":7,"var":"kernel.threads-max","value":"100000","type":"SYSCTL","comment":"","enabled":true,"orig_value":"50000"}`)

	api, id, err := decodeCreateResult(raw)
	if err != nil {
		t.Fatalf("decodeCreateResult returned error: %v", err)
	}
	if api == nil {
		t.Fatal("decodeCreateResult should return non-nil api for object shape")
	}
	if id != 0 {
		t.Errorf("decodeCreateResult id = %v, want 0 for object shape", id)
	}
	if api.ID != 7 {
		t.Errorf("api.ID = %v, want 7", api.ID)
	}
	if api.Var != "kernel.threads-max" {
		t.Errorf("api.Var = %v, want kernel.threads-max", api.Var)
	}
}

// TestDecodeCreateResult_BareIntShape verifies decodeCreateResult handles a
// job result that is a bare integer ID.
func TestDecodeCreateResult_BareIntShape(t *testing.T) {
	raw := json.RawMessage(`7`)

	api, id, err := decodeCreateResult(raw)
	if err != nil {
		t.Fatalf("decodeCreateResult returned error: %v", err)
	}
	if api != nil {
		t.Errorf("decodeCreateResult api = %+v, want nil for bare-int shape", api)
	}
	if id != 7 {
		t.Errorf("decodeCreateResult id = %v, want 7", id)
	}
}

// TestDecodeCreateResult_Invalid verifies decodeCreateResult returns an
// error for unparseable input.
func TestDecodeCreateResult_Invalid(t *testing.T) {
	raw := json.RawMessage(`"not valid"`)

	_, _, err := decodeCreateResult(raw)
	if err == nil {
		t.Fatal("decodeCreateResult should return an error for unparseable input")
	}
}

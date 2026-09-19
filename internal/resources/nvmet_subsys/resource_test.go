// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_subsys

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestNVMetSubsysSchema verifies key schema attribute shapes.
func TestNVMetSubsysSchema(t *testing.T) {
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

	optionalComputedString := []string{"subnqn", "ieee_oui"}
	for _, field := range optionalComputedString {
		attr, ok := s.Attributes[field]
		if !ok {
			t.Fatalf("schema missing %q attribute", field)
		}
		strAttr, ok := attr.(schema.StringAttribute)
		if !ok {
			t.Errorf("%q attribute is %T, want schema.StringAttribute", field, attr)
			continue
		}
		if !strAttr.IsOptional() || !strAttr.IsComputed() {
			t.Errorf("%q should be Optional and Computed", field)
		}
	}

	optionalComputedBool := []string{"allow_any_host", "ana", "pi_enable"}
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

	qidMaxAttr, ok := s.Attributes["qid_max"]
	if !ok {
		t.Fatal("schema missing 'qid_max' attribute")
	}
	qidMaxInt64, ok := qidMaxAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'qid_max' attribute is %T, want schema.Int64Attribute", qidMaxAttr)
	}
	if !qidMaxInt64.IsOptional() || !qidMaxInt64.IsComputed() {
		t.Error("'qid_max' should be Optional and Computed")
	}

	serialAttr, ok := s.Attributes["serial"]
	if !ok {
		t.Fatal("schema missing 'serial' attribute")
	}
	serialStr, ok := serialAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'serial' attribute is %T, want schema.StringAttribute", serialAttr)
	}
	if !serialStr.IsComputed() {
		t.Error("'serial' should be Computed")
	}
	if serialStr.IsOptional() || serialStr.IsRequired() {
		t.Error("'serial' should be Computed-only (not Optional/Required)")
	}
}

// TestCreatePayload_NameAlwaysPresent verifies that "name" is always sent on
// create, even when every optional field is null/unknown.
func TestCreatePayload_NameAlwaysPresent(t *testing.T) {
	ctx := context.Background()

	m := NVMetSubsysModel{
		Name:         types.StringValue("test-subsys"),
		SubNQN:       types.StringNull(),
		AllowAnyHost: types.BoolNull(),
		ANA:          types.BoolNull(),
		IEEEOUI:      types.StringNull(),
		PIEnable:     types.BoolNull(),
		QIDMax:       types.Int64Null(),
	}

	payload, diags := m.createPayload(ctx)
	if diags.HasError() {
		t.Fatalf("createPayload returned diagnostic errors: %v", diags)
	}

	if payload["name"] != "test-subsys" {
		t.Errorf("payload[name] = %v, want test-subsys", payload["name"])
	}
	for _, key := range []string{"subnqn", "allow_any_host", "ana", "ieee_oui", "pi_enable", "qid_max"} {
		if _, ok := payload[key]; ok {
			t.Errorf("expected %q to be omitted (null), got %v", key, payload[key])
		}
	}
	if len(payload) != 1 {
		t.Errorf("payload has %d keys (%v), want 1", len(payload), payload)
	}
}

// TestCreatePayload_EmptySubNQNSendsNull verifies the three-way rule for
// subnqn/ieee_oui: null/unknown is omitted, but an explicitly known empty
// string is sent as an explicit JSON null (clear), distinct from omission.
func TestCreatePayload_EmptySubNQNSendsNull(t *testing.T) {
	ctx := context.Background()

	m := NVMetSubsysModel{
		Name:     types.StringValue("test-subsys"),
		SubNQN:   types.StringValue(""),
		IEEEOUI:  types.StringValue(""),
		ANA:      types.BoolNull(),
		PIEnable: types.BoolNull(),
		QIDMax:   types.Int64Null(),
	}

	payload, diags := m.createPayload(ctx)
	if diags.HasError() {
		t.Fatalf("createPayload returned diagnostic errors: %v", diags)
	}

	subnqn, ok := payload["subnqn"]
	if !ok {
		t.Fatal("expected 'subnqn' key present (explicit null) for empty string, got omitted")
	}
	if subnqn != nil {
		t.Errorf("payload[subnqn] = %v, want nil", subnqn)
	}

	ieeeOUI, ok := payload["ieee_oui"]
	if !ok {
		t.Fatal("expected 'ieee_oui' key present (explicit null) for empty string, got omitted")
	}
	if ieeeOUI != nil {
		t.Errorf("payload[ieee_oui] = %v, want nil", ieeeOUI)
	}
}

// TestCreatePayload_NullSubNQNOmitted verifies that a null/unknown subnqn/
// ieee_oui (never set in config) is omitted entirely, distinct from an
// explicit empty string which sends a JSON null.
func TestCreatePayload_NullSubNQNOmitted(t *testing.T) {
	ctx := context.Background()

	m := NVMetSubsysModel{
		Name:     types.StringValue("test-subsys"),
		SubNQN:   types.StringNull(),
		IEEEOUI:  types.StringUnknown(),
		ANA:      types.BoolNull(),
		PIEnable: types.BoolNull(),
		QIDMax:   types.Int64Null(),
	}

	payload, diags := m.createPayload(ctx)
	if diags.HasError() {
		t.Fatalf("createPayload returned diagnostic errors: %v", diags)
	}

	if _, ok := payload["subnqn"]; ok {
		t.Errorf("expected 'subnqn' to be omitted for null, got %v", payload["subnqn"])
	}
	if _, ok := payload["ieee_oui"]; ok {
		t.Errorf("expected 'ieee_oui' to be omitted for unknown, got %v", payload["ieee_oui"])
	}
}

// TestCreatePayload_AllFieldsKnown verifies that every guarded field is
// present when all model values are known and non-empty.
func TestCreatePayload_AllFieldsKnown(t *testing.T) {
	ctx := context.Background()

	m := NVMetSubsysModel{
		Name:         types.StringValue("test-subsys"),
		SubNQN:       types.StringValue("nqn.2011-06.com.truenas:uuid:custom"),
		AllowAnyHost: types.BoolValue(true),
		ANA:          types.BoolValue(true),
		IEEEOUI:      types.StringValue("00:0A:0B"),
		PIEnable:     types.BoolValue(true),
		QIDMax:       types.Int64Value(16),
	}

	payload, diags := m.createPayload(ctx)
	if diags.HasError() {
		t.Fatalf("createPayload returned diagnostic errors: %v", diags)
	}

	want := map[string]any{
		"name":           "test-subsys",
		"subnqn":         "nqn.2011-06.com.truenas:uuid:custom",
		"allow_any_host": true,
		"ana":            true,
		"ieee_oui":       "00:0A:0B",
		"pi_enable":      true,
		"qid_max":        int64(16),
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

// TestUpdatePayload_NoNameKey verifies that "name" is never included in the
// update payload, even when the model's Name field is known.
func TestUpdatePayload_NoNameKey(t *testing.T) {
	ctx := context.Background()

	m := NVMetSubsysModel{
		Name:         types.StringValue("test-subsys"),
		SubNQN:       types.StringValue("nqn.2011-06.com.truenas:uuid:custom"),
		AllowAnyHost: types.BoolValue(true),
		ANA:          types.BoolValue(false),
		IEEEOUI:      types.StringValue(""),
		PIEnable:     types.BoolNull(),
		QIDMax:       types.Int64Null(),
	}

	payload, diags := m.updatePayload(ctx)
	if diags.HasError() {
		t.Fatalf("updatePayload returned diagnostic errors: %v", diags)
	}

	if _, ok := payload["name"]; ok {
		t.Error("update payload must never contain 'name' key")
	}
	if payload["subnqn"] != "nqn.2011-06.com.truenas:uuid:custom" {
		t.Errorf("payload[subnqn] = %v, want nqn.2011-06.com.truenas:uuid:custom", payload["subnqn"])
	}
	if payload["allow_any_host"] != true {
		t.Errorf("payload[allow_any_host] = %v, want true", payload["allow_any_host"])
	}
	if payload["ana"] != false {
		t.Errorf("payload[ana] = %v, want false", payload["ana"])
	}
	ieeeOUI, ok := payload["ieee_oui"]
	if !ok {
		t.Fatal("expected 'ieee_oui' key present (explicit null) for empty string, got omitted")
	}
	if ieeeOUI != nil {
		t.Errorf("payload[ieee_oui] = %v, want nil (explicit clear)", ieeeOUI)
	}
	if _, ok := payload["pi_enable"]; ok {
		t.Error("expected 'pi_enable' to be omitted (null)")
	}
	if _, ok := payload["qid_max"]; ok {
		t.Error("expected 'qid_max' to be omitted (null)")
	}
}

// TestResponseToModel_NilPointers verifies that nil ana/pi_enable/qid_max
// fields from the API map to null, while ieee_oui maps to empty string.
func TestResponseToModel_NilPointers(t *testing.T) {
	ctx := context.Background()

	api := &nvmetSubsysAPI{
		ID:           1,
		Name:         "test-subsys",
		SubNQN:       "nqn.2011-06.com.truenas:uuid:test",
		AllowAnyHost: true,
		ANA:          nil,
		IEEEOUI:      nil,
		PIEnable:     nil,
		QIDMax:       nil,
		Serial:       "abc123",
	}

	var m NVMetSubsysModel
	diags := responseToModel(ctx, api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned diagnostic errors: %v", diags)
	}

	if !m.ANA.IsNull() {
		t.Errorf("ANA = %v (IsNull=%v), want null when API returns nil", m.ANA, m.ANA.IsNull())
	}
	if m.IEEEOUI.ValueString() != "" {
		t.Errorf("IEEEOUI = %q, want empty string when API returns nil", m.IEEEOUI.ValueString())
	}
	if !m.PIEnable.IsNull() {
		t.Errorf("PIEnable = %v (IsNull=%v), want null when API returns nil", m.PIEnable, m.PIEnable.IsNull())
	}
	if !m.QIDMax.IsNull() {
		t.Errorf("QIDMax = %v (IsNull=%v), want null when API returns nil", m.QIDMax, m.QIDMax.IsNull())
	}
}

// TestResponseToModel_AllFields verifies that non-nil fields are correctly
// mapped from the API response, matching the live query shape from the box.
func TestResponseToModel_AllFields(t *testing.T) {
	ctx := context.Background()

	ana := true
	oui := "0A0B0C"
	pi := true
	qidMax := int64(32)

	api := &nvmetSubsysAPI{
		ID:           1,
		Name:         "proxmox-test",
		SubNQN:       "nqn.2011-06.com.truenas:uuid:269a...:proxmox-test",
		Serial:       "912f32032fb296cdaf9e",
		AllowAnyHost: true,
		ANA:          &ana,
		IEEEOUI:      &oui,
		PIEnable:     &pi,
		QIDMax:       &qidMax,
	}

	var m NVMetSubsysModel
	diags := responseToModel(ctx, api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned diagnostic errors: %v", diags)
	}

	if m.ID.ValueInt64() != 1 {
		t.Errorf("ID = %v, want 1", m.ID.ValueInt64())
	}
	if m.Name.ValueString() != "proxmox-test" {
		t.Errorf("Name = %v, want proxmox-test", m.Name.ValueString())
	}
	if m.SubNQN.ValueString() != "nqn.2011-06.com.truenas:uuid:269a...:proxmox-test" {
		t.Errorf("SubNQN = %v, want the expected NQN", m.SubNQN.ValueString())
	}
	if m.Serial.ValueString() != "912f32032fb296cdaf9e" {
		t.Errorf("Serial = %v, want 912f32032fb296cdaf9e", m.Serial.ValueString())
	}
	if !m.AllowAnyHost.ValueBool() {
		t.Error("AllowAnyHost should be true")
	}
	if !m.ANA.ValueBool() {
		t.Error("ANA should be true")
	}
	if m.IEEEOUI.ValueString() != "0A0B0C" {
		t.Errorf("IEEEOUI = %v, want 0A0B0C", m.IEEEOUI.ValueString())
	}
	if !m.PIEnable.ValueBool() {
		t.Error("PIEnable should be true")
	}
	if m.QIDMax.ValueInt64() != 32 {
		t.Errorf("QIDMax = %v, want 32", m.QIDMax.ValueInt64())
	}
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_namespace

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestNVMetNamespaceSchema verifies key schema attribute shapes.
func TestNVMetNamespaceSchema(t *testing.T) {
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
	if len(subsysIDInt64.PlanModifiers) == 0 {
		t.Error("'subsys_id' should have plan modifiers (RequiresReplace)")
	}

	devicePathAttr, ok := s.Attributes["device_path"]
	if !ok {
		t.Fatal("schema missing 'device_path' attribute")
	}
	devicePathStr, ok := devicePathAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'device_path' attribute is %T, want schema.StringAttribute", devicePathAttr)
	}
	if !devicePathStr.IsRequired() {
		t.Error("'device_path' should be Required")
	}

	optionalComputedInt := []string{"filesize", "nsid"}
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

	// nsid is immutable after creation (omitted from updatePayload), so it
	// must carry RequiresReplace in addition to UseStateForUnknown.
	nsidAttr, ok := s.Attributes["nsid"]
	if !ok {
		t.Fatal("schema missing 'nsid' attribute")
	}
	nsidInt64, ok := nsidAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'nsid' attribute is %T, want schema.Int64Attribute", nsidAttr)
	}
	if !hasUseStateForUnknown(nsidInt64.PlanModifiers) {
		t.Error("'nsid' should have UseStateForUnknown plan modifier")
	}
	if !hasRequiresReplace(nsidInt64.PlanModifiers) {
		t.Error("'nsid' should have RequiresReplace plan modifier (immutable after creation)")
	}

	deviceTypeAttr, ok := s.Attributes["device_type"]
	if !ok {
		t.Fatal("schema missing 'device_type' attribute")
	}
	deviceTypeStr, ok := deviceTypeAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'device_type' attribute is %T, want schema.StringAttribute", deviceTypeAttr)
	}
	if !deviceTypeStr.IsOptional() || !deviceTypeStr.IsComputed() {
		t.Error("'device_type' should be Optional and Computed")
	}

	enabledAttr, ok := s.Attributes["enabled"]
	if !ok {
		t.Fatal("schema missing 'enabled' attribute")
	}
	enabledBool, ok := enabledAttr.(schema.BoolAttribute)
	if !ok {
		t.Fatalf("'enabled' attribute is %T, want schema.BoolAttribute", enabledAttr)
	}
	if !enabledBool.IsOptional() || !enabledBool.IsComputed() {
		t.Error("'enabled' should be Optional and Computed")
	}

	// Computed-only fields
	for _, field := range []string{"device_nguid", "device_uuid"} {
		attr, ok := s.Attributes[field]
		if !ok {
			t.Fatalf("schema missing %q attribute", field)
		}
		strAttr, ok := attr.(schema.StringAttribute)
		if !ok {
			t.Errorf("%q attribute is %T, want schema.StringAttribute", field, attr)
			continue
		}
		if !strAttr.IsComputed() {
			t.Errorf("%q should be Computed", field)
		}
		if strAttr.IsOptional() || strAttr.IsRequired() {
			t.Errorf("%q should be Computed-only (not Optional/Required)", field)
		}
	}

	lockedAttr, ok := s.Attributes["locked"]
	if !ok {
		t.Fatal("schema missing 'locked' attribute")
	}
	lockedBool, ok := lockedAttr.(schema.BoolAttribute)
	if !ok {
		t.Fatalf("'locked' attribute is %T, want schema.BoolAttribute", lockedAttr)
	}
	if !lockedBool.IsComputed() {
		t.Error("'locked' should be Computed")
	}
	if lockedBool.IsOptional() || lockedBool.IsRequired() {
		t.Error("'locked' should be Computed-only (not Optional/Required)")
	}
	if len(lockedBool.PlanModifiers) != 0 {
		t.Error("'locked' should NOT have plan modifiers (server-mutable, no UseStateForUnknown)")
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

// TestCreatePayload_KeysAlwaysPresent verifies that "subsys_id" and
// "device_path" are always sent on create, and that "nsid" is omitted
// entirely when unset (auto-assign), while other guarded fields are also
// omitted when null/unknown.
func TestCreatePayload_KeysAlwaysPresent(t *testing.T) {
	ctx := context.Background()

	m := NVMetNamespaceModel{
		SubsysID:   types.Int64Value(1),
		DevicePath: types.StringValue("zvol/tank/proxmox/vm-10001-disk-0"),
		DeviceType: types.StringNull(),
		Enabled:    types.BoolNull(),
		Filesize:   types.Int64Null(),
		NSID:       types.Int64Null(),
	}

	payload, diags := m.createPayload(ctx)
	if diags.HasError() {
		t.Fatalf("createPayload returned diagnostic errors: %v", diags)
	}

	if payload["subsys_id"] != int64(1) {
		t.Errorf("payload[subsys_id] = %v, want 1", payload["subsys_id"])
	}
	if payload["device_path"] != "zvol/tank/proxmox/vm-10001-disk-0" {
		t.Errorf("payload[device_path] = %v, want zvol/tank/proxmox/vm-10001-disk-0", payload["device_path"])
	}
	for _, key := range []string{"device_type", "enabled", "filesize", "nsid"} {
		if _, ok := payload[key]; ok {
			t.Errorf("expected %q to be omitted (null), got %v", key, payload[key])
		}
	}
	if len(payload) != 2 {
		t.Errorf("payload has %d keys (%v), want 2", len(payload), payload)
	}
}

// TestCreatePayload_AllFieldsKnown verifies that every guarded field,
// including nsid, is present when all model values are known.
func TestCreatePayload_AllFieldsKnown(t *testing.T) {
	ctx := context.Background()

	m := NVMetNamespaceModel{
		SubsysID:   types.Int64Value(1),
		DevicePath: types.StringValue("zvol/tank/proxmox/vm-10001-disk-0"),
		DeviceType: types.StringValue("ZVOL"),
		Enabled:    types.BoolValue(true),
		Filesize:   types.Int64Value(0),
		NSID:       types.Int64Value(5),
	}

	payload, diags := m.createPayload(ctx)
	if diags.HasError() {
		t.Fatalf("createPayload returned diagnostic errors: %v", diags)
	}

	want := map[string]any{
		"subsys_id":   int64(1),
		"device_path": "zvol/tank/proxmox/vm-10001-disk-0",
		"device_type": "ZVOL",
		"enabled":     true,
		"filesize":    int64(0),
		"nsid":        int64(5),
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

// TestUpdatePayload_NoSubsysIDOrNSID verifies that "subsys_id" and "nsid"
// are never included in the update payload, even when the model's SubsysID
// and NSID fields are known.
func TestUpdatePayload_NoSubsysIDOrNSID(t *testing.T) {
	ctx := context.Background()

	m := NVMetNamespaceModel{
		SubsysID:   types.Int64Value(1),
		DevicePath: types.StringValue("zvol/tank/proxmox/vm-10001-disk-0"),
		DeviceType: types.StringValue("ZVOL"),
		Enabled:    types.BoolValue(true),
		Filesize:   types.Int64Null(),
		NSID:       types.Int64Value(5),
	}

	payload, diags := m.updatePayload(ctx)
	if diags.HasError() {
		t.Fatalf("updatePayload returned diagnostic errors: %v", diags)
	}

	if _, ok := payload["subsys_id"]; ok {
		t.Error("update payload must never contain 'subsys_id' key")
	}
	if _, ok := payload["nsid"]; ok {
		t.Error("update payload must never contain 'nsid' key")
	}
	if payload["device_path"] != "zvol/tank/proxmox/vm-10001-disk-0" {
		t.Errorf("payload[device_path] = %v, want zvol/tank/proxmox/vm-10001-disk-0", payload["device_path"])
	}
	if payload["device_type"] != "ZVOL" {
		t.Errorf("payload[device_type] = %v, want ZVOL", payload["device_type"])
	}
	if payload["enabled"] != true {
		t.Errorf("payload[enabled] = %v, want true", payload["enabled"])
	}
	if _, ok := payload["filesize"]; ok {
		t.Errorf("expected 'filesize' to be omitted (null), got %v", payload["filesize"])
	}
}

// TestDecodeSubsysID_EmbeddedObject verifies decoding of the embedded
// subsys object shape returned by query/get_instance.
func TestDecodeSubsysID_EmbeddedObject(t *testing.T) {
	raw := json.RawMessage(`{"id": 1, "name": "proxmox-test", "subnqn": "nqn.2011-06.com.truenas:test"}`)

	id, err := decodeSubsysID(raw)
	if err != nil {
		t.Fatalf("decodeSubsysID returned error: %v", err)
	}
	if id != 1 {
		t.Errorf("decodeSubsysID = %v, want 1", id)
	}
}

// TestDecodeSubsysID_BareInt verifies decoding of a bare integer ID shape,
// as used in create/update payloads and possibly some responses.
func TestDecodeSubsysID_BareInt(t *testing.T) {
	raw := json.RawMessage(`7`)

	id, err := decodeSubsysID(raw)
	if err != nil {
		t.Fatalf("decodeSubsysID returned error: %v", err)
	}
	if id != 7 {
		t.Errorf("decodeSubsysID = %v, want 7", id)
	}
}

// TestDecodeSubsysID_Null verifies that a null subsys field returns an
// error rather than silently defaulting to 0.
func TestDecodeSubsysID_Null(t *testing.T) {
	raw := json.RawMessage(`null`)

	_, err := decodeSubsysID(raw)
	if err == nil {
		t.Fatal("decodeSubsysID(null) should return an error")
	}
}

// TestResponseToModel_FilesizeNil verifies that a nil filesize from the API
// (typical for ZVOL-backed namespaces) maps to null rather than 0, so that a
// subsequent update payload built from this model omits "filesize" instead
// of sending an explicit 0 that would overwrite a server-side null.
func TestResponseToModel_FilesizeNil(t *testing.T) {
	ctx := context.Background()

	api := &nvmetNamespaceAPI{
		ID:          1,
		NSID:        1,
		Subsys:      json.RawMessage(`{"id": 1}`),
		DeviceType:  "ZVOL",
		DevicePath:  "zvol/tank/proxmox/vm-10001-disk-0",
		Filesize:    nil,
		Enabled:     true,
		DeviceNGUID: "3c84...",
		DeviceUUID:  "e93b...",
		Locked:      false,
	}

	var m NVMetNamespaceModel
	diags := responseToModel(ctx, api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned diagnostic errors: %v", diags)
	}

	if !m.Filesize.IsNull() {
		t.Errorf("Filesize = %v, want null when API returns nil", m.Filesize)
	}
}

// TestResponseToModel_AllFields verifies that fields are correctly mapped
// from the API response, matching the live query shape from the box.
func TestResponseToModel_AllFields(t *testing.T) {
	ctx := context.Background()

	filesize := int64(0)
	api := &nvmetNamespaceAPI{
		ID:          1,
		NSID:        1,
		Subsys:      json.RawMessage(`{"id": 1, "name": "proxmox-test"}`),
		DeviceType:  "ZVOL",
		DevicePath:  "zvol/tank/proxmox/vm-10001-disk-0",
		Filesize:    &filesize,
		Enabled:     true,
		DeviceNGUID: "3c84...",
		DeviceUUID:  "e93b...",
		Locked:      false,
	}

	var m NVMetNamespaceModel
	diags := responseToModel(ctx, api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned diagnostic errors: %v", diags)
	}

	if m.ID.ValueInt64() != 1 {
		t.Errorf("ID = %v, want 1", m.ID.ValueInt64())
	}
	if m.SubsysID.ValueInt64() != 1 {
		t.Errorf("SubsysID = %v, want 1", m.SubsysID.ValueInt64())
	}
	if m.DevicePath.ValueString() != "zvol/tank/proxmox/vm-10001-disk-0" {
		t.Errorf("DevicePath = %v, want zvol/tank/proxmox/vm-10001-disk-0", m.DevicePath.ValueString())
	}
	if m.DeviceType.ValueString() != "ZVOL" {
		t.Errorf("DeviceType = %v, want ZVOL", m.DeviceType.ValueString())
	}
	if !m.Enabled.ValueBool() {
		t.Error("Enabled should be true")
	}
	if m.NSID.ValueInt64() != 1 {
		t.Errorf("NSID = %v, want 1", m.NSID.ValueInt64())
	}
	if m.DeviceNGUID.ValueString() != "3c84..." {
		t.Errorf("DeviceNGUID = %v, want 3c84...", m.DeviceNGUID.ValueString())
	}
	if m.DeviceUUID.ValueString() != "e93b..." {
		t.Errorf("DeviceUUID = %v, want e93b...", m.DeviceUUID.ValueString())
	}
	if m.Locked.ValueBool() {
		t.Error("Locked should be false")
	}
	if m.Filesize.ValueInt64() != 0 {
		t.Errorf("Filesize = %v, want 0", m.Filesize.ValueInt64())
	}
}

// TestResponseToModel_InvalidSubsys verifies that a malformed subsys field
// surfaces as a diagnostic error rather than silently defaulting.
func TestResponseToModel_InvalidSubsys(t *testing.T) {
	ctx := context.Background()

	api := &nvmetNamespaceAPI{
		ID:     1,
		Subsys: json.RawMessage(`null`),
	}

	var m NVMetNamespaceModel
	diags := responseToModel(ctx, api, &m)
	if !diags.HasError() {
		t.Fatal("responseToModel should return an error when subsys is null")
	}
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package container_device

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// --- versionGateDiagnostics ------------------------------------------------

func TestVersionGateDiagnostics_BelowFloor(t *testing.T) {
	for _, version := range []string{"25.10.3.1", "25.10", "24.10.2", "1.0", ""} {
		diags := versionGateDiagnostics(version)
		if !diags.HasError() {
			t.Fatalf("version %q: expected an error diagnostic, got none", version)
		}
		if len(diags) != 1 {
			t.Fatalf("version %q: expected exactly 1 diagnostic, got %d: %v", version, len(diags), diags)
		}
		if diags[0].Detail() != "truenas_container_device requires TrueNAS 26.0 or later" {
			t.Errorf("version %q: detail = %q, want the documented message", version, diags[0].Detail())
		}
	}
}

func TestVersionGateDiagnostics_AtOrAboveFloor(t *testing.T) {
	for _, version := range []string{"26.0.0", "26.0.0-BETA.2", "26.0", "26.1.0", "27.0.0"} {
		diags := versionGateDiagnostics(version)
		if diags.HasError() {
			t.Errorf("version %q: unexpected error diagnostic: %v", version, diags)
		}
	}
}

// --- attributesMap -----------------------------------------------------------

func TestAttributesMap_InvalidJSON(t *testing.T) {
	m := ContainerDeviceModel{Attributes: types.StringValue("{not valid json")}

	attrs, diags := m.attributesMap()
	if !diags.HasError() {
		t.Fatal("expected error diagnostics for invalid JSON, got none")
	}
	if attrs != nil {
		t.Errorf("expected nil attrs on error, got %v", attrs)
	}
}

func TestAttributesMap_MissingDtype(t *testing.T) {
	m := ContainerDeviceModel{Attributes: types.StringValue(`{"source": "/mnt/tank/ds"}`)}

	attrs, diags := m.attributesMap()
	if !diags.HasError() {
		t.Fatal("expected error diagnostics for missing dtype, got none")
	}
	if attrs != nil {
		t.Errorf("expected nil attrs on error, got %v", attrs)
	}
}

func TestAttributesMap_Valid(t *testing.T) {
	m := ContainerDeviceModel{Attributes: types.StringValue(`{"dtype": "FILESYSTEM", "source": "/mnt/tank/ds", "target": "/data"}`)}

	attrs, diags := m.attributesMap()
	if diags.HasError() {
		t.Fatalf("unexpected error diagnostics: %v", diags)
	}
	if attrs["dtype"] != "FILESYSTEM" {
		t.Errorf("attrs[dtype] = %v, want FILESYSTEM", attrs["dtype"])
	}
}

// --- attributesDrifted -------------------------------------------------------

func TestAttributesDrifted_Identical(t *testing.T) {
	state := map[string]any{"dtype": "FILESYSTEM", "source": "/mnt/tank/ds", "target": "/data"}
	api := map[string]any{"dtype": "FILESYSTEM", "source": "/mnt/tank/ds", "target": "/data"}

	if attributesDrifted(state, api) {
		t.Error("attributesDrifted() = true for identical maps, want false")
	}
}

func TestAttributesDrifted_ChangedValue(t *testing.T) {
	state := map[string]any{"dtype": "FILESYSTEM", "source": "/mnt/tank/ds", "target": "/data"}
	api := map[string]any{"dtype": "FILESYSTEM", "source": "/mnt/tank/ds", "target": "/data2"}

	if !attributesDrifted(state, api) {
		t.Error("attributesDrifted() = false for changed value, want true")
	}
}

func TestAttributesDrifted_APIAddedKey(t *testing.T) {
	// e.g. NIC's server-generated "mac" appearing only in the API response.
	state := map[string]any{"dtype": "NIC", "nic_attach": "truenasbr0"}
	api := map[string]any{"dtype": "NIC", "nic_attach": "truenasbr0", "mac": "00:a0:98:34:6e:8d"}

	if attributesDrifted(state, api) {
		t.Error("attributesDrifted() = true for API-added extra key, want false")
	}
}

func TestAttributesDrifted_MissingUserKey(t *testing.T) {
	state := map[string]any{"dtype": "FILESYSTEM", "source": "/mnt/tank/ds"}
	api := map[string]any{"dtype": "FILESYSTEM"}

	if !attributesDrifted(state, api) {
		t.Error("attributesDrifted() = false for missing user key in API, want true")
	}
}

// --- createPayload / updatePayload -------------------------------------------

func TestCreatePayload_IncludesContainer(t *testing.T) {
	m := ContainerDeviceModel{
		Container:  types.Int64Value(3),
		Attributes: types.StringValue(`{"dtype": "NIC", "nic_attach": "truenasbr0"}`),
	}

	payload, diags := m.createPayload()
	if diags.HasError() {
		t.Fatalf("unexpected error diagnostics: %v", diags)
	}
	if payload["container"] != int64(3) {
		t.Errorf("payload[container] = %v, want 3", payload["container"])
	}
	if _, ok := payload["attributes"]; !ok {
		t.Error("create payload missing key 'attributes'")
	}
}

func TestCreatePayload_InvalidAttributes(t *testing.T) {
	m := ContainerDeviceModel{
		Container:  types.Int64Value(3),
		Attributes: types.StringValue(`not json`),
	}

	_, diags := m.createPayload()
	if !diags.HasError() {
		t.Fatal("expected error diagnostics for invalid attributes JSON")
	}
}

func TestUpdatePayload_OmitsContainer(t *testing.T) {
	// Even though container.device.update's own accepts schema technically
	// lists "container" as an optional key (probed live), this provider
	// forces reassignment through replacement (RequiresReplace in
	// schema.go) and never sends "container" in an update payload — see
	// updatePayload's doc comment.
	m := ContainerDeviceModel{
		ID:         types.Int64Value(5),
		Container:  types.Int64Value(1),
		Attributes: types.StringValue(`{"dtype": "FILESYSTEM", "source": "/mnt/tank/ds", "target": "/data"}`),
	}

	payload, diags := m.updatePayload()
	if diags.HasError() {
		t.Fatalf("unexpected error diagnostics: %v", diags)
	}

	if _, ok := payload["container"]; ok {
		t.Error("update payload should not contain key 'container'")
	}
	if _, ok := payload["attributes"]; !ok {
		t.Error("update payload missing key 'attributes'")
	}
}

func TestUpdatePayload_InvalidAttributes(t *testing.T) {
	m := ContainerDeviceModel{
		ID:         types.Int64Value(5),
		Container:  types.Int64Value(1),
		Attributes: types.StringValue(`not json`),
	}

	_, diags := m.updatePayload()
	if !diags.HasError() {
		t.Fatal("expected error diagnostics for invalid attributes JSON")
	}
}

// --- responseToModel / responseToDataSourceModel / apiAttributesJSON --------

func TestResponseToModel(t *testing.T) {
	api := &containerDeviceAPI{
		ID:         7,
		Container:  11,
		Attributes: map[string]any{"dtype": "FILESYSTEM", "source": "/mnt/tank/ds", "target": "/data"},
	}
	var m ContainerDeviceModel
	responseToModel(api, &m)

	if m.ID.ValueInt64() != 7 {
		t.Errorf("m.ID = %v, want 7", m.ID)
	}
	if m.Container.ValueInt64() != 11 {
		t.Errorf("m.Container = %v, want 11", m.Container)
	}
	// responseToModel intentionally leaves Attributes untouched (caller's
	// responsibility, write-what-you-said / drift-aware semantics).
	if !m.Attributes.IsNull() {
		t.Errorf("m.Attributes = %v, want untouched (null)", m.Attributes)
	}
}

func TestResponseToDataSourceModel(t *testing.T) {
	api := &containerDeviceAPI{
		ID:         7,
		Container:  11,
		Attributes: map[string]any{"dtype": "USB", "usb": map[string]any{"vendor_id": "0x046b", "product_id": "0xff10"}},
	}
	var m ContainerDeviceDataSourceModel
	diags := responseToDataSourceModel(api, &m)
	if diags.HasError() {
		t.Fatalf("unexpected error diagnostics: %v", diags)
	}

	if m.ID.ValueInt64() != 7 {
		t.Errorf("m.ID = %v, want 7", m.ID)
	}
	if m.Container.ValueInt64() != 11 {
		t.Errorf("m.Container = %v, want 11", m.Container)
	}
	if m.Attributes.IsNull() || m.Attributes.ValueString() == "" {
		t.Error("m.Attributes should be a non-empty JSON string")
	}
}

func TestApiAttributesJSON(t *testing.T) {
	api := &containerDeviceAPI{
		ID:         1,
		Container:  2,
		Attributes: map[string]any{"dtype": "NIC", "nic_attach": "truenasbr0"},
	}
	s, diags := apiAttributesJSON(api)
	if diags.HasError() {
		t.Fatalf("unexpected error diagnostics: %v", diags)
	}
	if s == "" {
		t.Error("expected non-empty JSON string")
	}
}

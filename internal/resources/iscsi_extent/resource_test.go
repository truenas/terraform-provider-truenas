// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_extent

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestISCSIExtentSchema verifies that the resource schema has the expected
// attributes and key attributes have the correct types.
func TestISCSIExtentSchema(t *testing.T) {
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

	// name must be Required.
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

	// type must be Required.
	typeAttr, ok := s.Attributes["type"]
	if !ok {
		t.Fatal("schema missing 'type' attribute")
	}
	typeStr, ok := typeAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'type' attribute is %T, want schema.StringAttribute", typeAttr)
	}
	if !typeStr.IsRequired() {
		t.Error("'type' should be Required")
	}

	// disk and path must be Optional+Computed.
	for _, field := range []string{"disk", "path"} {
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

	// Computed-only fields: naa, serial, product_id, vendor, locked.
	computedStringFields := []string{"naa", "serial", "product_id", "vendor"}
	for _, field := range computedStringFields {
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
}

// TestISCSIExtentApiPayload_DISK verifies that DISK type payload contains the
// 'disk' key and not 'path'.
func TestISCSIExtentApiPayload_DISK(t *testing.T) {
	ctx := context.Background()

	m := ISCSIExtentModel{
		Name:           types.StringValue("test-extent"),
		Type:           types.StringValue("DISK"),
		Disk:           types.StringValue("zvol/tank/myvol"),
		Path:           types.StringValue(""),
		Comment:        types.StringValue(""),
		Blocksize:      types.Int64Value(512),
		PBlocksize:     types.BoolValue(false),
		AvailThreshold: types.Int64Value(0),
		InsecureTPC:    types.BoolValue(true),
		Xen:            types.BoolValue(false),
		ReadOnly:       types.BoolValue(false),
		RPM:            types.StringValue("SSD"),
		Enabled:        types.BoolValue(true),
	}

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostic errors: %v", diags)
	}

	if _, ok := payload["disk"]; !ok {
		t.Error("DISK type payload missing 'disk' key")
	}
	if payload["disk"] != "zvol/tank/myvol" {
		t.Errorf("payload[disk] = %v, want zvol/tank/myvol", payload["disk"])
	}
	if _, ok := payload["path"]; ok {
		t.Error("DISK type payload should not contain 'path' key")
	}
}

// TestISCSIExtentApiPayload_FILE verifies that FILE type payload contains the
// 'path' key and not 'disk'.
func TestISCSIExtentApiPayload_FILE(t *testing.T) {
	ctx := context.Background()

	m := ISCSIExtentModel{
		Name:           types.StringValue("test-file-extent"),
		Type:           types.StringValue("FILE"),
		Disk:           types.StringValue(""),
		Path:           types.StringValue("/mnt/tank/extents/file.img"),
		Comment:        types.StringValue(""),
		Blocksize:      types.Int64Value(512),
		PBlocksize:     types.BoolValue(false),
		AvailThreshold: types.Int64Value(0),
		InsecureTPC:    types.BoolValue(true),
		Xen:            types.BoolValue(false),
		ReadOnly:       types.BoolValue(false),
		RPM:            types.StringValue("SSD"),
		Enabled:        types.BoolValue(true),
	}

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostic errors: %v", diags)
	}

	if _, ok := payload["path"]; !ok {
		t.Error("FILE type payload missing 'path' key")
	}
	if payload["path"] != "/mnt/tank/extents/file.img" {
		t.Errorf("payload[path] = %v, want /mnt/tank/extents/file.img", payload["path"])
	}
	if _, ok := payload["disk"]; ok {
		t.Error("FILE type payload should not contain 'disk' key")
	}
}

// TestISCSIExtentApiPayload_AvailThreshold verifies that avail_threshold=0
// sends nil (pointer), and non-zero sends the value.
func TestISCSIExtentApiPayload_AvailThreshold(t *testing.T) {
	ctx := context.Background()

	// Zero value → should send nil pointer.
	m := ISCSIExtentModel{
		Name:           types.StringValue("test"),
		Type:           types.StringValue("DISK"),
		Disk:           types.StringValue("zvol/tank/vol"),
		AvailThreshold: types.Int64Value(0),
		Blocksize:      types.Int64Value(512),
		RPM:            types.StringValue("SSD"),
	}
	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostic errors: %v", diags)
	}
	if payload["avail_threshold"] != (*int64)(nil) {
		t.Errorf("avail_threshold=0 should produce nil, got %v (%T)", payload["avail_threshold"], payload["avail_threshold"])
	}

	// Non-zero value → should send the value.
	m.AvailThreshold = types.Int64Value(80)
	payload, diags = m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostic errors: %v", diags)
	}
	avail, ok := payload["avail_threshold"].(*int64)
	if !ok {
		t.Fatalf("avail_threshold is %T, want *int64", payload["avail_threshold"])
	}
	if avail == nil || *avail != 80 {
		t.Errorf("avail_threshold = %v, want pointer to 80", avail)
	}
}

// TestISCSIExtentApiPayload_OmitsUnsetOptionalFields verifies that when the
// Optional+Computed fields are null/unknown (the state a Create call sees
// for attributes the caller never set in config), apiPayload omits them
// entirely rather than sending Go zero values (0, "", false) that TrueNAS
// TrueNAS rejects for enum-constrained fields like blocksize and rpm.
func TestISCSIExtentApiPayload_OmitsUnsetOptionalFields(t *testing.T) {
	ctx := context.Background()

	m := ISCSIExtentModel{
		Name:           types.StringValue("test-extent"),
		Type:           types.StringValue("DISK"),
		Disk:           types.StringValue("zvol/tank/myvol"),
		Path:           types.StringUnknown(),
		Comment:        types.StringUnknown(),
		Blocksize:      types.Int64Unknown(),
		PBlocksize:     types.BoolUnknown(),
		AvailThreshold: types.Int64Unknown(),
		InsecureTPC:    types.BoolNull(),
		Xen:            types.BoolNull(),
		ReadOnly:       types.BoolNull(),
		RPM:            types.StringNull(),
		Enabled:        types.BoolNull(),
	}

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostic errors: %v", diags)
	}

	for _, k := range []string{"blocksize", "rpm", "comment", "pblocksize", "insecure_tpc", "xen", "ro", "enabled"} {
		if v, ok := payload[k]; ok {
			t.Errorf("payload should omit key %q when null/unknown in model, got %v", k, v)
		}
	}

	// Required/type-dependent fields must still be present.
	if payload["name"] != "test-extent" {
		t.Errorf("payload[name] = %v, want test-extent", payload["name"])
	}
	if payload["type"] != "DISK" {
		t.Errorf("payload[type] = %v, want DISK", payload["type"])
	}
	if payload["disk"] != "zvol/tank/myvol" {
		t.Errorf("payload[disk] = %v, want zvol/tank/myvol", payload["disk"])
	}
	if payload["avail_threshold"] != (*int64)(nil) {
		t.Errorf("payload[avail_threshold] = %v, want nil (unknown AvailThreshold treated as unset)", payload["avail_threshold"])
	}
}

// TestISCSIExtentResponseToModel_NilDisk verifies that a nil Disk pointer maps
// to an empty string in the model.
func TestISCSIExtentResponseToModel_NilDisk(t *testing.T) {
	ctx := context.Background()

	api := &extentAPI{
		ID:        1,
		Name:      "test",
		Type:      "FILE",
		Disk:      nil,
		Path:      "/mnt/tank/extents/file.img",
		Blocksize: 512,
		RPM:       "SSD",
		Enabled:   true,
	}

	var m ISCSIExtentModel
	diags := responseToModel(ctx, api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned diagnostic errors: %v", diags)
	}

	if m.Disk.ValueString() != "" {
		t.Errorf("Disk = %q, want empty string when API returns nil", m.Disk.ValueString())
	}
	if m.Path.ValueString() != "/mnt/tank/extents/file.img" {
		t.Errorf("Path = %q, want /mnt/tank/extents/file.img", m.Path.ValueString())
	}
}

// TestISCSIExtentResponseToModel_NilAvailThreshold verifies that a nil
// avail_threshold from the API maps to 0 in the model.
func TestISCSIExtentResponseToModel_NilAvailThreshold(t *testing.T) {
	ctx := context.Background()

	api := &extentAPI{
		ID:             1,
		Name:           "test",
		Type:           "DISK",
		Disk:           strPtr("zvol/tank/myvol"),
		Path:           "zvol/tank/myvol",
		Blocksize:      512,
		AvailThreshold: nil,
		RPM:            "SSD",
		Enabled:        true,
	}

	var m ISCSIExtentModel
	diags := responseToModel(ctx, api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned diagnostic errors: %v", diags)
	}

	if m.AvailThreshold.ValueInt64() != 0 {
		t.Errorf("AvailThreshold = %v, want 0 when API returns nil", m.AvailThreshold.ValueInt64())
	}
}

// TestISCSIExtentResponseToModel_AllFields verifies that all fields are
// correctly mapped from the API response.
func TestISCSIExtentResponseToModel_AllFields(t *testing.T) {
	ctx := context.Background()

	threshold := int64(75)
	disk := "zvol/tank/myvol"
	api := &extentAPI{
		ID:             42,
		Name:           "my-extent",
		Type:           "DISK",
		Disk:           &disk,
		Path:           "zvol/tank/myvol",
		Comment:        "test comment",
		Blocksize:      512,
		PBlocksize:     true,
		AvailThreshold: &threshold,
		InsecureTPC:    true,
		Xen:            false,
		ReadOnly:       false,
		RPM:            "SSD",
		Enabled:        true,
		NAA:            "0x6589cfc000000e1b4f22a4f46a0b8a12",
		Serial:         "AE2T-V8WA",
		ProductID:      "iSCSI Disk",
		Vendor:         "TrueNAS",
		Locked:         false,
	}

	var m ISCSIExtentModel
	diags := responseToModel(ctx, api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned diagnostic errors: %v", diags)
	}

	if m.ID.ValueInt64() != 42 {
		t.Errorf("ID = %v, want 42", m.ID.ValueInt64())
	}
	if m.Name.ValueString() != "my-extent" {
		t.Errorf("Name = %v, want my-extent", m.Name.ValueString())
	}
	if m.Disk.ValueString() != "zvol/tank/myvol" {
		t.Errorf("Disk = %v, want zvol/tank/myvol", m.Disk.ValueString())
	}
	if m.AvailThreshold.ValueInt64() != 75 {
		t.Errorf("AvailThreshold = %v, want 75", m.AvailThreshold.ValueInt64())
	}
	if m.NAA.ValueString() != "0x6589cfc000000e1b4f22a4f46a0b8a12" {
		t.Errorf("NAA = %v, want 0x6589cfc000000e1b4f22a4f46a0b8a12", m.NAA.ValueString())
	}
	if m.Serial.ValueString() != "AE2T-V8WA" {
		t.Errorf("Serial = %v, want AE2T-V8WA", m.Serial.ValueString())
	}
	if m.ProductID.ValueString() != "iSCSI Disk" {
		t.Errorf("ProductID = %v, want iSCSI Disk", m.ProductID.ValueString())
	}
	if m.Vendor.ValueString() != "TrueNAS" {
		t.Errorf("Vendor = %v, want TrueNAS", m.Vendor.ValueString())
	}
}

// strPtr is a helper to create a *string from a literal.
func strPtr(s string) *string { return &s }

// TestCoerceInt64 covers the filesize decoder's accepted shapes (the API
// returns filesize as a JSON number or a numeric string).
func TestCoerceInt64(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want int64
	}{
		{"float64", float64(2048), 2048},
		{"int64", int64(4096), 4096},
		{"int", int(512), 512},
		{"numeric string", "65536", 65536},
		{"bad string", "notanumber", 0},
		{"nil", nil, 0},
		{"unexpected type", []int{1}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := coerceInt64(tc.in); got != tc.want {
				t.Errorf("coerceInt64(%v) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

// fileModel builds a minimal FILE-type extent model with everything optional
// left null so apiPayload omits it.
func fileExtentModel(filesize int64) *ISCSIExtentModel {
	m := &ISCSIExtentModel{
		Name:           types.StringValue("ext"),
		Type:           types.StringValue("FILE"),
		Path:           types.StringValue("/mnt/tank/x/e.img"),
		Disk:           types.StringNull(),
		Comment:        types.StringNull(),
		Blocksize:      types.Int64Null(),
		PBlocksize:     types.BoolNull(),
		AvailThreshold: types.Int64Null(),
		InsecureTPC:    types.BoolNull(),
		Xen:            types.BoolNull(),
		ReadOnly:       types.BoolNull(),
		RPM:            types.StringNull(),
		Enabled:        types.BoolNull(),
	}
	m.Filesize = types.Int64Value(filesize)
	return m
}

// TestISCSIExtentApiPayload_Filesize verifies filesize is sent for FILE extents
// when non-zero, omitted when zero, and never sent for DISK extents.
func TestISCSIExtentApiPayload_Filesize(t *testing.T) {
	ctx := context.Background()

	p, _ := fileExtentModel(67108864).apiPayload(ctx)
	if p["filesize"] != int64(67108864) {
		t.Errorf("FILE filesize = %v, want 67108864", p["filesize"])
	}

	p, _ = fileExtentModel(0).apiPayload(ctx)
	if _, ok := p["filesize"]; ok {
		t.Errorf("FILE filesize=0 should be omitted, got %v", p["filesize"])
	}

	disk := fileExtentModel(67108864)
	disk.Type = types.StringValue("DISK")
	disk.Disk = types.StringValue("zvol/tank/v")
	p, _ = disk.apiPayload(ctx)
	if _, ok := p["filesize"]; ok {
		t.Errorf("DISK extent should not send filesize, got %v", p["filesize"])
	}
}

// TestISCSIExtentResponseToModel_Filesize verifies filesize decodes from either
// a JSON number or a numeric string into the model.
func TestISCSIExtentResponseToModel_Filesize(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name string
		raw  any
		want int64
	}{
		{"number", float64(2048), 2048},
		{"string", "4096", 4096},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var m ISCSIExtentModel
			api := &extentAPI{ID: 1, Name: "e", Type: "FILE", Path: "/mnt/tank/x/e.img", Filesize: tc.raw}
			if d := responseToModel(ctx, api, &m); d.HasError() {
				t.Fatalf("responseToModel: %v", d)
			}
			if m.Filesize.ValueInt64() != tc.want {
				t.Errorf("Filesize = %d, want %d", m.Filesize.ValueInt64(), tc.want)
			}
		})
	}
}

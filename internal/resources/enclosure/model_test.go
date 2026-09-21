// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package enclosure

import (
	"context"
	"testing"
)

// apiShape mirrors the live TrueNAS 25.10.4 Enterprise HA probe of the single
// shared H-series chassis enclosure: see model.go's enclosureAPI doc
// comment.
func apiShape() *enclosureAPI {
	return &enclosureAPI{
		ID:            "3b0ad6d1c00006e0",
		Name:          "BROADCOM VirtualSES 03",
		Label:         "BROADCOM VirtualSES 03",
		Model:         "H10",
		Controller:    true,
		Vendor:        "BROADCOM",
		Product:       "VirtualSES",
		Revision:      "03",
		DMI:           "TRUENAS-H10-HA",
		BSG:           "/dev/bsg/0:0:12:0",
		SG:            "/dev/sg13",
		PCI:           "0:0:12:0",
		Rackmount:     true,
		FrontLoaded:   true,
		FrontSlots:    12,
		RearSlots:     0,
		TopLoaded:     false,
		TopSlots:      0,
		InternalSlots: 0,
		Status:        []string{"OK"},
	}
}

func TestResponseToDataSourceModel(t *testing.T) {
	ctx := context.Background()
	m := &EnclosureDataSourceModel{}
	diags := responseToDataSourceModel(ctx, apiShape(), m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if m.ID.ValueString() != "3b0ad6d1c00006e0" {
		t.Errorf("ID = %q, want 3b0ad6d1c00006e0", m.ID.ValueString())
	}
	if m.Name.ValueString() != "BROADCOM VirtualSES 03" {
		t.Errorf("Name = %q, want BROADCOM VirtualSES 03", m.Name.ValueString())
	}
	if m.Label.ValueString() != "BROADCOM VirtualSES 03" {
		t.Errorf("Label = %q, want BROADCOM VirtualSES 03", m.Label.ValueString())
	}
	if m.Model.ValueString() != "H10" {
		t.Errorf("Model = %q, want H10", m.Model.ValueString())
	}
	if !m.Controller.ValueBool() {
		t.Error("Controller should be true")
	}
	if m.FrontSlots.ValueInt64() != 12 {
		t.Errorf("FrontSlots = %d, want 12", m.FrontSlots.ValueInt64())
	}

	var status []string
	diags = m.Status.ElementsAs(ctx, &status, false)
	if diags.HasError() {
		t.Fatalf("reading back status: %v", diags)
	}
	if len(status) != 1 || status[0] != "OK" {
		t.Errorf("status = %v, want [OK]", status)
	}
}

// TestResponseToDataSourceModel_NilStatus verifies the nil-guard: a nil
// api.Status must decode to a non-null, empty types.List, not a null one
// (matching the house convention for API-returned lists, e.g. audit_config's
// stringListOrEmpty).
func TestResponseToDataSourceModel_NilStatus(t *testing.T) {
	ctx := context.Background()
	api := apiShape()
	api.Status = nil

	m := &EnclosureDataSourceModel{}
	diags := responseToDataSourceModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.Status.IsNull() {
		t.Error("Status should be an empty list, not null, when the API returns status: null")
	}
	var status []string
	diags = m.Status.ElementsAs(ctx, &status, false)
	if diags.HasError() {
		t.Fatalf("reading back status: %v", diags)
	}
	if len(status) != 0 {
		t.Errorf("status = %v, want empty", status)
	}
}

// TestResponseToDataSourceModel_LabelIndependentOfName pins the decisive
// live-probed evidence that "label" and "name" are independent fields (see
// enclosureAPI's doc comment): a customized label must decode distinctly
// from an unrelated name.
func TestResponseToDataSourceModel_LabelIndependentOfName(t *testing.T) {
	ctx := context.Background()
	api := apiShape()
	api.Label = "tf-acc-enclosure-abc123"

	m := &EnclosureDataSourceModel{}
	diags := responseToDataSourceModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.Label.ValueString() != "tf-acc-enclosure-abc123" {
		t.Errorf("Label = %q, want tf-acc-enclosure-abc123", m.Label.ValueString())
	}
	if m.Name.ValueString() != "BROADCOM VirtualSES 03" {
		t.Errorf("Name = %q, want BROADCOM VirtualSES 03 (unchanged)", m.Name.ValueString())
	}
}

// --- enclosureQueryArgs --------------------------------------------------

func TestEnclosureQueryArgs(t *testing.T) {
	args := enclosureQueryArgs("3b0ad6d1c00006e0")
	if len(args) != 1 {
		t.Fatalf("enclosureQueryArgs returned %d args, want 1 (filters only)", len(args))
	}
	filters, ok := args[0].([][]any)
	if !ok || len(filters) != 1 {
		t.Fatalf("enclosureQueryArgs()[0] = %#v, want a single-filter [][]any", args[0])
	}
	filter := filters[0]
	if len(filter) != 3 {
		t.Fatalf("filter = %#v, want [field, op, value]", filter)
	}
	if filter[0] != "id" || filter[1] != "=" || filter[2] != "3b0ad6d1c00006e0" {
		t.Errorf("filter = %#v, want [id = 3b0ad6d1c00006e0]", filter)
	}
}

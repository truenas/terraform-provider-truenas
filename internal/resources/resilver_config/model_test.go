// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package resilver_config

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func int64List(t *testing.T, vals ...int64) types.List {
	t.Helper()
	elems := make([]attr.Value, len(vals))
	for i, v := range vals {
		elems[i] = types.Int64Value(v)
	}
	l, diags := types.ListValue(types.Int64Type, elems)
	if diags.HasError() {
		t.Fatalf("building weekday list: %v", diags)
	}
	return l
}

// TestUpdatePayload_AllFieldsSet verifies that updatePayload includes every
// field with the exact keys observed in the pool.resilver.update probe.
func TestUpdatePayload_AllFieldsSet(t *testing.T) {
	ctx := context.Background()
	m := &ResilverConfigModel{
		Begin:   types.StringValue("19:00"),
		End:     types.StringValue("05:00"),
		Enabled: types.BoolValue(true),
		Weekday: int64List(t, 1, 2, 3, 4, 5),
	}

	p, diags := m.updatePayload(ctx)
	if diags.HasError() {
		t.Fatalf("updatePayload returned diagnostics errors: %v", diags)
	}

	want := map[string]any{
		"begin":   "19:00",
		"end":     "05:00",
		"enabled": true,
	}
	for k, v := range want {
		if p[k] != v {
			t.Errorf("payload[%q] = %v, want %v", k, p[k], v)
		}
	}
	weekday, ok := p["weekday"].([]int64)
	if !ok {
		t.Fatalf("payload[weekday] is %T, want []int64", p["weekday"])
	}
	wantWeekday := []int64{1, 2, 3, 4, 5}
	if len(weekday) != len(wantWeekday) {
		t.Fatalf("payload[weekday] = %v, want %v", weekday, wantWeekday)
	}
	for i, v := range wantWeekday {
		if weekday[i] != v {
			t.Errorf("payload[weekday][%d] = %v, want %v", i, weekday[i], v)
		}
	}
	if len(p) != 4 {
		t.Errorf("payload has %d keys (%v), want 4", len(p), p)
	}
}

// TestUpdatePayload_UnsetOptionalsOmitted verifies that every field is
// omitted when null/unknown, so the current TrueNAS-side value is left
// unchanged rather than overwritten with a zero value.
func TestUpdatePayload_UnsetOptionalsOmitted(t *testing.T) {
	ctx := context.Background()
	m := &ResilverConfigModel{
		Begin:   types.StringNull(),
		End:     types.StringUnknown(),
		Enabled: types.BoolNull(),
		Weekday: types.ListUnknown(types.Int64Type),
	}

	p, diags := m.updatePayload(ctx)
	if diags.HasError() {
		t.Fatalf("updatePayload returned diagnostics errors: %v", diags)
	}
	if len(p) != 0 {
		t.Errorf("payload has %d keys (%v), want 0 (all omitted)", len(p), p)
	}
}

// TestResponseToModel verifies responseToModel against the shape probed
// from a live pool.resilver.config call.
func TestResponseToModel(t *testing.T) {
	ctx := context.Background()
	api := &resilverConfigAPI{
		ID:      1,
		Begin:   "18:00",
		End:     "09:00",
		Enabled: false,
		Weekday: []int64{1, 2, 3, 4, 5, 6, 7},
	}

	m := &ResilverConfigModel{}
	diags := responseToModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if m.ID.ValueString() != resilverConfigResourceID {
		t.Errorf("ID = %q, want %q", m.ID.ValueString(), resilverConfigResourceID)
	}
	if m.Begin.ValueString() != "18:00" {
		t.Errorf("Begin = %q, want \"18:00\"", m.Begin.ValueString())
	}
	if m.End.ValueString() != "09:00" {
		t.Errorf("End = %q, want \"09:00\"", m.End.ValueString())
	}
	if m.Enabled.ValueBool() {
		t.Error("Enabled = true, want false")
	}

	var weekday []int64
	diags = m.Weekday.ElementsAs(ctx, &weekday, false)
	if diags.HasError() {
		t.Fatalf("reading back weekday: %v", diags)
	}
	if len(weekday) != 7 {
		t.Errorf("weekday = %v, want 7 elements", weekday)
	}
}

// TestResponseToModel_NilWeekdayBecomesEmptyList verifies that a nil
// weekday slice from the API maps to an empty (non-null) list, matching the
// nil-guard convention used elsewhere for API-returned lists.
func TestResponseToModel_NilWeekdayBecomesEmptyList(t *testing.T) {
	ctx := context.Background()
	api := &resilverConfigAPI{ID: 1, Begin: "18:00", End: "09:00", Enabled: true, Weekday: nil}

	m := &ResilverConfigModel{}
	diags := responseToModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.Weekday.IsNull() {
		t.Error("Weekday should not be null when API returns nil, want empty list")
	}
	var weekday []int64
	diags = m.Weekday.ElementsAs(ctx, &weekday, false)
	if diags.HasError() {
		t.Fatalf("reading back weekday: %v", diags)
	}
	if len(weekday) != 0 {
		t.Errorf("weekday = %v, want empty", weekday)
	}
}

// TestDeleteWarningDiagnostics verifies that Delete's diagnostic builder
// returns exactly one warning (no errors) and does not require or touch a
// client — this is what makes "Delete makes no client calls" verifiable.
func TestDeleteWarningDiagnostics(t *testing.T) {
	diags := deleteWarningDiagnostics()
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if len(diags) != 1 {
		t.Fatalf("expected exactly 1 diagnostic, got %d: %v", len(diags), diags)
	}
	if diags[0].Detail() != "Resilver configuration left in place; removed from Terraform state only" {
		t.Errorf("detail = %q, want the documented warning text", diags[0].Detail())
	}
}

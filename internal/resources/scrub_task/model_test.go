// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package scrub_task

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// buildSchedule constructs a known (non-null, non-unknown) schedule object
// value for use in tests.
func buildSchedule(t *testing.T, minute, hour, dom, month, dow string) types.Object {
	t.Helper()
	obj, diags := types.ObjectValueFrom(context.Background(), scheduleAttrTypes, ScheduleModel{
		Minute: types.StringValue(minute),
		Hour:   types.StringValue(hour),
		Dom:    types.StringValue(dom),
		Month:  types.StringValue(month),
		Dow:    types.StringValue(dow),
	})
	if diags.HasError() {
		t.Fatalf("building schedule object: %v", diags)
	}
	return obj
}

// TestApiPayload_AllFieldsSet verifies that apiPayload includes every
// field — pool, threshold, description, schedule, enabled — when all are
// known, with the exact keys observed in the pool.scrub.create probe.
func TestApiPayload_AllFieldsSet(t *testing.T) {
	ctx := context.Background()
	m := &ScrubTaskModel{
		Pool:        types.Int64Value(1),
		Threshold:   types.Int64Value(40),
		Description: types.StringValue("monthly scrub"),
		Schedule:    buildSchedule(t, "00", "03", "*", "*", "7"),
		Enabled:     types.BoolValue(true),
	}

	p, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostics errors: %v", diags)
	}

	requiredKeys := []string{"pool", "threshold", "description", "schedule", "enabled"}
	for _, key := range requiredKeys {
		if _, ok := p[key]; !ok {
			t.Errorf("payload missing key %q", key)
		}
	}
	if len(p) != len(requiredKeys) {
		t.Errorf("payload has %d keys (%v), want %d", len(p), p, len(requiredKeys))
	}

	if p["pool"] != int64(1) {
		t.Errorf("payload[pool] = %v, want 1", p["pool"])
	}
	if p["threshold"] != int64(40) {
		t.Errorf("payload[threshold] = %v, want 40", p["threshold"])
	}
	if p["description"] != "monthly scrub" {
		t.Errorf("payload[description] = %v, want \"monthly scrub\"", p["description"])
	}
	if p["enabled"] != true {
		t.Errorf("payload[enabled] = %v, want true", p["enabled"])
	}

	sched, ok := p["schedule"].(map[string]string)
	if !ok {
		t.Fatalf("payload[schedule] is %T, want map[string]string", p["schedule"])
	}
	want := map[string]string{"minute": "00", "hour": "03", "dom": "*", "month": "*", "dow": "7"}
	for k, v := range want {
		if sched[k] != v {
			t.Errorf("schedule[%q] = %q, want %q", k, sched[k], v)
		}
	}
}

// TestApiPayload_UnsetOptionalsOmitted verifies that threshold, description,
// schedule, and enabled are omitted from the payload when null or unknown,
// leaving only the Required "pool" field — so the TrueNAS-side defaults
// (threshold=35, description="", enabled=true, schedule=00 00 * * 7) take
// effect instead of an explicit zero value being sent.
func TestApiPayload_UnsetOptionalsOmitted(t *testing.T) {
	ctx := context.Background()
	m := &ScrubTaskModel{
		Pool:        types.Int64Value(2),
		Threshold:   types.Int64Null(),
		Description: types.StringUnknown(),
		Schedule:    types.ObjectNull(scheduleAttrTypes),
		Enabled:     types.BoolUnknown(),
	}

	p, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostics errors: %v", diags)
	}

	if len(p) != 1 {
		t.Fatalf("payload has %d keys (%v), want 1 (only pool)", len(p), p)
	}
	if p["pool"] != int64(2) {
		t.Errorf("payload[pool] = %v, want 2", p["pool"])
	}
	for _, key := range []string{"threshold", "description", "schedule", "enabled"} {
		if _, ok := p[key]; ok {
			t.Errorf("expected %q to be omitted (null/unknown), got %v", key, p[key])
		}
	}
}

// TestResponseToModel_QueryShape verifies responseToModel against the exact
// shape observed from a live pool.scrub.query call: "pool" is a plain
// integer (never an embedded object), accompanied by a "pool_name" string.
func TestResponseToModel_QueryShape(t *testing.T) {
	ctx := context.Background()
	api := &scrubTaskAPI{
		ID:          1,
		Pool:        1,
		PoolName:    "tank",
		Threshold:   35,
		Description: "",
		Schedule:    scheduleAPI{Minute: "00", Hour: "00", Dom: "*", Month: "*", Dow: "7"},
		Enabled:     true,
	}

	m := &ScrubTaskModel{}
	diags := responseToModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if m.ID.ValueInt64() != 1 {
		t.Errorf("ID = %v, want 1", m.ID)
	}
	if m.Pool.ValueInt64() != 1 {
		t.Errorf("Pool = %v, want 1", m.Pool)
	}
	if m.PoolName.ValueString() != "tank" {
		t.Errorf("PoolName = %q, want \"tank\"", m.PoolName.ValueString())
	}
	if m.Threshold.ValueInt64() != 35 {
		t.Errorf("Threshold = %v, want 35", m.Threshold)
	}
	if !m.Enabled.ValueBool() {
		t.Error("Enabled = false, want true")
	}

	var sched ScheduleModel
	diags = m.Schedule.As(ctx, &sched, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("reading back schedule: %v", diags)
	}
	if sched.Minute.ValueString() != "00" {
		t.Errorf("schedule.minute = %q, want \"00\"", sched.Minute.ValueString())
	}
	if sched.Dow.ValueString() != "7" {
		t.Errorf("schedule.dow = %q, want \"7\"", sched.Dow.ValueString())
	}
}

// TestResponseToDataSourceModel_QueryShape mirrors
// TestResponseToModel_QueryShape for the datasource model.
func TestResponseToDataSourceModel_QueryShape(t *testing.T) {
	ctx := context.Background()
	api := &scrubTaskAPI{
		ID:          1,
		Pool:        1,
		PoolName:    "tank",
		Threshold:   35,
		Description: "",
		Schedule:    scheduleAPI{Minute: "00", Hour: "00", Dom: "*", Month: "*", Dow: "7"},
		Enabled:     true,
	}

	m := &ScrubTaskDataSourceModel{}
	diags := responseToDataSourceModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.PoolName.ValueString() != "tank" {
		t.Errorf("PoolName = %q, want \"tank\"", m.PoolName.ValueString())
	}
}

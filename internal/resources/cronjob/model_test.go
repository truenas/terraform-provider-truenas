// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cronjob

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

// baseUnsetModel returns a model with every field null/unknown except the
// two Required fields, for tests that only care about a subset.
func baseUnsetModel(command, user string) *CronjobModel {
	return &CronjobModel{
		Command:     types.StringValue(command),
		User:        types.StringValue(user),
		Description: types.StringNull(),
		Schedule:    types.ObjectNull(scheduleAttrTypes),
		Enabled:     types.BoolNull(),
		Stdout:      types.BoolNull(),
		Stderr:      types.BoolNull(),
	}
}

// TestApiPayload_UnsetOptionalsOmitted verifies that every Optional field is
// omitted from the payload when null/unknown, leaving only the two Required
// fields "command" and "user" — so TrueNAS-side defaults take effect.
func TestApiPayload_UnsetOptionalsOmitted(t *testing.T) {
	ctx := context.Background()
	m := baseUnsetModel("/usr/bin/true", "root")

	p, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostics errors: %v", diags)
	}

	if len(p) != 2 {
		t.Fatalf("payload has %d keys (%v), want 2 (command, user)", len(p), p)
	}
	if p["command"] != "/usr/bin/true" || p["user"] != "root" {
		t.Errorf("payload = %v, want only command/user set", p)
	}
}

// TestApiPayload_FullySet verifies every optional field is included when
// known, matching the probed create shape.
func TestApiPayload_FullySet(t *testing.T) {
	ctx := context.Background()
	m := baseUnsetModel("/usr/bin/true", "root")
	m.Description = types.StringValue("tf-acc cron")
	m.Schedule = buildSchedule(t, "00", "3", "*", "*", "*")
	m.Enabled = types.BoolValue(false)
	m.Stdout = types.BoolValue(true)
	m.Stderr = types.BoolValue(false)

	p, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostics errors: %v", diags)
	}

	if p["description"] != "tf-acc cron" {
		t.Errorf("payload[description] = %v, want \"tf-acc cron\"", p["description"])
	}
	if p["enabled"] != false {
		t.Errorf("payload[enabled] = %v, want false", p["enabled"])
	}
	if p["stdout"] != true {
		t.Errorf("payload[stdout] = %v, want true", p["stdout"])
	}
	if p["stderr"] != false {
		t.Errorf("payload[stderr] = %v, want false", p["stderr"])
	}
	sched, ok := p["schedule"].(map[string]string)
	if !ok {
		t.Fatalf("payload[schedule] is %T, want map[string]string", p["schedule"])
	}
	if sched["minute"] != "00" || sched["hour"] != "3" {
		t.Errorf("payload[schedule] = %v, want minute=00 hour=3", sched)
	}
}

// TestResponseToModel_ProbedShape verifies responseToModel against the
// exact shape observed from a live cronjob.create/query call: no nullable
// fields, no extra runtime-status fields.
func TestResponseToModel_ProbedShape(t *testing.T) {
	ctx := context.Background()
	api := &cronjobAPI{
		ID:          1,
		Command:     "/usr/bin/true",
		User:        "root",
		Description: "",
		Schedule:    scheduleAPI{Minute: "00", Hour: "*", Dom: "*", Month: "*", Dow: "*"},
		Enabled:     false,
		Stdout:      true,
		Stderr:      false,
	}

	m := &CronjobModel{}
	diags := responseToModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if m.ID.ValueInt64() != 1 {
		t.Errorf("ID = %v, want 1", m.ID)
	}
	if m.Command.ValueString() != "/usr/bin/true" {
		t.Errorf("Command = %q, want /usr/bin/true", m.Command.ValueString())
	}
	if m.Description.ValueString() != "" {
		t.Errorf("Description = %q, want empty string", m.Description.ValueString())
	}
	if m.Enabled.ValueBool() {
		t.Error("Enabled = true, want false")
	}
	if !m.Stdout.ValueBool() {
		t.Error("Stdout = false, want true")
	}
	if m.Stderr.ValueBool() {
		t.Error("Stderr = true, want false")
	}

	var sched ScheduleModel
	diags = m.Schedule.As(ctx, &sched, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("reading back schedule: %v", diags)
	}
	if sched.Minute.ValueString() != "00" {
		t.Errorf("schedule.minute = %q, want \"00\"", sched.Minute.ValueString())
	}
}

// TestResponseToDataSourceModel_ProbedShape mirrors
// TestResponseToModel_ProbedShape for the datasource model.
func TestResponseToDataSourceModel_ProbedShape(t *testing.T) {
	ctx := context.Background()
	api := &cronjobAPI{
		ID:          1,
		Command:     "/usr/bin/true",
		User:        "root",
		Description: "tf-probe cron",
		Schedule:    scheduleAPI{Minute: "00", Hour: "*", Dom: "*", Month: "*", Dow: "*"},
		Enabled:     false,
		Stdout:      true,
		Stderr:      false,
	}

	m := &CronjobDataSourceModel{}
	diags := responseToDataSourceModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.Description.ValueString() != "tf-probe cron" {
		t.Errorf("Description = %q, want \"tf-probe cron\"", m.Description.ValueString())
	}
	if m.User.ValueString() != "root" {
		t.Errorf("User = %q, want root", m.User.ValueString())
	}
}

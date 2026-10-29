// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package periodic_snapshot

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestPeriodicSnapshotSchema(t *testing.T) {
	s := resourceSchema()

	schedAttr, ok := s.Attributes["schedule"]
	if !ok {
		t.Fatal("schema missing 'schedule' attribute")
	}
	nested, ok := schedAttr.(schema.SingleNestedAttribute)
	if !ok {
		t.Fatalf("'schedule' attribute is %T, want schema.SingleNestedAttribute", schedAttr)
	}
	for _, field := range []string{"minute", "hour", "dom", "month", "dow"} {
		a, exists := nested.Attributes[field]
		if !exists {
			t.Errorf("schedule missing field %q", field)
			continue
		}
		sa, ok := a.(schema.StringAttribute)
		if !ok {
			t.Errorf("schedule.%s is %T, want schema.StringAttribute", field, a)
			continue
		}
		if !sa.IsRequired() {
			t.Errorf("schedule.%s should be Required", field)
		}
	}

	idAttr, ok := s.Attributes["id"]
	if !ok {
		t.Fatal("schema missing 'id' attribute")
	}
	if _, ok := idAttr.(schema.Int64Attribute); !ok {
		t.Fatalf("'id' attribute is %T, want schema.Int64Attribute", idAttr)
	}
}

func TestPeriodicSnapshotPayload(t *testing.T) {
	ctx := context.Background()

	m := PeriodicSnapshotModel{
		Dataset:       types.StringValue("tank/mydata"),
		Recursive:     types.BoolValue(false),
		Exclude:       types.ListValueMust(types.StringType, []attr.Value{}),
		LifetimeValue: types.Int64Value(2),
		LifetimeUnit:  types.StringValue("WEEK"),
		NamingSchema:  types.StringValue("auto-%Y-%m-%d_%H-%M"),
		Schedule: types.ObjectValueMust(scheduleAttrTypes, map[string]attr.Value{
			"minute": types.StringValue("0"),
			"hour":   types.StringValue("0"),
			"dom":    types.StringValue("*"),
			"month":  types.StringValue("*"),
			"dow":    types.StringValue("*"),
			"begin":  types.StringValue("08:00"),
			"end":    types.StringValue("17:00"),
		}),
		AllowEmpty: types.BoolValue(false),
		Enabled:    types.BoolValue(true),
	}

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostics errors: %v", diags)
	}

	requiredKeys := []string{
		"dataset", "recursive", "exclude",
		"lifetime_value", "lifetime_unit", "naming_schema",
		"schedule", "allow_empty", "enabled",
	}
	for _, key := range requiredKeys {
		if _, ok := payload[key]; !ok {
			t.Errorf("payload missing required key %q", key)
		}
	}

	if payload["dataset"] != "tank/mydata" {
		t.Errorf("payload[dataset] = %v, want tank/mydata", payload["dataset"])
	}
	if payload["lifetime_value"] != int64(2) {
		t.Errorf("payload[lifetime_value] = %v, want 2", payload["lifetime_value"])
	}

	sched, ok := payload["schedule"].(map[string]string)
	if !ok {
		t.Fatalf("payload[schedule] is %T, want map[string]string", payload["schedule"])
	}
	if sched["minute"] != "0" {
		t.Errorf("schedule.minute = %q, want \"0\"", sched["minute"])
	}
	if sched["dom"] != "*" {
		t.Errorf("schedule.dom = %q, want \"*\"", sched["dom"])
	}
	// The server replaces the whole schedule object, so a known window must
	// always be sent back (B-036).
	if sched["begin"] != "08:00" || sched["end"] != "17:00" {
		t.Errorf("schedule window = %q-%q, want 08:00-17:00", sched["begin"], sched["end"])
	}
}

// TestPeriodicSnapshotPayload_UnsetOmitted: Optional+Computed attributes left
// unset are unknown in the create plan and must be omitted so the server
// defaults apply. Sending ValueBool() of an unknown sent enabled=false and
// allow_empty=false, creating a task that never took a snapshot (B-010).
func TestPeriodicSnapshotPayload_UnsetOmitted(t *testing.T) {
	ctx := context.Background()
	m := PeriodicSnapshotModel{
		Dataset:       types.StringValue("tank/mydata"),
		Recursive:     types.BoolUnknown(),
		Exclude:       types.ListUnknown(types.StringType),
		LifetimeValue: types.Int64Value(2),
		LifetimeUnit:  types.StringValue("WEEK"),
		NamingSchema:  types.StringValue("auto-%Y-%m-%d_%H-%M"),
		Schedule: types.ObjectValueMust(scheduleAttrTypes, map[string]attr.Value{
			"minute": types.StringValue("0"),
			"hour":   types.StringValue("0"),
			"dom":    types.StringValue("*"),
			"month":  types.StringValue("*"),
			"dow":    types.StringValue("*"),
			"begin":  types.StringUnknown(),
			"end":    types.StringUnknown(),
		}),
		AllowEmpty: types.BoolUnknown(),
		Enabled:    types.BoolUnknown(),
	}
	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostics errors: %v", diags)
	}
	for _, key := range []string{"recursive", "exclude", "allow_empty", "enabled"} {
		if v, ok := payload[key]; ok {
			t.Errorf("payload[%s] = %v, want it omitted when unset", key, v)
		}
	}
	sched := payload["schedule"].(map[string]string)
	for _, key := range []string{"begin", "end"} {
		if v, ok := sched[key]; ok {
			t.Errorf("schedule[%s] = %q, want it omitted when unset", key, v)
		}
	}
}

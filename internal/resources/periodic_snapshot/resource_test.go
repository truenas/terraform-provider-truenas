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
		Schedule: ScheduleModel{
			Minute: types.StringValue("0"),
			Hour:   types.StringValue("0"),
			Dom:    types.StringValue("*"),
			Month:  types.StringValue("*"),
			Dow:    types.StringValue("*"),
		},
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
}

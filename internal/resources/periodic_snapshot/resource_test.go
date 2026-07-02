package periodic_snapshot

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestPeriodicSnapshotSchema verifies that the resource schema has a schedule
// SingleNestedAttribute containing exactly the five required cron string fields.
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

	// Verify id is Int64Attribute
	idAttr, ok := s.Attributes["id"]
	if !ok {
		t.Fatal("schema missing 'id' attribute")
	}
	if _, ok := idAttr.(schema.Int64Attribute); !ok {
		t.Fatalf("'id' attribute is %T, want schema.Int64Attribute", idAttr)
	}
}

// TestPeriodicSnapshotPayload verifies that apiPayload produces a map with all
// required API fields and that the schedule sub-map is well-formed.
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

// TestAccPeriodicSnapshot_basic is an acceptance test that requires TF_ACC=1
// and a live TrueNAS at 192.168.1.68 with the tank/mydata dataset.
func TestAccPeriodicSnapshot_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccPeriodicSnapshotConfig(true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_periodic_snapshot_task.test", "dataset", "tank/mydata"),
					resource.TestCheckResourceAttr("truenas_periodic_snapshot_task.test", "enabled", "true"),
					resource.TestCheckResourceAttr("truenas_periodic_snapshot_task.test", "lifetime_value", "2"),
					resource.TestCheckResourceAttr("truenas_periodic_snapshot_task.test", "lifetime_unit", "WEEK"),
					resource.TestCheckResourceAttr("truenas_periodic_snapshot_task.test", "schedule.minute", "0"),
					resource.TestCheckResourceAttr("truenas_periodic_snapshot_task.test", "schedule.hour", "0"),
					resource.TestCheckResourceAttrSet("truenas_periodic_snapshot_task.test", "id"),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccPeriodicSnapshotConfig(false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_periodic_snapshot_task.test", "enabled", "false"),
				),
			},
			{
				ResourceName:      "truenas_periodic_snapshot_task.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccPeriodicSnapshotConfig(enabled bool) string {
	return fmt.Sprintf(`
resource "truenas_periodic_snapshot_task" "test" {
  dataset        = "tank/mydata"
  recursive      = false
  lifetime_value = 2
  lifetime_unit  = "WEEK"
  naming_schema  = "auto-%%Y-%%m-%%d_%%H-%%M"
  enabled        = %v
  schedule = {
    minute = "0"
    hour   = "0"
    dom    = "*"
    month  = "*"
    dow    = "*"
  }
}
`, enabled)
}

package periodic_snapshot_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

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

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package periodic_snapshot_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccPeriodicSnapshot_dataSourceAmbiguous: with an hourly and a daily task
// on one dataset, a lookup by dataset silently returned the first (B-131). It
// is now an error, and naming_schema picks one.
func TestAccPeriodicSnapshot_dataSourceAmbiguous(t *testing.T) {
	ds := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tffix3shtasks-pst"))
	base := acctest.ProviderConfig() + pstMinimalConfig(ds, 2, "") + `
resource "truenas_periodic_snapshot_task" "daily" {
  dataset        = truenas_dataset.fixture.name
  lifetime_value = 1
  lifetime_unit  = "WEEK"
  naming_schema  = "tffix3-daily-%Y%m%d-%H%M"
  schedule = { minute = "0", hour = "1", dom = "*", month = "*", dow = "*" }
}
`
	byDataset := base + `
data "truenas_periodic_snapshot_task" "d" {
  dataset    = truenas_dataset.fixture.name
  depends_on = [truenas_periodic_snapshot_task.test, truenas_periodic_snapshot_task.daily]
}
`
	bySchema := base + `
data "truenas_periodic_snapshot_task" "d" {
  dataset       = truenas_dataset.fixture.name
  naming_schema = truenas_periodic_snapshot_task.daily.naming_schema
  depends_on    = [truenas_periodic_snapshot_task.test]
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: base},
			{Config: byDataset, ExpectError: regexp.MustCompile(`Ambiguous lookup`)},
			{
				Config: bySchema,
				Check:  resource.TestCheckResourceAttrPair("data.truenas_periodic_snapshot_task.d", "id", "truenas_periodic_snapshot_task.daily", "id"),
			},
		},
	})
}

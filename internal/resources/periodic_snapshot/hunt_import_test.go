// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package periodic_snapshot_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

var _ = fmt.Sprintf

func huntImportPersistSteps(cfg, addr string) []resource.TestStep {
	var id string
	parts := strings.SplitN(addr, ".", 2)
	return []resource.TestStep{
		{
			Config: cfg,
			Check: func(s *terraform.State) error {
				id = s.RootModule().Resources[addr].Primary.ID
				return nil
			},
		},
		{
			Config: strings.Replace(cfg, fmt.Sprintf("resource %q %q {", parts[0], parts[1]),
				fmt.Sprintf("removed {\n  from = %s\n  lifecycle {\n    destroy = false\n  }\n}\n\nresource %q \"tfhunt_unused\" {\n  count = 0\n", addr, parts[0]), 1),
		},
		{
			Config:             cfg,
			ResourceName:       addr,
			ImportState:        true,
			ImportStatePersist: true,
			ImportStateIdFunc:  func(*terraform.State) (string, error) { return id, nil },
		},
		{
			Config: cfg,
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
			},
		},
	}
}

// TestHuntImport_pstMinimal: minimal config (optional attrs unset), persisted import, plan must be empty.
func TestHuntImport_pstMinimal(t *testing.T) {
	name := acctest.RandName("tfhunt-imp")
	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "src" {
  name = "%[1]s/%[2]s"
}

resource "truenas_periodic_snapshot_task" "test" {
  dataset        = truenas_dataset.src.name
  lifetime_value = 1
  lifetime_unit  = "DAY"
  naming_schema  = "tfhunt-%%Y%%m%%d-%%H%%M"
  enabled        = false
  schedule = { minute = "0", hour = "0", dom = "*", month = "*", dow = "*" }
}
`, acctest.TestPool(), name)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps:                    huntImportPersistSteps(cfg, "truenas_periodic_snapshot_task.test"),
	})
}

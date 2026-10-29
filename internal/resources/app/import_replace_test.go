// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package app_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func flameAppConfig(name, extra, values string) string {
	return fmt.Sprintf(`
resource "truenas_app" "test" {
  name        = %q
  catalog_app = "flame"
  train       = "community"
%s
  values      = %s
}
`, name, extra, values)
}

// Pattern 4: import an app, then plan with the same config. catalog_app is
// never read -> RequiresReplace from null -> "flame" forces replacement.
func TestAccApp_importReplan(t *testing.T) {
	name := "tfhuntappim"
	vals := `jsonencode({ flame = { password = "x" }, network = { web_port = { port_number = 30996 } } })`
	cfg := acctest.ProviderConfig() + flameAppConfig(name, "", vals)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AppsCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg},
			{Config: acctest.ProviderConfig() + `
removed {
  from = truenas_app.test
  lifecycle { destroy = false }
}
`},
			{
				Config:             cfg,
				ResourceName:       "truenas_app.test",
				ImportState:        true,
				ImportStatePersist: true,
				ImportStateId:      name,
			},
			{
				// catalog_app is read from metadata.name (fix C1). values is not
				// read back on import (separate class C8), so it is recorded in place.
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						acctest.ExpectNoReplace("truenas_app.test"),
						acctest.ExpectOnlyAttrChanges("truenas_app.test", "values"),
					},
				},
			},
			{
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			appCleanupStep(t, name, ""),
		},
	})
}

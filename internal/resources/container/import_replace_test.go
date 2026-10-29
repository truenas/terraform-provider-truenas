// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package container_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func containerConfig(name, version, extra string) string {
	return fmt.Sprintf(`
data "truenas_container_image" "alpine" {
  name = "alpine:3.22:amd64:default"
}

resource "truenas_container" "test" {
  name      = %q
  pool      = %q
  autostart = false
  image = {
    name    = "alpine:3.22:amd64:default"
    version = %s
  }
%s
}
`, name, acctest.TestPool(), version, extra)
}

const latest = "data.truenas_container_image.alpine.latest_version"

// captureID stores the resource's id attribute for a later import step.
func captureID(addr string, dst *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[addr]
		if !ok {
			return fmt.Errorf("%s not in state", addr)
		}
		*dst = rs.Primary.ID
		return nil
	}
}

// importReplanSteps: forget the resource (removed block), import it by id,
// then plan with the original config; the plan must be empty.
func importReplanSteps(cfg, forgetCfg, addr string, id *string) []resource.TestStep {
	return []resource.TestStep{
		{Config: forgetCfg},
		{Config: cfg, ResourceName: addr, ImportState: true, ImportStatePersist: true,
			ImportStateIdFunc: func(*terraform.State) (string, error) { return *id, nil }},
		// image is never returned: recorded in place, never a replacement (fix C1).
		{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
			acctest.ExpectNoReplace(addr), acctest.ExpectOnlyAttrChanges(addr, "image"),
		}}},
		{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}
}

const forgetCt = `
removed {
  from = truenas_container.test
  lifecycle { destroy = false }
}
`

// idmap DEFAULT spelled with jsonencode; import then replan.
func TestAccContainer_idmapDefaultImport(t *testing.T) {
	if !acctest.ServerVersionAtLeast(t, 26, 0) {
		t.Skip("truenas_container requires TrueNAS 26.0 or later")
	}
	name := "tfhuntct5"
	var id string
	cfg := acctest.ProviderConfig() + containerConfig(name, latest, `  idmap = jsonencode({ type = "DEFAULT" })`)
	steps := []resource.TestStep{{Config: cfg, Check: captureID("truenas_container.test", &id)}}
	steps = append(steps, importReplanSteps(cfg, acctest.ProviderConfig()+forgetCt, "truenas_container.test", &id)...)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps:                    steps,
	})
}

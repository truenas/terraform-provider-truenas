// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func huntVM(name, extra string) string {
	return fmt.Sprintf(`
resource "truenas_vm" "test" {
  name      = %q
  memory    = 512
  autostart = false
%s
}
`, name, extra)
}

// Import a VM with many attributes set, then re-plan.
func TestHuntVM_importReplan(t *testing.T) {
	name := "tfhuntvm6"
	var id string
	cfg := acctest.ProviderConfig() + huntVM(name, `
  description       = "d"
  vcpus             = 1
  cores             = 2
  threads           = 1
  min_memory        = 256
  cpu_mode          = "CUSTOM"
  cpu_model         = "Haswell"
  machine_type      = "q35"
  arch_type         = "x86_64"
  command_line_args = "-no-hpet"
  cpuset            = "0-1"
  nodeset           = "0"
  pin_vcpus         = true
  time              = "UTC"
  shutdown_timeout  = 30
  bootloader        = "UEFI"
`)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg, Check: func(s *terraform.State) error {
				id = s.RootModule().Resources["truenas_vm.test"].Primary.ID
				return nil
			}},
			{Config: acctest.ProviderConfig() + `
removed {
  from = truenas_vm.test
  lifecycle { destroy = false }
}
`},
			{Config: cfg, ResourceName: "truenas_vm.test", ImportState: true, ImportStatePersist: true,
				ImportStateIdFunc: func(*terraform.State) (string, error) { return id, nil }},
			{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			// set -> unset: drop everything optional
			{Config: acctest.ProviderConfig() + huntVM(name, ``)},
		},
	})
}

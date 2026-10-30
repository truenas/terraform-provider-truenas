// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm_device_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func huntVMDev(vmName, devs string) string {
	return fmt.Sprintf(`
resource "truenas_vm" "test" {
  name      = %q
  memory    = 512
  autostart = false
}
%s
`, vmName, devs)
}

// Pattern 4 (#30 class): forget sparse devices, import them, re-plan.
func TestHuntVMDevice_importSparse(t *testing.T) {
	acctest.PreCheck(t) // nicAttach needs the API before resource.Test's own PreCheck
	nic := nicAttach(t)
	devs := fmt.Sprintf(`
resource "truenas_vm_device" "nic" {
  vm         = truenas_vm.test.id
  attributes = jsonencode({ dtype = "NIC", type = "VIRTIO", nic_attach = %q })
}
resource "truenas_vm_device" "disp" {
  vm                            = truenas_vm.test.id
  attributes                    = jsonencode({ dtype = "DISPLAY", type = "SPICE", bind = "0.0.0.0", port = 15910, web_port = 15911 })
  attributes_secrets_wo         = jsonencode({ password = "tfhuntpw" })
  attributes_secrets_wo_version = 1
}
`, nic)
	forget := `
removed {
  from = truenas_vm_device.nic
  lifecycle { destroy = false }
}
removed {
  from = truenas_vm_device.disp
  lifecycle { destroy = false }
}
`
	cfg := acctest.ProviderConfig() + huntVMDev("tfhuntvmd2", devs)
	var nicID, dispID string
	capture := func(addr string, dst *string) resource.TestCheckFunc {
		return func(s *terraform.State) error {
			*dst = s.RootModule().Resources[addr].Primary.ID
			return nil
		}
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg, Check: resource.ComposeTestCheckFunc(
				capture("truenas_vm_device.nic", &nicID), capture("truenas_vm_device.disp", &dispID))},
			{Config: acctest.ProviderConfig() + huntVMDev("tfhuntvmd2", forget)},
			{Config: cfg, ResourceName: "truenas_vm_device.nic", ImportState: true, ImportStatePersist: true,
				ImportStateIdFunc: func(*terraform.State) (string, error) { return nicID, nil }},
			{Config: cfg, ResourceName: "truenas_vm_device.disp", ImportState: true, ImportStatePersist: true,
				ImportStateIdFunc: func(*terraform.State) (string, error) { return dispID, nil }},
			// first plan after import: the imported state has no
			// attributes_secrets_wo_version, so this one apply sends the
			// write-only password (by design: the version is Terraform-owned);
			// the nic must not change.
			{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
				acctest.LogPlanDiff(t),
				plancheck.ExpectResourceAction("truenas_vm_device.nic", plancheck.ResourceActionNoop),
				plancheck.ExpectResourceAction("truenas_vm_device.disp", plancheck.ResourceActionUpdate),
			}}},
			{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{acctest.LogPlanDiff(t), plancheck.ExpectEmptyPlan()}}},
			// second apply after import: change one key
			{Config: acctest.ProviderConfig() + huntVMDev("tfhuntvmd2", fmt.Sprintf(`
resource "truenas_vm_device" "nic" {
  vm         = truenas_vm.test.id
  attributes = jsonencode({ dtype = "NIC", type = "E1000", nic_attach = %q })
}
resource "truenas_vm_device" "disp" {
  vm                            = truenas_vm.test.id
  attributes                    = jsonencode({ dtype = "DISPLAY", type = "SPICE", bind = "0.0.0.0", port = 15910, web_port = 15911 })
  attributes_secrets_wo         = jsonencode({ password = "tfhuntpw" })
  attributes_secrets_wo_version = 1
}
`, nic))},
		},
	})
}

// Coordinator suspect: with attributes_secrets_wo in use, Read rewrites
// attributes via ProjectOntoKeys (canonical JSON), so a heredoc/hand-written
// attributes string diffs forever.
func TestHuntVMDevice_woHeredoc(t *testing.T) {
	cfg := acctest.ProviderConfig() + huntVMDev("tfhuntvmd3", `
resource "truenas_vm_device" "disp" {
  vm         = truenas_vm.test.id
  attributes = <<-EOT
    {"dtype": "DISPLAY", "type": "SPICE", "bind": "0.0.0.0", "port": 15920, "web_port": 15921}
  EOT
  attributes_secrets_wo         = jsonencode({ password = "tfhuntpw" })
  attributes_secrets_wo_version = 1
}
`)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps:                    []resource.TestStep{{Config: cfg}},
	})
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package container_device_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

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

func nicDevicesConfig(containerName, datasetName, devs string) string {
	return fmt.Sprintf(`
data "truenas_container_image" "alpine" {
  name = "alpine:3.22:amd64:default"
}

resource "truenas_container" "test" {
  name      = %q
  pool      = %q
  autostart = false
  running   = false
  image = {
    name    = "alpine:3.22:amd64:default"
    version = data.truenas_container_image.alpine.latest_version
  }
  lifecycle { ignore_changes = [image] }
}

resource "truenas_dataset" "test" {
  name = %q
}
%s
`, containerName, acctest.TestPool(), datasetName, devs)
}

// TestAccContainerDevice_nicMacCaseAndImport creates a NIC with an upper-case
// mac (the server stores it lower case) and a sparse FILESYSTEM device, checks
// the re-plan is empty, then forgets both devices, imports them and checks the
// plan is still empty.
//
// Before the fix the refresh plan wanted mac "00:a0:..." -> "00:A0:..." and to
// remove the server-filled type/trust_guest_rx_filters keys (B-082), and the
// plan after import showed an update because state held the full server map
// (B-084; the #30 fix had been applied to vm_device only).
func TestAccContainerDevice_nicMacCaseAndImport(t *testing.T) {
	if !acctest.ServerVersionAtLeast(t, 26, 0) {
		t.Skip("truenas_container_device requires TrueNAS 26.0 or later")
	}
	containerName := "tffix3virtct" + acctest.RandName("")
	datasetName := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tffix3virtds"))
	devs := `
resource "truenas_container_device" "nic" {
  container  = truenas_container.test.id
  attributes = jsonencode({ dtype = "NIC", nic_attach = "truenasbr0", mac = "00:A0:98:12:34:5A" })
}
resource "truenas_container_device" "fs" {
  container  = truenas_container.test.id
  attributes = jsonencode({ dtype = "FILESYSTEM", source = truenas_dataset.test.mountpoint, target = "/data" })
}
`
	forget := `
removed {
  from = truenas_container_device.nic
  lifecycle { destroy = false }
}
removed {
  from = truenas_container_device.fs
  lifecycle { destroy = false }
}
`
	cfg := acctest.ProviderConfig() + nicDevicesConfig(containerName, datasetName, devs)
	empty := resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}
	var nicID, fsID string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckContainerDestroyed(containerName),
		Steps: []resource.TestStep{
			{Config: cfg, Check: resource.ComposeTestCheckFunc(
				captureID("truenas_container_device.nic", &nicID),
				captureID("truenas_container_device.fs", &fsID))},
			{Config: cfg, ConfigPlanChecks: empty},
			{Config: acctest.ProviderConfig() + nicDevicesConfig(containerName, datasetName, forget)},
			{Config: cfg, ResourceName: "truenas_container_device.fs", ImportState: true, ImportStatePersist: true,
				ImportStateIdFunc: func(*terraform.State) (string, error) { return fsID, nil }},
			{Config: cfg, ResourceName: "truenas_container_device.nic", ImportState: true, ImportStatePersist: true,
				ImportStateIdFunc: func(*terraform.State) (string, error) { return nicID, nil }},
			{Config: cfg, ConfigPlanChecks: empty},
		},
	})
}

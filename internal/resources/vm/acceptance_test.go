// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccVM_basic tests create, update, and import of a VM. It creates and
// destroys its own RandName-suffixed VM, with autostart disabled and
// running left false throughout, so it never starts a guest on the target
// host.
func TestAccVM_basic(t *testing.T) {
	name := "tfaccvm" + strings.ReplaceAll(acctest.RandName(""), "-", "") // vm_create.name: alphanumeric only

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckVMDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccVMConfig(name, "initial description", 1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_vm.test", "name", name),
					resource.TestCheckResourceAttr("truenas_vm.test", "description", "initial description"),
					resource.TestCheckResourceAttr("truenas_vm.test", "memory", "536870912"),
					resource.TestCheckResourceAttr("truenas_vm.test", "vcpus", "1"),
					resource.TestCheckResourceAttr("truenas_vm.test", "autostart", "false"),
					resource.TestCheckResourceAttr("truenas_vm.test", "running", "false"),
					resource.TestCheckResourceAttrSet("truenas_vm.test", "id"),
					resource.TestCheckResourceAttrSet("truenas_vm.test", "status"),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccVMConfig(name, "updated description", 2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_vm.test", "description", "updated description"),
					resource.TestCheckResourceAttr("truenas_vm.test", "vcpus", "2"),
					resource.TestCheckResourceAttr("truenas_vm.test", "running", "false"),
				),
			},
			{
				ResourceName:      "truenas_vm.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccVMConfig(name, description string, vcpus int) string {
	return fmt.Sprintf(`
resource "truenas_vm" "test" {
  name        = %q
  description = %q
  memory      = 536870912
  vcpus       = %d
  autostart   = false
  running     = false

  min_memory                    = 268435456
  cores                         = 2
  threads                       = 1
  bootloader                    = "UEFI"
  time                          = "UTC"
  shutdown_timeout              = 120
  cpu_mode                      = "HOST-MODEL"
  cpuset                        = "0"
  nodeset                       = "0"
  pin_vcpus                     = false
  command_line_args             = ""
  enable_secure_boot            = false
  trusted_platform_module       = true
  hide_from_msr                 = true
  hyperv_enlightenments         = true
  enable_cpu_topology_extension = true
  suspend_on_snapshot           = false
}
`, name, description, vcpus)
}

// TestAccVM_hardware sets the hardware/firmware options (arch_type,
// machine_type, bootloader_ovmf, cpu_mode CUSTOM + cpu_model) on a STOPPED VM.
// The guest is never started, so no bootable disk is needed. The framework's
// post-apply empty-plan check asserts each value round-trips; import-verify
// compares the read-back state to the config.
func TestAccVM_hardware(t *testing.T) {
	name := "tfaccvmhw" + strings.ReplaceAll(acctest.RandName(""), "-", "")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckVMDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccVMHardwareConfig(name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_vm.test", "arch_type", "x86_64"),
					resource.TestCheckResourceAttr("truenas_vm.test", "machine_type", "q35"),
					resource.TestCheckResourceAttr("truenas_vm.test", "bootloader_ovmf", "OVMF_CODE_4M.fd"),
					resource.TestCheckResourceAttr("truenas_vm.test", "bootloader", "UEFI"),
					resource.TestCheckResourceAttr("truenas_vm.test", "cpu_mode", "CUSTOM"),
					resource.TestCheckResourceAttr("truenas_vm.test", "cpu_model", "qemu64"),
					resource.TestCheckResourceAttr("truenas_vm.test", "running", "false"),
				),
			},
			{
				ResourceName:      "truenas_vm.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccVMHardwareConfig(name string) string {
	return fmt.Sprintf(`
resource "truenas_vm" "test" {
  name      = %q
  memory    = 536870912
  vcpus     = 1
  autostart = false
  running   = false

  arch_type       = "x86_64"
  machine_type    = "q35"
  bootloader      = "UEFI"
  bootloader_ovmf = "OVMF_CODE_4M.fd"
  cpu_mode        = "CUSTOM"
  cpu_model       = "qemu64"
}
`, name)
}

func testAccCheckVMDestroyed(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "vm.query", [][]any{{"name", "=", name}})
		if err != nil {
			return fmt.Errorf("error checking vm %s: %v", name, err)
		}
		var results []struct {
			ID any `json:"id"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing vm.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("vm %s still exists", name)
		}
		return nil
	}
}

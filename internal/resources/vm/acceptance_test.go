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
}
`, name, description, vcpus)
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

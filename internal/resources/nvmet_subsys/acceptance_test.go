// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_subsys_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccNVMetSubsys_basic tests create, update, and import of an NVMe-oF
// subsystem. It creates and destroys its own RandName-suffixed subsystem and
// never touches the box's existing LIVE subsystem (id=1,
// name="proxmox-test"), which serves live storage.
func TestAccNVMetSubsys_basic(t *testing.T) {
	name := acctest.RandName("tf-acc-subsys")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckNVMetSubsysDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccNVMetSubsysConfig(name, false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_nvmet_subsys.test", "name", name),
					resource.TestCheckResourceAttr("truenas_nvmet_subsys.test", "allow_any_host", "false"),
					resource.TestCheckResourceAttrSet("truenas_nvmet_subsys.test", "id"),
					resource.TestCheckResourceAttrSet("truenas_nvmet_subsys.test", "subnqn"),
					resource.TestCheckResourceAttrSet("truenas_nvmet_subsys.test", "serial"),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccNVMetSubsysConfig(name, true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_nvmet_subsys.test", "allow_any_host", "true"),
				),
			},
			{
				ResourceName:      "truenas_nvmet_subsys.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccNVMetSubsysConfig(name string, allowAnyHost bool) string {
	return fmt.Sprintf(`
resource "truenas_nvmet_subsys" "test" {
  name           = %q
  allow_any_host = %v
}
`, name, allowAnyHost)
}

func testAccCheckNVMetSubsysDestroyed(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "nvmet.subsys.query", [][]any{{"name", "=", name}})
		if err != nil {
			return fmt.Errorf("error checking nvmet subsys %s: %v", name, err)
		}
		var results []struct {
			ID any `json:"id"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing nvmet.subsys.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("nvmet subsys %s still exists", name)
		}
		return nil
	}
}

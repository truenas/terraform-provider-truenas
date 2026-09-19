// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_namespace_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccNVMetNamespace_basic tests create, update, and import of an
// NVMe-oF namespace. It creates and destroys its own RandName-suffixed
// subsystem, zvol, and namespace, and never touches the box's existing LIVE
// subsystem/namespace (id=1), which serves live storage.
func TestAccNVMetNamespace_basic(t *testing.T) {
	subsysName := acctest.RandName("tf-acc-ns-subsys")
	zvolName := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-ns-zvol"))
	devicePath := fmt.Sprintf("zvol/%s", zvolName)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckNVMetNamespaceDestroyed(devicePath, zvolName),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccNVMetNamespaceConfig(subsysName, zvolName, true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_nvmet_namespace.test", "device_path", devicePath),
					resource.TestCheckResourceAttr("truenas_nvmet_namespace.test", "device_type", "ZVOL"),
					resource.TestCheckResourceAttr("truenas_nvmet_namespace.test", "enabled", "true"),
					resource.TestCheckResourceAttrSet("truenas_nvmet_namespace.test", "id"),
					resource.TestCheckResourceAttrSet("truenas_nvmet_namespace.test", "nsid"),
					resource.TestCheckResourceAttrSet("truenas_nvmet_namespace.test", "device_nguid"),
					resource.TestCheckResourceAttrSet("truenas_nvmet_namespace.test", "device_uuid"),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccNVMetNamespaceConfig(subsysName, zvolName, false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_nvmet_namespace.test", "enabled", "false"),
				),
			},
			{
				ResourceName:      "truenas_nvmet_namespace.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccNVMetNamespaceConfig(subsysName, zvolName string, enabled bool) string {
	return fmt.Sprintf(`
resource "truenas_nvmet_subsys" "test" {
  name = %q
}

resource "truenas_zvol" "test" {
  name    = %q
  volsize = 67108864
}

resource "truenas_nvmet_namespace" "test" {
  subsys_id   = truenas_nvmet_subsys.test.id
  device_path = "zvol/${truenas_zvol.test.name}"
  device_type = "ZVOL"
  enabled     = %v
}
`, subsysName, zvolName, enabled)
}

func testAccCheckNVMetNamespaceDestroyed(devicePath, zvolName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		ctx := context.Background()

		raw, err := c.Call(ctx, "nvmet.namespace.query", [][]any{{"device_path", "=", devicePath}})
		if err != nil {
			return fmt.Errorf("error checking nvmet namespace %s: %v", devicePath, err)
		}
		var results []struct {
			ID any `json:"id"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing nvmet.namespace.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("nvmet namespace %s still exists", devicePath)
		}

		rawZ, err := c.Call(ctx, "pool.dataset.query", [][]any{{"id", "=", zvolName}})
		if err != nil {
			return fmt.Errorf("error checking zvol %s: %v", zvolName, err)
		}
		var zResults []struct {
			ID any `json:"id"`
		}
		if err := json.Unmarshal(rawZ, &zResults); err != nil {
			return fmt.Errorf("error parsing pool.dataset.query response: %v", err)
		}
		if len(zResults) > 0 {
			return fmt.Errorf("zvol %s still exists", zvolName)
		}

		return nil
	}
}

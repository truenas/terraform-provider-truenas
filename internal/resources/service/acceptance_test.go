// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package service_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccService_ftp enables and then restores the "ftp" service's enabled
// (autostart) flag. ftp is verified stopped/disabled on the target box, so
// this test only ever toggles "enabled" and never sets "running", leaving
// the service's actual running state untouched throughout. The final step
// restores enabled=false (its original state) and a post-test API read
// confirms that restoration, since truenas_service resources are never
// created or destroyed (services are not create/delete-able) and so
// CheckDestroy cannot assert absence.
func TestAccService_ftp(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckServiceFTPRestored,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccServiceFTPConfig(true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_service.test", "name", "ftp"),
					resource.TestCheckResourceAttr("truenas_service.test", "enabled", "true"),
					resource.TestCheckResourceAttr("truenas_service.test", "id", "ftp"),
				),
			},
			{
				ResourceName:      "truenas_service.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: acctest.ProviderConfig() + testAccServiceFTPConfig(false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_service.test", "enabled", "false"),
				),
			},
		},
	})
}

func testAccServiceFTPConfig(enabled bool) string {
	return fmt.Sprintf(`
resource "truenas_service" "test" {
  name    = "ftp"
  enabled = %v
}
`, enabled)
}

// testAccCheckServiceFTPRestored runs after the test's implicit final
// destroy (which, for truenas_service, best-effort disables and stops the
// service) and independently confirms via the live API that ftp ended up
// disabled, restoring the box to how it was found.
func testAccCheckServiceFTPRestored(s *terraform.State) error {
	c := acctest.Client()
	raw, err := c.Call(context.Background(), "service.query", [][]any{{"service", "=", "ftp"}})
	if err != nil {
		return fmt.Errorf("error reading back ftp service state: %v", err)
	}
	var results []struct {
		Enable bool `json:"enable"`
	}
	if err := json.Unmarshal(raw, &results); err != nil {
		return fmt.Errorf("error parsing service.query response: %v", err)
	}
	if len(results) == 0 {
		return fmt.Errorf("ftp service not found")
	}
	if results[0].Enable {
		return fmt.Errorf("ftp service was not restored to enabled=false")
	}
	return nil
}

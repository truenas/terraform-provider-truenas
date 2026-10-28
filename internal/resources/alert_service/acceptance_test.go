// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package alert_service_test

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

// TestAccAlertService_basic tests create, update, and import of a Mail
// alert service.
func TestAccAlertService_basic(t *testing.T) {
	name := acctest.RandName("tf-acc-alert")
	email := acctest.RandName("tf-acc-alert") + "@example.com"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAlertServiceDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccAlertServiceConfig(name, "WARNING", email),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_alert_service.test", "id"),
					resource.TestCheckResourceAttr("truenas_alert_service.test", "name", name),
					resource.TestCheckResourceAttr("truenas_alert_service.test", "level", "WARNING"),
				),
			},
			// Update the level in place.
			{
				Config: acctest.ProviderConfig() + testAccAlertServiceConfig(name, "ERROR", email),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_alert_service.test", "level", "ERROR"),
				),
			},
			{
				ResourceName:            "truenas_alert_service.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"attributes"},
			},
		},
	})
}

// TestAccAlertService_coverage creates a disabled Slack service whose webhook
// url is supplied only via the write-only attributes_secrets_wo overlay, and
// confirms the secret stays out of state and the plan is empty afterward.
func TestAccAlertService_coverage(t *testing.T) {
	name := acctest.RandName("tf-acc-alertwo")
	const secretURL = "https://hooks.slack.com/services/T000/B000/tfaccsecret"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAlertServiceDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccAlertServiceWOConfig(name, secretURL),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_alert_service.test", "id"),
					resource.TestCheckResourceAttr("truenas_alert_service.test", "enabled", "false"),
					resource.TestCheckResourceAttr("truenas_alert_service.test", "attributes_secrets_wo_version", "1"),
					resource.TestCheckNoResourceAttr("truenas_alert_service.test", "attributes_secrets_wo"),
					resource.TestCheckResourceAttrWith("truenas_alert_service.test", "attributes", func(v string) error {
						if strings.Contains(v, "tfaccsecret") {
							return fmt.Errorf("secret leaked into attributes state: %s", v)
						}
						return nil
					}),
				),
			},
			{
				ResourceName:            "truenas_alert_service.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"attributes", "attributes_secrets_wo_version"},
			},
		},
	})
}

func testAccAlertServiceWOConfig(name, secretURL string) string {
	return fmt.Sprintf(`
resource "truenas_alert_service" "test" {
  name    = %q
  level   = "WARNING"
  enabled = false
  attributes = jsonencode({
    type = "Slack"
  })
  attributes_secrets_wo         = jsonencode({ url = %q })
  attributes_secrets_wo_version = 1
}
`, name, secretURL)
}

func testAccAlertServiceConfig(name, level, email string) string {
	return fmt.Sprintf(`
resource "truenas_alert_service" "test" {
  name  = %q
  level = %q
  attributes = jsonencode({
    type  = "Mail"
    email = %q
  })
}
`, name, level, email)
}

func testAccCheckAlertServiceDestroyed(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "alertservice.query", [][]any{{"name", "=", name}})
		if err != nil {
			return fmt.Errorf("error checking alert service %s: %v", name, err)
		}
		var results []struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing alertservice.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("alert service %s still exists", name)
		}
		return nil
	}
}

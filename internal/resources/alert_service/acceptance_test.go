package alert_service_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccAlertService_basic tests create, update, and import of an alert
// service. It requires:
//   - TrueNAS_API_KEY set in the environment (checked by acctest.PreCheck)
//   - A running TrueNAS SCALE instance
func TestAccAlertService_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure a TrueNAS instance is available")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccAlertServiceConfig("tf-acc-alert", "WARNING", `{\"type\": \"Mail\", \"to\": [\"admin@example.com\"]}`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_alert_service.test", "id"),
					resource.TestCheckResourceAttr("truenas_alert_service.test", "name", "tf-acc-alert"),
					resource.TestCheckResourceAttr("truenas_alert_service.test", "level", "WARNING"),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccAlertServiceConfig("tf-acc-alert-renamed", "CRITICAL", `{\"type\": \"Mail\", \"to\": [\"admin@example.com\"]}`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_alert_service.test", "id"),
					resource.TestCheckResourceAttr("truenas_alert_service.test", "name", "tf-acc-alert-renamed"),
					resource.TestCheckResourceAttr("truenas_alert_service.test", "level", "CRITICAL"),
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

func testAccAlertServiceConfig(name, level, attributes string) string {
	return fmt.Sprintf(`
resource "truenas_alert_service" "test" {
  name       = %q
  level      = %q
  attributes = "%s"
}
`, name, level, attributes)
}

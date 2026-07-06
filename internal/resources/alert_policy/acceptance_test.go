package alert_policy_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccAlertPolicy_basic tests update and import of the singleton
// truenas_alert_policy resource. It requires:
//   - TF_ACC=1
//   - TRUENAS_API_KEY (or TRUENAS_USERNAME+TRUENAS_PASSWORD) set for
//     acctest.PreCheck
//   - A running TrueNAS SCALE instance
func TestAccAlertPolicy_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure a TrueNAS instance is available")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccAlertPolicyConfig(`{\"UPSBatteryLow\": {\"level\": \"CRITICAL\", \"policy\": \"IMMEDIATELY\"}}`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_alert_policy.test", "id", "alert_policy"),
					resource.TestCheckResourceAttrSet("truenas_alert_policy.test", "classes"),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccAlertPolicyConfig(`{\"UPSBatteryLow\": {\"level\": \"WARNING\", \"policy\": \"DAILY\"}}`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_alert_policy.test", "id", "alert_policy"),
					resource.TestCheckResourceAttrSet("truenas_alert_policy.test", "classes"),
				),
			},
			{
				ResourceName:      "truenas_alert_policy.test",
				ImportState:       true,
				ImportStateId:     "alert_policy",
				ImportStateVerify: true,
			},
		},
	})
}

func testAccAlertPolicyConfig(classes string) string {
	return fmt.Sprintf(`
resource "truenas_alert_policy" "test" {
  classes = "%s"
}
`, classes)
}

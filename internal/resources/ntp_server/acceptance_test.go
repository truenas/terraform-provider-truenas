package ntp_server_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccNTPServer_basic tests create, update, and import of an NTP server.
// It requires:
//   - TrueNAS_API_KEY set in the environment (checked by acctest.PreCheck)
//   - A running TrueNAS SCALE instance
func TestAccNTPServer_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure a TrueNAS instance is available")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccNTPServerConfig("tf-acc-ntp-1.example.com", 4, 10),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_ntp_server.test", "address", "tf-acc-ntp-1.example.com"),
					resource.TestCheckResourceAttr("truenas_ntp_server.test", "minpoll", "4"),
					resource.TestCheckResourceAttr("truenas_ntp_server.test", "maxpoll", "10"),
					resource.TestCheckResourceAttrSet("truenas_ntp_server.test", "id"),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccNTPServerConfig("tf-acc-ntp-1.example.com", 5, 9),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_ntp_server.test", "minpoll", "5"),
					resource.TestCheckResourceAttr("truenas_ntp_server.test", "maxpoll", "9"),
				),
			},
			{
				ResourceName:            "truenas_ntp_server.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"force"},
			},
		},
	})
}

func testAccNTPServerConfig(address string, minpoll, maxpoll int) string {
	return fmt.Sprintf(`
resource "truenas_ntp_server" "test" {
  address = %q
  minpoll = %d
  maxpoll = %d
  force   = true
}
`, address, minpoll, maxpoll)
}

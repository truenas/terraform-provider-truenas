package cloudsync_credentials_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccCloudsyncCredentials_basic tests create, update, and import of
// cloud sync credentials. It requires:
//   - TrueNAS_API_KEY set in the environment (checked by acctest.PreCheck)
//   - A running TrueNAS SCALE instance
func TestAccCloudsyncCredentials_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure a TrueNAS instance is available")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccCloudsyncCredentialsConfig("tf-acc-creds", `{\"type\": \"S3\", \"access_key_id\": \"AKIAEXAMPLE\", \"secret_access_key\": \"secretexample\"}`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_cloudsync_credentials.test", "id"),
					resource.TestCheckResourceAttr("truenas_cloudsync_credentials.test", "name", "tf-acc-creds"),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccCloudsyncCredentialsConfig("tf-acc-creds-renamed", `{\"type\": \"S3\", \"access_key_id\": \"AKIAEXAMPLE\", \"secret_access_key\": \"secretexample\"}`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_cloudsync_credentials.test", "id"),
					resource.TestCheckResourceAttr("truenas_cloudsync_credentials.test", "name", "tf-acc-creds-renamed"),
				),
			},
			{
				ResourceName:            "truenas_cloudsync_credentials.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"provider_config"},
			},
		},
	})
}

func testAccCloudsyncCredentialsConfig(name, providerConfig string) string {
	return fmt.Sprintf(`
resource "truenas_cloudsync_credentials" "test" {
  name            = %q
  provider_config = "%s"
}
`, name, providerConfig)
}

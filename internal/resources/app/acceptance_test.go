package app_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccApp_basic installs a catalog app, verifies computed attributes are
// populated, then stops it via the "running" attribute, and finally imports
// it to verify import parity.
//
// Requires TF_ACC=1 and credentials set via TRUENAS_API_KEY or
// TRUENAS_USERNAME+TRUENAS_PASSWORD, plus a reachable TrueNAS SCALE 24.10+
// system with the ix-truecharts (or default) catalog available.
func TestAccApp_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccAppConfig("tf-acc-mailhog", "mailhog", true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_app.test", "name", "tf-acc-mailhog"),
					resource.TestCheckResourceAttr("truenas_app.test", "catalog_app", "mailhog"),
					resource.TestCheckResourceAttr("truenas_app.test", "id", "tf-acc-mailhog"),
					resource.TestCheckResourceAttr("truenas_app.test", "running", "true"),
					resource.TestCheckResourceAttrSet("truenas_app.test", "state"),
					resource.TestCheckResourceAttrSet("truenas_app.test", "version"),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccAppConfig("tf-acc-mailhog", "mailhog", false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_app.test", "running", "false"),
					resource.TestCheckResourceAttr("truenas_app.test", "state", "STOPPED"),
				),
			},
			{
				ResourceName:      "truenas_app.test",
				ImportState:       true,
				ImportStateVerify: true,
				// values/custom_compose_config_string/catalog_app are
				// write-only and never echoed back by the API, so they
				// cannot be verified across an import (state will be
				// empty post-import until a config apply repopulates it).
				ImportStateVerifyIgnore: []string{"values", "custom_compose_config_string", "catalog_app"},
			},
		},
	})
}

// TestAccApp_datasource verifies that the truenas_app datasource can look up
// an app created by the resource in the same config.
func TestAccApp_datasource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccAppConfig("tf-acc-mailhog-ds", "mailhog", true) + `
data "truenas_app" "lookup" {
  name = truenas_app.test.name
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_app.lookup", "name", "tf-acc-mailhog-ds"),
					resource.TestCheckResourceAttrSet("data.truenas_app.lookup", "state"),
				),
			},
		},
	})
}

func testAccAppConfig(name, catalogApp string, running bool) string {
	return fmt.Sprintf(`
resource "truenas_app" "test" {
  name        = %q
  catalog_app = %q
  running     = %v
}
`, name, catalogApp, running)
}

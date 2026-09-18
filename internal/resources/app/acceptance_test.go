// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package app_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccApp_basic installs the "syncthing" catalog app with default values,
// verifies computed attributes are populated, stops it via the "running"
// attribute, and finally destroys it.
//
// Gated behind TRUENAS_APPS=1 (acctest.AppsCheck) since it pulls container
// images from the network and can be slow/bandwidth-heavy, in addition to
// TF_ACC=1 and credentials via TRUENAS_API_KEY or
// TRUENAS_USERNAME+TRUENAS_PASSWORD.
func TestAccApp_basic(t *testing.T) {
	// App names may have length limits; keep the RandName-suffixed name
	// comfortably under 40 chars.
	name := acctest.RandName("tf-acc-app")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AppsCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAppDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccAppConfig(name, "syncthing", true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_app.test", "name", name),
					resource.TestCheckResourceAttr("truenas_app.test", "catalog_app", "syncthing"),
					resource.TestCheckResourceAttr("truenas_app.test", "id", name),
					resource.TestCheckResourceAttr("truenas_app.test", "running", "true"),
					resource.TestCheckResourceAttrSet("truenas_app.test", "state"),
					resource.TestCheckResourceAttrSet("truenas_app.test", "version"),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccAppConfig(name, "syncthing", false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_app.test", "running", "false"),
					resource.TestCheckResourceAttr("truenas_app.test", "state", "STOPPED"),
				),
			},
		},
	})
}

// TestAccApp_datasource verifies that the truenas_app datasource can look up
// an app created by the resource in the same config. Gated behind
// TRUENAS_APPS=1 like TestAccApp_basic, since it also installs a real app.
func TestAccApp_datasource(t *testing.T) {
	name := acctest.RandName("tf-acc-app-ds")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AppsCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAppDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccAppConfig(name, "syncthing", true) + `
data "truenas_app" "lookup" {
  name = truenas_app.test.name
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_app.lookup", "name", name),
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

func testAccCheckAppDestroyed(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "app.query", [][]any{{"name", "=", name}})
		if err != nil {
			return fmt.Errorf("error checking app %s: %v", name, err)
		}
		var results []struct {
			ID any `json:"id"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing app.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("app %s still exists", name)
		}
		return nil
	}
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package app_registry_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func checkRegistryPassword(want string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		id, _ := strconv.ParseInt(s.RootModule().Resources["truenas_app_registry.test"].Primary.ID, 10, 64)
		raw, err := acctest.Client().CallRead(context.Background(), "app.registry.get_instance", id)
		if err != nil {
			return err
		}
		var got struct {
			Password string `json:"password"`
		}
		if err := json.Unmarshal(raw, &got); err != nil {
			return err
		}
		if got.Password != want {
			return fmt.Errorf("stored password = %q, want %q", got.Password, want)
		}
		return nil
	}
}

// TestAccAppRegistry_passwordSentOnlyOnVersionChange: a description change
// with a different password_wo and the same password_wo_version leaves the
// stored token alone (before, the new value was sent, and app.registry.update
// logs in with it, so a not-yet-valid token failed the apply); bumping the
// version sends it. Needs a second account on the same registry in
// TRUENAS_TEST_REGISTRY_USERNAME2 / TRUENAS_TEST_REGISTRY_PASSWORD2. (B-141)
func TestAccAppRegistry_passwordSentOnlyOnVersionChange(t *testing.T) {
	uri, user, pass := registryEnv(t)
	user2, pass2 := os.Getenv("TRUENAS_TEST_REGISTRY_USERNAME2"), os.Getenv("TRUENAS_TEST_REGISTRY_PASSWORD2")
	if user2 == "" || pass2 == "" {
		t.Skip("set TRUENAS_TEST_REGISTRY_USERNAME2 and TRUENAS_TEST_REGISTRY_PASSWORD2 (a second registry account)")
	}
	name := acctest.RandName("tf-acc-regwo")
	cfg := func(u, p string, version int, desc string) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_app_registry" "test" {
  name                = %q
  uri                 = %q
  username            = %q
  password_wo         = %q
  password_wo_version = %d
  description         = %q
}
`, name, uri, u, p, version, desc)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_11_0)},
		Steps: []resource.TestStep{
			{Config: cfg(user, pass, 1, "one"), Check: checkRegistryPassword(pass)},
			{
				Config: cfg(user, "tfwo-not-yet-valid", 1, "two"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_app_registry.test", "description", "two"),
					checkRegistryPassword(pass),
				),
			},
			{Config: cfg(user2, pass2, 2, "two"), Check: checkRegistryPassword(pass2)},
		},
	})
}

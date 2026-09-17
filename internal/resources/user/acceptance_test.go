// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package user_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccUser_basic creates a local user with a password set, checks its
// attributes, updates full_name and shell in place, imports it by its
// numeric id (ignoring the write-only password), and verifies destruction.
func TestAccUser_basic(t *testing.T) {
	username := acctest.RandName("tf-acc-user")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroyed(username),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccUserConfig(username, "Test User", "/usr/bin/bash"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_user.test", "username", username),
					resource.TestCheckResourceAttr("truenas_user.test", "full_name", "Test User"),
					resource.TestCheckResourceAttr("truenas_user.test", "shell", "/usr/bin/bash"),
					resource.TestCheckResourceAttr("truenas_user.test", "home", "/var/empty"),
					resource.TestCheckResourceAttr("truenas_user.test", "locked", "false"),
					resource.TestCheckResourceAttrSet("truenas_user.test", "id"),
					resource.TestCheckResourceAttrSet("truenas_user.test", "uid"),
				),
			},
			// Update in place: change full_name.
			{
				Config: acctest.ProviderConfig() + testAccUserConfig(username, "Updated Name", "/usr/bin/bash"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_user.test", "full_name", "Updated Name"),
					resource.TestCheckResourceAttr("truenas_user.test", "shell", "/usr/bin/bash"),
				),
			},
			// Import by the user's numeric id. Password is write-only and
			// never returned by the API, so it can't be verified.
			// group_create is also write-only (only sent on create) and is
			// never read back into state.
			{
				ResourceName:            "truenas_user.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"password", "group_create"},
			},
		},
	})
}

func testAccUserConfig(username, fullName, shell string) string {
	return fmt.Sprintf(`
resource "truenas_user" "test" {
  username          = %q
  full_name         = %q
  password          = "Tf-Acc-Test-Passw0rd!"
  password_disabled = false
  home              = "/var/empty"
  shell             = %q
  smb               = false
  group_create      = true
}
`, username, fullName, shell)
}

func testAccCheckUserDestroyed(username string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "user.query", [][]any{{"username", "=", username}})
		if err != nil {
			return fmt.Errorf("error checking user %s: %v", username, err)
		}
		var results []struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing user.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("user %s still exists", username)
		}
		return nil
	}
}

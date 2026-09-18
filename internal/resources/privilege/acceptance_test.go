// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package privilege_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccPrivilege_basic creates a truenas_group fixture and a privilege
// referencing its gid via local_groups, checks attributes, adds a role in
// place, imports the privilege by its numeric id, and verifies destruction
// of both the privilege and the group fixture. CheckDestroy only ever
// queries by this test's own randomized name — it never touches the
// server's builtin privileges (Local Administrator, Read-Only
// Administrator, Sharing Administrator).
func TestAccPrivilege_basic(t *testing.T) {
	groupName := acctest.RandName("tf-acc-priv-grp")
	privName := acctest.RandName("tf-acc-priv")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			testAccCheckPrivilegeDestroyed(privName),
			testAccCheckGroupDestroyed(groupName),
		),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccPrivilegeConfig(groupName, privName, `["READONLY_ADMIN"]`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_privilege.test", "id"),
					resource.TestCheckResourceAttr("truenas_privilege.test", "name", privName),
					resource.TestCheckResourceAttr("truenas_privilege.test", "local_groups.#", "1"),
					resource.TestCheckResourceAttrPair("truenas_privilege.test", "local_groups.0", "truenas_group.fixture", "gid"),
					resource.TestCheckResourceAttr("truenas_privilege.test", "ds_groups.#", "0"),
					resource.TestCheckResourceAttr("truenas_privilege.test", "roles.#", "1"),
					resource.TestCheckResourceAttr("truenas_privilege.test", "roles.0", "READONLY_ADMIN"),
					resource.TestCheckResourceAttr("truenas_privilege.test", "web_shell", "false"),
				),
			},
			// Update in place: add SHARING_READ to roles.
			{
				Config: acctest.ProviderConfig() + testAccPrivilegeConfig(groupName, privName, `["READONLY_ADMIN", "SHARING_READ"]`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_privilege.test", "roles.#", "2"),
					resource.TestCheckResourceAttr("truenas_privilege.test", "roles.0", "READONLY_ADMIN"),
					resource.TestCheckResourceAttr("truenas_privilege.test", "roles.1", "SHARING_READ"),
				),
			},
			// Import by the privilege's numeric id.
			{
				ResourceName:      "truenas_privilege.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccPrivilegeConfig(groupName, privName, rolesHCL string) string {
	return fmt.Sprintf(`
resource "truenas_group" "fixture" {
  name = %q
  smb  = false
}

resource "truenas_privilege" "test" {
  name         = %q
  local_groups = [truenas_group.fixture.gid]
  roles        = %s
  web_shell    = false
}
`, groupName, privName, rolesHCL)
}

func testAccCheckPrivilegeDestroyed(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "privilege.query", [][]any{{"name", "=", name}})
		if err != nil {
			return fmt.Errorf("error checking privilege %s: %v", name, err)
		}
		var results []struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing privilege.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("privilege %s still exists", name)
		}
		return nil
	}
}

func testAccCheckGroupDestroyed(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "group.query", [][]any{{"group", "=", name}})
		if err != nil {
			return fmt.Errorf("error checking group %s: %v", name, err)
		}
		var results []struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing group.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("group %s still exists", name)
		}
		return nil
	}
}

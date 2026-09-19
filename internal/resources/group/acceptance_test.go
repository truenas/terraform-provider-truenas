// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package group_test

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

// TestAccGroup_basic creates a local group, checks its attributes, flips
// the smb flag and updates sudo_commands in place, imports it by its
// numeric id, and verifies destruction.
func TestAccGroup_basic(t *testing.T) {
	name := acctest.RandName("tf-acc-grp")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckGroupDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccGroupConfig(name, false, nil),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_group.test", "name", name),
					resource.TestCheckResourceAttr("truenas_group.test", "smb", "false"),
					resource.TestCheckResourceAttr("truenas_group.test", "sudo_commands.#", "0"),
					resource.TestCheckResourceAttrSet("truenas_group.test", "id"),
					resource.TestCheckResourceAttrSet("truenas_group.test", "gid"),
				),
			},
			// Update in place: flip smb and add sudo_commands.
			{
				Config: acctest.ProviderConfig() + testAccGroupConfig(name, true, []string{"/usr/bin/whoami", "/usr/bin/id"}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_group.test", "smb", "true"),
					resource.TestCheckResourceAttr("truenas_group.test", "sudo_commands.#", "2"),
					resource.TestCheckResourceAttr("truenas_group.test", "sudo_commands.0", "/usr/bin/whoami"),
					resource.TestCheckResourceAttr("truenas_group.test", "sudo_commands.1", "/usr/bin/id"),
				),
			},
			// Import by the group's numeric id.
			{
				ResourceName:      "truenas_group.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccGroupConfig(name string, smb bool, sudoCommands []string) string {
	quoted := make([]string, len(sudoCommands))
	for i, c := range sudoCommands {
		quoted[i] = fmt.Sprintf("%q", c)
	}
	sudoCmdsHCL := "[" + strings.Join(quoted, ", ") + "]"
	return fmt.Sprintf(`
resource "truenas_group" "test" {
  name          = %q
  smb           = %v
  sudo_commands = %s
}
`, name, smb, sudoCmdsHCL)
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

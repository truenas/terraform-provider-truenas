// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package tunable_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccTunable_basic tests create, update, and import of a sysctl
// tunable.
//
// It uses fs.suid_dumpable=0. This is not guaranteed to be a no-op: the
// kernel default for this sysctl varies by system, so the applied value may
// differ from whatever the box already had. Safety here comes from
// truenas_tunable's delete behavior, which captures the pre-apply value as
// orig_value and restores it on destroy, not from the test value happening
// to match a default.
func TestAccTunable_basic(t *testing.T) {
	const varName = "fs.suid_dumpable"
	const value = "0"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckTunableDestroyed(varName),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccTunableConfig(varName, value, "tf-acc tunable"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_tunable.test", "var", varName),
					resource.TestCheckResourceAttr("truenas_tunable.test", "value", value),
					resource.TestCheckResourceAttr("truenas_tunable.test", "comment", "tf-acc tunable"),
					resource.TestCheckResourceAttr("truenas_tunable.test", "enabled", "true"),
					resource.TestCheckResourceAttrSet("truenas_tunable.test", "id"),
					resource.TestCheckResourceAttrSet("truenas_tunable.test", "type"),
				),
			},
			// Update the comment in place.
			{
				Config: acctest.ProviderConfig() + testAccTunableConfig(varName, value, "tf-acc tunable updated"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_tunable.test", "comment", "tf-acc tunable updated"),
				),
			},
			{
				ResourceName:            "truenas_tunable.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"update_initramfs"},
			},
		},
	})
}

func testAccTunableConfig(varName, value, comment string) string {
	return fmt.Sprintf(`
resource "truenas_tunable" "test" {
  var     = %q
  value   = %q
  type    = "SYSCTL"
  comment = %q
  enabled = true
}
`, varName, value, comment)
}

func testAccCheckTunableDestroyed(varName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "tunable.query", [][]any{{"var", "=", varName}})
		if err != nil {
			return fmt.Errorf("error checking tunable %s: %v", varName, err)
		}
		var results []struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing tunable.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("tunable %s still exists", varName)
		}
		return nil
	}
}

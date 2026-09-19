// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package init_shutdown_script_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccInitShutdownScript_basic creates a COMMAND-type init/shutdown
// script running /usr/bin/true at POSTINIT, disabled so nothing is ever
// actually scheduled to run, checks its attributes, updates its comment in
// place, imports it by numeric id, and verifies destruction via a live
// initshutdownscript.query.
func TestAccInitShutdownScript_basic(t *testing.T) {
	comment := acctest.RandName("tf-acc-ish")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckInitShutdownScriptDestroyed(comment),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccInitShutdownScriptConfig(comment),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_init_shutdown_script.test", "id"),
					resource.TestCheckResourceAttr("truenas_init_shutdown_script.test", "type", "COMMAND"),
					resource.TestCheckResourceAttr("truenas_init_shutdown_script.test", "command", "/usr/bin/true"),
					resource.TestCheckResourceAttr("truenas_init_shutdown_script.test", "when", "POSTINIT"),
					resource.TestCheckResourceAttr("truenas_init_shutdown_script.test", "enabled", "false"),
					resource.TestCheckResourceAttr("truenas_init_shutdown_script.test", "comment", comment),
				),
			},
			// Update comment in place.
			{
				Config: acctest.ProviderConfig() + testAccInitShutdownScriptConfig(comment+"-updated"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_init_shutdown_script.test", "comment", comment+"-updated"),
				),
			},
			// Import by the task's numeric id. Every field the API tracks
			// is echoed back on read, so ImportStateVerify needs no ignore
			// list.
			{
				ResourceName:      "truenas_init_shutdown_script.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccInitShutdownScriptConfig(comment string) string {
	return fmt.Sprintf(`
resource "truenas_init_shutdown_script" "test" {
  type    = "COMMAND"
  command = "/usr/bin/true"
  when    = "POSTINIT"
  enabled = false
  comment = %q
}
`, comment)
}

// initShutdownScriptSummary is the subset of initshutdownscript.query
// fields this test package needs directly (outside of the provider's own
// resource code).
type initShutdownScriptSummary struct {
	ID int64 `json:"id"`
}

// testAccCheckInitShutdownScriptDestroyed queries initshutdownscript by the
// fixture's comment since the task's ID is not known outside of Terraform
// state at CheckDestroy time.
func testAccCheckInitShutdownScriptDestroyed(comment string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		for _, cm := range []string{comment, comment + "-updated"} {
			raw, err := c.Call(context.Background(), "initshutdownscript.query", [][]any{{"comment", "=", cm}})
			if err != nil {
				return fmt.Errorf("error checking init/shutdown script for comment %q: %v", cm, err)
			}
			var results []initShutdownScriptSummary
			if err := json.Unmarshal(raw, &results); err != nil {
				return fmt.Errorf("error parsing initshutdownscript.query response: %v", err)
			}
			if len(results) > 0 {
				return fmt.Errorf("init/shutdown script with comment %q still exists", cm)
			}
		}
		return nil
	}
}

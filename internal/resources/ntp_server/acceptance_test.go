// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package ntp_server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccNTPServer_basic tests create, update, and import of an NTP server.
//
// It uses 203.0.113.1, the TEST-NET-3 documentation address (RFC 5737),
// which is never reachable, so force=true is required at create time. It
// never references or modifies the box's own (debian pool) NTP servers.
func TestAccNTPServer_basic(t *testing.T) {
	const address = "203.0.113.1"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckNTPServerDestroyed(address),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccNTPServerConfig(address, false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_ntp_server.test", "address", address),
					resource.TestCheckResourceAttr("truenas_ntp_server.test", "iburst", "false"),
					resource.TestCheckResourceAttr("truenas_ntp_server.test", "burst", "false"),
					resource.TestCheckResourceAttr("truenas_ntp_server.test", "prefer", "false"),
					resource.TestCheckResourceAttrSet("truenas_ntp_server.test", "id"),
				),
			},
			// Update prefer in place.
			{
				Config: acctest.ProviderConfig() + testAccNTPServerConfig(address, true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_ntp_server.test", "prefer", "true"),
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

func testAccNTPServerConfig(address string, prefer bool) string {
	return fmt.Sprintf(`
resource "truenas_ntp_server" "test" {
  address = %q
  iburst  = false
  burst   = false
  prefer  = %v
  force   = true
}
`, address, prefer)
}

func testAccCheckNTPServerDestroyed(address string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "system.ntpserver.query", [][]any{{"address", "=", address}})
		if err != nil {
			return fmt.Errorf("error checking ntp server %s: %v", address, err)
		}
		var results []struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing system.ntpserver.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("ntp server %s still exists", address)
		}
		return nil
	}
}

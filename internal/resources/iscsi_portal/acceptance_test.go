// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_portal_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccISCSIPortal_basic creates an iSCSI portal listening on the box IP (0.0.0.0 collides with the live portal on 26.0 — one portal per IP)
// (port is not settable per-listen on TrueNAS 26.0+; the global iSCSI
// listen_port applies), checks its attributes, updates its comment, imports
// it by id, and verifies destruction.
func TestAccISCSIPortal_basic(t *testing.T) {
	comment := acctest.RandName("tf-acc-portal")
	updatedComment := comment + "-updated"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		// CheckDestroy runs against the last-applied config, in which the
		// comment has already been updated, so it must check for the
		// updated comment rather than the original one.
		CheckDestroy: testAccCheckISCSIPortalDestroyed(updatedComment),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccISCSIPortalConfig(comment),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_iscsi_portal.test", "id"),
					resource.TestCheckResourceAttr("truenas_iscsi_portal.test", "comment", comment),
					resource.TestCheckResourceAttr("truenas_iscsi_portal.test", "listen.0.ip", acctest.EndpointHost()),
					resource.TestCheckResourceAttrSet("truenas_iscsi_portal.test", "tag"),
				),
			},
			// Update the comment in place.
			{
				Config: acctest.ProviderConfig() + testAccISCSIPortalConfig(updatedComment),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_iscsi_portal.test", "comment", updatedComment),
				),
			},
			{
				ResourceName:      "truenas_iscsi_portal.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccISCSIPortalConfig(comment string) string {
	return fmt.Sprintf(`
resource "truenas_iscsi_portal" "test" {
  comment = %q
  listen = [
    {
      ip = %q
    }
  ]
}
`, comment, acctest.EndpointHost())
}

func testAccCheckISCSIPortalDestroyed(comment string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "iscsi.portal.query", [][]any{{"comment", "=", comment}})
		if err != nil {
			return fmt.Errorf("error checking iscsi portal %s: %v", comment, err)
		}
		var results []struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing iscsi.portal.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("iscsi portal %s still exists", comment)
		}
		return nil
	}
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_target_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccISCSITarget_basic creates an own portal fixture (listening on
// 0.0.0.0, distinct from the box's live portal/target, id=1 "proxmox";
// port is not settable per-listen on TrueNAS 26.0+), creates an iSCSI target
// referencing it, checks attributes, updates the alias in place, imports by
// id, and verifies destruction.
func TestAccISCSITarget_basic(t *testing.T) {
	name := acctest.RandName("tf-acc-target")
	portalComment := acctest.RandName("tf-acc-target-portal")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckISCSITargetDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccISCSITargetConfig(portalComment, name, "initial-alias"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_iscsi_target.test", "id"),
					resource.TestCheckResourceAttr("truenas_iscsi_target.test", "name", name),
					resource.TestCheckResourceAttr("truenas_iscsi_target.test", "mode", "ISCSI"),
					resource.TestCheckResourceAttr("truenas_iscsi_target.test", "alias", "initial-alias"),
					resource.TestCheckResourceAttr("truenas_iscsi_target.test", "groups.0.authmethod", "NONE"),
					resource.TestCheckResourceAttrPair("truenas_iscsi_target.test", "groups.0.portal", "truenas_iscsi_portal.fixture", "id"),
					resource.TestCheckResourceAttrSet("truenas_iscsi_target.test", "rel_tgt_id"),
				),
			},
			// Update the alias in place.
			{
				Config: acctest.ProviderConfig() + testAccISCSITargetConfig(portalComment, name, "updated-alias"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_iscsi_target.test", "alias", "updated-alias"),
				),
			},
			{
				ResourceName:      "truenas_iscsi_target.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccISCSITarget_params exercises the nested iscsi_parameters block
// (queued_commands), verifies it round trips, and imports.
func TestAccISCSITarget_params(t *testing.T) {
	portalComment := acctest.RandName("tf-acc-tgtparam-portal")
	name := acctest.RandName("tfacctgtparam")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckISCSITargetDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccISCSITargetParamsConfig(portalComment, name, 128),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_iscsi_target.test", "iscsi_parameters.queued_commands", "128"),
				),
			},
			// In-place update: change the queued_commands value.
			{
				Config: acctest.ProviderConfig() + testAccISCSITargetParamsConfig(portalComment, name, 32),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_iscsi_target.test", "iscsi_parameters.queued_commands", "32"),
				),
			},
			{
				ResourceName:      "truenas_iscsi_target.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccISCSITargetParamsConfig(portalComment, name string, queuedCommands int64) string {
	return fmt.Sprintf(`
resource "truenas_iscsi_portal" "fixture" {
  comment = %q
  listen = [
    {
      ip = %q
    }
  ]
}

resource "truenas_iscsi_target" "test" {
  name = %q
  mode = "ISCSI"
  groups = [
    {
      portal     = truenas_iscsi_portal.fixture.id
      authmethod = "NONE"
    }
  ]
  iscsi_parameters = {
    queued_commands = %d
  }
}
`, portalComment, acctest.EndpointHost(), name, queuedCommands)
}

func testAccISCSITargetConfig(portalComment, name, alias string) string {
	return fmt.Sprintf(`
resource "truenas_iscsi_portal" "fixture" {
  comment = %q
  listen = [
    {
      ip = %q
    }
  ]
}

resource "truenas_iscsi_target" "test" {
  name  = %q
  alias = %q
  mode  = "ISCSI"
  groups = [
    {
      portal     = truenas_iscsi_portal.fixture.id
      authmethod = "NONE"
    }
  ]
}
`, portalComment, acctest.EndpointHost(), name, alias)
}

func testAccCheckISCSITargetDestroyed(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "iscsi.target.query", [][]any{{"name", "=", name}})
		if err != nil {
			return fmt.Errorf("error checking iscsi target %s: %v", name, err)
		}
		var results []struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing iscsi.target.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("iscsi target %s still exists", name)
		}
		return nil
	}
}

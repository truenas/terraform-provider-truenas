// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_targetextent_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

// TestAccISCSIEndToEnd wires up a full, self-contained iSCSI configuration
// (portal, initiator, CHAP auth, a zvol fixture, an extent backed by that
// zvol, a target referencing the portal+initiator, and a targetextent
// association) entirely from own-created tf-acc objects.
//
// Safety: this test never references the box's live iSCSI configuration
// (portal id=1, target id=1 "proxmox", extent id=2, targetextent id=2). The
// test portal listens on the box IP (port is not settable per-listen on TrueNAS
// 26.0+) and the auth group uses tag 999 (distinct from any live tag).
func TestAccISCSIEndToEnd(t *testing.T) {
	portalComment := acctest.RandName("tf-acc-portal")
	initiatorComment := acctest.RandName("tf-acc-initiator")
	authUser := acctest.RandName("tf-acc-authuser")
	const authTag = 999
	const authSecret = "tfacc-secret12" // 14 chars, within the 12-16 range
	zvolName := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-zvol"))
	extentName := acctest.RandName("tf-acc-extent")
	targetName := acctest.RandName("tf-acc-target")
	diskPath := fmt.Sprintf("zvol/%s", zvolName)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy: testAccCheckISCSIEndToEndDestroyed(
			targetName, extentName, portalComment, initiatorComment, authUser, authTag, zvolName,
		),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccISCSIEndToEndConfig(
					portalComment, initiatorComment, authUser, authTag, authSecret,
					zvolName, extentName, "initial extent comment",
					targetName, "initial-alias",
				),
				Check: resource.ComposeTestCheckFunc(
					// Portal
					resource.TestCheckResourceAttrSet("truenas_iscsi_portal.test", "id"),
					resource.TestCheckResourceAttr("truenas_iscsi_portal.test", "comment", portalComment),
					resource.TestCheckResourceAttrSet("truenas_iscsi_portal.test", "tag"),
					resource.TestCheckResourceAttr("truenas_iscsi_portal.test", "listen.0.ip", acctest.EndpointHost()),

					// Initiator
					resource.TestCheckResourceAttrSet("truenas_iscsi_initiator.test", "id"),
					resource.TestCheckResourceAttr("truenas_iscsi_initiator.test", "comment", initiatorComment),

					// Auth
					resource.TestCheckResourceAttrSet("truenas_iscsi_auth.test", "id"),
					resource.TestCheckResourceAttr("truenas_iscsi_auth.test", "tag", fmt.Sprintf("%d", authTag)),
					resource.TestCheckResourceAttr("truenas_iscsi_auth.test", "user", authUser),

					// Zvol fixture
					resource.TestCheckResourceAttr("truenas_zvol.test", "name", zvolName),
					resource.TestCheckResourceAttrSet("truenas_zvol.test", "id"),

					// Extent
					resource.TestCheckResourceAttrSet("truenas_iscsi_extent.test", "id"),
					resource.TestCheckResourceAttr("truenas_iscsi_extent.test", "name", extentName),
					resource.TestCheckResourceAttr("truenas_iscsi_extent.test", "type", "DISK"),
					resource.TestCheckResourceAttr("truenas_iscsi_extent.test", "disk", diskPath),
					resource.TestCheckResourceAttr("truenas_iscsi_extent.test", "comment", "initial extent comment"),
					resource.TestCheckResourceAttrSet("truenas_iscsi_extent.test", "naa"),
					resource.TestCheckResourceAttrSet("truenas_iscsi_extent.test", "serial"),

					// Target
					resource.TestCheckResourceAttrSet("truenas_iscsi_target.test", "id"),
					resource.TestCheckResourceAttr("truenas_iscsi_target.test", "name", targetName),
					resource.TestCheckResourceAttr("truenas_iscsi_target.test", "alias", "initial-alias"),
					resource.TestCheckResourceAttr("truenas_iscsi_target.test", "groups.0.authmethod", "CHAP"),
					resource.TestCheckResourceAttrPair("truenas_iscsi_target.test", "groups.0.portal", "truenas_iscsi_portal.test", "id"),
					resource.TestCheckResourceAttrPair("truenas_iscsi_target.test", "groups.0.initiator", "truenas_iscsi_initiator.test", "id"),
					resource.TestCheckResourceAttrPair("truenas_iscsi_target.test", "groups.0.auth", "truenas_iscsi_auth.test", "tag"),

					// TargetExtent
					resource.TestCheckResourceAttrSet("truenas_iscsi_targetextent.test", "id"),
					resource.TestCheckResourceAttrPair("truenas_iscsi_targetextent.test", "target", "truenas_iscsi_target.test", "id"),
					resource.TestCheckResourceAttrPair("truenas_iscsi_targetextent.test", "extent", "truenas_iscsi_extent.test", "id"),
					resource.TestCheckResourceAttr("truenas_iscsi_targetextent.test", "lunid", "0"),
				),
			},
			// Update: change extent comment and target alias in place; the
			// rest of the chain is left untouched.
			{
				Config: acctest.ProviderConfig() + testAccISCSIEndToEndConfig(
					portalComment, initiatorComment, authUser, authTag, authSecret,
					zvolName, extentName, "updated extent comment",
					targetName, "updated-alias",
				),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_iscsi_extent.test", "comment", "updated extent comment"),
					resource.TestCheckResourceAttr("truenas_iscsi_target.test", "alias", "updated-alias"),
				),
			},
			{
				ResourceName:      "truenas_iscsi_targetextent.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:      "truenas_iscsi_target.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:      "truenas_iscsi_extent.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccISCSIEndToEndConfig(
	portalComment, initiatorComment, authUser string, authTag int, authSecret string,
	zvolName, extentName, extentComment, targetName, targetAlias string,
) string {
	return fmt.Sprintf(`
resource "truenas_iscsi_portal" "test" {
  comment = %q
  listen = [
    {
      ip = %q
    }
  ]
}

resource "truenas_iscsi_initiator" "test" {
  comment    = %q
  initiators = []
}

resource "truenas_iscsi_auth" "test" {
  tag    = %d
  user   = %q
  secret = %q
}

resource "truenas_zvol" "test" {
  name    = %q
  volsize = 67108864
}

resource "truenas_iscsi_extent" "test" {
  name    = %q
  type    = "DISK"
  disk    = "zvol/${truenas_zvol.test.name}"
  comment = %q
  enabled = true
}

resource "truenas_iscsi_target" "test" {
  name  = %q
  alias = %q
  groups = [
    {
      portal     = truenas_iscsi_portal.test.id
      initiator  = truenas_iscsi_initiator.test.id
      auth       = truenas_iscsi_auth.test.tag
      authmethod = "CHAP"
    }
  ]
}

resource "truenas_iscsi_targetextent" "test" {
  target = truenas_iscsi_target.test.id
  extent = truenas_iscsi_extent.test.id
  lunid  = 0
}
`, portalComment, acctest.EndpointHost(), initiatorComment, authTag, authUser, authSecret,
		zvolName, extentName, extentComment, targetName, targetAlias)
}

// idResults is the shape returned by every TrueNAS <namespace>.query method
// this test cares about: a list of objects each carrying at least an "id".
type idResults []struct {
	ID any `json:"id"`
}

// checkQueryEmpty calls a TrueNAS <namespace>.query method with the given
// filters and fails if any results are returned.
func checkQueryEmpty(ctx context.Context, c *client.Client, method string, filters [][]any, describe string) error {
	raw, err := c.Call(ctx, method, filters)
	if err != nil {
		return fmt.Errorf("error checking %s: %v", describe, err)
	}
	var results idResults
	if err := json.Unmarshal(raw, &results); err != nil {
		return fmt.Errorf("error parsing %s response: %v", method, err)
	}
	if len(results) > 0 {
		return fmt.Errorf("%s still exists", describe)
	}
	return nil
}

// stateAttr reads a primary instance attribute for a resource address from
// the (pre-destroy) terraform state.
func stateAttr(s *terraform.State, address, attr string) (string, error) {
	rs, ok := s.RootModule().Resources[address]
	if !ok {
		return "", fmt.Errorf("resource %s not found in state", address)
	}
	v, ok := rs.Primary.Attributes[attr]
	if !ok {
		return "", fmt.Errorf("resource %s has no attribute %q in state", address, attr)
	}
	return v, nil
}

func testAccCheckISCSIEndToEndDestroyed(
	targetName, extentName, portalComment, initiatorComment, authUser string, authTag int, zvolName string,
) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		ctx := context.Background()

		if err := checkQueryEmpty(ctx, c, "iscsi.target.query", [][]any{{"name", "=", targetName}}, "iscsi target "+targetName); err != nil {
			return err
		}
		if err := checkQueryEmpty(ctx, c, "iscsi.extent.query", [][]any{{"name", "=", extentName}}, "iscsi extent "+extentName); err != nil {
			return err
		}
		if err := checkQueryEmpty(ctx, c, "iscsi.portal.query", [][]any{{"comment", "=", portalComment}}, "iscsi portal "+portalComment); err != nil {
			return err
		}
		if err := checkQueryEmpty(ctx, c, "iscsi.initiator.query", [][]any{{"comment", "=", initiatorComment}}, "iscsi initiator "+initiatorComment); err != nil {
			return err
		}
		if err := checkQueryEmpty(ctx, c, "iscsi.auth.query", [][]any{{"tag", "=", authTag}, {"user", "=", authUser}}, "iscsi auth "+authUser); err != nil {
			return err
		}
		if err := checkQueryEmpty(ctx, c, "pool.dataset.query", [][]any{{"id", "=", zvolName}}, "zvol "+zvolName); err != nil {
			return err
		}

		// The targetextent association is keyed by the (now-deleted)
		// numeric target/extent IDs, which are still readable from the
		// pre-destroy state captured in s.
		targetIDStr, err := stateAttr(s, "truenas_iscsi_target.test", "id")
		if err != nil {
			return err
		}
		extentIDStr, err := stateAttr(s, "truenas_iscsi_extent.test", "id")
		if err != nil {
			return err
		}
		targetID, err := strconv.ParseInt(targetIDStr, 10, 64)
		if err != nil {
			return fmt.Errorf("parsing target id %q: %v", targetIDStr, err)
		}
		extentID, err := strconv.ParseInt(extentIDStr, 10, 64)
		if err != nil {
			return fmt.Errorf("parsing extent id %q: %v", extentIDStr, err)
		}
		if err := checkQueryEmpty(ctx, c, "iscsi.targetextent.query",
			[][]any{{"target", "=", targetID}, {"extent", "=", extentID}},
			fmt.Sprintf("targetextent (target=%d extent=%d)", targetID, extentID),
		); err != nil {
			return err
		}

		return nil
	}
}

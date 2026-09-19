// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package smb_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccSMBShare_basic creates a dataset fixture and an SMB share on its
// mountpoint, checks its attributes, updates comment and abe in place,
// imports it by its numeric id, and verifies destruction of both the share
// and the dataset fixture.
func TestAccSMBShare_basic(t *testing.T) {
	datasetName := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-ds-smb"))
	shareName := acctest.RandName("tf-acc-smb")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			testAccCheckSMBShareDestroyed(shareName),
			testAccCheckSMBDatasetDestroyed(datasetName),
		),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccSMBShareConfig(datasetName, shareName, "initial comment", false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_smb_share.test", "name", shareName),
					resource.TestCheckResourceAttr("truenas_smb_share.test", "comment", "initial comment"),
					resource.TestCheckResourceAttr("truenas_smb_share.test", "abe", "false"),
					resource.TestCheckResourceAttr("truenas_smb_share.test", "enabled", "true"),
					resource.TestCheckResourceAttr("truenas_smb_share.test", "hostsallow.#", "2"),
					resource.TestCheckResourceAttr("truenas_smb_share.test", "hostsallow.0", "127.0.0.1"),
					resource.TestCheckResourceAttrSet("truenas_smb_share.test", "path"),
					resource.TestCheckResourceAttrSet("truenas_smb_share.test", "id"),
					resource.TestCheckResourceAttrSet("truenas_smb_share.test", "vuid"),
				),
			},
			// Update in place: change comment and enable ABE.
			{
				Config: acctest.ProviderConfig() + testAccSMBShareConfig(datasetName, shareName, "updated comment", true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_smb_share.test", "comment", "updated comment"),
					resource.TestCheckResourceAttr("truenas_smb_share.test", "abe", "true"),
				),
			},
			// Import by the share's numeric id.
			{
				ResourceName:      "truenas_smb_share.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccSMBShare_audit exercises the nested audit block: enable auditing with
// a watch_list, verify the values round trip, and import.
func TestAccSMBShare_audit(t *testing.T) {
	datasetName := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-smb-audit-ds"))
	shareName := acctest.RandName("tfaccsmbaudit")
	groupName := acctest.RandName("tfaccsmbauditgrp")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckSMBShareDestroyed(shareName),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccSMBShareAuditConfig(datasetName, shareName, groupName,
					"enable = true\n    watch_list = [truenas_group.audit.name]"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_smb_share.test", "audit.enable", "true"),
					resource.TestCheckResourceAttr("truenas_smb_share.test", "audit.watch_list.#", "1"),
					resource.TestCheckResourceAttr("truenas_smb_share.test", "audit.watch_list.0", groupName),
				),
			},
			// In-place update: disable auditing (clears the watch_list requirement).
			{
				Config: acctest.ProviderConfig() + testAccSMBShareAuditConfig(datasetName, shareName, groupName,
					"enable = false"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_smb_share.test", "audit.enable", "false"),
				),
			},
			{
				ResourceName:      "truenas_smb_share.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccSMBShareAuditConfig(datasetName, shareName, groupName, auditBody string) string {
	return fmt.Sprintf(`
resource "truenas_dataset" "test" {
  name = %q
}

# Auditing requires a non-empty watch_list or ignore_list of real SMB groups,
# so provision a throwaway SMB-enabled group to audit.
resource "truenas_group" "audit" {
  name = %q
  smb  = true
}

resource "truenas_smb_share" "test" {
  path    = truenas_dataset.test.mountpoint
  name    = %q
  enabled = true
  audit = {
    %s
  }
}
`, datasetName, groupName, shareName, auditBody)
}

func testAccSMBShareConfig(datasetName, shareName, comment string, abe bool) string {
	return fmt.Sprintf(`
resource "truenas_dataset" "test" {
  name = %q
}

resource "truenas_smb_share" "test" {
  path       = truenas_dataset.test.mountpoint
  name       = %q
  comment    = %q
  enabled    = true
  abe        = %v
  hostsallow = ["127.0.0.1", "192.168.1.0/24"]
}
`, datasetName, shareName, comment, abe)
}

func testAccCheckSMBShareDestroyed(shareName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "sharing.smb.query", [][]any{{"name", "=", shareName}})
		if err != nil {
			return fmt.Errorf("error checking SMB share %s: %v", shareName, err)
		}
		var results []struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing sharing.smb.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("SMB share %s still exists", shareName)
		}
		return nil
	}
}

func testAccCheckSMBDatasetDestroyed(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "pool.dataset.query", [][]any{{"id", "=", name}})
		if err != nil {
			return fmt.Errorf("error checking dataset %s: %v", name, err)
		}
		var results []struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing pool.dataset.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("dataset %s still exists", name)
		}
		return nil
	}
}

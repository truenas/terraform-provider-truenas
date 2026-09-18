// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_extent_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccISCSIExtent_basic creates a zvol fixture and a DISK-backed iSCSI
// extent on top of it, checks attributes, updates the comment in place,
// imports by id, and verifies destruction.
func TestAccISCSIExtent_basic(t *testing.T) {
	zvolName := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-extent-vol"))
	extentName := acctest.RandName("tf-acc-extent")
	diskPath := fmt.Sprintf("zvol/%s", zvolName)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckISCSIExtentDestroyed(extentName),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccISCSIExtentConfig(zvolName, extentName, "initial comment"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_iscsi_extent.test", "name", extentName),
					resource.TestCheckResourceAttr("truenas_iscsi_extent.test", "type", "DISK"),
					resource.TestCheckResourceAttr("truenas_iscsi_extent.test", "disk", diskPath),
					resource.TestCheckResourceAttr("truenas_iscsi_extent.test", "comment", "initial comment"),
					resource.TestCheckResourceAttr("truenas_iscsi_extent.test", "enabled", "true"),
					resource.TestCheckResourceAttrSet("truenas_iscsi_extent.test", "id"),
					resource.TestCheckResourceAttrSet("truenas_iscsi_extent.test", "naa"),
					resource.TestCheckResourceAttrSet("truenas_iscsi_extent.test", "serial"),
				),
			},
			// Update the comment in place.
			{
				Config: acctest.ProviderConfig() + testAccISCSIExtentConfig(zvolName, extentName, "updated comment"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_iscsi_extent.test", "comment", "updated comment"),
				),
			},
			{
				ResourceName:      "truenas_iscsi_extent.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccISCSIExtent_fileType creates a FILE-type extent with an explicit
// filesize (TrueNAS creates the backing file), verifies filesize round trips,
// and imports.
func TestAccISCSIExtent_fileType(t *testing.T) {
	datasetName := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-ext-file-ds"))
	extentName := acctest.RandName("tfaccextfile")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckISCSIExtentDestroyed(extentName),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccISCSIExtentFileConfig(datasetName, extentName, 67108864),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_iscsi_extent.test", "type", "FILE"),
					resource.TestCheckResourceAttr("truenas_iscsi_extent.test", "filesize", "67108864"),
				),
			},
			// In-place update: grow the file extent.
			{
				Config: acctest.ProviderConfig() + testAccISCSIExtentFileConfig(datasetName, extentName, 134217728),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_iscsi_extent.test", "filesize", "134217728"),
				),
			},
			{
				ResourceName:      "truenas_iscsi_extent.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccISCSIExtentFileConfig(datasetName, extentName string, filesize int64) string {
	return fmt.Sprintf(`
resource "truenas_dataset" "fixture" {
  name = %q
}

resource "truenas_iscsi_extent" "test" {
  name     = %q
  type     = "FILE"
  path     = "${truenas_dataset.fixture.mountpoint}/extent.img"
  filesize = %d
  enabled  = true
}
`, datasetName, extentName, filesize)
}

func testAccISCSIExtentConfig(zvolName, extentName, comment string) string {
	return fmt.Sprintf(`
resource "truenas_zvol" "fixture" {
  name    = %q
  volsize = 67108864
}

resource "truenas_iscsi_extent" "test" {
  name    = %q
  type    = "DISK"
  disk    = "zvol/${truenas_zvol.fixture.name}"
  comment = %q
  enabled = true
}
`, zvolName, extentName, comment)
}

func testAccCheckISCSIExtentDestroyed(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "iscsi.extent.query", [][]any{{"name", "=", name}})
		if err != nil {
			return fmt.Errorf("error checking iscsi extent %s: %v", name, err)
		}
		var results []struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing iscsi.extent.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("iscsi extent %s still exists", name)
		}
		return nil
	}
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_extent_test

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

const extentAddr = "truenas_iscsi_extent.test"

func emptyPlan() resource.ConfigPlanChecks {
	return resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}
}

func inPlace() resource.ConfigPlanChecks {
	return resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(extentAddr, plancheck.ResourceActionUpdate)}}
}

// twoZvols declares zvols <pool>/<base>-a and -b plus a dataset for FILE
// extents.
func twoZvols(pool, base string) string {
	return fmt.Sprintf(`
resource "truenas_zvol" "a" {
  name    = "%[1]s/%[2]s-a"
  volsize = 67108864
}
resource "truenas_zvol" "b" {
  name    = "%[1]s/%[2]s-b"
  volsize = 67108864
}
resource "truenas_dataset" "d" {
  name = "%[1]s/%[2]s-d"
}
`, pool, base)
}

func diskExtent(name, zvol string) string {
	return fmt.Sprintf(`
resource "truenas_iscsi_extent" "test" {
  name = %q
  type = "DISK"
  disk = "zvol/${truenas_zvol.%s.name}"
}
`, name, zvol)
}

func fileExtent(name string) string {
	return fmt.Sprintf(`
resource "truenas_iscsi_extent" "test" {
  name     = %q
  type     = "FILE"
  path     = "${truenas_dataset.d.mountpoint}/x.img"
  filesize = 67108864
}
`, name)
}

// TestAccISCSIExtent_changeDisk re-points a DISK extent at another zvol in
// place. path is derived from disk for DISK extents; it used to be pinned
// with UseStateForUnknown and the apply failed with "inconsistent result"
// after the server had already switched (B-070).
func TestAccISCSIExtent_changeDisk(t *testing.T) {
	base := acctest.RandName("tfaccextdisk")
	pool := acctest.TestPool()
	cfg := func(which string) string {
		return acctest.ProviderConfig() + twoZvols(pool, base) + diskExtent(base, which)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckISCSIExtentDestroyed(base),
		Steps: []resource.TestStep{
			{Config: cfg("a")},
			{
				Config:           cfg("b"),
				ConfigPlanChecks: inPlace(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(extentAddr, "disk", fmt.Sprintf("zvol/%s/%s-b", pool, base)),
					resource.TestCheckResourceAttr(extentAddr, "path", fmt.Sprintf("zvol/%s/%s-b", pool, base)),
				),
			},
			{Config: cfg("b"), ConfigPlanChecks: emptyPlan()},
		},
	})
}

// TestAccISCSIExtent_diskToFile switches an extent DISK -> FILE -> DISK in
// place. disk is "" for FILE extents and path follows disk for DISK
// extents; both used to keep the stale prior value and fail the apply
// (B-071).
func TestAccISCSIExtent_diskToFile(t *testing.T) {
	base := acctest.RandName("tfaccextd2f")
	pool := acctest.TestPool()
	disk := acctest.ProviderConfig() + twoZvols(pool, base) + diskExtent(base, "a")
	file := acctest.ProviderConfig() + twoZvols(pool, base) + fileExtent(base)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckISCSIExtentDestroyed(base),
		Steps: []resource.TestStep{
			{Config: disk},
			{
				Config:           file,
				ConfigPlanChecks: inPlace(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(extentAddr, "type", "FILE"),
					resource.TestCheckResourceAttr(extentAddr, "disk", ""),
				),
			},
			{Config: file, ConfigPlanChecks: emptyPlan()},
			{
				Config:           disk,
				ConfigPlanChecks: inPlace(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(extentAddr, "type", "DISK"),
					resource.TestCheckResourceAttr(extentAddr, "path", fmt.Sprintf("zvol/%s/%s-a", pool, base)),
				),
			},
			{Config: disk, ConfigPlanChecks: emptyPlan()},
		},
	})
}

// TestAccISCSIExtent_diskPathValidation checks the plan-time errors for
// disk/path combinations TrueNAS would refuse or rewrite.
func TestAccISCSIExtent_diskPathValidation(t *testing.T) {
	cfg := func(body string) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_iscsi_extent" "test" {
  name = "tfaccextval"
%s
}
`, body)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg(`  type = "DISK"`), PlanOnly: true, ExpectError: regexp.MustCompile(`disk is required for a DISK extent`)},
			{Config: cfg(`  type = "DISK"
  disk = "zvol/tank/a"
  path = "/mnt/tank/x"`), PlanOnly: true, ExpectError: regexp.MustCompile(`path must be unset for a DISK extent`)},
			{Config: cfg(`  type = "FILE"`), PlanOnly: true, ExpectError: regexp.MustCompile(`path is required for a FILE extent`)},
			{Config: cfg(`  type = "FILE"
  path = "/mnt/tank/x.img"
  disk = "zvol/tank/a"`), PlanOnly: true, ExpectError: regexp.MustCompile(`disk must be unset for a FILE extent`)},
		},
	})
}

// TestAccISCSIExtent_literalDiskNoReference reproduces issue #40: the
// extent's disk is a literal string, not a reference to the zvol, so
// Terraform creates the zvols and extents at the same time and an extent
// create can run before /dev/zvol/<name> exists. Create now waits for the
// device instead of failing (B-048). Dotted/hyphenated names match the
// reporter's config.
func TestAccISCSIExtent_literalDiskNoReference(t *testing.T) {
	base := acctest.RandName("tfacc40")
	pool := acctest.TestPool()
	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_zvol" "z" {
  count   = 3
  name    = "%[1]s/sda.host-${count.index}.%[2]s.frednet"
  volsize = 67108864
}
`, pool, base)
	// Nothing orders the extents after the zvols, so destroy them in their
	// own step first; the final teardown then only deletes the zvols.
	zvolsOnly := cfg
	cfg += fmt.Sprintf(`
resource "truenas_iscsi_extent" "e" {
  count   = 3
  name    = "sda.host-${count.index}.%[2]s.frednet"
  type    = "DISK"
  disk    = "zvol/%[1]s/sda.host-${count.index}.%[2]s.frednet"
  enabled = true
}
`, pool, base)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_iscsi_extent.e.0", "path", fmt.Sprintf("zvol/%s/sda.host-0.%s.frednet", pool, base)),
					resource.TestCheckResourceAttr("truenas_iscsi_extent.e.2", "path", fmt.Sprintf("zvol/%s/sda.host-2.%s.frednet", pool, base)),
				),
			},
			{Config: cfg, ConfigPlanChecks: emptyPlan()},
			{Config: zvolsOnly},
		},
	})
}

// TestAccISCSIExtent_availThresholdKept sets avail_threshold outside
// Terraform on an extent whose config leaves it out, then applies an
// unrelated change. The server value must survive: an unknown
// avail_threshold used to be sent as null and cleared it (B-105).
func TestAccISCSIExtent_availThresholdKept(t *testing.T) {
	base := acctest.RandName("tfaccextavail")
	pool := acctest.TestPool()
	cfg := func(comment string) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_zvol" "a" {
  name    = "%s/%s"
  volsize = 67108864
}
resource "truenas_iscsi_extent" "test" {
  name    = %q
  type    = "DISK"
  disk    = "zvol/${truenas_zvol.a.name}"
  comment = %q
}
`, pool, base, base, comment)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckISCSIExtentDestroyed(base),
		Steps: []resource.TestStep{
			{Config: cfg("c1"), Check: setAvailThresholdOutOfBand(t, 50)},
			{
				Config: cfg("c2"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(extentAddr, "avail_threshold", "50"),
					checkServerExtent(t, "avail_threshold", float64(50)),
				),
			},
			{Config: cfg("c2"), ConfigPlanChecks: emptyPlan()},
		},
	})
}

// setAvailThresholdOutOfBand changes the extent's avail_threshold through
// the API, behind Terraform's back.
func setAvailThresholdOutOfBand(t *testing.T, v int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		id, err := strconv.ParseInt(s.RootModule().Resources[extentAddr].Primary.ID, 10, 64)
		if err != nil {
			return err
		}
		_, err = acctest.Client().Call(context.Background(), "iscsi.extent.update", id, map[string]any{"avail_threshold": v})
		return err
	}
}

// checkServerExtent asserts one field of the extent on the server.
func checkServerExtent(t *testing.T, field string, want any) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		id, err := strconv.ParseInt(s.RootModule().Resources[extentAddr].Primary.ID, 10, 64)
		if err != nil {
			return err
		}
		if got := acctest.GetInstance(t, "iscsi.extent", id)[field]; got != want {
			return fmt.Errorf("server %s = %v, want %v", field, got, want)
		}
		return nil
	}
}

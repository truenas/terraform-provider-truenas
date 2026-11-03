// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package zvol_test

// Hunt finding F8: renaming a zvol in config used to be a replacement that
// wiped the volume. The ZFS guid is now the resource identity, so a rename is
// an in-place pool.dataset.rename: the volume and its snapshots move, the guid
// is unchanged. A zvol under an iSCSI extent (or any other reference) is
// refused unless allow_rename_in_use = true, because TrueNAS does not re-point
// the extent and the LUN goes offline. Blocks cannot be written through the
// API from a test, so "the data survives" is proven by the unchanged guid
// (the same ZFS object) and the surviving snapshot; scripts/hunt/rename-probe.sh
// writes a signature over ssh and checks it after the rename.

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

func snapshotCreate(ctx context.Context, c *client.Client, dataset, name string) error {
	_, err := c.Call(ctx, "pool.snapshot.create", map[string]any{"dataset": dataset, "name": name})
	if err != nil && strings.Contains(err.Error(), "Method does not exist") {
		_, err = c.Call(ctx, "zfs.snapshot.create", map[string]any{"dataset": dataset, "name": name})
	}
	return err
}

func snapshotExists(ctx context.Context, c *client.Client, full string) (bool, error) {
	raw, err := c.Call(ctx, "pool.snapshot.query", []any{[]any{"name", "=", full}})
	if err != nil && strings.Contains(err.Error(), "Method does not exist") {
		raw, err = c.Call(ctx, "zfs.snapshot.query", []any{[]any{"name", "=", full}})
	}
	if err != nil {
		return false, err
	}
	return string(raw) != "[]", nil
}

func stateAttr(s *terraform.State, addr, attr string) (string, error) {
	rs, ok := s.RootModule().Resources[addr]
	if !ok {
		return "", fmt.Errorf("%s not in state", addr)
	}
	v, ok := rs.Primary.Attributes[attr]
	if !ok {
		return "", fmt.Errorf("%s has no %s in state", addr, attr)
	}
	return v, nil
}

// TestAccZvol_renameInPlace: a zvol with a snapshot is renamed in place; the
// snapshot and guid survive, the device node exists under the new name, the
// plan is empty afterwards, and a rename plus a volsize change apply together.
func TestAccZvol_renameInPlace(t *testing.T) {
	acctest.PreCheck(t)
	base := acctest.TestPool() + "/" + acctest.RandName("tfaccrenz")
	renamed := base + "-renamed"
	cfg := func(name string, volsize int64) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_zvol" "z" {
  name          = %q
  volsize       = %d
  sparse        = true
  comments      = "rename test"
  force_destroy = true # snapshot
}
`, name, volsize)
	}
	c := acctest.Client()
	ctx := context.Background()
	var guid, sum string
	// 1 MiB of random bytes at a known offset, checksummed on the box before
	// and after the rename: the proof the blocks moved with the zvol.
	const ddRange = "bs=1M skip=4 count=1"
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg(base, 16777216), Check: func(s *terraform.State) error {
				acctest.SSH(t, "dd if=/dev/urandom of=/dev/zvol/"+base+" bs=1M seek=4 count=1 conv=fsync status=none")
				sum = strings.TrimSpace(acctest.SSH(t, "dd if=/dev/zvol/"+base+" "+ddRange+" status=none | sha256sum"))
				if len(sum) < 64 {
					return fmt.Errorf("could not checksum the zvol: %q", sum)
				}
				if err := snapshotCreate(ctx, c, base, "keep"); err != nil {
					return err
				}
				var err error
				guid, err = stateAttr(s, "truenas_zvol.z", "guid")
				if err == nil && guid == "" {
					err = fmt.Errorf("guid empty in state")
				}
				return err
			}},
			{
				Config: cfg(renamed, 16777216),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("truenas_zvol.z", plancheck.ResourceActionUpdate),
				}},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectIdentityValueMatchesState("truenas_zvol.z", tfjsonpath.New("guid")),
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_zvol.z", "id", renamed),
					resource.TestCheckResourceAttr("truenas_zvol.z", "name", renamed),
					func(s *terraform.State) error {
						g, err := stateAttr(s, "truenas_zvol.z", "guid")
						if err != nil {
							return err
						}
						if g != guid {
							return fmt.Errorf("guid changed on rename: %s -> %s (a different ZFS object: the data would be gone)", guid, g)
						}
						if ok, err := snapshotExists(ctx, c, renamed+"@keep"); err != nil || !ok {
							return fmt.Errorf("snapshot did not move with the rename (exists=%v, err=%v)", ok, err)
						}
						if ok, err := snapshotExists(ctx, c, base+"@keep"); err != nil || ok {
							return fmt.Errorf("old snapshot %s@keep still exists (exists=%v, err=%v)", base, ok, err)
						}
						if _, err := c.Call(ctx, "filesystem.stat", "/dev/zvol/"+renamed); err != nil {
							return fmt.Errorf("device node /dev/zvol/%s missing after rename: %w", renamed, err)
						}
						if got := strings.TrimSpace(acctest.SSH(t, "dd if=/dev/zvol/"+renamed+" "+ddRange+" status=none | sha256sum")); got != sum {
							return fmt.Errorf("block checksum after rename %s, want %s: the data did not move", got, sum)
						}
						return nil
					},
				),
			},
			{Config: cfg(renamed, 16777216), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			// Rename back and grow in the same apply (rename first, then update
			// under the new name).
			{
				Config: cfg(base, 33554432),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_zvol.z", "name", base),
					resource.TestCheckResourceAttr("truenas_zvol.z", "volsize", "33554432"),
					func(s *terraform.State) error {
						// guid is only known at run time: compare in a closure.
						g, err := stateAttr(s, "truenas_zvol.z", "guid")
						if err == nil && g != guid {
							err = fmt.Errorf("guid after rename back %s, want %s", g, guid)
						}
						return err
					},
					func(*terraform.State) error {
						if got := strings.TrimSpace(acctest.SSH(t, "dd if=/dev/zvol/"+base+" "+ddRange+" status=none | sha256sum")); got != sum {
							return fmt.Errorf("block checksum after renaming back and growing %s, want %s", got, sum)
						}
						return nil
					},
				),
			},
			{Config: cfg(base, 33554432), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}

// TestAccZvol_renameWithReplaceRefusedWithSnapshot is the combined case: the
// config changes name AND volblocksize (replace-forcing) with no force_destroy,
// so the plan is a replacement. The destroy half must be refused by the v1.6.1
// guard (the zvol has a snapshot) and the original zvol must still exist under
// its old name with the snapshot and its blocks intact. Before the guard this
// test would have been red: the volume was destroyed and an empty one created.
func TestAccZvol_renameWithReplaceRefusedWithSnapshot(t *testing.T) {
	acctest.PreCheck(t)
	base := acctest.TestPool() + "/" + acctest.RandName("tfaccrenrz")
	renamed := base + "-renamed"
	cfg := func(name string, volblocksize int, force bool) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_zvol" "z" {
  name          = %q
  volsize       = 16777216
  volblocksize  = %d
  sparse        = true
  force_destroy = %t
}
`, name, volblocksize, force)
	}
	c := acctest.Client()
	ctx := context.Background()
	var sum string
	const ddRange = "bs=1M skip=2 count=1"
	intact := func(*terraform.State) error {
		if ok, err := snapshotExists(ctx, c, base+"@keep"); err != nil || !ok {
			return fmt.Errorf("original zvol %s or its snapshot is gone (exists=%v, err=%v): the refused replacement destroyed it", base, ok, err)
		}
		if _, err := c.Call(ctx, "filesystem.stat", "/dev/zvol/"+renamed); err == nil {
			return fmt.Errorf("a zvol exists under the new name %s although the apply was refused", renamed)
		}
		if got := strings.TrimSpace(acctest.SSH(t, "dd if=/dev/zvol/"+base+" "+ddRange+" status=none | sha256sum")); got != sum {
			return fmt.Errorf("block checksum %s, want %s", got, sum)
		}
		return nil
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg(base, 16384, false), Check: func(*terraform.State) error {
				acctest.SSH(t, "dd if=/dev/urandom of=/dev/zvol/"+base+" bs=1M seek=2 count=1 conv=fsync status=none")
				sum = strings.TrimSpace(acctest.SSH(t, "dd if=/dev/zvol/"+base+" "+ddRange+" status=none | sha256sum"))
				if len(sum) < 64 {
					return fmt.Errorf("could not checksum the zvol: %q", sum)
				}
				return snapshotCreate(ctx, c, base, "keep")
			}},
			{
				Config: cfg(renamed, 32768, false),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("truenas_zvol.z", plancheck.ResourceActionReplace),
				}},
				ExpectError: regexp.MustCompile(`Delete zvol (refused|failed)`), // holds data (random blocks), or the snapshot blocker
			},
			{Config: cfg(base, 16384, false), Check: intact,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{Config: cfg(base, 16384, true)},
		},
	})
}

// TestAccZvol_renameRefusedUnderExtent: a zvol backing an iSCSI extent cannot
// be renamed (the LUN would go offline) unless allow_rename_in_use = true; the
// extent keeps the old disk until re-pointed, which then plans its update.
func TestAccZvol_renameRefusedUnderExtent(t *testing.T) {
	acctest.PreCheck(t)
	base := acctest.TestPool() + "/" + acctest.RandName("tfaccrenx")
	renamed := base + "-renamed"
	extent := strings.ReplaceAll(acctest.RandName("tfaccrenx"), "-", "")
	cfg := func(name, disk string, allow bool) string {
		allowLine := ""
		if allow {
			allowLine = "  allow_rename_in_use = true\n"
		}
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_zvol" "z" {
  name    = %q
  volsize = 16777216
  sparse  = true
%s}
resource "truenas_iscsi_extent" "e" {
  name       = %q
  type       = "DISK"
  disk       = %s
  depends_on = [truenas_zvol.z] # the disk is a literal in the first steps
}
`, name, allowLine, extent, disk)
	}
	c := acctest.Client()
	ctx := context.Background()
	extentDisk := func() (string, error) {
		raw, err := c.Call(ctx, "iscsi.extent.query", []any{[]any{"name", "=", extent}})
		if err != nil {
			return "", err
		}
		var rows []struct {
			Disk string `json:"disk"`
		}
		if err := json.Unmarshal(raw, &rows); err != nil || len(rows) != 1 {
			return "", fmt.Errorf("extent %s: %d rows, %v", extent, len(rows), err)
		}
		return rows[0].Disk, nil
	}
	literal := fmt.Sprintf("%q", "zvol/"+base)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg(base, literal, false)},
			{
				Config:      cfg(renamed, literal, false),
				ExpectError: regexp.MustCompile(`(?s)Rename of zvol refused: it is in use.*iSCSI extent "` + extent + `"`),
			},
			{Config: cfg(base, literal, false), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{
				Config: cfg(renamed, literal, true),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("truenas_zvol.z", plancheck.ResourceActionUpdate),
					plancheck.ExpectResourceAction("truenas_iscsi_extent.e", plancheck.ResourceActionNoop),
				}},
				Check: func(*terraform.State) error {
					d, err := extentDisk()
					if err != nil {
						return err
					}
					if d != "zvol/"+base {
						return fmt.Errorf("extent disk is %s; expected TrueNAS to leave it at the old zvol/%s (documentation claims so)", d, base)
					}
					return nil
				},
			},
			// Re-pointing the extent at the zvol's name plans its own change.
			{
				Config: cfg(renamed, `"zvol/${truenas_zvol.z.name}"`, true),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("truenas_zvol.z", plancheck.ResourceActionNoop),
					plancheck.ExpectNonEmptyPlan(),
				}},
				Check: func(*terraform.State) error {
					d, err := extentDisk()
					if err != nil {
						return err
					}
					if d != "zvol/"+renamed {
						return fmt.Errorf("extent disk %s, want zvol/%s", d, renamed)
					}
					return nil
				},
			},
			{Config: cfg(renamed, `"zvol/${truenas_zvol.z.name}"`, true), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}

// TestAccZvol_importByNameAndGuid: import by name, by ZFS guid and by identity
// { guid } all land on the same zvol with an empty plan after.
func TestAccZvol_importByNameAndGuid(t *testing.T) {
	acctest.PreCheck(t)
	name := acctest.TestPool() + "/" + acctest.RandName("tfaccimpgz")
	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_zvol" "z" {
  name    = %q
  volsize = 16777216
  sparse  = true
}
`, name)
	var guid string
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg, Check: func(s *terraform.State) error {
				var err error
				guid, err = stateAttr(s, "truenas_zvol.z", "guid")
				if err == nil && guid == "" {
					err = fmt.Errorf("guid empty in state")
				}
				return err
			}},
			{Config: cfg, ResourceName: "truenas_zvol.z", ImportState: true, ImportStateId: name, ImportStateVerify: true},
			{Config: cfg, ResourceName: "truenas_zvol.z", ImportState: true, ImportStateVerify: true,
				ImportStateIdFunc: func(*terraform.State) (string, error) { return guid, nil }},
			{Config: cfg, ResourceName: "truenas_zvol.z", ImportState: true, ImportStateKind: resource.ImportBlockWithResourceIdentity},
			{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}

// TestAccZvol_destroyRefusedWithSnapshot keeps the v1.6.1 destroy guard under
// test: a zvol with a snapshot is not destroyed unless force_destroy is set.
func TestAccZvol_destroyRefusedWithSnapshot(t *testing.T) {
	acctest.PreCheck(t)
	base := acctest.TestPool() + "/" + acctest.RandName("tfaccdnz")
	cfg := func(force bool) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_zvol" "z" {
  name          = %q
  volsize       = 16777216
  sparse        = true
  force_destroy = %t
}
`, base, force)
	}
	c := acctest.Client()
	ctx := context.Background()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg(false), Check: func(*terraform.State) error { return snapshotCreate(ctx, c, base, "keep") }},
			{Config: cfg(false), Destroy: true, ExpectError: regexp.MustCompile(`snapshot|holds data`)},
			{Config: cfg(false), Check: func(*terraform.State) error {
				ok, err := snapshotExists(ctx, c, base+"@keep")
				if err != nil {
					return err
				}
				if !ok {
					return fmt.Errorf("snapshot %s@keep is gone: the refused destroy destroyed the zvol", base)
				}
				return nil
			}},
			{Config: cfg(true)},
		},
	})
}

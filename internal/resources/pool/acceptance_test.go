// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package pool_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccPoolDataSource_basic reads the acceptance-test pool
// (acctest.TestPool(), default "tank") via the truenas_pool data source and
// checks that the name round-trips and a status is reported. It does not
// create or destroy any pool.
func TestAccPoolDataSource_basic(t *testing.T) {
	poolName := acctest.TestPool()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccPoolDataSourceConfig(poolName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_pool.t", "name", poolName),
					resource.TestCheckResourceAttrSet("data.truenas_pool.t", "status"),
				),
			},
		},
	})
}

func testAccPoolDataSourceConfig(name string) string {
	return fmt.Sprintf(`
data "truenas_pool" "t" {
  name = %q
}
`, name)
}

// testDisks returns the blank disk device names to build test pools from,
// read from TRUENAS_TEST_DISKS (comma-separated, e.g. "sdb,sdc,sdd,sdf,sdg").
// Pool-create tests are skipped unless it is set, since they require dedicated
// blank disks on the target box.
func testDisks(t *testing.T, min int) []string {
	t.Helper()
	v := os.Getenv("TRUENAS_TEST_DISKS")
	if v == "" {
		t.Skip("set TRUENAS_TEST_DISKS (comma-separated blank disk names) to run pool-create tests")
	}
	disks := strings.Split(v, ",")
	for i := range disks {
		disks[i] = strings.TrimSpace(disks[i])
	}
	if len(disks) < min {
		t.Skipf("TRUENAS_TEST_DISKS has %d disks, need at least %d", len(disks), min)
	}
	return disks
}

// TestAccPool_createMirror creates a MIRROR pool with autotrim = true — the
// exact shape from issue #7 — then toggles autotrim in place and imports.
// autotrim = true specifically exercises the post-create pool.update, since
// autotrim is not a pool.create input.
func TestAccPool_createMirror(t *testing.T) {
	disks := testDisks(t, 2)
	name := acctest.RandName("tfaccpool")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPoolDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccPoolConfig(name, "MIRROR", disks[:2], true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_pool.test", "name", name),
					resource.TestCheckResourceAttr("truenas_pool.test", "topology.data.0.type", "MIRROR"),
					resource.TestCheckResourceAttr("truenas_pool.test", "autotrim", "true"),
					resource.TestCheckResourceAttrSet("truenas_pool.test", "guid"),
					resource.TestCheckResourceAttr("truenas_pool.test", "healthy", "true"),
				),
			},
			// In-place update: toggle autotrim off (pool.update path).
			{
				Config: acctest.ProviderConfig() + testAccPoolConfig(name, "MIRROR", disks[:2], false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_pool.test", "autotrim", "false"),
				),
			},
			{
				ResourceName:            "truenas_pool.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"allocated", "free"},
			},
		},
	})
}

// TestAccPool_createRaidz2 creates a RAIDZ2 pool (4 disks) — the other topology
// from issue #7 — verifying the fix across vdev types.
func TestAccPool_createRaidz2(t *testing.T) {
	disks := testDisks(t, 4)
	name := acctest.RandName("tfaccpoolz2")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPoolDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccPoolConfig(name, "RAIDZ2", disks[:4], false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_pool.test", "name", name),
					resource.TestCheckResourceAttr("truenas_pool.test", "topology.data.0.type", "RAIDZ2"),
					resource.TestCheckResourceAttr("truenas_pool.test", "healthy", "true"),
					resource.TestCheckResourceAttrSet("truenas_pool.test", "guid"),
				),
			},
			{
				ResourceName:            "truenas_pool.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"allocated", "free"},
			},
		},
	})
}

// TestAccPool_fullTopology creates a pool that uses every topology component
// the provider supports at once — data (mirror), log, cache, and spare — using
// all five test disks. This is the live verification for the log/cache/spare
// payload fixes (log/cache/spare were never created against a real box before):
// the cache-vdev "STRIPE" type and the "spares" (plural) create key in
// particular were corrected from the schema but not exercised end to end.
func TestAccPool_fullTopology(t *testing.T) {
	disks := testDisks(t, 5)
	name := acctest.RandName("tfaccpoolfull")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPoolDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccPoolFullTopologyConfig(name, disks[:5]),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_pool.test", "name", name),
					resource.TestCheckResourceAttr("truenas_pool.test", "topology.data.0.type", "MIRROR"),
					resource.TestCheckResourceAttr("truenas_pool.test", "topology.log.0.type", "STRIPE"),
					resource.TestCheckResourceAttr("truenas_pool.test", "topology.cache.#", "1"),
					resource.TestCheckResourceAttr("truenas_pool.test", "topology.spare.#", "1"),
					resource.TestCheckResourceAttr("truenas_pool.test", "healthy", "true"),
				),
			},
			{
				ResourceName:            "truenas_pool.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"allocated", "free"},
			},
		},
	})
}

// testAccPoolFullTopologyConfig builds a config using data(mirror 2) + log(1) +
// cache(1) + spare(1) from a 5-disk list. Note the Terraform attribute is
// "spare" (the provider maps it to the create API's "spares" key).
func testAccPoolFullTopologyConfig(name string, d []string) string {
	return fmt.Sprintf(`
resource "truenas_pool" "test" {
  name = %q
  topology = {
    data  = [{ type = "MIRROR", disks = [%q, %q] }]
    log   = [{ type = "STRIPE", disks = [%q] }]
    cache = [%q]
    spare = [%q]
  }
}
`, name, d[0], d[1], d[2], d[3], d[4])
}

func testAccPoolConfig(name, vdevType string, disks []string, autotrim bool) string {
	quoted := make([]string, len(disks))
	for i, d := range disks {
		quoted[i] = fmt.Sprintf("%q", d)
	}
	return fmt.Sprintf(`
resource "truenas_pool" "test" {
  name = %q
  topology = {
    data = [{
      type  = %q
      disks = [%s]
    }]
  }
  autotrim = %v
}
`, name, vdevType, strings.Join(quoted, ", "), autotrim)
}

// diskSerial looks up the stable serial of a disk given its kernel device
// name (sdX) via disk.query, so the stable-identifier test can drive the same
// physical disk by both forms. Skips the test if the disk or its serial is not
// found (e.g. a QEMU disk created without serial=).
func diskSerial(t *testing.T, name string) string {
	t.Helper()
	raw, err := acctest.Client().Call(context.Background(), "disk.query",
		[][]any{{"name", "=", name}})
	if err != nil {
		t.Fatalf("disk.query for %q: %v", name, err)
	}
	var disks []struct {
		Name   string `json:"name"`
		Serial string `json:"serial"`
	}
	if err := json.Unmarshal(raw, &disks); err != nil {
		t.Fatalf("parse disk.query: %v", err)
	}
	if len(disks) == 0 || disks[0].Serial == "" {
		t.Skipf("disk %q has no serial in disk.query; cannot run stable-id test", name)
	}
	return disks[0].Serial
}

// TestAccPool_stableDiskSerial is the live verification for issue #9. It
// creates a single-disk pool whose config names the disk by its STABLE SERIAL
// (not the volatile sdX), then re-applies an equivalent config that names the
// SAME physical disk by its kernel sdX name AND spells the type "DISK" instead
// of "STRIPE" — and requires that plan to be EMPTY. That is the crux: a config
// using a stable serial reconciles against the live sdX in state as the same
// physical disk, so neither a kernel renumber nor the DISK/STRIPE spelling
// produces a spurious diff (which, since topology is RequiresReplace, would
// destroy and recreate the pool). State stores the live device name; the
// stability comes from plan-time resolution, so the create step asserts the
// pool comes up healthy rather than a particular stored disk form.
func TestAccPool_stableDiskSerial(t *testing.T) {
	disks := testDisks(t, 1)
	acctest.PreCheck(t)
	dev := disks[0]
	serial := diskSerial(t, dev)
	name := acctest.RandName("tfaccpoolsn")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPoolDestroyed(name),
		Steps: []resource.TestStep{
			// Create naming the disk by its stable serial.
			{
				Config: acctest.ProviderConfig() + testAccPoolConfig(name, "STRIPE", []string{serial}, false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_pool.test", "name", name),
					resource.TestCheckResourceAttr("truenas_pool.test", "topology.data.0.type", "STRIPE"),
					resource.TestCheckResourceAttr("truenas_pool.test", "healthy", "true"),
					resource.TestCheckResourceAttrSet("truenas_pool.test", "guid"),
				),
			},
			// Same physical disk by sdX + type "DISK": must be a no-op plan
			// (proves serial-in-config is renumber/spelling-proof). This is the
			// crux of the test. No import step: import reads back the live sdX
			// device name, which by design differs from the serial this pool's
			// state was created with, so ImportStateVerify is exercised by the
			// sdX-config pool tests instead (TestAccPool_createRaidz2 etc.).
			{
				Config:             acctest.ProviderConfig() + testAccPoolConfig(name, "DISK", []string{dev}, false),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// TestAccPool_stableDiskSerialMirror is the multi-disk counterpart of
// TestAccPool_stableDiskSerial: it creates a two-disk MIRROR naming both disks
// by their stable serials, then re-applies an equivalent config naming the
// SAME two disks by their sdX device names, and requires an empty plan. This
// exercises the per-disk, positional sameDisk reconciliation across a
// multi-disk vdev (not just the single-disk path), against real disks.
func TestAccPool_stableDiskSerialMirror(t *testing.T) {
	disks := testDisks(t, 2)
	acctest.PreCheck(t)
	dev0, dev1 := disks[0], disks[1]
	serial0, serial1 := diskSerial(t, dev0), diskSerial(t, dev1)
	name := acctest.RandName("tfaccpoolsnm")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPoolDestroyed(name),
		Steps: []resource.TestStep{
			// Create the mirror naming both disks by serial.
			{
				Config: acctest.ProviderConfig() + testAccPoolConfig(name, "MIRROR", []string{serial0, serial1}, false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_pool.test", "topology.data.0.type", "MIRROR"),
					resource.TestCheckResourceAttr("truenas_pool.test", "healthy", "true"),
				),
			},
			// Same two physical disks named by sdX: must be a no-op plan.
			{
				Config:             acctest.ProviderConfig() + testAccPoolConfig(name, "MIRROR", []string{dev0, dev1}, false),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// poolSummary is the subset of pool.query fields this test package needs
// directly (outside of the provider's own resource code).
type poolSummary struct {
	ID int64 `json:"id"`
}

func testAccCheckPoolDestroyed(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "pool.query", [][]any{{"name", "=", name}})
		if err != nil {
			return fmt.Errorf("error checking pool %s: %v", name, err)
		}
		var results []poolSummary
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing pool.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("pool %s still exists", name)
		}
		return nil
	}
}

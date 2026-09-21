// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package system_dataset_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

// TestAccSystemDatasetDataSource_basic reads the current TrueNAS system
// dataset configuration through the truenas_system_dataset datasource only.
// It never writes: migrating the system dataset between pools is a
// disruptive, long-running operation, and the box's real configuration must
// not be touched by an acceptance test run.
//
// Requires:
//   - TF_ACC=1
//   - TRUENAS_API_KEY (or TRUENAS_USERNAME+TRUENAS_PASSWORD) set for
//     acctest.PreCheck
//   - A running TrueNAS instance
func TestAccSystemDatasetDataSource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure a TrueNAS instance is available")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
data "truenas_system_dataset" "test" {}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_system_dataset.test", "id", "system_dataset"),
					resource.TestCheckResourceAttrSet("data.truenas_system_dataset.test", "pool"),
					resource.TestCheckResourceAttrSet("data.truenas_system_dataset.test", "basename"),
					resource.TestCheckResourceAttrSet("data.truenas_system_dataset.test", "path"),
					resource.TestCheckResourceAttrSet("data.truenas_system_dataset.test", "uuid"),
				),
			},
		},
	})
}

// TestAccSystemDataset_basic is intentionally skipped by default.
// truenas_system_dataset is a SINGLETON resource whose "pool" attribute, when
// changed, triggers a long-running migration of core system state (logs,
// reporting, syslog, samba4 data) to a different pool on the live target
// system. A naive create/update/destroy acceptance test would move the
// system dataset off its current pool, which is disruptive and can be slow
// or risky to reverse against a shared/production TrueNAS instance.
//
// If this test is ever enabled against a disposable/non-production TrueNAS
// instance with at least two pools available, it should:
//  1. Read the current config via the datasource first, noting the current
//     pool.
//  2. Drive "pool" through a value and back to the original pool, so the net
//     effect on the box is a no-op (accepting that this still triggers two
//     real migrations).
//  3. Use ImportState with ImportStateId "system_dataset" to verify import
//     normalizes any ID to the fixed singleton ID.
//
// It is gated behind TRUENAS_TEST_SYSTEM_DATASET (a real migration job that
// restarts services) and needs TRUENAS_TEST_DISKS (a spare disk to build a
// second pool). It moves the system dataset to that pool and back, verifying
// both moves; a t.Cleanup returns it to the original pool (a pool can't be
// destroyed while it holds the system dataset) and destroys the test pool. Run
// only against a disposable box.
//
// CAVEATS when enabling:
//   - TRUENAS_TEST_DISKS must list disks that are actually unused right now;
//     TrueNAS reassigns sdX letters across reboots (check disk.get_unused).
//   - The final migration back can restart middleware and drop this client's
//     WebSocket, so the t.Cleanup pool-destroy may fail with "not connected".
//     The test's own second step already returns the system dataset to the
//     original pool, so the box is safe; only the throwaway test pool may be
//     left behind — destroy it manually with pool.export {destroy:true} if so.
func TestAccSystemDataset_basic(t *testing.T) {
	if os.Getenv("TRUENAS_TEST_SYSTEM_DATASET") == "" {
		t.Skip("set TRUENAS_TEST_SYSTEM_DATASET=1 (on a disposable box) to run the system_dataset migration test")
	}
	disksEnv := os.Getenv("TRUENAS_TEST_DISKS")
	if disksEnv == "" {
		t.Skip("set TRUENAS_TEST_DISKS (a spare disk) for the second pool")
	}
	disk := strings.TrimSpace(strings.Split(disksEnv, ",")[0])
	acctest.PreCheck(t)

	c := acctest.Client()
	ctx := context.Background()
	orig := systemDatasetPool(t)
	const testPool = "sysdstest"

	if _, err := c.CallJob(ctx, "pool.create", map[string]any{
		"name": testPool,
		"topology": map[string]any{
			"data": []map[string]any{{"type": "STRIPE", "disks": []string{disk}}},
		},
	}); err != nil {
		t.Fatalf("create test pool %q: %v", testPool, err)
	}

	t.Cleanup(func() {
		// The system dataset must be off the test pool before it can be destroyed.
		if p := systemDatasetPoolNoFatal(c); p != "" && p != orig {
			if _, err := c.CallJob(ctx, "systemdataset.update", map[string]any{"pool": orig}); err != nil {
				t.Logf("WARNING: failed to move system dataset back to %q: %v", orig, err)
			}
		}
		id, err := poolIDByName(c, testPool)
		if err != nil {
			t.Logf("WARNING: could not find test pool %q to destroy: %v", testPool, err)
			return
		}
		if _, err := c.CallJob(ctx, "pool.export", id, map[string]any{"cascade": true, "destroy": true}); err != nil {
			t.Logf("WARNING: failed to destroy test pool %q: %v", testPool, err)
		}
	})

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccSystemDatasetConfig(testPool),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_system_dataset.test", "id", "system_dataset"),
					resource.TestCheckResourceAttr("truenas_system_dataset.test", "pool", testPool),
				),
			},
			// Move it back to the original pool (update path).
			{
				Config: acctest.ProviderConfig() + testAccSystemDatasetConfig(orig),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_system_dataset.test", "pool", orig),
				),
			},
			{
				ResourceName:      "truenas_system_dataset.test",
				ImportState:       true,
				ImportStateId:     "system_dataset",
				ImportStateVerify: true,
			},
		},
	})
}

func testAccSystemDatasetConfig(pool string) string {
	return fmt.Sprintf(`
resource "truenas_system_dataset" "test" {
  pool = %q
}
`, pool)
}

// systemDatasetPool returns the pool the system dataset currently lives on.
func systemDatasetPool(t *testing.T) string {
	t.Helper()
	raw, err := acctest.Client().Call(context.Background(), "systemdataset.config")
	if err != nil {
		t.Fatalf("reading systemdataset.config: %v", err)
	}
	var cfg struct {
		Pool string `json:"pool"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("parsing systemdataset.config: %v", err)
	}
	return cfg.Pool
}

// systemDatasetPoolNoFatal is the cleanup-safe variant (returns "" on error).
func systemDatasetPoolNoFatal(c *client.Client) string {
	raw, err := c.Call(context.Background(), "systemdataset.config")
	if err != nil {
		return ""
	}
	var cfg struct {
		Pool string `json:"pool"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return ""
	}
	return cfg.Pool
}

// poolIDByName looks up a pool's numeric id by name.
func poolIDByName(c *client.Client, name string) (int64, error) {
	raw, err := c.Call(context.Background(), "pool.query", [][]any{{"name", "=", name}})
	if err != nil {
		return 0, err
	}
	var pools []struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(raw, &pools); err != nil {
		return 0, err
	}
	if len(pools) == 0 {
		return 0, fmt.Errorf("no pool named %q", name)
	}
	return pools[0].ID, nil
}

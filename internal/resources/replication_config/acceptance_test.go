// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package replication_config_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccReplicationConfigDataSource_basic reads the current TrueNAS
// replication configuration through the truenas_replication_config
// datasource only.
//
// Requires:
//   - TF_ACC=1
//   - TRUENAS_API_KEY (or TRUENAS_USERNAME+TRUENAS_PASSWORD) set for
//     acctest.PreCheck
//   - A running TrueNAS instance
func TestAccReplicationConfigDataSource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure a TrueNAS instance is available")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
data "truenas_replication_config" "test" {}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_replication_config.test", "id", "replication_config"),
				),
			},
		},
	})
}

// replicationConfigOriginal captures the one field
// TestAccReplicationConfig_setAndRestore touches.
// MaxParallelReplicationTasks is nullable on the wire (nil means
// "unlimited").
type replicationConfigOriginal struct {
	MaxParallelReplicationTasks *int64 `json:"max_parallel_replication_tasks"`
}

// readReplicationConfigOriginal reads the box's current
// max_parallel_replication_tasks via replication.config.config, so the test
// can restore it exactly afterward.
func readReplicationConfigOriginal(t *testing.T) replicationConfigOriginal {
	t.Helper()
	raw, err := acctest.Client().Call(context.Background(), "replication.config.config")
	if err != nil {
		t.Fatalf("error reading current replication config: %v", err)
	}
	var orig replicationConfigOriginal
	if err := json.Unmarshal(raw, &orig); err != nil {
		t.Fatalf("error parsing replication.config.config response: %v", err)
	}
	return orig
}

// restoreReplicationConfig sends only max_parallel_replication_tasks back
// to its original value (nil if it was originally unlimited/unset) via
// replication.config.update. It runs from t.Cleanup, so it restores the box
// even if the Terraform steps themselves fail partway through.
func restoreReplicationConfig(t *testing.T, orig replicationConfigOriginal) {
	t.Helper()
	var v any
	if orig.MaxParallelReplicationTasks != nil {
		v = *orig.MaxParallelReplicationTasks
	}
	if _, err := acctest.Client().Call(context.Background(), "replication.config.update", map[string]any{
		"max_parallel_replication_tasks": v,
	}); err != nil {
		t.Fatalf("error restoring replication config max_parallel_replication_tasks: %v", err)
	}
}

// TestAccReplicationConfig_setAndRestore drives the singleton
// truenas_replication_config resource's "max_parallel_replication_tasks"
// field through a test value, then imports it. It requires TF_ACC=1 and
// TRUENAS_DISRUPTIVE=1 (acctest.DisruptiveCheck), since it mutates the
// box's live replication configuration; a t.Cleanup-registered API restore
// is the safety net if the Terraform steps fail.
//
// The model represents a null max_parallel_replication_tasks (meaning
// "unlimited") as the state value 0 (see updatePayload's three-way
// handling: explicit 0 in config sends JSON nil to the API), so an
// originally-null value cannot be distinguished, in Terraform config, from
// an originally-explicit-0 value. When the box's original value is nil,
// this test therefore restores ONLY via the t.Cleanup API call above and
// does not attempt a terraform-driven restore step (which would require
// asserting on state 0 either way and would not exercise anything the
// null-handling unit tests don't already cover). When the original value is
// non-nil, the test drives Terraform through the test value and back to the
// exact original number, exercising the "update back" path.
func TestAccReplicationConfig_setAndRestore(t *testing.T) {
	acctest.DisruptiveCheck(t)

	orig := readReplicationConfigOriginal(t)
	t.Cleanup(func() { restoreReplicationConfig(t, orig) })

	var testValue int64 = 2
	if orig.MaxParallelReplicationTasks != nil {
		testValue = *orig.MaxParallelReplicationTasks + 1
	}

	steps := []resource.TestStep{
		{
			Config: acctest.ProviderConfig() + testAccReplicationConfigConfig(testValue),
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("truenas_replication_config.test", "id", "replication_config"),
				resource.TestCheckResourceAttr("truenas_replication_config.test", "max_parallel_replication_tasks", fmt.Sprintf("%d", testValue)),
			),
		},
	}

	if orig.MaxParallelReplicationTasks != nil {
		steps = append(steps, resource.TestStep{
			Config: acctest.ProviderConfig() + testAccReplicationConfigConfig(*orig.MaxParallelReplicationTasks),
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("truenas_replication_config.test", "max_parallel_replication_tasks", fmt.Sprintf("%d", *orig.MaxParallelReplicationTasks)),
			),
		})
	}

	steps = append(steps, resource.TestStep{
		ResourceName:      "truenas_replication_config.test",
		ImportState:       true,
		ImportStateId:     "replication_config",
		ImportStateVerify: true,
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps:                    steps,
	})
}

func testAccReplicationConfigConfig(maxParallelReplicationTasks int64) string {
	return fmt.Sprintf(`
resource "truenas_replication_config" "test" {
  max_parallel_replication_tasks = %d
}
`, maxParallelReplicationTasks)
}

package replication_config_test

import (
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
//   - A running TrueNAS SCALE instance
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

// TestAccReplicationConfig_basic is intentionally skipped by default, for
// consistency with the other singleton resources in this provider
// (truenas_system_general, truenas_system_advanced, truenas_system_dataset):
// even though max_parallel_replication_tasks is a comparatively low-risk
// setting, this resource still manages a live, shared system-wide
// configuration value, and a naive create/update/destroy acceptance test run
// against a shared target box could race with or clobber a value set by
// another process.
//
// If this test is ever enabled against a disposable/non-production TrueNAS
// instance, it should:
//  1. Read the current config via the datasource first.
//  2. Drive max_parallel_replication_tasks through a value and back to the
//     value observed in step 1, so the net effect on the box is a no-op.
//  3. Use ImportState with ImportStateId "replication_config" to verify
//     import normalizes any ID to the fixed singleton ID.
func TestAccReplicationConfig_basic(t *testing.T) {
	t.Skip("truenas_replication_config manages a live, shared system-wide configuration value; skipped for consistency with the other singleton resources in this provider. See comment on TestAccReplicationConfig_basic for how to safely enable this against a disposable instance.")
}

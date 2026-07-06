package nfs_config_test

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccNFSConfigDataSource_basic reads the current TrueNAS NFS
// configuration through the truenas_nfs_config datasource only. It never
// writes: NFS is a system-critical singleton service (existing exports and
// clients may depend on it), and the box's real NFS configuration must not
// be overwritten by an acceptance test run.
//
// Requires:
//   - TF_ACC=1
//   - TRUENAS_API_KEY (or TRUENAS_USERNAME+TRUENAS_PASSWORD) set for
//     acctest.PreCheck
//   - A running TrueNAS SCALE instance
func TestAccNFSConfigDataSource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure a TrueNAS instance is available")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
data "truenas_nfs_config" "test" {}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_nfs_config.test", "id", "nfs_config"),
					resource.TestCheckResourceAttrSet("data.truenas_nfs_config.test", "servers"),
					resource.TestCheckResourceAttrSet("data.truenas_nfs_config.test", "managed_nfsd"),
				),
			},
		},
	})
}

// TestAccNFSConfig_basic is intentionally skipped by default.
// truenas_nfs_config is a SINGLETON resource that manages a system-critical
// service: bringing it under Terraform management mutates the box's actual
// NFS configuration (nfs.update), and a naive create/update/destroy
// acceptance test risks disrupting existing NFS exports or clients on
// whatever system runs it (e.g. by rewriting protocols, bindip, or the
// mountd/statd/lockd ports).
//
// If this test is ever enabled against a disposable/non-production TrueNAS
// instance, it should:
//  1. Read the current config via the datasource first.
//  2. Only touch a low-risk, additive field (e.g. mountd_log) in the
//     resource config, driving it through a value and back to the value the
//     datasource observed in step 1, so the net effect on the box is a
//     no-op. Never touch protocols, bindip, servers, or the mountd/statd/
//     lockd ports in an automated test: changing those on a live system can
//     disrupt NFS clients.
//  3. Use ImportState with ImportStateId "nfs_config" to verify import
//     normalizes any ID to the fixed singleton ID.
func TestAccNFSConfig_basic(t *testing.T) {
	t.Skip("truenas_nfs_config manages a system-critical service; skipped to avoid mutating the target box's NFS configuration and risking disruption of existing exports or clients. See comment on TestAccNFSConfig_basic for how to safely enable this against a disposable instance.")
}

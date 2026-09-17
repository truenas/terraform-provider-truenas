// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nfs_config_test

import (
	"context"
	"encoding/json"
	"fmt"
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
//   - A running TrueNAS instance
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

// nfsConfigOriginal captures the one field TestAccNFSConfig_setAndRestore
// touches.
type nfsConfigOriginal struct {
	V4Domain string `json:"v4_domain"`
}

// readNFSConfigOriginal reads the box's current v4_domain via nfs.config, so
// the test can restore it exactly afterward.
func readNFSConfigOriginal(t *testing.T) nfsConfigOriginal {
	t.Helper()
	raw, err := acctest.Client().Call(context.Background(), "nfs.config")
	if err != nil {
		t.Fatalf("error reading current nfs config: %v", err)
	}
	var orig nfsConfigOriginal
	if err := json.Unmarshal(raw, &orig); err != nil {
		t.Fatalf("error parsing nfs.config response: %v", err)
	}
	return orig
}

// restoreNFSConfig sends only v4_domain back to its original value via
// nfs.update. It runs from t.Cleanup, so it restores the box even if the
// Terraform steps themselves fail partway through. protocols, bindip,
// servers, and the mountd/statd/lockd ports are never touched by this test.
func restoreNFSConfig(t *testing.T, orig nfsConfigOriginal) {
	t.Helper()
	if _, err := acctest.Client().Call(context.Background(), "nfs.update", map[string]any{
		"v4_domain": orig.V4Domain,
	}); err != nil {
		t.Fatalf("error restoring nfs v4_domain: %v", err)
	}
}

// TestAccNFSConfig_setAndRestore drives the singleton truenas_nfs_config
// resource's "v4_domain" field (a plain string, not part of the
// nullable-three-way group) through a test value and back to the value read
// from the box before the test ran, then imports it. It requires TF_ACC=1
// and TRUENAS_DISRUPTIVE=1 (acctest.DisruptiveCheck), since it mutates the
// box's live NFS configuration; a t.Cleanup-registered API restore is the
// safety net if the Terraform steps fail. protocols, bindip, servers, and
// the mountd/statd/lockd ports are never touched: changing those on a live
// system can disrupt NFS clients.
func TestAccNFSConfig_setAndRestore(t *testing.T) {
	acctest.DisruptiveCheck(t)

	orig := readNFSConfigOriginal(t)
	t.Cleanup(func() { restoreNFSConfig(t, orig) })

	testValue := "tf-acc.example"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccNFSConfigConfig(testValue),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_nfs_config.test", "id", "nfs_config"),
					resource.TestCheckResourceAttr("truenas_nfs_config.test", "v4_domain", testValue),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccNFSConfigConfig(orig.V4Domain),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_nfs_config.test", "v4_domain", orig.V4Domain),
				),
			},
			{
				ResourceName:      "truenas_nfs_config.test",
				ImportState:       true,
				ImportStateId:     "nfs_config",
				ImportStateVerify: true,
			},
		},
	})
}

func testAccNFSConfigConfig(v4Domain string) string {
	return fmt.Sprintf(`
resource "truenas_nfs_config" "test" {
  v4_domain = %q
}
`, v4Domain)
}

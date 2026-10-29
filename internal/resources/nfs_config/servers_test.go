// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nfs_config_test

// Adversarial bug-hunt tests (throwaway, not for commit).

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func snapshotRestore(t *testing.T, ns string, drop ...string) {
	t.Helper()
	ctx := context.Background()
	raw, err := acctest.RestoreCall(ctx, ns+".config")
	if err != nil {
		t.Fatalf("read %s.config: %v", ns, err)
	}
	var orig map[string]any
	if err := json.Unmarshal(raw, &orig); err != nil {
		t.Fatal(err)
	}
	delete(orig, "id")
	for _, d := range drop {
		delete(orig, d)
	}
	t.Cleanup(func() {
		if _, err := acctest.RestoreCall(context.Background(), ns+".update", orig); err != nil {
			t.Errorf("RESTORE %s.update failed: %v", ns, err)
		}
	})
}

// TestAccNFSConfig_serversAuto guards B-068: servers = 0 is documented as
// automatic thread management. The provider sends servers = null; nfs.config
// then reports the computed thread count with managed_nfsd = true, and the
// resource must read that back as 0 (it read the count, failing the apply).
// Import must give 0 as well.
func TestAccNFSConfig_serversAuto(t *testing.T) {
	acctest.DisruptiveCheck(t)
	snapshotRestore(t, "nfs", "managed_nfsd", "v4_krb_enabled", "keytab_has_nfs_spn")
	c1 := acctest.ProviderConfig() + `
resource "truenas_nfs_config" "test" {
  servers = 4
}
`
	c2 := acctest.ProviderConfig() + `
resource "truenas_nfs_config" "test" {
  servers = 0
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: c1, Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("truenas_nfs_config.test", "servers", "4"),
				resource.TestCheckResourceAttr("truenas_nfs_config.test", "managed_nfsd", "false"),
			)},
			{Config: c2, Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("truenas_nfs_config.test", "servers", "0"),
				resource.TestCheckResourceAttr("truenas_nfs_config.test", "managed_nfsd", "true"),
			)},
			{Config: c2, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{
				ResourceName:      "truenas_nfs_config.test",
				ImportState:       true,
				ImportStateId:     "nfs_config",
				ImportStateVerify: true,
			},
		},
	})
}

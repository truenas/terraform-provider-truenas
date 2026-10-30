// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nfs_config_test

// Adversarial bug-hunt tests (throwaway, not for commit).

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// mountd_port = 0 is documented as "restores the default (dynamic) port".
func TestHuntNFSConfig_portZero(t *testing.T) {
	acctest.DisruptiveCheck(t)
	snapshotRestore(t, "nfs", "managed_nfsd", "v4_krb_enabled", "keytab_has_nfs_spn")
	c := acctest.ProviderConfig() + `
resource "truenas_nfs_config" "test" {
  mountd_port   = 0
  rpcstatd_port = 0
  rpclockd_port = 0
  protocols     = ["NFSV4", "NFSV3"]
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: c},
			{Config: c, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}

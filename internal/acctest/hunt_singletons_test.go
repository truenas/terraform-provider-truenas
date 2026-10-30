// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest_test

// Adversarial bug hunt: adopt every singleton config resource the way a user
// does — `terraform import` with an empty resource body — and require an
// empty plan. A non-empty plan means Terraform would rewrite the box's live
// configuration on the first apply (the share import bug, F4, for
// singletons). The plan check runs before apply, so nothing is changed.
// "Missing required argument" means the singleton cannot be adopted without
// transcribing attributes first — recorded, not a rewrite.

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

var singletonTypes = []string{
	"alert_policy", "audit_config", "catalog_config", "directoryservices", "docker_config",
	"failover_config", "ftp_config", "iscsi_global", "kerberos_config", "lxc_config", "mail",
	"network_config", "nfs_config", "nvmet_global", "replication_config", "resilver_config",
	"smb_config", "snmp_config", "ssh_config", "system_advanced", "system_dataset",
	"system_general", "tn_connect_config", "truecommand_config", "twofactor_auth",
	"ups_config", "webshare_config",
}

func TestHuntSingletonImport(t *testing.T) {
	acctest.PreCheck(t)
	for _, typ := range singletonTypes {
		typ := typ
		t.Run(typ, func(t *testing.T) {
			addr := "truenas_" + typ + ".s"
			cfg := acctest.ProviderConfig() + fmt.Sprintf("\nresource \"truenas_%s\" \"s\" {\n}\n", typ)
			resource.Test(t, resource.TestCase{
				ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{Config: cfg, ResourceName: addr, ImportState: true, ImportStatePersist: true, ImportStateId: "singleton"},
					{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
					// forget it again so the test's destroy does not try to delete a singleton
					{Config: acctest.ProviderConfig() + fmt.Sprintf("\nremoved {\n  from = %s\n  lifecycle {\n    destroy = false\n  }\n}\n", addr)},
				},
			})
		})
	}
}

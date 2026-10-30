// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_initiator_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestHuntISCSIInitiator_lifecycle: duplicates/mixed case/CIDR, empty plan,
// import, empty list.
func TestHuntISCSIInitiator_lifecycle(t *testing.T) {
	c := acctest.RandName("tfhuntblkinitlc")
	full := fmt.Sprintf("  comment = %q\n  initiators = [\"iqn.2005-10.org.Example:B\", \"iqn.2005-10.org.example:a\", \"192.168.1.0/24\"]", c)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: initCfg(full)},
			{Config: initCfg(full), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{ResourceName: "truenas_iscsi_initiator.test", ImportState: true, ImportStateVerify: true},
			{Config: initCfg(full), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{Config: initCfg(fmt.Sprintf("  comment = %q\n  initiators = []", c))},
			{Config: initCfg(fmt.Sprintf("  comment = %q\n  initiators = []", c)), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}

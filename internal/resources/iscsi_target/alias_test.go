// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_target_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// checkServerAlias asserts the target's alias on the server (nil = null).
func checkServerAlias(t *testing.T, want any) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		id, err := strconv.ParseInt(s.RootModule().Resources["truenas_iscsi_target.test"].Primary.ID, 10, 64)
		if err != nil {
			return err
		}
		if got := acctest.GetInstance(t, "iscsi.target", id)["alias"]; got != want {
			return fmt.Errorf("server alias = %v, want %v", got, want)
		}
		return nil
	}
}

// TestAccISCSITarget_aliasClear sets an alias and then clears it with
// alias = "". The empty value used to be omitted from the payload, so the
// server kept the old alias and the apply failed with "inconsistent result"
// (B-100). It also checks that an unrelated update plans alias and mode as
// known values rather than "(known after apply)", and that leaving alias out
// of config keeps the server value.
func TestAccISCSITarget_aliasClear(t *testing.T) {
	pc := acctest.RandName("tf-acc-tgt-portal")
	name := acctest.RandName("tfacctgtalias")
	const r = "truenas_iscsi_target.test"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: targetCfg(pc, name, `  alias = "tfaccalias"`), Check: checkServerAlias(t, "tfaccalias")},
			{
				// Unrelated change (auth_networks): alias/mode stay known.
				Config: targetCfg(pc, name, `  alias = "tfaccalias"
  auth_networks = ["10.0.0.0/8"]`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectKnownValue(r, tfjsonpath.New("alias"), knownvalue.StringExact("tfaccalias")),
					plancheck.ExpectKnownValue(r, tfjsonpath.New("mode"), knownvalue.StringExact("ISCSI")),
				}},
			},
			{
				// alias left out of config: the server value is kept.
				Config:           targetCfg(pc, name, `  auth_networks = ["10.0.0.0/8"]`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				Config: targetCfg(pc, name, `  alias = ""
  auth_networks = ["10.0.0.0/8"]`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(r, "alias", ""),
					checkServerAlias(t, nil),
				),
			},
			{
				Config: targetCfg(pc, name, `  alias = ""
  auth_networks = ["10.0.0.0/8"]`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
		},
	})
}

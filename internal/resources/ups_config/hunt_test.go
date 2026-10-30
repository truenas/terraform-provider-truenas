// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package ups_config_test

// Adversarial bug-hunt test: apply every API-accepted value and assert an
// empty re-plan, then import. readUPSConfigOriginal / restoreUPSConfig are
// defined in acceptance_test.go.

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestHuntUPSConfig_allFields sets every writable truenas_ups_config attribute
// (including the write-only monpwd / monpwd_wo_version pair) to a non-default
// value, asserts the read-back re-plans empty, then imports. driver and port
// are set to the box's own current values so ups.update stays valid (an
// arbitrary driver name would be rejected), which is also why the test skips
// on a box with no UPS configured. The original configuration is captured
// first and restored via t.Cleanup (full ups.update payload).
//
// mode stays MASTER so complete_identifier resolves to "<id>@localhost" and
// does not depend on a reachable remote host; because the config is identical
// across the two apply steps nothing changes, so the derived (and
// intentionally not-UseStateForUnknown) complete_identifier stays known and
// the re-plan is empty. monpwd is write-only (absent from state) and
// monpwd_wo_version is a local rotation trigger, so both are ignored on
// import.
func TestHuntUPSConfig_allFields(t *testing.T) {
	acctest.DisruptiveCheck(t)

	orig := readUPSConfigOriginal(t)
	if orig.Driver == "" || orig.Port == "" {
		t.Skip("UPS is unconfigured on the target box (empty driver/port); ups.update requires a valid driver/port, which this test cannot synthesize")
	}
	t.Cleanup(func() { restoreUPSConfig(t, orig) })

	const extraUsers = "[tfhuntextra]\n\tpassword = tfhuntpass\n\tupsmon slave"

	c := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_ups_config" "test" {
  identifier        = "tfhuntups"
  mode              = "MASTER"
  remotehost        = "192.0.2.10"
  remoteport        = 3494
  driver            = %q
  port              = %q
  options           = "pollinterval = 10"
  optionsupsd       = "MAXAGE 25"
  description       = "tf-hunt-ups-allfields"
  shutdown          = "BATT"
  shutdowntimer     = 90
  shutdowncmd       = "/sbin/poweroff"
  monuser           = "tfhuntmon"
  monpwd            = "tfhuntmonpass1"
  monpwd_wo_version = 1
  extrausers        = %q
  rmonitor          = true
  powerdown         = true
  hostsync          = 25
  nocommwarntime    = 400
}
`, orig.Driver, orig.Port, extraUsers)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_11_0)},
		Steps: []resource.TestStep{
			{Config: c},
			{Config: c, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{
				ResourceName:            "truenas_ups_config.test",
				ImportState:             true,
				ImportStateId:           "ups_config",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"monpwd", "monpwd_wo_version"},
			},
		},
	})
}

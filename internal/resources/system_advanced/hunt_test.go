// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package system_advanced_test

// Adversarial bug-hunt test: apply every API-accepted value and assert an
// empty re-plan, then import. advancedTestValues, testAccSystemAdvancedConfig,
// readSystemAdvancedOriginal and restoreSystemAdvanced are defined in
// acceptance_test.go.

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestHuntSystemAdvanced_allFields writes every field in advancedFields in a
// single config: the low-impact fields (advancedmode, anonstats, boot_scrub,
// fqdn_syslog, login_banner, motd, syslog_audit, sysloglevel, traceback,
// uploadcrash) are flipped to a non-default value by advancedTestValues, and
// every remaining field (serialconsole, serialport, serialspeed, consolemenu,
// consolemsg, debugkernel, kdump_enabled, autotune, powerdaemon, sed_user,
// kernel_extra_options, and nvidia on 26.0+) is written with the box's own
// value. The box's own values are used for the boot/console/power/SED fields
// deliberately: forcing non-default values there could break the console or
// boot path, which no empty-plan assertion is worth. The same config is
// applied twice and the second apply is asserted to re-plan empty.
//
// sed_passwd (write-only, never read back, clears a configured SED password
// if sent blank) is intentionally not set and is ignored on import.
// overprovision is not written (it reconfigures swap-on-ZFS); it stays
// Computed and its state value is preserved. The original configuration is
// captured first and every written field restored via t.Cleanup.
func TestHuntSystemAdvanced_allFields(t *testing.T) {
	acctest.DisruptiveCheck(t)

	// nvidia exists only on TrueNAS 26.0+.
	v26 := acctest.ServerVersionAtLeast(t, 26, 0)
	orig := readSystemAdvancedOriginal(t)
	t.Cleanup(func() { restoreSystemAdvanced(t, orig, v26) })

	c := acctest.ProviderConfig() + testAccSystemAdvancedConfig(orig, advancedTestValues(orig), v26)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: c},
			{Config: c, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{
				ResourceName:            "truenas_system_advanced.test",
				ImportState:             true,
				ImportStateId:           "system_advanced",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"sed_passwd"},
			},
		},
	})
}

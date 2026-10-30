// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package audit_config_test

// Adversarial bug-hunt test: apply every API-accepted value and assert an
// empty re-plan, then import. restoreConfigKeys is defined in restore_test.go.

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestHuntAuditConfig_allFields sets every writable audit.config field to a
// valid non-default value, asserts the read-back re-plans empty, then
// imports. The read-only space.* usage counters and the enabled_services
// summary are driven by other subsystems and are ignored on import.
func TestHuntAuditConfig_allFields(t *testing.T) {
	acctest.DisruptiveCheck(t)
	// Restore every field the configuration below writes.
	restoreConfigKeys(t, "retention", "reservation", "quota", "quota_fill_warning", "quota_fill_critical")

	c := acctest.ProviderConfig() + `
resource "truenas_audit_config" "test" {
  retention           = 20
  reservation         = 1
  quota               = 2
  quota_fill_warning  = 70
  quota_fill_critical = 85
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: c},
			{Config: c, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{
				ResourceName:            "truenas_audit_config.test",
				ImportState:             true,
				ImportStateId:           "audit_config",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"space", "enabled_services"},
			},
		},
	})
}

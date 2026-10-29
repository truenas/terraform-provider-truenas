// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package snmp_config_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccSNMPConfig_contactEmailCase guards B-077: an e-mail contact with an
// upper-case domain must apply without an inconsistent-result error (28.0
// stores the domain in lower case), re-plan empty, and import cleanly.
func TestAccSNMPConfig_contactEmailCase(t *testing.T) {
	acctest.DisruptiveCheck(t)

	raw, err := acctest.RestoreCall(context.Background(), "snmp.config")
	if err != nil {
		t.Fatalf("read snmp.config: %v", err)
	}
	var orig struct {
		Contact string `json:"contact"`
	}
	if err := json.Unmarshal(raw, &orig); err != nil {
		t.Fatalf("decode: %v", err)
	}
	t.Cleanup(func() {
		if _, err := acctest.RestoreCall(context.Background(), "snmp.update", map[string]any{"contact": orig.Contact}); err != nil {
			t.Errorf("restore contact: %v", err)
		}
	})

	cfg := acctest.ProviderConfig() + `
resource "truenas_snmp_config" "test" {
  contact = "Admin@Example.COM"
}
`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg, Check: resource.TestCheckResourceAttr("truenas_snmp_config.test", "contact", "Admin@Example.COM")},
			{Config: cfg, PlanOnly: true},
			{Config: acctest.ProviderConfig() + `
removed {
  from = truenas_snmp_config.test
  lifecycle {
    destroy = false
  }
}
`},
			{Config: cfg, ResourceName: "truenas_snmp_config.test", ImportState: true, ImportStatePersist: true, ImportStateId: "snmp_config"},
			{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}

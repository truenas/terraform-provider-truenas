// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package snmp_config_test

// Adversarial bug-hunt tests: apply every API-accepted value and assert an
// empty re-plan, then import.

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
	raw, err := acctest.RestoreCall(context.Background(), ns+".config")
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

// Full SNMPv3 config with every field populated plus plaintext community,
// traps/zilstat on, empty options. Assert the read-back re-plans empty.
func TestHuntSNMPConfig_v3Roundtrip(t *testing.T) {
	acctest.DisruptiveCheck(t)
	// v3 secrets are not read back; leave them unchanged on restore.
	snapshotRestore(t, "snmp", "v3_password", "v3_privpassphrase")
	c := acctest.ProviderConfig() + `
resource "truenas_snmp_config" "test" {
  community         = "hunt-public"
  contact           = "hunt@example.com"
  location          = "hunt-rack-1"
  options           = ""
  traps             = true
  zilstat           = true
  v3                = true
  v3_username       = "huntv3user"
  v3_authtype       = "SHA"
  v3_password       = "huntauthpass123"
  v3_privproto      = "AES"
  v3_privpassphrase = "huntprivpass123"
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: c},
			{Config: c, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{
				ResourceName:            "truenas_snmp_config.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"v3_password", "v3_privpassphrase", "community_wo", "community_wo_version"},
			},
		},
	})
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package ftp_config_test

// Adversarial bug-hunt tests (throwaway, not for commit).

import (
	"context"
	"encoding/json"
	"regexp"
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

// TestAccFTPConfig_anonpathAlone guards B-072: anonpath without
// onlyanonymous = true is rejected at plan time (ftp.update silently drops
// anonpath to null, which failed the apply).
func TestAccFTPConfig_anonpathAlone(t *testing.T) {
	acctest.DisruptiveCheck(t)
	snapshotRestore(t, "ftp")
	c := acctest.ProviderConfig() + `
resource "truenas_ftp_config" "test" {
  anonpath = "/mnt/` + acctest.TestPool() + `"
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: c, ExpectError: regexp.MustCompile(`anonpath\s+requires\s+onlyanonymous\s+=\s+true`)},
		},
	})
}

// TestAccFTPConfig_anonToggle guards B-072: onlyanonymous true -> false.
// Keeping anonpath in the configuration is a plan-time error; dropping it
// plans anonpath = "" (the server clears it) and applies cleanly.
func TestAccFTPConfig_anonToggle(t *testing.T) {
	acctest.DisruptiveCheck(t)
	snapshotRestore(t, "ftp")
	path := "/mnt/" + acctest.TestPool()
	c1 := acctest.ProviderConfig() + `
resource "truenas_ftp_config" "test" {
  onlyanonymous = true
  anonpath      = "` + path + `"
}
`
	c2 := acctest.ProviderConfig() + `
resource "truenas_ftp_config" "test" {
  onlyanonymous = false
  anonpath      = "` + path + `"
}
`
	c3 := acctest.ProviderConfig() + `
resource "truenas_ftp_config" "test" {
  onlyanonymous = false
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: c1},
			{Config: c1, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{Config: c2, ExpectError: regexp.MustCompile(`anonpath\s+requires\s+onlyanonymous\s+=\s+true`)},
			{Config: c3, Check: resource.TestCheckResourceAttr("truenas_ftp_config.test", "anonpath", "")},
			{Config: c3, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}

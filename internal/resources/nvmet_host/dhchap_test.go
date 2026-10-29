// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_host_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

const (
	dhKey1  = "DHHC-1:01:hnvzNuiX9Bi/Z6CwYPoi9KQ3SDHzgRhYGq8n5ZDCDiIPCaWe:"
	dhKey2  = "DHHC-1:01:8//S93VwLZz0kJJv3ENC8FfxlHiZbXkgEWsR0EsWp54I+FSJ:"
	dhCtrl1 = "DHHC-1:01:HTIM44KJK/u+Ih5Bxaqz1vCRGhcEen4UjWZyZKhtEG8sxusU:"
)

func hostCfg(nqn, body string) string {
	return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_nvmet_host" "test" {
  hostnqn = %q
%s
}
`, nqn, body)
}

// checkServerHostField asserts a field of the host on the server. want nil
// also accepts "" (28.0 reports no key as "", 26.0 as null).
func checkServerHostField(t *testing.T, field string, want any) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		id, err := strconv.ParseInt(s.RootModule().Resources["truenas_nvmet_host.test"].Primary.ID, 10, 64)
		if err != nil {
			return err
		}
		got := acctest.GetInstance(t, "nvmet.host", id)[field]
		if want == nil && got == "" {
			return nil
		}
		if got != want {
			return fmt.Errorf("server %s = %v, want %v", field, got, want)
		}
		return nil
	}
}

// TestAccNVMetHost_rotateKey changes dhchap_key and bumps
// dhchap_key_wo_version. A write-only value never reaches the plan, so the
// new key used to be silently ignored (B-025).
func TestAccNVMetHost_rotateKey(t *testing.T) {
	nqn := "nqn.2014-08.org.example:" + acctest.RandName("tfacchostrot")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: hostCfg(nqn, fmt.Sprintf("  dhchap_key = %q\n  dhchap_key_wo_version = 1", dhKey1)),
				Check: resource.ComposeTestCheckFunc(
					checkServerHostField(t, "dhchap_key", dhKey1),
					resource.TestCheckResourceAttr("truenas_nvmet_host.test", "dhchap_key_set", "true"),
				),
			},
			{
				Config:           hostCfg(nqn, fmt.Sprintf("  dhchap_key = %q\n  dhchap_key_wo_version = 2", dhKey2)),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("truenas_nvmet_host.test", plancheck.ResourceActionUpdate)}},
				Check:            checkServerHostField(t, "dhchap_key", dhKey2),
			},
			{
				Config:           hostCfg(nqn, fmt.Sprintf("  dhchap_key = %q\n  dhchap_key_wo_version = 2", dhKey2)),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
		},
	})
}

// TestAccNVMetHost_removeKey removes dhchap_key and dhchap_ctrl_key from
// config, which must turn DH-CHAP off on the server, and clears
// dhchap_dhgroup with "". Removing a key used to plan nothing and leave the
// old key active, and dhgroup could not be cleared (B-024). A key added
// outside Terraform is detected through dhchap_key_set and removed again.
func TestAccNVMetHost_removeKey(t *testing.T) {
	nqn := "nqn.2014-08.org.example:" + acctest.RandName("tfacchostrm")
	withKeys := fmt.Sprintf("  description = \"d1\"\n  dhchap_key = %q\n  dhchap_ctrl_key = %q\n  dhchap_ctrl_key_wo_version = 1\n  dhchap_dhgroup = \"2048-BIT\"", dhKey1, dhCtrl1)
	noKeys := "  description = \"d1\"\n  dhchap_dhgroup = \"\""
	if !acctest.ServerVersionAtLeast(t, 26, 0) {
		withKeys = fmt.Sprintf("  dhchap_key = %q\n  dhchap_ctrl_key = %q\n  dhchap_ctrl_key_wo_version = 1\n  dhchap_dhgroup = \"2048-BIT\"", dhKey1, dhCtrl1)
		noKeys = "  dhchap_dhgroup = \"\""
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: hostCfg(nqn, withKeys), Check: checkServerHostField(t, "dhchap_ctrl_key", dhCtrl1)},
			{
				Config: hostCfg(nqn, noKeys),
				Check: resource.ComposeTestCheckFunc(
					checkServerHostField(t, "dhchap_key", nil),
					checkServerHostField(t, "dhchap_ctrl_key", nil),
					checkServerHostField(t, "dhchap_dhgroup", nil),
					resource.TestCheckResourceAttr("truenas_nvmet_host.test", "dhchap_key_set", "false"),
					resource.TestCheckResourceAttr("truenas_nvmet_host.test", "dhchap_ctrl_key_set", "false"),
					resource.TestCheckResourceAttr("truenas_nvmet_host.test", "dhchap_dhgroup", ""),
				),
			},
			{Config: hostCfg(nqn, noKeys), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{ResourceName: "truenas_nvmet_host.test", ImportState: true, ImportStateVerify: true},
			{
				// A key set outside Terraform shows up as a change and is
				// removed again.
				PreConfig: func() {
					setHostKeyOutOfBand(t, nqn, dhKey2)
				},
				Config: hostCfg(nqn, noKeys),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("truenas_nvmet_host.test", plancheck.ResourceActionUpdate),
				}},
				Check: checkServerHostField(t, "dhchap_key", nil),
			},
		},
	})
}

// setHostKeyOutOfBand sets the host's dhchap_key through the API, behind
// Terraform's back.
func setHostKeyOutOfBand(t *testing.T, nqn, key string) {
	t.Helper()
	ctx := context.Background()
	raw, err := acctest.Client().Call(ctx, "nvmet.host.query", [][]any{{"hostnqn", "=", nqn}})
	if err != nil {
		t.Fatalf("nvmet.host.query: %v", err)
	}
	var hosts []struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(raw, &hosts); err != nil || len(hosts) != 1 {
		t.Fatalf("nvmet.host.query %s: %d hosts, err %v", nqn, len(hosts), err)
	}
	if _, err := acctest.Client().Call(ctx, "nvmet.host.update", hosts[0].ID, map[string]any{"dhchap_key": key}); err != nil {
		t.Fatalf("nvmet.host.update: %v", err)
	}
}

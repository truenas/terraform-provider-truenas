// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package webshare_config_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestHuntWebshareConfig_allFields is an adversarial value round-trip for the
// singleton truenas_webshare_config resource. It sets the settable,
// live-safe fields to non-default values at once — search (toggled from its
// current value), passkey (ENABLED/DISABLED, never REQUIRED, which could lock
// users out), and bindip (a real address from webshare.bindip_choices, when
// one other than the current binding exists) — asserts they read back, that
// a re-plan is empty, and that import round-trips.
//
// groups is NOT exercised: it takes AD/LDAP group names that require a
// directory service on the box, so (like the acceptance coverage test) it
// stays unset. The mcp_* fields exist only on TrueNAS 28.0+ and are an
// apply-time error below that; they are added to the config and restored only
// when the box is 28.0 or later.
//
// Singleton, box-wide state: every field this test touches is captured before
// the run and restored from t.Cleanup — search via restoreWebshareConfigSearch
// and the remaining fields via a direct webshare.update — which is the safety
// net even if the Terraform steps fail. Requires TrueNAS 26.0+ and
// TRUENAS_DISRUPTIVE=1.
func TestHuntWebshareConfig_allFields(t *testing.T) {
	acctest.DisruptiveCheck(t)
	if !acctest.ServerVersionAtLeast(t, 26, 0) {
		t.Skip("truenas_webshare_config requires TrueNAS 26.0 or later (webshare namespace absent on 25.10, confirmed live)")
	}

	type cfgShape struct {
		Search           bool     `json:"search"`
		Bindip           []string `json:"bindip"`
		Passkey          string   `json:"passkey"`
		Groups           []string `json:"groups"`
		MCPEnabled       bool     `json:"mcp_enabled"`
		MCPAllowedGroups []string `json:"mcp_allowed_groups"`
		MCPAllowWrite    bool     `json:"mcp_allow_write"`
	}
	raw, err := acctest.RestoreCall(context.Background(), "webshare.config")
	if err != nil {
		t.Skip("webshare.config could not be read on the target box")
	}
	var orig cfgShape
	if err := json.Unmarshal(raw, &orig); err != nil {
		t.Fatalf("error parsing webshare.config response: %v", err)
	}
	if orig.Bindip == nil {
		orig.Bindip = []string{}
	}
	if orig.Groups == nil {
		orig.Groups = []string{}
	}
	if orig.MCPAllowedGroups == nil {
		orig.MCPAllowedGroups = []string{}
	}

	has28 := acctest.ServerVersionAtLeast(t, 28, 0)

	t.Cleanup(func() {
		// Restore search with the dedicated helper, and the rest with a
		// direct partial webshare.update (job:false, accepts partials).
		restoreWebshareConfigSearch(t, orig.Search)
		restore := map[string]any{
			"bindip":  orig.Bindip,
			"passkey": orig.Passkey,
			"groups":  orig.Groups,
		}
		if has28 {
			restore["mcp_enabled"] = orig.MCPEnabled
			restore["mcp_allowed_groups"] = orig.MCPAllowedGroups
			restore["mcp_allow_write"] = orig.MCPAllowWrite
		}
		if _, err := acctest.RestoreCall(context.Background(), "webshare.update", restore); err != nil {
			t.Errorf("error restoring webshare config: %v", err)
		}
	})

	search := !orig.Search

	passkey := "ENABLED"
	if orig.Passkey == "ENABLED" {
		passkey = "DISABLED"
	}

	// Pick a valid bind address different from the current binding, if one is
	// offered by the box. The schema does not validate bindip at plan time;
	// the API does, so only a real choice is safe.
	newIP := ""
	if raw, err := acctest.RestoreCall(context.Background(), "webshare.bindip_choices"); err == nil {
		var choices map[string]string
		if json.Unmarshal(raw, &choices) == nil {
			for k := range choices {
				if k != "" && (len(orig.Bindip) != 1 || orig.Bindip[0] != k) {
					newIP = k
					break
				}
			}
		}
	}

	body := fmt.Sprintf("  search  = %s\n  passkey = %q\n", boolStr(search), passkey)
	checks := []resource.TestCheckFunc{
		resource.TestCheckResourceAttr("truenas_webshare_config.test", "id", "webshare_config"),
		resource.TestCheckResourceAttr("truenas_webshare_config.test", "search", boolStr(search)),
		resource.TestCheckResourceAttr("truenas_webshare_config.test", "passkey", passkey),
	}
	if newIP != "" {
		body += fmt.Sprintf("  bindip  = [%q]\n", newIP)
		checks = append(checks,
			resource.TestCheckResourceAttr("truenas_webshare_config.test", "bindip.#", "1"),
			resource.TestCheckResourceAttr("truenas_webshare_config.test", "bindip.0", newIP),
		)
	}
	if has28 {
		body += "  mcp_enabled        = true\n  mcp_allowed_groups = [\"truenas_webshare\"]\n  mcp_allow_write    = true\n"
		checks = append(checks,
			resource.TestCheckResourceAttr("truenas_webshare_config.test", "mcp_enabled", "true"),
			resource.TestCheckResourceAttr("truenas_webshare_config.test", "mcp_allowed_groups.#", "1"),
			resource.TestCheckResourceAttr("truenas_webshare_config.test", "mcp_allowed_groups.0", "truenas_webshare"),
			resource.TestCheckResourceAttr("truenas_webshare_config.test", "mcp_allow_write", "true"),
		)
	}

	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_webshare_config" "test" {
%s}
`, body)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
			},
			{
				Config:           cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				ResourceName:      "truenas_webshare_config.test",
				ImportState:       true,
				ImportStateId:     "webshare_config",
				ImportStateVerify: true,
			},
		},
	})
}

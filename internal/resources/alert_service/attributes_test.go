// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package alert_service_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func expectEmptyPlan() resource.ConfigPlanChecks {
	return resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}
}

// TestAccAlertService_importPlan: import stored the server document (the
// server adds channel/icon_url to a Mattermost service), so the plan re-sent
// the attributes (B-120).
func TestAccAlertService_importPlan(t *testing.T) {
	name := acctest.RandName("tffix3shtasks-as")
	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_alert_service" "test" {
  name  = %q
  level = "WARNING"
  attributes = jsonencode({
    type     = "Mattermost"
    url      = "https://mm.example.com/hooks/x"
    username = "tffix3"
  })
}
`, name)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAlertServiceDestroyed(name),
		Steps: []resource.TestStep{
			{Config: cfg},
			{Config: cfg, ResourceName: "truenas_alert_service.test", ImportState: true, ImportStateKind: resource.ImportBlockWithID},
		},
	})
}

// TestAccAlertService_removeKey: removing a key from attributes is a change
// (the import-only equivalence must not hide it).
func TestAccAlertService_removeKey(t *testing.T) {
	name := acctest.RandName("tffix3shtasks-as")
	cfg := func(extra string) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_alert_service" "test" {
  name  = %q
  level = "WARNING"
  attributes = jsonencode({
    type     = "Mattermost"
    url      = "https://mm.example.com/hooks/x"
    username = "tffix3"
%s  })
}
`, name, extra)
	}
	with, without := cfg("    channel  = \"tffix3\"\n"), cfg("")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAlertServiceDestroyed(name),
		Steps: []resource.TestStep{
			{Config: with},
			{Config: without, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
				plancheck.ExpectResourceAction("truenas_alert_service.test", plancheck.ResourceActionUpdate),
			}}},
			{Config: without, ConfigPlanChecks: expectEmptyPlan()},
		},
	})
}

// TestAccAlertService_heredocWriteOnly: with the write-only overlay, Read
// rewrote attributes as compact JSON, so a heredoc spelling diffed on every
// plan (B-061).
func TestAccAlertService_heredocWriteOnly(t *testing.T) {
	name := acctest.RandName("tffix3shtasks-as")
	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_alert_service" "test" {
  name  = %q
  level = "WARNING"
  attributes = <<-EOT
    {
      "type": "Mattermost",
      "username": "tffix3"
    }
  EOT
  attributes_secrets_wo         = jsonencode({ url = "https://mm.example.com/hooks/secret" })
  attributes_secrets_wo_version = 1
}
`, name)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAlertServiceDestroyed(name),
		Steps: []resource.TestStep{
			{Config: cfg},
			{Config: cfg, ConfigPlanChecks: expectEmptyPlan()},
		},
	})
}

// TestAccAlertService_readOnlyKey: a READONLY_ADMIN session sees SNMP v3
// keys as "********"; the data source returned the mask as data (B-034), and
// a refresh would plan to overwrite the real keys with it. The object is
// created with the admin key; the read-only step's configuration has only the
// read-only provider (the in-process test provider serves every provider
// block from one instance); the last step switches back so destroy runs as
// admin.
func TestAccAlertService_readOnlyKey(t *testing.T) {
	key, user := os.Getenv("TRUENAS_RO_API_KEY"), os.Getenv("TRUENAS_RO_USERNAME")
	if key == "" || user == "" {
		t.Skip("TRUENAS_RO_API_KEY and TRUENAS_RO_USERNAME (a READONLY_ADMIN API key) are not set")
	}
	ro := fmt.Sprintf(`
provider "truenas" {
  endpoint = %q
  insecure = true
  api_key  = %q
  username = %q
}
`, acctest.Endpoint(), key, user)
	name := acctest.RandName("tffix3shtasks-as")
	res := fmt.Sprintf(`
resource "truenas_alert_service" "test" {
  name    = %q
  level   = "WARNING"
  enabled = false
  attributes = jsonencode({
    type            = "SNMPTrap"
    host            = "192.0.2.10"
    port            = 162
    v3              = true
    v3_username     = "tffix3"
    v3_authkey      = "tffix3-authkey"
    v3_privkey      = "tffix3-privkey"
    v3_authprotocol = "SHA"
    v3_privprotocol = "AESCFB128"
    community       = "public"
  })
}
`, name)
	ds := fmt.Sprintf(`
data "truenas_alert_service" "ro" {
  name = %q
}
`, name)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAlertServiceDestroyed(name),
		Steps: []resource.TestStep{
			{Config: acctest.ProviderConfig() + res},
			{
				Config: ro + res + ds,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("truenas_alert_service.test", plancheck.ResourceActionNoop),
				}},
				Check: resource.ComposeTestCheckFunc(
					noRedactedValue("data.truenas_alert_service.ro"),
					noRedactedValue("truenas_alert_service.test"),
				),
			},
			{Config: acctest.ProviderConfig() + res},
		},
	})
}

func noRedactedValue(addr string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[addr]
		if !ok {
			return fmt.Errorf("%s not in state", addr)
		}
		for k, v := range rs.Primary.Attributes {
			if strings.Contains(v, "********") {
				return fmt.Errorf("%s.%s holds the redaction mask: %q", addr, k, v)
			}
		}
		return nil
	}
}

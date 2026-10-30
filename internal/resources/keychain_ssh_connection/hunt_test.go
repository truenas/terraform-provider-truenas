// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package keychain_ssh_connection_test

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// Cross-provider #57 class: read secret-bearing objects through a
// READONLY_ADMIN API key (TRUENAS_RO_API_KEY / TRUENAS_RO_USERNAME). Each data
// source must either fail with a clear message or return real values — never
// "********" presented as data. The objects are pre-existing on the box,
// named by TRUENAS_RO_<KIND> env vars so each case can be pointed at a live
// object.
func roProvider() string {
	return acctest.ProviderConfig() + fmt.Sprintf(`
provider "truenas" {
  alias    = "ro"
  endpoint = %q
  insecure = true
  api_key  = %q
  username = %q
}
`, acctest.Endpoint(), os.Getenv("TRUENAS_RO_API_KEY"), os.Getenv("TRUENAS_RO_USERNAME"))
}

func noRedaction(addr string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[addr]
		if !ok {
			return fmt.Errorf("%s not in state", addr)
		}
		keys := make([]string, 0, len(rs.Primary.Attributes))
		for k := range rs.Primary.Attributes {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var bad []string
		for _, k := range keys {
			v := rs.Primary.Attributes[k]
			if strings.Contains(v, "********") {
				bad = append(bad, fmt.Sprintf("%s=%q", k, v))
			}
		}
		if len(bad) > 0 {
			return fmt.Errorf("%s returned redacted values as data: %s", addr, strings.Join(bad, ", "))
		}
		return nil
	}
}

func roDS(t *testing.T, dsType, args string) {
	cfg := roProvider() + fmt.Sprintf(`
data %q "x" {
  provider = truenas.ro
%s
}
`, dsType, args)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg, Check: noRedaction("data." + dsType + ".x")},
		},
	})
}

func TestHuntRedaction_systemGeneral(t *testing.T) { roDS(t, "truenas_system_general", "") }
func TestHuntRedaction_truecommand(t *testing.T)   { roDS(t, "truenas_truecommand_config", "") }
func TestHuntRedaction_kerberosKeytab(t *testing.T) {
	roDS(t, "truenas_kerberos_keytab", fmt.Sprintf("name = %q", os.Getenv("TRUENAS_RO_KEYTAB")))
}
func TestHuntRedaction_appRegistry(t *testing.T) {
	roDS(t, "truenas_app_registry", fmt.Sprintf("name = %q", os.Getenv("TRUENAS_RO_REGISTRY")))
}
func TestHuntRedaction_acmeDNS(t *testing.T) {
	roDS(t, "truenas_acme_dns_authenticator", fmt.Sprintf("name = %q", os.Getenv("TRUENAS_RO_ACME")))
}
func TestHuntRedaction_alertService(t *testing.T) {
	roDS(t, "truenas_alert_service", fmt.Sprintf("name = %q", os.Getenv("TRUENAS_RO_ALERT")))
}
func TestHuntRedaction_cloudsyncCreds(t *testing.T) {
	roDS(t, "truenas_cloudsync_credentials", fmt.Sprintf("name = %q", os.Getenv("TRUENAS_RO_CLOUDSYNC")))
}
func TestHuntRedaction_vmDevice(t *testing.T) {
	roDS(t, "truenas_vm_device", fmt.Sprintf("id = %s", os.Getenv("TRUENAS_RO_VMDEVICE")))
}
func TestHuntRedaction_mail(t *testing.T) { roDS(t, "truenas_mail", "") }
func TestHuntRedaction_snmp(t *testing.T) { roDS(t, "truenas_snmp_config", "") }

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package network_config_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccNetworkConfigDataSource_basic reads the current TrueNAS global
// network configuration through the truenas_network_config datasource only.
// It never writes: this configuration controls hostname, DNS, and default
// gateways, and the box's real configuration must not be overwritten by an
// acceptance test run.
//
// Requires:
//   - TF_ACC=1
//   - TRUENAS_API_KEY (or TRUENAS_USERNAME+TRUENAS_PASSWORD) set for
//     acctest.PreCheck
//   - A running TrueNAS instance
func TestAccNetworkConfigDataSource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure a TrueNAS instance is available")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
data "truenas_network_config" "test" {}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_network_config.test", "id", "network_config"),
					resource.TestCheckResourceAttrSet("data.truenas_network_config.test", "hostname"),
				),
			},
		},
	})
}

// TestAccNetworkConfig_basic is intentionally skipped by default.
// truenas_network_config is a SINGLETON resource that controls hostname,
// DNS, and default gateways: the box this suite runs against is reached
// live over that network configuration, and bringing it under Terraform
// management mutates the box's actual global network configuration
// (network.configuration.update). A naive create/update/destroy acceptance
// test risks changing hostname, ipv4gateway, ipv6gateway, or nameserver1-3
// on a live system and cutting off connectivity entirely.
//
// If this test is ever enabled against a disposable/non-production TrueNAS
// instance, it should:
//  1. Read the current config via the datasource first.
//  2. Only touch a low-risk field (e.g. "httpproxy" or "domains") in the
//     resource config, driving it through a value and back to the value the
//     datasource observed in step 1, so the net effect on the box is a
//     no-op. Never touch hostname, ipv4gateway, ipv6gateway, or
//     nameserver1-3 in an automated test: changing those on a live system
//     can cut off network connectivity.
//  3. Use ImportState with ImportStateId "network_config" to verify import
//     normalizes any ID to the fixed singleton ID.
//
// It is gated behind TRUENAS_TEST_NETWORK_CONFIG (not just TF_ACC). It touches
// ONLY `hostname`: updatePayload sends only the fields set in config, so DNS,
// gateways, and domains are never rewritten and connectivity is preserved (a
// hostname change does not drop the box's IP). A t.Cleanup restores the box's
// original hostname regardless of outcome. Run only against a disposable box.
func TestAccNetworkConfig_basic(t *testing.T) {
	if os.Getenv("TRUENAS_TEST_NETWORK_CONFIG") == "" {
		t.Skip("set TRUENAS_TEST_NETWORK_CONFIG=1 (on a disposable box) to run the network_config set/restore test")
	}
	acctest.PreCheck(t)

	orig := currentHostname(t)
	t.Cleanup(func() {
		if _, err := acctest.Client().Call(context.Background(), "network.configuration.update",
			map[string]any{"hostname": orig}); err != nil {
			t.Logf("WARNING: failed to restore hostname=%q: %v", orig, err)
		}
	})

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccNetworkConfigHostname("tftest-nc-a"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_network_config.test", "id", "network_config"),
					resource.TestCheckResourceAttr("truenas_network_config.test", "hostname", "tftest-nc-a"),
				),
			},
			// Update path: change the hostname in place.
			{
				Config: acctest.ProviderConfig() + testAccNetworkConfigHostname("tftest-nc-b"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_network_config.test", "hostname", "tftest-nc-b"),
				),
			},
			{
				ResourceName:      "truenas_network_config.test",
				ImportState:       true,
				ImportStateId:     "network_config",
				ImportStateVerify: true,
			},
		},
	})
}

// netConfigSnapshot is the full network.configuration.config record, captured
// before the all-attributes test mutates anything so t.Cleanup can put every
// writable field back.
type netConfigSnapshot struct {
	Hostname            string   `json:"hostname"`
	Domain              string   `json:"domain"`
	Domains             []string `json:"domains"`
	Hosts               []string `json:"hosts"`
	HTTPProxy           string   `json:"httpproxy"`
	IPv4Gateway         *string  `json:"ipv4gateway"`
	IPv6Gateway         *string  `json:"ipv6gateway"`
	Nameserver1         *string  `json:"nameserver1"`
	Nameserver2         *string  `json:"nameserver2"`
	Nameserver3         *string  `json:"nameserver3"`
	ServiceAnnouncement struct {
		Mdns    bool `json:"mdns"`
		Netbios bool `json:"netbios"`
		Wsd     bool `json:"wsd"`
	} `json:"service_announcement"`
}

func strOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func readNetConfigSnapshot(t *testing.T) netConfigSnapshot {
	t.Helper()
	raw, err := acctest.Client().Call(context.Background(), "network.configuration.config")
	if err != nil {
		t.Fatalf("reading network.configuration.config: %v", err)
	}
	var snap netConfigSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		t.Fatalf("parsing network.configuration.config: %v", err)
	}
	return snap
}

// restore writes every field of the snapshot back with one
// network.configuration.update. Gateways and nameservers go back as "" when
// they were null (the API rejects null).
func (s netConfigSnapshot) restore(t *testing.T) {
	t.Helper()
	domains, hosts := s.Domains, s.Hosts
	if domains == nil {
		domains = []string{}
	}
	if hosts == nil {
		hosts = []string{}
	}
	payload := map[string]any{
		"hostname":    s.Hostname,
		"domain":      s.Domain,
		"domains":     domains,
		"hosts":       hosts,
		"httpproxy":   s.HTTPProxy,
		"ipv4gateway": strOrEmpty(s.IPv4Gateway),
		"ipv6gateway": strOrEmpty(s.IPv6Gateway),
		"nameserver1": strOrEmpty(s.Nameserver1),
		"nameserver2": strOrEmpty(s.Nameserver2),
		"nameserver3": strOrEmpty(s.Nameserver3),
		"service_announcement": map[string]bool{
			"mdns":    s.ServiceAnnouncement.Mdns,
			"netbios": s.ServiceAnnouncement.Netbios,
			"wsd":     s.ServiceAnnouncement.Wsd,
		},
	}
	if _, err := acctest.Client().Call(context.Background(), "network.configuration.update", payload); err != nil {
		t.Errorf("RESTORE FAILED for network.configuration: %v (original: %+v)", err, s)
	}
}

// TestAccNetworkConfig_allAttrs sets every attribute of the singleton
// network configuration, drives an in-place update, and checks a clean
// re-plan.
//
// SAFETY (same gate as TestAccNetworkConfig_basic: TRUENAS_TEST_NETWORK_CONFIG
// on a disposable box):
//   - the whole original config is snapshotted first and written back by
//     t.Cleanup (all fields, one network.configuration.update), pass or fail;
//   - ipv4gateway and ipv6gateway are set to their ORIGINAL values, so the
//     route carrying the API connection never changes (the attribute is still
//     set and round-tripped);
//   - nameserver1 keeps its original value when one exists (a TEST-NET-1
//     address otherwise); nameserver2/3 use TEST-NET-1 addresses (RFC 5737,
//     unroutable). Only DNS lookups on the box are affected during the test,
//     not the IP-addressed API connection;
//   - domain/domains/hosts/httpproxy/service_announcement use reserved
//     example names and TEST-NET addresses.
func TestAccNetworkConfig_allAttrs(t *testing.T) {
	if os.Getenv("TRUENAS_TEST_NETWORK_CONFIG") == "" {
		t.Skip("set TRUENAS_TEST_NETWORK_CONFIG=1 (on a disposable box) to run the network_config set/restore test")
	}
	acctest.PreCheck(t)

	orig := readNetConfigSnapshot(t)
	t.Cleanup(func() { orig.restore(t) })

	ns1 := strOrEmpty(orig.Nameserver1)
	if ns1 == "" {
		ns1 = "192.0.2.53"
	}
	gw4, gw6 := strOrEmpty(orig.IPv4Gateway), strOrEmpty(orig.IPv6Gateway)

	cfg := func(extra string) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_network_config" "test" {
  hostname    = "tftest-nc-all"
  ipv4gateway = %q
  ipv6gateway = %q
  nameserver1 = %q
%s
}
`, gw4, gw6, ns1, extra)
	}

	const r = "truenas_network_config.test"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg(fmt.Sprintf(`
  domain      = "tftest-a.example"
  domains     = ["tftest-a.example"]
  hosts       = ["192.0.2.10 tftest-a-host.example"]
  httpproxy   = "http://192.0.2.80:3128"
  nameserver2 = "192.0.2.54"
  nameserver3 = "192.0.2.55"
  service_announcement = {
    mdns    = %t
    netbios = %t
    wsd     = %t
  }
`, !orig.ServiceAnnouncement.Mdns, !orig.ServiceAnnouncement.Netbios, !orig.ServiceAnnouncement.Wsd)),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(r, "hostname", "tftest-nc-all"),
					resource.TestCheckResourceAttr(r, "domain", "tftest-a.example"),
					resource.TestCheckResourceAttr(r, "domains.#", "1"),
					resource.TestCheckResourceAttr(r, "domains.0", "tftest-a.example"),
					resource.TestCheckResourceAttr(r, "hosts.#", "1"),
					resource.TestCheckResourceAttr(r, "hosts.0", "192.0.2.10 tftest-a-host.example"),
					resource.TestCheckResourceAttr(r, "httpproxy", "http://192.0.2.80:3128"),
					resource.TestCheckResourceAttr(r, "ipv4gateway", gw4),
					resource.TestCheckResourceAttr(r, "ipv6gateway", gw6),
					resource.TestCheckResourceAttr(r, "nameserver1", ns1),
					resource.TestCheckResourceAttr(r, "nameserver2", "192.0.2.54"),
					resource.TestCheckResourceAttr(r, "nameserver3", "192.0.2.55"),
					resource.TestCheckResourceAttr(r, "service_announcement.mdns", fmt.Sprint(!orig.ServiceAnnouncement.Mdns)),
					resource.TestCheckResourceAttr(r, "service_announcement.netbios", fmt.Sprint(!orig.ServiceAnnouncement.Netbios)),
					resource.TestCheckResourceAttr(r, "service_announcement.wsd", fmt.Sprint(!orig.ServiceAnnouncement.Wsd)),
				),
			},
			// Update path: change the list/string fields in place, clear
			// nameserver3 (""), and put service_announcement back to the
			// original toggles.
			{
				Config: cfg(fmt.Sprintf(`
  domain      = "tftest-b.example"
  domains     = ["tftest-b.example", "tftest-c.example"]
  hosts       = ["192.0.2.11 tftest-b-host.example", "192.0.2.12 tftest-c-host.example"]
  httpproxy   = "http://192.0.2.81:3128"
  nameserver2 = "192.0.2.56"
  nameserver3 = ""
  service_announcement = {
    mdns    = %t
    netbios = %t
    wsd     = %t
  }
`, orig.ServiceAnnouncement.Mdns, orig.ServiceAnnouncement.Netbios, orig.ServiceAnnouncement.Wsd)),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(r, "domain", "tftest-b.example"),
					resource.TestCheckResourceAttr(r, "domains.#", "2"),
					resource.TestCheckResourceAttr(r, "domains.1", "tftest-c.example"),
					resource.TestCheckResourceAttr(r, "hosts.#", "2"),
					resource.TestCheckResourceAttr(r, "httpproxy", "http://192.0.2.81:3128"),
					resource.TestCheckResourceAttr(r, "nameserver2", "192.0.2.56"),
					resource.TestCheckResourceAttr(r, "nameserver3", ""),
					resource.TestCheckResourceAttr(r, "service_announcement.mdns", fmt.Sprint(orig.ServiceAnnouncement.Mdns)),
					resource.TestCheckResourceAttr(r, "service_announcement.netbios", fmt.Sprint(orig.ServiceAnnouncement.Netbios)),
					resource.TestCheckResourceAttr(r, "service_announcement.wsd", fmt.Sprint(orig.ServiceAnnouncement.Wsd)),
				),
			},
			{
				// No drift after the update.
				Config:   cfg(""),
				PlanOnly: true,
			},
			{
				ResourceName:      r,
				ImportState:       true,
				ImportStateId:     "network_config",
				ImportStateVerify: true,
			},
		},
	})
}

func testAccNetworkConfigHostname(name string) string {
	return fmt.Sprintf(`
resource "truenas_network_config" "test" {
  hostname = %q
}
`, name)
}

// currentHostname reads the box's current network.configuration hostname.
func currentHostname(t *testing.T) string {
	t.Helper()
	raw, err := acctest.Client().Call(context.Background(), "network.configuration.config")
	if err != nil {
		t.Fatalf("reading network.configuration.config: %v", err)
	}
	var cfg struct {
		Hostname string `json:"hostname"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("parsing network.configuration.config: %v", err)
	}
	return cfg.Hostname
}

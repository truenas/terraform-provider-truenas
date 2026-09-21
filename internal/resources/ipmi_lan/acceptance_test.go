// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package ipmi_lan_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// firstIPMIChannel returns the first channel number reported by
// ipmi.lan.channels, skipping the test if the box reports none (probed
// live on the disposable Enterprise HA test box: always [1], but this
// avoids hardcoding that number).
func firstIPMIChannel(t *testing.T) int64 {
	t.Helper()
	raw, err := acctest.RestoreCall(context.Background(), "ipmi.lan.channels")
	if err != nil {
		t.Fatalf("error reading ipmi.lan.channels: %v", err)
	}
	var channels []int64
	if err := json.Unmarshal(raw, &channels); err != nil {
		t.Fatalf("error parsing ipmi.lan.channels response: %v", err)
	}
	if len(channels) == 0 {
		t.Skip("ipmi.lan.channels reports no IPMI LAN channels on this box")
	}
	return channels[0]
}

// TestAccIPMILanDataSource_basic reads a channel's current LAN
// configuration through the truenas_ipmi_lan datasource only. It never
// writes. Gated by acctest.IPMICheck: requires TRUENAS_IPMI=1 and
// TRUENAS_IPMI_ALLOWED_ENDPOINT matching TRUENAS_ENDPOINT — see
// acctest.IPMICheck's doc comment. IPMI LAN is a plain BMC feature, so this
// no longer requires an Enterprise HA license (any box with a BMC qualifies).
func TestAccIPMILanDataSource_basic(t *testing.T) {
	acctest.IPMICheck(t)
	channel := firstIPMIChannel(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccIPMILanDataSourceConfig(channel),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_ipmi_lan.test", "channel", strconv.FormatInt(channel, 10)),
					resource.TestCheckResourceAttrSet("data.truenas_ipmi_lan.test", "id"),
					resource.TestCheckResourceAttrSet("data.truenas_ipmi_lan.test", "dhcp"),
					resource.TestCheckResourceAttrSet("data.truenas_ipmi_lan.test", "ipaddress"),
					resource.TestCheckResourceAttrSet("data.truenas_ipmi_lan.test", "netmask"),
					resource.TestCheckResourceAttrSet("data.truenas_ipmi_lan.test", "gateway"),
					resource.TestCheckResourceAttrSet("data.truenas_ipmi_lan.test", "mac_address"),
				),
			},
		},
	})
}

func testAccIPMILanDataSourceConfig(channel int64) string {
	return fmt.Sprintf(`
data "truenas_ipmi_lan" "test" {
  channel = %d
}
`, channel)
}

// ipmiLanOriginal captures the fields TestAccIPMILan_setAndRestoreVlan needs
// to fully restore a channel's config, as read from ipmi.lan.query before
// the test runs. Field names mirror the API's own query response shape
// (see model.go's ipmiLanAPI doc comment) rather than the update shape,
// since this is populated straight from ipmi.lan.query.
type ipmiLanOriginal struct {
	IPAddressSource         string `json:"ip_address_source"`
	IPAddress               string `json:"ip_address"`
	SubnetMask              string `json:"subnet_mask"`
	DefaultGatewayIPAddress string `json:"default_gateway_ip_address"`
	VlanID                  *int64 `json:"vlan_id"`
}

// readIPMILanOriginal reads a channel's current ipmi.lan.query record via
// acctest.RestoreCall, so the test can restore its exact original vlan
// (and, for a static channel, ip/netmask/gateway) afterward.
func readIPMILanOriginal(t *testing.T, channel int64) ipmiLanOriginal {
	t.Helper()
	raw, err := acctest.RestoreCall(context.Background(), "ipmi.lan.query", map[string]any{
		"query-filters": [][]any{{"channel", "=", channel}},
	})
	if err != nil {
		t.Fatalf("error reading ipmi.lan.query for channel %d: %v", channel, err)
	}
	var results []ipmiLanOriginal
	if err := json.Unmarshal(raw, &results); err != nil {
		t.Fatalf("error parsing ipmi.lan.query response: %v", err)
	}
	if len(results) == 0 {
		t.Fatalf("ipmi.lan.query returned no result for channel %d", channel)
	}
	return results[0]
}

// restoreIPMILanVlan sends orig's dhcp/ip/netmask/gateway/vlan back to the
// channel via ipmi.lan.update, unconditionally: this is the ONLY path that
// can restore "vlan" all the way back to JSON null (a Terraform HCL step
// cannot distinguish an explicitly-null attribute from an omitted one, so
// the config-driven resource itself can never send an explicit null — see
// model.go's updatePayload doc comment), which is exactly the state
// TestAccIPMILan_setAndRestoreVlan's channel was in before the test ran.
// Registered via t.Cleanup BEFORE the test's own mutating Terraform apply,
// per this task's safety requirement: the BMC LAN channel must never be left
// in a different state than it started in.
//
// It issues the restoring update EXACTLY ONCE, then POLLS ipmi.lan.query
// read-only until ip_address/subnet_mask actually match orig. Re-sending
// ipmi.lan.update on every poll iteration (an earlier version of this helper
// did) restarts the BMC LAN controller's settle-time window each time and can
// keep it from ever converging: live-traced against .68 channel 1, a static
// ipmi.lan.update is followed by ~10-15s of the controller FLAPPING
// ip_address/subnet_mask through "0.0.0.0" (under both a "static" and an
// "unspecified" ip_address_source) before it stabilizes to the values just
// sent — see resource.go's settlePollInterval / transientZeroRead doc
// comments for the same behavior the resource's own reads wait out. Sending
// once and polling lets that single settle window run to completion. The poll
// budget is deliberately generous (75s) because a full test run hammers the
// channel with several updates before this cleanup runs, which can lengthen
// the final settle. Only a genuinely unresolved mismatch after the whole
// budget is a t.Fatal.
func restoreIPMILanVlan(t *testing.T, channel int64, orig ipmiLanOriginal) {
	t.Helper()
	dhcp := isDHCPSource(orig.IPAddressSource)
	payload := map[string]any{"dhcp": dhcp}
	if !dhcp {
		payload["ipaddress"] = orig.IPAddress
		payload["netmask"] = orig.SubnetMask
		payload["gateway"] = orig.DefaultGatewayIPAddress
	}
	if orig.VlanID != nil {
		payload["vlan"] = *orig.VlanID
	} else {
		payload["vlan"] = nil
	}

	if _, err := acctest.RestoreCall(context.Background(), "ipmi.lan.update", channel, payload); err != nil {
		t.Fatalf("RESTORE FAILED for IPMI LAN channel %d: ipmi.lan.update: %v", channel, err)
	}
	// A DHCP channel has no deterministic address to converge to (the BMC
	// takes whatever its DHCP server hands out), so there is nothing to poll
	// for — the single update above is the whole restore.
	if dhcp {
		return
	}

	const (
		attempts = 25
		interval = 3 * time.Second
	)
	var last ipmiLanOriginal
	for attempt := 1; attempt <= attempts; attempt++ {
		time.Sleep(interval)
		last = readIPMILanOriginal(t, channel)
		if last.IPAddress == orig.IPAddress && last.SubnetMask == orig.SubnetMask {
			return
		}
	}
	t.Fatalf("RESTORE FAILED for IPMI LAN channel %d after %d polls (~%ds): ip_address/subnet_mask not settled to original (got %q/%q, want %q/%q)",
		channel, attempts, attempts*int(interval/time.Second), last.IPAddress, last.SubnetMask, orig.IPAddress, orig.SubnetMask)
}

func isDHCPSource(ipAddressSource string) bool {
	return ipAddressSource == "DHCP" || ipAddressSource == "dhcp"
}

// TestAccIPMILan_setAndRestoreVlan drives the truenas_ipmi_lan resource's
// "vlan" field through a different value, verifies it round-trips via
// Terraform (apply + import), then restores the channel's ENTIRE original
// config (dhcp/ip/netmask/gateway/vlan, not just vlan) via a direct API
// call registered in t.Cleanup before the mutating apply runs.
//
// "vlan" is the field this task's probe judged safest to exercise live:
// dhcp/ipaddress/netmask/gateway are left completely unchanged throughout
// (the static config values captured from the box itself are resent
// verbatim on every step, whether dhcp is true or false), so this test
// never risks changing how the BMC obtains or is reached at its own IP
// address — only whether 802.1Q tagging is applied to its LAN traffic.
// That still carries real risk if the physical switch port doesn't trunk
// the test VLAN (see schema.go's resourceSchema Description SAFETY note),
// but this provider's own connection to TrueNAS runs over the host's
// separate management network, not the BMC's, so ipmi.lan.update itself
// remains callable to correct it regardless — which is exactly what the
// t.Cleanup restore does, unconditionally, the moment this test ends.
//
// Gated by acctest.IPMICheck (TRUENAS_IPMI=1, TRUENAS_IPMI_ALLOWED_ENDPOINT
// endpoint guard) AND acctest.DisruptiveCheck (TRUENAS_DISRUPTIVE=1). No
// Enterprise HA license is required — IPMI LAN is a plain BMC feature.
func TestAccIPMILan_setAndRestoreVlan(t *testing.T) {
	acctest.IPMICheck(t)
	acctest.DisruptiveCheck(t)

	channel := firstIPMIChannel(t)
	orig := readIPMILanOriginal(t, channel)

	// Registered BEFORE any mutating call below, per this task's safety
	// requirement.
	t.Cleanup(func() { restoreIPMILanVlan(t, channel, orig) })

	toggled := int64(100)
	if orig.VlanID != nil {
		toggled = *orig.VlanID + 1
		if toggled > 4096 {
			toggled = 1
		}
	}

	dhcp := isDHCPSource(orig.IPAddressSource)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccIPMILanConfig(channel, dhcp, orig.IPAddress, orig.SubnetMask, orig.DefaultGatewayIPAddress, toggled),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_ipmi_lan.test", "channel", strconv.FormatInt(channel, 10)),
					resource.TestCheckResourceAttr("truenas_ipmi_lan.test", "vlan", strconv.FormatInt(toggled, 10)),
				),
			},
			{
				ResourceName:            "truenas_ipmi_lan.test",
				ImportState:             true,
				ImportStateId:           strconv.FormatInt(channel, 10),
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"password", "apply_remote"},
			},
		},
	})
}

func testAccIPMILanConfig(channel int64, dhcp bool, ipaddress, netmask, gateway string, vlan int64) string {
	if dhcp {
		return fmt.Sprintf(`
resource "truenas_ipmi_lan" "test" {
  channel = %d
  dhcp    = true
  vlan    = %d
}
`, channel, vlan)
	}
	return fmt.Sprintf(`
resource "truenas_ipmi_lan" "test" {
  channel   = %d
  dhcp      = false
  ipaddress = %q
  netmask   = %q
  gateway   = %q
  vlan      = %d
}
`, channel, ipaddress, netmask, gateway, vlan)
}

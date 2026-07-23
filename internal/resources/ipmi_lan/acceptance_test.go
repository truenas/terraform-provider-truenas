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
// writes. Gated by acctest.HACheck: requires TRUENAS_HA=1,
// TRUENAS_HA_ALLOWED_ENDPOINT matching TRUENAS_ENDPOINT, and a live
// failover.licensed probe — see acctest.HACheck's doc comment.
func TestAccIPMILanDataSource_basic(t *testing.T) {
	acctest.HACheck(t)
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
// TestAccIPMILan_setAndRestoreVlan's channel was in before the test ran on
// the disposable Enterprise HA box this was developed against. Registered
// via t.Cleanup BEFORE the test's own mutating Terraform apply, per this
// task's safety requirement: the BMC LAN channel must never be left in a
// different state than it started in.
//
// It then POLLS ipmi.lan.query (mirroring resource.go's
// lookupChannelSettled) until ip_address/subnet_mask actually match orig,
// rather than trusting a single successful ipmi.lan.update call:
// PROBED LIVE during this task's development, a static-IP ipmi.lan.update
// call that itself succeeded was followed by ip_address/subnet_mask
// transiently reading back "0.0.0.0"/"0.0.0.0" — real BMC LAN controller
// settle-time behavior after a config change, not an error — before
// stabilizing back to the values just sent moments later. Only a genuinely
// unresolved mismatch after the full retry budget is a t.Fatal.
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

	const (
		attempts = 8
		interval = 3 * time.Second
	)
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if _, err := acctest.RestoreCall(context.Background(), "ipmi.lan.update", channel, payload); err != nil {
			lastErr = err
			time.Sleep(interval)
			continue
		}
		time.Sleep(interval)

		cur := readIPMILanOriginal(t, channel)
		if dhcp || (cur.IPAddress == orig.IPAddress && cur.SubnetMask == orig.SubnetMask) {
			return
		}
		lastErr = fmt.Errorf("ip_address/subnet_mask not yet settled to original (got %q/%q, want %q/%q)",
			cur.IPAddress, cur.SubnetMask, orig.IPAddress, orig.SubnetMask)
	}
	t.Fatalf("RESTORE FAILED for IPMI LAN channel %d after %d attempts (BMC LAN config may not match its original state): %v",
		channel, attempts, lastErr)
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
// Gated by acctest.HACheck (TRUENAS_HA=1, endpoint guard, live
// failover.licensed probe) AND acctest.DisruptiveCheck (TRUENAS_DISRUPTIVE=1).
func TestAccIPMILan_setAndRestoreVlan(t *testing.T) {
	acctest.HACheck(t)
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

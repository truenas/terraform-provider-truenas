// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_port_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccNVMetPort_basic tests create, update, and import of an NVMe-oF
// port. It creates and destroys its own port on trsvcid 14421 (distinct from
// both the box's LIVE port, id=1 TCP 4420, and the NVMe-oF end-to-end test's
// port on 14420) and never touches the box's existing configuration, which
// serves live storage.
func TestAccNVMetPort_basic(t *testing.T) {
	const trsvcid = 14421

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckNVMetPortDestroyed,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccNVMetPortConfig(trsvcid, false, 0),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_nvmet_port.test", "addr_trtype", "TCP"),
					resource.TestCheckResourceAttr("truenas_nvmet_port.test", "addr_traddr", acctest.EndpointHost()),
					resource.TestCheckResourceAttr("truenas_nvmet_port.test", "addr_trsvcid", fmt.Sprintf("%d", trsvcid)),
					resource.TestCheckResourceAttr("truenas_nvmet_port.test", "enabled", "false"),
					resource.TestCheckResourceAttrSet("truenas_nvmet_port.test", "id"),
					resource.TestCheckResourceAttrSet("truenas_nvmet_port.test", "index"),
					resource.TestCheckResourceAttrSet("truenas_nvmet_port.test", "addr_adrfam"),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccNVMetPortConfig(trsvcid, false, 128),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_nvmet_port.test", "max_queue_size", "128"),
				),
			},
			{
				ResourceName:      "truenas_nvmet_port.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccNVMetPort_dataProtection sets inline_data_size and pi_enable on a
// TCP port (distinct trsvcid 14422) and asserts they read back and import.
// NOTE: inline_data_size / pi_enable may be rejected by the middleware for the
// TCP transport on some releases; a failure here is a finding, not a test bug.
func TestAccNVMetPort_dataProtection(t *testing.T) {
	const trsvcid = 14422
	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_nvmet_port" "dp" {
  addr_trtype      = "TCP"
  addr_traddr      = %q
  addr_trsvcid     = %d
  enabled          = false
  inline_data_size = 8192
  pi_enable        = true
}
`, acctest.EndpointHost(), trsvcid)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckNVMetPortDestroyedByName("truenas_nvmet_port.dp"),
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_nvmet_port.dp", "inline_data_size", "8192"),
					resource.TestCheckResourceAttr("truenas_nvmet_port.dp", "pi_enable", "true"),
				),
			},
			{
				ResourceName:      "truenas_nvmet_port.dp",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccNVMetPortConfig(trsvcid int, enabled bool, maxQueueSize int) string {
	if maxQueueSize == 0 {
		return fmt.Sprintf(`
resource "truenas_nvmet_port" "test" {
  addr_trtype  = "TCP"
  addr_traddr  = %q
  addr_trsvcid = %d
  enabled      = %v
}
`, acctest.EndpointHost(), trsvcid, enabled)
	}
	return fmt.Sprintf(`
resource "truenas_nvmet_port" "test" {
  addr_trtype    = "TCP"
  addr_traddr    = %q
  addr_trsvcid   = %d
  enabled        = %v
  max_queue_size = %d
}
`, acctest.EndpointHost(), trsvcid, enabled, maxQueueSize)
}

func testAccCheckNVMetPortDestroyed(s *terraform.State) error {
	return checkNVMetPortGone(s, "truenas_nvmet_port.test")
}

func testAccCheckNVMetPortDestroyedByName(addr string) resource.TestCheckFunc {
	return func(s *terraform.State) error { return checkNVMetPortGone(s, addr) }
}

func checkNVMetPortGone(s *terraform.State, addr string) error {
	rs, ok := s.RootModule().Resources[addr]
	if !ok {
		return fmt.Errorf("resource %s not found in pre-destroy state", addr)
	}
	idStr, ok := rs.Primary.Attributes["id"]
	if !ok {
		return fmt.Errorf("%s has no id attribute in pre-destroy state", addr)
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return fmt.Errorf("parsing nvmet port id %q: %v", idStr, err)
	}

	c := acctest.Client()
	raw, err := c.Call(context.Background(), "nvmet.port.query", [][]any{{"id", "=", id}})
	if err != nil {
		return fmt.Errorf("error checking nvmet port id=%d: %v", id, err)
	}
	var results []struct {
		ID any `json:"id"`
	}
	if err := json.Unmarshal(raw, &results); err != nil {
		return fmt.Errorf("error parsing nvmet.port.query response: %v", err)
	}
	if len(results) > 0 {
		return fmt.Errorf("nvmet port id=%d still exists", id)
	}
	return nil
}

// TestAccNVMetPort_addrFamilyChange is the live regression test for the
// addr_adrfam stale-computed-value bug: addr_adrfam is derived by the server
// from addr_traddr, so updating addr_traddr from an IPv4 to an IPv6 address
// must flip addr_adrfam IPV4 -> IPV6 without a "Provider produced inconsistent
// result after apply" error. With the schema's UseStateForUnknown on
// addr_adrfam (the pre-fix state), the plan would hold the stale IPV4 and this
// test fails on the update step; without it (the fix) the server recomputes it.
func TestAccNVMetPort_addrFamilyChange(t *testing.T) {
	cfg := func(addr string) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_nvmet_port" "test" {
  addr_trtype  = "TCP"
  addr_traddr  = %q
  addr_trsvcid = 14499
  enabled      = false
}
`, addr)
	}
	const r = "truenas_nvmet_port.test"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckNVMetPortDestroyed,
		Steps: []resource.TestStep{
			{
				// IPv4 listen address -> server derives addr_adrfam = IPV4.
				Config: cfg(acctest.EndpointHost()),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(r, "addr_adrfam", "IPV4"),
				),
			},
			{
				// Flip addr_traddr to IPv6 (loopback is a valid listen address);
				// the server must recompute addr_adrfam = IPV6 with no
				// inconsistent-result error on apply.
				Config: cfg("::1"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(r, "addr_traddr", "::1"),
					resource.TestCheckResourceAttr(r, "addr_adrfam", "IPV6"),
				),
			},
		},
	})
}

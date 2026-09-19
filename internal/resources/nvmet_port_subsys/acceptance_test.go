// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_port_subsys_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

// TestAccNVMeTEndToEnd wires up a full, self-contained NVMe-oF configuration
// (subsystem, port, a zvol fixture, a namespace backed by that zvol, a host,
// a host/subsystem association, and a port/subsystem association) entirely
// from own-created tf-acc objects.
//
// Safety: this test never references the box's LIVE NVMe-oF configuration
// (subsys id=1 "proxmox-test", port id=1 TCP 4420, namespace id=1,
// port_subsys id=1). The test port listens on the box IP at :14420 (distinct
// from the live port's service ID) and is created with enabled=false so it
// never opens a live listener.
func TestAccNVMeTEndToEnd(t *testing.T) {
	subsysName := acctest.RandName("tf-acc-nvmet-subsys")
	zvolName := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-nvmet-zvol"))
	devicePath := fmt.Sprintf("zvol/%s", zvolName)
	hostNQN := acctest.RandNQN()

	// The host description field exists on the wire only from TrueNAS 26.0;
	// on older releases the test omits it.
	desc, descUpdated := "", ""
	if acctest.ServerVersionAtLeast(t, 26, 0) {
		desc, descUpdated = "initial host description", "updated host description"
	}

	step1Checks := []resource.TestCheckFunc{
		// Host
		resource.TestCheckResourceAttrSet("truenas_nvmet_host.test", "id"),
		resource.TestCheckResourceAttr("truenas_nvmet_host.test", "hostnqn", hostNQN),
	}
	step2Checks := []resource.TestCheckFunc{
		resource.TestCheckResourceAttr("truenas_nvmet_namespace.test", "enabled", "false"),
	}
	if desc != "" {
		step1Checks = append(step1Checks,
			resource.TestCheckResourceAttr("truenas_nvmet_host.test", "description", desc))
		step2Checks = append(step2Checks,
			resource.TestCheckResourceAttr("truenas_nvmet_host.test", "description", descUpdated))
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckNVMeTEndToEndDestroyed(subsysName, devicePath, hostNQN, zvolName),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccNVMeTEndToEndConfig(
					subsysName, zvolName, hostNQN, true, desc,
				),
				Check: resource.ComposeTestCheckFunc(
					// Subsys
					resource.TestCheckResourceAttrSet("truenas_nvmet_subsys.test", "id"),
					resource.TestCheckResourceAttr("truenas_nvmet_subsys.test", "name", subsysName),
					resource.TestCheckResourceAttr("truenas_nvmet_subsys.test", "allow_any_host", "false"),
					resource.TestCheckResourceAttrSet("truenas_nvmet_subsys.test", "serial"),

					// Port
					resource.TestCheckResourceAttrSet("truenas_nvmet_port.test", "id"),
					resource.TestCheckResourceAttr("truenas_nvmet_port.test", "addr_trtype", "TCP"),
					resource.TestCheckResourceAttr("truenas_nvmet_port.test", "addr_traddr", acctest.EndpointHost()),
					resource.TestCheckResourceAttr("truenas_nvmet_port.test", "addr_trsvcid", "14420"),
					resource.TestCheckResourceAttr("truenas_nvmet_port.test", "enabled", "false"),

					// Zvol fixture
					resource.TestCheckResourceAttrSet("truenas_zvol.test", "id"),
					resource.TestCheckResourceAttr("truenas_zvol.test", "name", zvolName),

					// Namespace
					resource.TestCheckResourceAttrSet("truenas_nvmet_namespace.test", "id"),
					resource.TestCheckResourceAttrSet("truenas_nvmet_namespace.test", "nsid"),
					resource.TestCheckResourceAttr("truenas_nvmet_namespace.test", "device_path", devicePath),
					resource.TestCheckResourceAttr("truenas_nvmet_namespace.test", "device_type", "ZVOL"),
					resource.TestCheckResourceAttr("truenas_nvmet_namespace.test", "enabled", "true"),
					resource.TestCheckResourceAttrPair("truenas_nvmet_namespace.test", "subsys_id", "truenas_nvmet_subsys.test", "id"),

					// Host (description checks are in step1Checks: the field
					// is 26.0+ only)
					resource.ComposeTestCheckFunc(step1Checks...),

					// Host/subsys association
					resource.TestCheckResourceAttrSet("truenas_nvmet_host_subsys.test", "id"),
					resource.TestCheckResourceAttrPair("truenas_nvmet_host_subsys.test", "host_id", "truenas_nvmet_host.test", "id"),
					resource.TestCheckResourceAttrPair("truenas_nvmet_host_subsys.test", "subsys_id", "truenas_nvmet_subsys.test", "id"),

					// Port/subsys association
					resource.TestCheckResourceAttrSet("truenas_nvmet_port_subsys.test", "id"),
					resource.TestCheckResourceAttrPair("truenas_nvmet_port_subsys.test", "port_id", "truenas_nvmet_port.test", "id"),
					resource.TestCheckResourceAttrPair("truenas_nvmet_port_subsys.test", "subsys_id", "truenas_nvmet_subsys.test", "id"),
				),
			},
			// Update: change the host description and disable the namespace in
			// place. subsys.allow_any_host is deliberately left untouched here
			// since toggling it would conflict with the explicit host_subsys
			// grant created above.
			{
				Config: acctest.ProviderConfig() + testAccNVMeTEndToEndConfig(
					subsysName, zvolName, hostNQN, false, descUpdated,
				),
				Check: resource.ComposeTestCheckFunc(step2Checks...),
			},
			{
				ResourceName:      "truenas_nvmet_port_subsys.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:      "truenas_nvmet_host_subsys.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:      "truenas_nvmet_namespace.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccNVMeTEndToEndConfig(subsysName, zvolName, hostNQN string, namespaceEnabled bool, hostDescription string) string {
	descLine := ""
	if hostDescription != "" {
		descLine = fmt.Sprintf("  description = %q\n", hostDescription)
	}
	return fmt.Sprintf(`
resource "truenas_nvmet_subsys" "test" {
  name           = %q
  allow_any_host = false
}

resource "truenas_nvmet_port" "test" {
  addr_trtype  = "TCP"
  addr_traddr  = %q
  addr_trsvcid = 14420
  enabled      = false
}

resource "truenas_zvol" "test" {
  name    = %q
  volsize = 67108864
}

resource "truenas_nvmet_namespace" "test" {
  subsys_id   = truenas_nvmet_subsys.test.id
  device_path = "zvol/${truenas_zvol.test.name}"
  device_type = "ZVOL"
  enabled     = %v
}

resource "truenas_nvmet_host" "test" {
  hostnqn = %q
%s}

resource "truenas_nvmet_host_subsys" "test" {
  host_id   = truenas_nvmet_host.test.id
  subsys_id = truenas_nvmet_subsys.test.id
}

resource "truenas_nvmet_port_subsys" "test" {
  port_id   = truenas_nvmet_port.test.id
  subsys_id = truenas_nvmet_subsys.test.id
}
`, subsysName, acctest.EndpointHost(), zvolName, namespaceEnabled, hostNQN, descLine)
}

// idResults is the shape returned by every TrueNAS <namespace>.query method
// this test cares about: a list of objects each carrying at least an "id".
type idResults []struct {
	ID any `json:"id"`
}

// checkQueryEmpty calls a TrueNAS <namespace>.query method with the given
// filters and fails if any results are returned.
func checkQueryEmpty(ctx context.Context, c *client.Client, method string, filters [][]any, describe string) error {
	raw, err := c.Call(ctx, method, filters)
	if err != nil {
		return fmt.Errorf("error checking %s: %v", describe, err)
	}
	var results idResults
	if err := json.Unmarshal(raw, &results); err != nil {
		return fmt.Errorf("error parsing %s response: %v", method, err)
	}
	if len(results) > 0 {
		return fmt.Errorf("%s still exists", describe)
	}
	return nil
}

// stateAttr reads a primary instance attribute for a resource address from
// the (pre-destroy) terraform state.
func stateAttr(s *terraform.State, address, attr string) (string, error) {
	rs, ok := s.RootModule().Resources[address]
	if !ok {
		return "", fmt.Errorf("resource %s not found in state", address)
	}
	v, ok := rs.Primary.Attributes[attr]
	if !ok {
		return "", fmt.Errorf("resource %s has no attribute %q in state", address, attr)
	}
	return v, nil
}

func testAccCheckNVMeTEndToEndDestroyed(subsysName, devicePath, hostNQN, zvolName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		ctx := context.Background()

		if err := checkQueryEmpty(ctx, c, "nvmet.subsys.query", [][]any{{"name", "=", subsysName}}, "nvmet subsys "+subsysName); err != nil {
			return err
		}
		if err := checkQueryEmpty(ctx, c, "nvmet.port.query",
			[][]any{{"addr_traddr", "=", acctest.EndpointHost()}, {"addr_trsvcid", "=", 14420}},
			"nvmet port <endpoint-host>:14420",
		); err != nil {
			return err
		}
		if err := checkQueryEmpty(ctx, c, "nvmet.namespace.query", [][]any{{"device_path", "=", devicePath}}, "nvmet namespace "+devicePath); err != nil {
			return err
		}
		if err := checkQueryEmpty(ctx, c, "nvmet.host.query", [][]any{{"hostnqn", "=", hostNQN}}, "nvmet host "+hostNQN); err != nil {
			return err
		}
		if err := checkQueryEmpty(ctx, c, "pool.dataset.query", [][]any{{"id", "=", zvolName}}, "zvol "+zvolName); err != nil {
			return err
		}

		// The host_subsys and port_subsys associations are keyed by the
		// (now-deleted) numeric host/subsys/port IDs, which are still
		// readable from the pre-destroy state captured in s.
		hostIDStr, err := stateAttr(s, "truenas_nvmet_host.test", "id")
		if err != nil {
			return err
		}
		subsysIDStr, err := stateAttr(s, "truenas_nvmet_subsys.test", "id")
		if err != nil {
			return err
		}
		portIDStr, err := stateAttr(s, "truenas_nvmet_port.test", "id")
		if err != nil {
			return err
		}
		hostID, err := strconv.ParseInt(hostIDStr, 10, 64)
		if err != nil {
			return fmt.Errorf("parsing host id %q: %v", hostIDStr, err)
		}
		subsysID, err := strconv.ParseInt(subsysIDStr, 10, 64)
		if err != nil {
			return fmt.Errorf("parsing subsys id %q: %v", subsysIDStr, err)
		}
		portID, err := strconv.ParseInt(portIDStr, 10, 64)
		if err != nil {
			return fmt.Errorf("parsing port id %q: %v", portIDStr, err)
		}
		if err := checkQueryEmpty(ctx, c, "nvmet.host_subsys.query",
			[][]any{{"host_id", "=", hostID}, {"subsys_id", "=", subsysID}},
			fmt.Sprintf("host_subsys (host_id=%d subsys_id=%d)", hostID, subsysID),
		); err != nil {
			return err
		}
		if err := checkQueryEmpty(ctx, c, "nvmet.port_subsys.query",
			[][]any{{"port_id", "=", portID}, {"subsys_id", "=", subsysID}},
			fmt.Sprintf("port_subsys (port_id=%d subsys_id=%d)", portID, subsysID),
		); err != nil {
			return err
		}

		return nil
	}
}

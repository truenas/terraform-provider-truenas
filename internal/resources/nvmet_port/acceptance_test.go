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
	rs, ok := s.RootModule().Resources["truenas_nvmet_port.test"]
	if !ok {
		return fmt.Errorf("resource truenas_nvmet_port.test not found in pre-destroy state")
	}
	idStr, ok := rs.Primary.Attributes["id"]
	if !ok {
		return fmt.Errorf("truenas_nvmet_port.test has no id attribute in pre-destroy state")
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

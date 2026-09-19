// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_host_subsys_test

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

// TestAccNVMetHostSubsys_basic tests create and import of an NVMe-oF
// host/subsystem association. It creates and destroys its own RandName
// host and subsystem. It never touches the box's existing LIVE host_subsys
// association (id=1), which serves live storage. Both host_id and subsys_id
// are RequiresReplace, so there is no in-place update to exercise here.
func TestAccNVMetHostSubsys_basic(t *testing.T) {
	hostNQN := acctest.RandNQN()
	subsysName := acctest.RandName("tf-acc-host-subsys")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckNVMetHostSubsysDestroyed,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccNVMetHostSubsysConfig(hostNQN, subsysName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_nvmet_host_subsys.test", "id"),
					resource.TestCheckResourceAttrPair("truenas_nvmet_host_subsys.test", "host_id", "truenas_nvmet_host.test", "id"),
					resource.TestCheckResourceAttrPair("truenas_nvmet_host_subsys.test", "subsys_id", "truenas_nvmet_subsys.test", "id"),
				),
			},
			{
				ResourceName:      "truenas_nvmet_host_subsys.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccNVMetHostSubsysConfig(hostNQN, subsysName string) string {
	return fmt.Sprintf(`
resource "truenas_nvmet_host" "test" {
  hostnqn = %q
}

resource "truenas_nvmet_subsys" "test" {
  name = %q
}

resource "truenas_nvmet_host_subsys" "test" {
  host_id   = truenas_nvmet_host.test.id
  subsys_id = truenas_nvmet_subsys.test.id
}
`, hostNQN, subsysName)
}

func testAccCheckNVMetHostSubsysDestroyed(s *terraform.State) error {
	rs, ok := s.RootModule().Resources["truenas_nvmet_host_subsys.test"]
	if !ok {
		return fmt.Errorf("resource truenas_nvmet_host_subsys.test not found in pre-destroy state")
	}
	idStr, ok := rs.Primary.Attributes["id"]
	if !ok {
		return fmt.Errorf("truenas_nvmet_host_subsys.test has no id attribute in pre-destroy state")
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return fmt.Errorf("parsing host_subsys id %q: %v", idStr, err)
	}

	c := acctest.Client()
	raw, err := c.Call(context.Background(), "nvmet.host_subsys.query", [][]any{{"id", "=", id}})
	if err != nil {
		return fmt.Errorf("error checking nvmet host_subsys id=%d: %v", id, err)
	}
	var results []struct {
		ID any `json:"id"`
	}
	if err := json.Unmarshal(raw, &results); err != nil {
		return fmt.Errorf("error parsing nvmet.host_subsys.query response: %v", err)
	}
	if len(results) > 0 {
		return fmt.Errorf("nvmet host_subsys id=%d still exists", id)
	}
	return nil
}

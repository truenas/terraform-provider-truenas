// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_initiator_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccISCSIInitiator_basic tests create, update, and import of an iSCSI
// initiator group.
func TestAccISCSIInitiator_basic(t *testing.T) {
	comment := acctest.RandName("tf-acc-initiator")
	iqn := fmt.Sprintf("iqn.2023-01.com.example:%s", acctest.RandName("host"))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckISCSIInitiatorDestroyed(),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccISCSIInitiatorConfig(comment, []string{}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_iscsi_initiator.test", "comment", comment),
					resource.TestCheckResourceAttr("truenas_iscsi_initiator.test", "initiators.#", "0"),
					resource.TestCheckResourceAttrSet("truenas_iscsi_initiator.test", "id"),
				),
			},
			// Update the comment and initiators list in place.
			{
				Config: acctest.ProviderConfig() + testAccISCSIInitiatorConfig(comment+"-updated", []string{iqn}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_iscsi_initiator.test", "comment", comment+"-updated"),
					resource.TestCheckResourceAttr("truenas_iscsi_initiator.test", "initiators.0", iqn),
				),
			},
			{
				ResourceName:      "truenas_iscsi_initiator.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccISCSIInitiatorConfig(comment string, initiators []string) string {
	initsStr := "[]"
	if len(initiators) > 0 {
		initsStr = fmt.Sprintf(`["%s"]`, initiators[0])
		for _, iqn := range initiators[1:] {
			initsStr = fmt.Sprintf(`%s, "%s"`, initsStr, iqn)
		}
	}
	return fmt.Sprintf(`
resource "truenas_iscsi_initiator" "test" {
  comment    = %q
  initiators = %s
}
`, comment, initsStr)
}

// testAccCheckISCSIInitiatorDestroyed verifies the initiator group was
// removed after the test finishes. The initiator group has no unique
// name/comment guaranteed by the API, so the numeric id is captured from
// the pre-destroy terraform state instead.
func testAccCheckISCSIInitiatorDestroyed() resource.TestCheckFunc {
	return func(s *terraform.State) error {
		var id string
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "truenas_iscsi_initiator" {
				continue
			}
			id = rs.Primary.ID
		}
		if id == "" {
			return fmt.Errorf("no truenas_iscsi_initiator resource found in state")
		}

		c := acctest.Client()
		raw, err := c.Call(context.Background(), "iscsi.initiator.query", [][]any{{"id", "=", jsonID(id)}})
		if err != nil {
			return fmt.Errorf("error checking iscsi initiator %s: %v", id, err)
		}
		var results []struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing iscsi.initiator.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("iscsi initiator %s still exists", id)
		}
		return nil
	}
}

// jsonID converts a decimal string id from state into an int64 for use in a
// query filter.
func jsonID(s string) int64 {
	var n int64
	_, _ = fmt.Sscanf(s, "%d", &n)
	return n
}

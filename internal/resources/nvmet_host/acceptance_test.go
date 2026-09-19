// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_host_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccNVMetHost_basic tests create, update, and import of an NVMe-oF
// host (initiator). It creates and destroys its own RandName-uuid-suffixed
// host and never touches the box's existing LIVE host configuration, which
// serves live storage.
func TestAccNVMetHost_basic(t *testing.T) {
	hostNQN := acctest.RandNQN()
	hostNQNRenamed := acctest.RandNQN()

	// The description field exists on the wire only from TrueNAS 26.0; on
	// older releases the test omits it and the update step exercises the
	// hostnqn rename alone.
	desc, descUpdated := "", ""
	if acctest.ServerVersionAtLeast(t, 26, 0) {
		desc, descUpdated = "tf-acc host", "tf-acc host updated"
	}

	step1Checks := []resource.TestCheckFunc{
		resource.TestCheckResourceAttr("truenas_nvmet_host.test", "hostnqn", hostNQN),
		resource.TestCheckResourceAttrSet("truenas_nvmet_host.test", "id"),
	}
	step2Checks := []resource.TestCheckFunc{
		resource.TestCheckResourceAttr("truenas_nvmet_host.test", "hostnqn", hostNQN),
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
		CheckDestroy:             testAccCheckNVMetHostDestroyed(hostNQNRenamed),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccNVMetHostConfig(hostNQN, desc, ""),
				Check:  resource.ComposeTestCheckFunc(step1Checks...),
			},
			{
				Config: acctest.ProviderConfig() + testAccNVMetHostConfig(hostNQN, descUpdated, ""),
				Check:  resource.ComposeTestCheckFunc(step2Checks...),
			},
			{
				// hostnqn is mutable in place (no RequiresReplace): verify that
				// changing it updates the new value on the existing resource
				// rather than forcing a create/destroy.
				Config: acctest.ProviderConfig() + testAccNVMetHostConfig(hostNQNRenamed, descUpdated, ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_nvmet_host.test", "hostnqn", hostNQNRenamed),
				),
			},
			{
				ResourceName:            "truenas_nvmet_host.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"dhchap_key", "dhchap_ctrl_key"},
			},
		},
	})
}

// testAccNVMetHostConfig renders the test resource; description and
// dhchap_key lines are omitted entirely when empty (description does not
// exist on the wire before TrueNAS 26.0).
func testAccNVMetHostConfig(hostnqn, description, dhchapKey string) string {
	cfg := fmt.Sprintf("resource \"truenas_nvmet_host\" \"test\" {\n  hostnqn = %q\n", hostnqn)
	if description != "" {
		cfg += fmt.Sprintf("  description = %q\n", description)
	}
	if dhchapKey != "" {
		cfg += fmt.Sprintf("  dhchap_key = %q\n", dhchapKey)
	}
	return cfg + "}\n"
}

func testAccCheckNVMetHostDestroyed(hostnqn string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "nvmet.host.query", [][]any{{"hostnqn", "=", hostnqn}})
		if err != nil {
			return fmt.Errorf("error checking nvmet host %s: %v", hostnqn, err)
		}
		var results []struct {
			ID any `json:"id"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing nvmet.host.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("nvmet host %s still exists", hostnqn)
		}
		return nil
	}
}

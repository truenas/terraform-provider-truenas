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

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckNVMetHostDestroyed(hostNQNRenamed),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccNVMetHostConfig(hostNQN, "tf-acc host", ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_nvmet_host.test", "hostnqn", hostNQN),
					resource.TestCheckResourceAttr("truenas_nvmet_host.test", "description", "tf-acc host"),
					resource.TestCheckResourceAttrSet("truenas_nvmet_host.test", "id"),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccNVMetHostConfig(hostNQN, "tf-acc host updated", ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_nvmet_host.test", "description", "tf-acc host updated"),
				),
			},
			{
				// hostnqn is mutable in place (no RequiresReplace): verify that
				// changing it updates the new value on the existing resource
				// rather than forcing a create/destroy.
				Config: acctest.ProviderConfig() + testAccNVMetHostConfig(hostNQNRenamed, "tf-acc host updated", ""),
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

func testAccNVMetHostConfig(hostnqn, description, dhchapKey string) string {
	if dhchapKey == "" {
		return fmt.Sprintf(`
resource "truenas_nvmet_host" "test" {
  hostnqn     = %q
  description = %q
}
`, hostnqn, description)
	}
	return fmt.Sprintf(`
resource "truenas_nvmet_host" "test" {
  hostnqn     = %q
  description = %q
  dhchap_key  = %q
}
`, hostnqn, description, dhchapKey)
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

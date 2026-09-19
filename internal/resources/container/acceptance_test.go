// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package container_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccContainer_basic drives the full lifecycle from the design spec:
// look up a current image version via truenas_container_image, create a
// container (running=false, autostart=false, an explicit pool — this
// provider's safety convention for anything container/VM-shaped, see
// vm's acceptance_test.go), update its description, start it and verify
// RUNNING, stop it again, import, then destroy + CheckDestroy.
//
// SAFETY: uses a RandName-suffixed container name and creates/destroys
// only its own object — this is a normal Tier-1 acceptance test (not
// DisruptiveCheck-gated), matching vm's precedent: it never touches any
// pre-existing container. Requires TrueNAS 26.0+ (the container
// namespace does not exist on 25.10, confirmed live via
// core.get_methods) — self-skips cleanly via acctest.ServerVersionAtLeast
// on any older box.
func TestAccContainer_basic(t *testing.T) {
	if !acctest.ServerVersionAtLeast(t, 26, 0) {
		t.Skip("truenas_container requires TrueNAS 26.0 or later (container namespace absent on 25.10, confirmed live)")
	}

	name := "tf-acc-" + acctest.RandName("container")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckContainerDestroyed(name),
		Steps: []resource.TestStep{
			{
				// Step 1: create, running=false.
				Config: acctest.ProviderConfig() + testAccContainerConfig(name, "initial description", false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_container.test", "name", name),
					resource.TestCheckResourceAttr("truenas_container.test", "pool", acctest.TestPool()),
					resource.TestCheckResourceAttr("truenas_container.test", "description", "initial description"),
					resource.TestCheckResourceAttr("truenas_container.test", "autostart", "false"),
					resource.TestCheckResourceAttr("truenas_container.test", "running", "false"),
					resource.TestCheckResourceAttr("truenas_container.test", "status", "STOPPED"),
					resource.TestCheckResourceAttrSet("truenas_container.test", "id"),
					resource.TestCheckResourceAttrSet("truenas_container.test", "dataset"),
					resource.TestCheckResourceAttrSet("data.truenas_container_image.alpine", "latest_version"),
				),
			},
			{
				// Step 2: update description only, still stopped.
				Config: acctest.ProviderConfig() + testAccContainerConfig(name, "updated description", false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_container.test", "description", "updated description"),
					resource.TestCheckResourceAttr("truenas_container.test", "running", "false"),
					resource.TestCheckResourceAttr("truenas_container.test", "status", "STOPPED"),
				),
			},
			{
				// Step 3: start it, verify RUNNING.
				Config: acctest.ProviderConfig() + testAccContainerConfig(name, "updated description", true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_container.test", "running", "true"),
					resource.TestCheckResourceAttr("truenas_container.test", "status", "RUNNING"),
				),
			},
			{
				// Step 4: stop it again before import/destroy.
				Config: acctest.ProviderConfig() + testAccContainerConfig(name, "updated description", false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_container.test", "running", "false"),
					resource.TestCheckResourceAttr("truenas_container.test", "status", "STOPPED"),
				),
			},
			{
				// Step 5: import. "image" cannot be recovered from the API
				// (see model.go's ContainerModel doc comment) so
				// ImportStateVerify must ignore it.
				ResourceName:            "truenas_container.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"image"},
			},
		},
	})
}

func testAccContainerConfig(name, description string, running bool) string {
	return fmt.Sprintf(`
data "truenas_container_image" "alpine" {
  name = "alpine:3.22:amd64:default"
}

resource "truenas_container" "test" {
  name        = %q
  pool        = %q
  description = %q
  autostart   = false
  running     = %t
  image = {
    name    = "alpine:3.22:amd64:default"
    version = data.truenas_container_image.alpine.latest_version
  }
}
`, name, acctest.TestPool(), description, running)
}

func testAccCheckContainerDestroyed(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "container.query", [][]any{{"name", "=", name}})
		if err != nil {
			return fmt.Errorf("error checking container %s: %v", name, err)
		}
		var results []struct {
			ID any `json:"id"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing container.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("container %s still exists", name)
		}
		return nil
	}
}

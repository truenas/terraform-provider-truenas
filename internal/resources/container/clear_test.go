// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package container_test

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// checkContainerServer asserts the server value of one container.query field
// (JSON-compared).
func checkContainerServer(name, key string, want any) resource.TestCheckFunc {
	return func(*terraform.State) error {
		raw, err := acctest.Client().Call(context.Background(), "container.query", [][]any{{"name", "=", name}})
		if err != nil {
			return err
		}
		var res []map[string]any
		if err := json.Unmarshal(raw, &res); err != nil {
			return err
		}
		if len(res) != 1 {
			return fmt.Errorf("container %s: %d matches, want 1", name, len(res))
		}
		got, _ := json.Marshal(res[0][key])
		w, _ := json.Marshal(want)
		if string(got) != string(w) {
			return fmt.Errorf("server %s = %s, want %s", key, got, w)
		}
		return nil
	}
}

// TestAccContainer_clearOptional sets cpuset/inituser/initdir/initgroup, then
// removes them from the config, which must clear them on the server. Before
// the fix the plan was empty and the server kept cpuset = "0-1" (they were
// Optional+Computed with UseStateForUnknown, so the null branch of
// threeWayString was unreachable; B-099). The update steps also check that
// status and dataset are planned as known values when running and name do not
// change, and that a rename (which renames the dataset) applies cleanly
// (B-137).
func TestAccContainer_clearOptional(t *testing.T) {
	if !acctest.ServerVersionAtLeast(t, 26, 0) {
		t.Skip("truenas_container requires TrueNAS 26.0 or later")
	}
	name := "tffix3virtct" + acctest.RandName("")
	set := `
  cpuset    = "0-1"
  inituser  = "root"
  initgroup = "root"
  initdir   = "/tmp"
  lifecycle { ignore_changes = [image] }
`
	unset := `
  description = "cleared"
  lifecycle { ignore_changes = [image] }
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             resource.ComposeTestCheckFunc(testAccCheckContainerDestroyed(name), testAccCheckContainerDestroyed(name+"r")),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + containerConfig(name, latest, set),
				Check: resource.ComposeTestCheckFunc(
					checkContainerServer(name, "cpuset", "0-1"),
					checkContainerServer(name, "inituser", "root"),
				),
			},
			{
				Config: acctest.ProviderConfig() + containerConfig(name, latest, unset),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("truenas_container.test", plancheck.ResourceActionUpdate),
					plancheck.ExpectKnownValue("truenas_container.test", tfjsonpath.New("status"), knownvalue.StringExact("STOPPED")),
					plancheck.ExpectKnownValue("truenas_container.test", tfjsonpath.New("dataset"), knownvalue.NotNull()),
				}},
				Check: resource.ComposeTestCheckFunc(
					checkContainerServer(name, "cpuset", nil),
					checkContainerServer(name, "inituser", nil),
					checkContainerServer(name, "initgroup", nil),
					checkContainerServer(name, "initdir", nil),
					resource.TestCheckNoResourceAttr("truenas_container.test", "cpuset"),
				),
			},
			{
				Config: acctest.ProviderConfig() + containerConfig(name+"r", latest, unset),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_container.test", "name", name+"r"),
					resource.TestMatchResourceAttr("truenas_container.test", "dataset", regexp.MustCompile("/"+name+"r$")),
				),
			},
		},
	})
}

// TestAccContainer_startFailNotOrphaned creates a container pinned to CPUs the
// host does not have, so container.start fails after container.create. The
// container must be in state (tainted), not orphaned: the next apply replaces
// it instead of failing on the duplicate name. Before the fix the failed start
// returned before saving state. (B-045)
func TestAccContainer_startFailNotOrphaned(t *testing.T) {
	if !acctest.ServerVersionAtLeast(t, 26, 0) {
		t.Skip("truenas_container requires TrueNAS 26.0 or later")
	}
	name := "tffix3virtct" + acctest.RandName("")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckContainerDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + containerConfig(name, latest, `
  cpuset  = "1000-1001"
  running = true
  lifecycle { ignore_changes = [image] }
`),
				ExpectError: regexp.MustCompile(`Start container failed`),
			},
			{
				Config: acctest.ProviderConfig() + containerConfig(name, latest, `
  running = false
  lifecycle { ignore_changes = [image] }
`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("truenas_container.test", plancheck.ResourceActionDestroyBeforeCreate),
				}},
				Check: resource.TestCheckResourceAttr("truenas_container.test", "running", "false"),
			},
		},
	})
}

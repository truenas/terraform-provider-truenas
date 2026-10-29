// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm_test

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func vmConfig(name, extra string) string {
	return fmt.Sprintf(`
resource "truenas_vm" "test" {
  name      = %q
  memory    = 512
  autostart = false
%s
}
`, name, extra)
}

// checkVMServer asserts the server value of one vm.query field (JSON-compared).
func checkVMServer(name, key string, want any) resource.TestCheckFunc {
	return func(*terraform.State) error {
		raw, err := acctest.Client().Call(context.Background(), "vm.query", [][]any{{"name", "=", name}})
		if err != nil {
			return err
		}
		var res []map[string]any
		if err := json.Unmarshal(raw, &res); err != nil {
			return err
		}
		if len(res) != 1 {
			return fmt.Errorf("vm %s: %d matches, want 1", name, len(res))
		}
		got, _ := json.Marshal(res[0][key])
		w, _ := json.Marshal(want)
		if string(got) != string(w) {
			return fmt.Errorf("server %s = %s, want %s", key, got, w)
		}
		return nil
	}
}

// TestAccVM_clearOptions sets every clearable option, clears each with its
// zero value (""/0 is sent as null, or "" for command_line_args), then removes
// them from the config, which leaves the server values alone. The last step
// also changes description only, which must not plan status as unknown
// (B-137). Before the fix, step 2 failed with "Provider produced inconsistent
// result" (command_line_args was "", but now "-no-hpet"; min_memory was 0, but
// now 256; ...) because ""/0 were dropped from the payload. (B-098)
func TestAccVM_clearOptions(t *testing.T) {
	name := "tffix3virtvm" + strings.ReplaceAll(acctest.RandName(""), "-", "")
	set := `
  cpu_mode          = "CUSTOM"
  cpu_model         = "Haswell"
  min_memory        = 256
  command_line_args = "-no-hpet"
  cpuset            = "0-1"
  nodeset           = "0"
  arch_type         = "x86_64"
  machine_type      = "q35"
`
	cleared := `
  cpu_mode          = "CUSTOM"
  cpu_model         = ""
  min_memory        = 0
  command_line_args = ""
  cpuset            = ""
  nodeset           = ""
  arch_type         = ""
  machine_type      = ""
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckVMDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + vmConfig(name, set),
				Check: resource.ComposeTestCheckFunc(
					checkVMServer(name, "cpuset", "0-1"),
					checkVMServer(name, "min_memory", 256),
				),
			},
			{
				Config: acctest.ProviderConfig() + vmConfig(name, cleared),
				Check: resource.ComposeTestCheckFunc(
					checkVMServer(name, "cpu_model", nil),
					checkVMServer(name, "min_memory", nil),
					checkVMServer(name, "command_line_args", ""),
					checkVMServer(name, "cpuset", nil),
					checkVMServer(name, "nodeset", nil),
					checkVMServer(name, "arch_type", nil),
					checkVMServer(name, "machine_type", nil),
					resource.TestCheckResourceAttr("truenas_vm.test", "cpuset", ""),
					resource.TestCheckResourceAttr("truenas_vm.test", "min_memory", "0"),
				),
			},
			// Set again, then remove from config: removal keeps the server value.
			{Config: acctest.ProviderConfig() + vmConfig(name, set)},
			{
				Config: acctest.ProviderConfig() + vmConfig(name, `  description = "changed"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("truenas_vm.test", plancheck.ResourceActionUpdate),
					plancheck.ExpectKnownValue("truenas_vm.test", tfjsonpath.New("status"), knownvalue.StringExact("STOPPED")),
				}},
				Check: resource.ComposeTestCheckFunc(
					checkVMServer(name, "cpuset", "0-1"),
					checkVMServer(name, "command_line_args", "-no-hpet"),
					checkVMServer(name, "min_memory", 256),
				),
			},
		},
	})
}

// TestAccVM_startFailNotOrphaned creates a VM whose start fails (4 TiB of
// memory). The VM must be in state (tainted), not orphaned: the next apply
// replaces it instead of failing on the duplicate name. Before the fix the
// failed start returned before saving state, leaving the VM on the server
// with nothing in state. (B-045)
func TestAccVM_startFailNotOrphaned(t *testing.T) {
	name := "tffix3virtvm" + strings.ReplaceAll(acctest.RandName(""), "-", "")
	cfg := func(running bool) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_vm" "test" {
  name      = %q
  memory    = 4194304
  autostart = false
  running   = %t
}
`, name, running)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckVMDestroyed(name),
		Steps: []resource.TestStep{
			{Config: cfg(true), ExpectError: regexp.MustCompile(`Start VM failed`)},
			{
				Config: cfg(false),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("truenas_vm.test", plancheck.ResourceActionDestroyBeforeCreate),
				}},
				Check: resource.TestCheckResourceAttr("truenas_vm.test", "running", "false"),
			},
		},
	})
}

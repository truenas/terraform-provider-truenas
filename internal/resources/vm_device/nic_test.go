// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm_device_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// nicAttach returns a NIC attach target that exists on the server under test
// (the first bridge, else the first interface for macvtap).
func nicAttach(t *testing.T) string {
	t.Helper()
	raw, err := acctest.Client().CallRead(context.Background(), "vm.device.nic_attach_choices")
	if err != nil {
		t.Fatalf("vm.device.nic_attach_choices: %v", err)
	}
	// 26.0+: {"BRIDGE": [...], "MACVLAN": [...]}; 25.10: {"enp6s18": "enp6s18"}.
	var choices map[string]any
	if err := json.Unmarshal(raw, &choices); err != nil {
		t.Fatalf("parse nic_attach_choices %s: %v", raw, err)
	}
	for _, k := range []string{"BRIDGE", "MACVLAN"} {
		if list, ok := choices[k].([]any); ok && len(list) > 0 {
			if s, ok := list[0].(string); ok {
				return s
			}
		}
	}
	for _, v := range choices {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	t.Skipf("no NIC attach choices on this server: %s", raw)
	return ""
}

func vmWithDevices(vmName, devs string) string {
	return fmt.Sprintf(`
resource "truenas_vm" "test" {
  name      = %q
  memory    = 512
  autostart = false
}
%s
`, vmName, devs)
}

// TestAccVMDevice_nicMacCase creates a NIC with an upper-case mac (the server
// stores it lower case), checks the re-plan is empty, then forgets the device,
// imports it and checks the plan is still empty. Before the fix the refresh
// saw the lower-case mac as drift, rewrote state with the full server map, and
// every plan wanted to send the upper-case mac again. (B-083)
func TestAccVMDevice_nicMacCase(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("acceptance test (TF_ACC not set)")
	}
	vmName := "tffix3virtvmd" + strings.ReplaceAll(acctest.RandName(""), "-", "")
	dev := fmt.Sprintf(`
resource "truenas_vm_device" "nic" {
  vm         = truenas_vm.test.id
  attributes = jsonencode({ dtype = "NIC", type = "VIRTIO", nic_attach = %q, mac = "00:A0:98:12:34:5B" })
}
`, nicAttach(t))
	forget := `
removed {
  from = truenas_vm_device.nic
  lifecycle { destroy = false }
}
`
	cfg := acctest.ProviderConfig() + vmWithDevices(vmName, dev)
	empty := resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}
	var id string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg, Check: func(s *terraform.State) error {
				id = s.RootModule().Resources["truenas_vm_device.nic"].Primary.ID
				return nil
			}},
			{Config: cfg, ConfigPlanChecks: empty},
			{Config: acctest.ProviderConfig() + vmWithDevices(vmName, forget)},
			{Config: cfg, ResourceName: "truenas_vm_device.nic", ImportState: true, ImportStatePersist: true,
				ImportStateIdFunc: func(*terraform.State) (string, error) { return id, nil }},
			{Config: cfg, ConfigPlanChecks: empty},
		},
	})
}

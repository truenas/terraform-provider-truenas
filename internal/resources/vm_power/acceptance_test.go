// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm_power_test

// truenas_vm_power (usage log #1): the power state of a VM as a resource
// ordered after its devices. Starting a guest needs free host memory: the
// 25.10 and 26.0 lab boxes refuse every vm.start with ENOMEM, so the test
// checks vm.get_available_memory first and skips there with that reason.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

const vmMemoryMiB = 512

// vmStartCheck skips when the box cannot start a 512 MiB guest.
func vmStartCheck(t *testing.T) {
	t.Helper()
	acctest.PreCheck(t)
	raw, err := acctest.Client().Call(context.Background(), "vm.get_available_memory", false)
	if err != nil {
		t.Fatalf("vm.get_available_memory: %v", err)
	}
	var avail int64
	if err := json.Unmarshal(raw, &avail); err != nil {
		t.Fatalf("parse vm.get_available_memory: %v", err)
	}
	if avail < vmMemoryMiB*1024*1024 {
		t.Skipf("this box cannot start a guest: %d MiB available for VMs, %d MiB needed (ENOMEM lab dead end)", avail/1024/1024, vmMemoryMiB)
	}
}

func vmState(t *testing.T, name string) (int64, string) {
	t.Helper()
	raw, err := acctest.Client().Call(context.Background(), "vm.query", [][]any{{"name", "=", name}})
	if err != nil {
		t.Fatalf("vm.query: %v", err)
	}
	var rows []struct {
		ID     int64 `json:"id"`
		Status struct {
			State string `json:"state"`
			PID   *int64 `json:"pid"`
		} `json:"status"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil || len(rows) != 1 {
		t.Fatalf("vm.query %s: %d rows, %v", name, len(rows), err)
	}
	return rows[0].ID, rows[0].Status.State
}

func vmPID(t *testing.T, name string) int64 {
	t.Helper()
	raw, err := acctest.Client().Call(context.Background(), "vm.query", [][]any{{"name", "=", name}})
	if err != nil {
		t.Fatalf("vm.query: %v", err)
	}
	var rows []struct {
		Status struct {
			PID *int64 `json:"pid"`
		} `json:"status"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil || len(rows) != 1 || rows[0].Status.PID == nil {
		t.Fatalf("vm.query %s pid: %v %v", name, rows, err)
	}
	return *rows[0].Status.PID
}

// TestAccVMPower_afterDevices: a VM (running unset) with a display device,
// powered on by truenas_vm_power after the device; stopped by running=false;
// a restart_triggers change restarts the running VM (new QEMU pid); import;
// destroying the power resource leaves the VM running; the VM's own destroy
// stops it.
func TestAccVMPower_afterDevices(t *testing.T) {
	vmStartCheck(t)
	name := "tfaccpwr" + strings.ReplaceAll(acctest.RandName(""), "-", "")
	cfg := func(running bool, trigger string, withPower bool) string {
		power := ""
		if withPower {
			power = fmt.Sprintf(`
resource "truenas_vm_power" "p" {
  vm_id      = truenas_vm.v.id
  running    = %t
  overcommit = false
  restart_triggers = {
    marker = %q
  }
  depends_on = [truenas_vm_device.d]
}
`, running, trigger)
		}
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_vm" "v" {
  name      = %q
  memory    = %d
  vcpus     = 1
  autostart = false
}

resource "truenas_vm_device" "d" {
  vm = truenas_vm.v.id
  attributes = jsonencode({
    dtype      = "DISPLAY"
    type       = "SPICE"
    bind       = "0.0.0.0"
    resolution = "1024x768"
    wait       = false
    web        = true
  })
  attributes_secrets_wo         = jsonencode({ password = "tfacc-spice-pw" })
  attributes_secrets_wo_version = 1
}
`, name, vmMemoryMiB) + power
	}
	var pid int64
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg(true, "a", true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_vm_power.p", "status", "RUNNING"),
					resource.TestCheckResourceAttr("truenas_vm_power.p", "running", "true"),
					func(*terraform.State) error {
						if _, st := vmState(t, name); st != "RUNNING" {
							return fmt.Errorf("vm state %s after power on", st)
						}
						pid = vmPID(t, name)
						return nil
					},
				),
			},
			// The VM was created stopped and started by the power resource in the
			// same apply; its running (left unset) follows the real state on the
			// next refresh without planning a change.
			{Config: cfg(true, "a", true), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
				Check: resource.TestCheckResourceAttr("truenas_vm.v", "running", "true")},
			// Trigger change: restart, new pid, still running.
			{
				Config: cfg(true, "b", true),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("truenas_vm_power.p", plancheck.ResourceActionUpdate),
					plancheck.ExpectResourceAction("truenas_vm.v", plancheck.ResourceActionNoop),
				}},
				Check: func(*terraform.State) error {
					if _, st := vmState(t, name); st != "RUNNING" {
						return fmt.Errorf("vm state %s after restart", st)
					}
					if p := vmPID(t, name); p == pid {
						return fmt.Errorf("qemu pid %d unchanged: the VM was not restarted", p)
					}
					return nil
				},
			},
			// Stop.
			{
				Config: cfg(false, "b", true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_vm_power.p", "status", "STOPPED"),
					func(*terraform.State) error {
						if _, st := vmState(t, name); st != "STOPPED" {
							return fmt.Errorf("vm state %s after power off", st)
						}
						return nil
					},
				),
			},
			// Start again, then import by VM id.
			{Config: cfg(true, "b", true)},
			{
				Config:       cfg(true, "b", true),
				ResourceName: "truenas_vm_power.p",
				ImportState:  true,
				ImportStateIdFunc: func(*terraform.State) (string, error) {
					id, _ := vmState(t, name)
					return fmt.Sprint(id), nil
				},
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"restart_triggers", "overcommit"},
			},
			// Removing the power resource leaves the VM running.
			{
				Config: cfg(true, "b", false),
				Check: func(*terraform.State) error {
					if _, st := vmState(t, name); st != "RUNNING" {
						return fmt.Errorf("vm state %s after the power resource was removed; it must be left alone", st)
					}
					return nil
				},
			},
			// Stop it before the destroy: TrueNAS refuses to detach a device
			// from a running VM, and Terraform destroys devices before the VM.
			{Config: cfg(false, "b", true)},
		},
	})
	// The VM's own destroy (CheckDestroy equivalent): gone.
	raw, err := acctest.Client().Call(context.Background(), "vm.query", [][]any{{"name", "=", name}})
	if err != nil || string(raw) != "[]" {
		t.Fatalf("vm %s still exists after destroy (%s, %v)", name, raw, err)
	}
}

// TestAccVM_restartOnUpdate: with restart_on_update = true a memory change on
// a running VM restarts it (new pid); without it the VM keeps running
// untouched (same pid) and the plan is clean either way.
func TestAccVM_restartOnUpdate(t *testing.T) {
	vmStartCheck(t)
	name := "tfaccrst" + strings.ReplaceAll(acctest.RandName(""), "-", "")
	cfg := func(memory int, restart bool) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_vm" "v" {
  name              = %q
  memory            = %d
  vcpus             = 1
  autostart         = false
  running           = true
  restart_on_update = %t
}
`, name, memory, restart)
	}
	var pid int64
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg(vmMemoryMiB, false), Check: func(*terraform.State) error {
				if _, st := vmState(t, name); st != "RUNNING" {
					return fmt.Errorf("vm state %s", st)
				}
				pid = vmPID(t, name)
				return nil
			}},
			// No restart_on_update: the setting changes, the guest keeps running.
			{Config: cfg(vmMemoryMiB+64, false), Check: func(*terraform.State) error {
				if p := vmPID(t, name); p != pid {
					return fmt.Errorf("pid changed %d -> %d without restart_on_update", pid, p)
				}
				return nil
			}},
			// restart_on_update alone is a state-only change: no restart.
			{Config: cfg(vmMemoryMiB+64, true), Check: func(*terraform.State) error {
				if p := vmPID(t, name); p != pid {
					return fmt.Errorf("pid changed %d -> %d on a restart_on_update-only change", pid, p)
				}
				return nil
			}},
			{Config: cfg(vmMemoryMiB+128, true), Check: func(*terraform.State) error {
				if _, st := vmState(t, name); st != "RUNNING" {
					return fmt.Errorf("vm state %s after restart", st)
				}
				if p := vmPID(t, name); p == pid {
					return fmt.Errorf("pid %d unchanged: restart_on_update did not restart", p)
				}
				return nil
			}},
			{Config: cfg(vmMemoryMiB+128, true), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}

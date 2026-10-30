// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest_test

// Adversarial bug hunt: out-of-band DRIFT. Create a resource with Terraform,
// change a configured attribute behind its back through the API, then plan.
// Terraform must see the drift (an in-place update back to the configured
// value) and apply must restore it; the plan after that must be empty.
//
// Why this matters: several resources keep "what you said" in state when the
// server normalizes a value. Done carelessly, that hides real changes made in
// the UI — the provider would report "no changes" while the box disagrees
// with the config. That is the opposite of what a user expects from plan.

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// huntDrift applies cfg, runs mutate (an API call that changes a configured
// attribute), then requires: plan shows an Update for addr; after apply, an
// empty plan.
func huntDrift(t *testing.T, addr, cfg string, mutate func(s *terraform.State) error) {
	t.Helper()
	acctest.PreCheck(t)
	full := acctest.ProviderConfig() + cfg
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: full},
			{Config: full, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			// mutate out-of-band; the Check runs after this (no-op) apply, so the
			// framework's post-step refresh plan is expected to be non-empty.
			{Config: full, Check: mutate, ExpectNonEmptyPlan: true},
			// drift must be visible and corrected
			{Config: full, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
				plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
			}}},
			{Config: full, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}

func idOf(s *terraform.State, addr string) string { return s.RootModule().Resources[addr].Primary.ID }

func apiCall(method string, params ...any) error {
	_, err := acctest.Client().Call(context.Background(), method, params...)
	return err
}

func TestHuntDrift_datasetCompression(t *testing.T) {
	n := acctest.TestPool() + "/" + acctest.RandName("tfhuntdrift")
	huntDrift(t, "truenas_dataset.d", fmt.Sprintf(`
resource "truenas_dataset" "d" {
  name        = %q
  compression = "lz4"
  atime       = "ON"
  comments    = "configured"
}
`, n), func(s *terraform.State) error {
		return apiCall("pool.dataset.update", n, map[string]any{"compression": "ZSTD", "atime": "OFF", "comments": "changed in UI"})
	})
}

func TestHuntDrift_smbShareFlags(t *testing.T) {
	n := acctest.RandName("tfhuntdrsmb")
	huntDrift(t, "truenas_smb_share.s", huntDS(n)+fmt.Sprintf(`
resource "truenas_smb_share" "s" {
  name      = %q
  path      = truenas_dataset.fx.mountpoint
  ro        = false
  browsable = true
  comment   = "configured"
  enabled   = true
}
`, n), func(s *terraform.State) error {
		var id int64
		fmt.Sscanf(idOf(s, "truenas_smb_share.s"), "%d", &id)
		return apiCall("sharing.smb.update", id, map[string]any{"readonly": true, "browsable": false, "comment": "changed in UI", "enabled": false})
	})
}

func TestHuntDrift_nfsShareNetworks(t *testing.T) {
	n := acctest.RandName("tfhuntdrnfs")
	huntDrift(t, "truenas_nfs_share.s", huntDS(n)+`
resource "truenas_nfs_share" "s" {
  path     = truenas_dataset.fx.mountpoint
  ro       = true
  networks = ["192.0.2.0/24"]
  comment  = "configured"
}
`, func(s *terraform.State) error {
		var id int64
		fmt.Sscanf(idOf(s, "truenas_nfs_share.s"), "%d", &id)
		// the dangerous direction: someone opens the share up in the UI
		return apiCall("sharing.nfs.update", id, map[string]any{"ro": false, "networks": []string{}, "comment": "changed in UI"})
	})
}

func TestHuntDrift_userShellEmail(t *testing.T) {
	n := acctest.RandName("tfhuntdru")
	huntDrift(t, "truenas_user.u", fmt.Sprintf(`
resource "truenas_user" "u" {
  username     = %q
  full_name    = "configured"
  group_create = true
  password     = "TfHunt-Drift-Pw-1"
  email        = "configured@example.com"
  shell        = "/usr/bin/bash"
  locked       = false
}
`, n), func(s *terraform.State) error {
		var id int64
		fmt.Sscanf(idOf(s, "truenas_user.u"), "%d", &id)
		return apiCall("user.update", id, map[string]any{"email": "changed@example.com", "shell": "/usr/sbin/nologin", "locked": true, "full_name": "changed in UI"})
	})
}

func TestHuntDrift_cronjobEnabled(t *testing.T) {
	desc := acctest.RandName("tfhuntdrcron")
	huntDrift(t, "truenas_cronjob.c", fmt.Sprintf(`
resource "truenas_cronjob" "c" {
  command     = "echo configured"
  user        = "root"
  description = %q
  enabled     = true
  schedule = {
    minute = "0"
    hour   = "1"
    dom    = "*"
    month  = "*"
    dow    = "*"
  }
}
`, desc), func(s *terraform.State) error {
		var id int64
		fmt.Sscanf(idOf(s, "truenas_cronjob.c"), "%d", &id)
		return apiCall("cronjob.update", id, map[string]any{"enabled": false, "command": "echo changed", "schedule": map[string]any{"minute": "30", "hour": "1", "dom": "*", "month": "*", "dow": "*"}})
	})
}

func TestHuntDrift_containerDeviceNIC(t *testing.T) {
	// container_device keeps the plan's attribute keys in state
	// ("write-what-you-said") because the server normalizes the JSON blob.
	// Does a real change to one of those keys still surface?
	if !acctest.ServerVersionAtLeast(t, 26, 0) {
		t.Skip("container.device needs 26.0+")
	}
	n := acctest.RandName("tfhuntdrcd")
	ds := acctest.TestPool() + "/" + n
	huntDrift(t, "truenas_container_device.nic", fmt.Sprintf(`
resource "truenas_dataset" "fx" {
  name = %q
}
data "truenas_container_image" "alpine" {
  name = "alpine:3.22:amd64:default"
}
resource "truenas_container" "test" {
  name      = "tfhuntdr%s"
  pool      = truenas_dataset.fx.pool
  autostart = false
  running   = false
  image = {
    name    = "alpine:3.22:amd64:default"
    version = data.truenas_container_image.alpine.latest_version
  }
}
resource "truenas_container_device" "nic" {
  container  = truenas_container.test.id
  attributes = jsonencode({
    dtype      = "NIC"
    nic_attach = "truenasbr0"
    mac        = "00:a0:98:12:34:5c"
  })
}
`, ds, acctest.RandName("")), func(s *terraform.State) error {
		var id int64
		fmt.Sscanf(idOf(s, "truenas_container_device.nic"), "%d", &id)
		return apiCall("container.device.update", id, map[string]any{"attributes": map[string]any{"dtype": "NIC", "nic_attach": "truenasbr0", "mac": "00:a0:98:12:34:5d"}})
	})
}

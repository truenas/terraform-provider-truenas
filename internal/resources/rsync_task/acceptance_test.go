// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package rsync_task_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccRsyncTask_basic creates a dataset fixture and a MODULE-mode rsync
// task pointed at an unreachable remote host, checks its attributes,
// updates desc in place, imports the task by its numeric id, and verifies
// destruction of both the task and the dataset fixture.
//
// mode=MODULE, remotehost=192.0.2.10 (TEST-NET-1, guaranteed unreachable),
// remotemodule=tfacc, enabled=false, validate_rpath=false is deliberately
// unreachable/never-run: probed directly against a live TrueNAS
// 25.10 box (rsynctask.create with this exact payload, then deleted) and
// confirmed rsynctask.create does NOT validate remote connectivity at
// create time in MODULE mode — this succeeded even with validate_rpath
// left at its schema default (true) and enabled=false. That's because
// validate_rpath only checks "remotepath" (an SSH-mode concept: it needs a
// remote shell to run `ls` against); MODULE mode's "remote path" is a
// module name resolved by the remote rsync daemon, so validate_rpath is a
// no-op there regardless of its value. (The design doc's SSH-mode fallback
// was also probed and found to fail for an unrelated reason: rsynctask
// over SSH additionally requires the run-as user to have an SSH keypair in
// its home directory, independent of validate_rpath/connectivity — so it
// would not have been a usable fallback either.) validate_rpath=false is
// still set explicitly here to keep the test's intent self-documenting.
func TestAccRsyncTask_basic(t *testing.T) {
	datasetName := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-rsync-ds"))
	desc := acctest.RandName("tf-acc-rsync")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			testAccCheckRsyncTaskDestroyed(datasetName),
			testAccCheckRsyncDatasetDestroyed(datasetName),
		),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccRsyncTaskConfig(datasetName, desc),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_rsync_task.test", "id"),
					resource.TestCheckResourceAttrPair("truenas_rsync_task.test", "path", "truenas_dataset.fixture", "mountpoint"),
					resource.TestCheckResourceAttr("truenas_rsync_task.test", "user", "root"),
					resource.TestCheckResourceAttr("truenas_rsync_task.test", "mode", "MODULE"),
					resource.TestCheckResourceAttr("truenas_rsync_task.test", "remotehost", "192.0.2.10"),
					resource.TestCheckResourceAttr("truenas_rsync_task.test", "remotemodule", "tfacc"),
					resource.TestCheckResourceAttr("truenas_rsync_task.test", "enabled", "false"),
					resource.TestCheckResourceAttr("truenas_rsync_task.test", "desc", desc),
					resource.TestCheckResourceAttr("truenas_rsync_task.test", "schedule.minute", "0"),
					resource.TestCheckResourceAttr("truenas_rsync_task.test", "schedule.hour", "3"),
				),
			},
			// Update desc in place.
			{
				Config: acctest.ProviderConfig() + testAccRsyncTaskConfig(datasetName, desc+"-updated"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_rsync_task.test", "desc", desc+"-updated"),
				),
			},
			// Import by the task's numeric id. validate_rpath/ssh_keyscan
			// are never returned by the API, so ImportStateVerify must
			// ignore both (see model.go's RsyncTaskModel doc comment).
			{
				ResourceName:            "truenas_rsync_task.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"validate_rpath", "ssh_keyscan"},
			},
		},
	})
}

func testAccRsyncTaskConfig(datasetName, desc string) string {
	return fmt.Sprintf(`
resource "truenas_dataset" "fixture" {
  name = %q
}

resource "truenas_rsync_task" "test" {
  path           = truenas_dataset.fixture.mountpoint
  user           = "root"
  mode           = "MODULE"
  remotehost     = "192.0.2.10"
  remotemodule   = "tfacc"
  enabled        = false
  validate_rpath = false
  desc           = %q
  schedule = {
    minute = "0"
    hour   = "3"
    dom    = "*"
    month  = "*"
    dow    = "*"
  }
}
`, datasetName, desc)
}

// rsyncTaskSummary is the subset of rsynctask.query fields this test
// package needs directly (outside of the provider's own resource code).
type rsyncTaskSummary struct {
	ID   int64  `json:"id"`
	Path string `json:"path"`
}

// testAccCheckRsyncTaskDestroyed queries rsynctask by the fixture dataset's
// mountpoint path since the task's ID is not known outside of Terraform
// state at CheckDestroy time.
func testAccCheckRsyncTaskDestroyed(datasetName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		path := "/mnt/" + datasetName
		raw, err := c.Call(context.Background(), "rsynctask.query", [][]any{{"path", "=", path}})
		if err != nil {
			return fmt.Errorf("error checking rsync task for %s: %v", path, err)
		}
		var results []rsyncTaskSummary
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing rsynctask.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("rsync task for %s still exists", path)
		}
		return nil
	}
}

func testAccCheckRsyncDatasetDestroyed(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "pool.dataset.query", [][]any{{"id", "=", name}})
		if err != nil {
			return fmt.Errorf("error checking dataset %s: %v", name, err)
		}
		var results []struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing pool.dataset.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("dataset %s still exists", name)
		}
		return nil
	}
}

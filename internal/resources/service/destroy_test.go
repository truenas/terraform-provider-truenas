// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package service_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

type serviceRow struct {
	ID     int64  `json:"id"`
	Enable bool   `json:"enable"`
	State  string `json:"state"`
}

func queryService(name string) (serviceRow, error) {
	raw, err := acctest.RestoreCall(context.Background(), "service.query", [][]any{{"service", "=", name}})
	if err != nil {
		return serviceRow{}, err
	}
	var out []serviceRow
	if err := json.Unmarshal(raw, &out); err != nil || len(out) != 1 {
		return serviceRow{}, fmt.Errorf("decode service.query %s: %v (%d rows)", name, err, len(out))
	}
	return out[0], nil
}

// setService puts a service into the given enable/running state through the
// API (test setup and cleanup).
func setService(name string, enable, running bool) error {
	ctx := context.Background()
	svc, err := queryService(name)
	if err != nil {
		return err
	}
	if _, err := acctest.RestoreCall(ctx, "service.update", svc.ID, map[string]any{"enable": enable}); err != nil {
		return err
	}
	verb := "STOP"
	if running {
		verb = "START"
	}
	_, err = acctest.Client().CallJob(ctx, "service.control", verb, name, map[string]any{})
	return err
}

// restoreServiceOnCleanup records a service's enable/running state and puts
// it back when the test ends.
func restoreServiceOnCleanup(t *testing.T, name string) {
	t.Helper()
	orig, err := queryService(name)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := setService(name, orig.Enable, orig.State == "RUNNING"); err != nil {
			t.Errorf("RESTORE %s: %v", name, err)
		}
	})
}

// TestAccService_destroyLeavesServiceAsIs guards B-106: removing a
// truenas_service from the configuration must not disable or stop the
// service (it used to do both, ignoring errors, so removing "ssh" stopped
// SSH). ftp stands in for ssh.
func TestAccService_destroyLeavesServiceAsIs(t *testing.T) {
	acctest.DisruptiveCheck(t)
	restoreServiceOnCleanup(t, "ftp")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy: func(*terraform.State) error {
			now, err := queryService("ftp")
			if err != nil {
				return err
			}
			if !now.Enable || now.State != "RUNNING" {
				return fmt.Errorf("destroy changed the service: enable=%v state=%s, want enable=true state=RUNNING", now.Enable, now.State)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
resource "truenas_service" "test" {
  name    = "ftp"
  enabled = true
  running = true
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_service.test", "enabled", "true"),
					resource.TestCheckResourceAttr("truenas_service.test", "running", "true"),
				),
			},
		},
	})
}

// TestAccService_createWithoutEnabledKeepsAutostart: a configuration that
// does not set enabled must not touch the autostart flag. Create used to
// send enable = ValueBool() of the unknown planned value, i.e. false,
// disabling autostart of a service that had it on. (running is left unset
// too: other activity on a shared box can restart ftp.)
func TestAccService_createWithoutEnabledKeepsAutostart(t *testing.T) {
	acctest.DisruptiveCheck(t)
	restoreServiceOnCleanup(t, "ftp")
	if err := setService("ftp", true, false); err != nil {
		t.Fatal(err)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
resource "truenas_service" "test" {
  name = "ftp"
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_service.test", "enabled", "true"),
					func(*terraform.State) error {
						now, err := queryService("ftp")
						if err != nil {
							return err
						}
						if !now.Enable {
							return fmt.Errorf("ftp autostart was turned off by a config that did not set enabled")
						}
						return nil
					},
				),
			},
		},
	})
}

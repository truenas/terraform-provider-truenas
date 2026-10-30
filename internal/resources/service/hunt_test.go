// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package service_test

// Adversarial bug-hunt tests (throwaway, not for commit).

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

type huntSvc struct {
	ID     int64  `json:"id"`
	Enable bool   `json:"enable"`
	State  string `json:"state"`
}

func huntQuerySvc(name string) (huntSvc, error) {
	raw, err := acctest.RestoreCall(context.Background(), "service.query", [][]any{{"service", "=", name}})
	if err != nil {
		return huntSvc{}, err
	}
	var out []huntSvc
	if err := json.Unmarshal(raw, &out); err != nil || len(out) != 1 {
		return huntSvc{}, fmt.Errorf("decode service.query: %v (%d)", err, len(out))
	}
	return out[0], nil
}

// running=true then false on ftp (stopped by default). Exercises the
// service.start / service.stop calls, which the existing acceptance test
// never touches. Restores ftp to its original enable/state.
func TestHuntService_runningToggle(t *testing.T) {
	acctest.DisruptiveCheck(t)
	orig, err := huntQuerySvc("ftp")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = acctest.RestoreCall(ctx, "service.update", orig.ID, map[string]any{"enable": orig.Enable})
		verb := "STOP"
		if orig.State == "RUNNING" {
			verb = "START"
		}
		if _, err := acctest.Client().CallJob(ctx, "service.control", verb, "ftp", map[string]any{}); err != nil {
			t.Errorf("RESTORE service.control %s ftp: %v", verb, err)
		}
		now, _ := huntQuerySvc("ftp")
		t.Logf("ftp restored: enable=%v state=%s (orig enable=%v state=%s)", now.Enable, now.State, orig.Enable, orig.State)
	})
	on := acctest.ProviderConfig() + `
resource "truenas_service" "test" {
  name    = "ftp"
  enabled = false
  running = true
}
`
	off := acctest.ProviderConfig() + `
resource "truenas_service" "test" {
  name    = "ftp"
  enabled = false
  running = false
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy: func(*terraform.State) error {
			now, err := huntQuerySvc("ftp")
			t.Logf("after destroy: ftp enable=%v state=%s", now.Enable, now.State)
			return err
		},
		Steps: []resource.TestStep{
			{Config: on, Check: resource.TestCheckResourceAttr("truenas_service.test", "running", "true")},
			{Config: on, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{Config: off, Check: resource.TestCheckResourceAttr("truenas_service.test", "running", "false")},
			{Config: off, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{Config: on},
		},
	})
}

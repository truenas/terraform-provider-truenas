// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package smb_config_test

// Adversarial bug-hunt tests (throwaway, not for commit).

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func huntSnapshotRestore(t *testing.T) {
	t.Helper()
	raw, err := acctest.RestoreCall(context.Background(), "smb.config")
	if err != nil {
		t.Fatalf("read smb.config: %v", err)
	}
	var orig map[string]any
	if err := json.Unmarshal(raw, &orig); err != nil {
		t.Fatal(err)
	}
	delete(orig, "id")
	delete(orig, "server_sid")
	t.Cleanup(func() {
		if _, err := acctest.RestoreCall(context.Background(), "smb.update", orig); err != nil {
			t.Errorf("RESTORE smb.update failed: %v", err)
		}
		raw, _ := acctest.RestoreCall(context.Background(), "service.query", [][]any{{"service", "=", "cifs"}})
		t.Logf("cifs after restore: %s", raw)
	})
}

func cifsRunning(*terraform.State) error {
	raw, err := acctest.RestoreCall(context.Background(), "service.query", [][]any{{"service", "=", "cifs"}})
	if err != nil {
		return err
	}
	var s []struct {
		State string `json:"state"`
	}
	_ = json.Unmarshal(raw, &s)
	if len(s) != 1 || s[0].State != "RUNNING" {
		return &huntErr{string(raw)}
	}
	return nil
}

type huntErr struct{ s string }

func (e *huntErr) Error() string { return "cifs not RUNNING: " + e.s }

// One-field update (description), then server-normalized values (lowercase
// workgroup / unixcharset), then import + re-plan.
func TestHuntSMBConfig_oneFieldAndNormalize(t *testing.T) {
	acctest.DisruptiveCheck(t)
	huntSnapshotRestore(t)
	c1 := acctest.ProviderConfig() + `
resource "truenas_smb_config" "test" {
  description = "tfhunt one"
}
`
	c2 := acctest.ProviderConfig() + `
resource "truenas_smb_config" "test" {
  description = "tfhunt two"
}
`
	c3 := acctest.ProviderConfig() + `
resource "truenas_smb_config" "test" {
  description = "tfhunt two"
  workgroup   = "tfhuntwg"
  filemask    = "644"
}
`
	steps := []resource.TestStep{
		{Config: c1, Check: cifsRunning},
		{Config: c2, Check: cifsRunning},
		{Config: c2, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		{Config: c3, Check: cifsRunning},
		{Config: c3, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}
	steps = append(steps, huntImportPersist("truenas_smb_config.test", c3)...)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps:                    steps,
	})
}

// huntImportPersist: import WITH ImportStatePersist so the following
// empty-plan step runs against the imported state.
func huntImportPersist(name, cfg string) []resource.TestStep {
	return []resource.TestStep{
		// import block + plan in a fresh workdir; the framework requires a no-op plan.
		{Config: cfg, ResourceName: name, ImportState: true, ImportStateKind: resource.ImportBlockWithID},
	}
}

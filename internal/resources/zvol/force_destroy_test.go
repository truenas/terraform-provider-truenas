// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package zvol_test

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// The tests in this file guard force_destroy on truenas_zvol: a zvol with
// snapshots is not deleted unless force_destroy = true, and the error names
// the snapshots instead of passing on the server's version-dependent text.

func zvolCfg(name, extra string) string {
	return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_zvol" "test" {
  name    = %q
  volsize = 67108864
  %s
}
`, name, extra)
}

func zfsExistsZ(name string) (bool, error) {
	method := "pool.dataset.query"
	if strings.Contains(name, "@") {
		method = "pool.snapshot.query"
	}
	raw, err := acctest.Client().Call(context.Background(), method, [][]any{{"name", "=", name}})
	if err != nil {
		return false, err
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(raw, &rows); err != nil {
		return false, err
	}
	return len(rows) > 0, nil
}

func checkPresence(want bool, names ...string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		for _, n := range names {
			ok, err := zfsExistsZ(n)
			if err != nil {
				return err
			}
			if ok != want {
				return fmt.Errorf("%s exists=%v, want %v", n, ok, want)
			}
		}
		return nil
	}
}

func snapshotZvol(t *testing.T, name string) {
	t.Helper()
	if _, err := acctest.Client().Call(context.Background(), "pool.snapshot.create", map[string]any{"dataset": name, "name": "keep"}); err != nil {
		t.Fatalf("snapshot create: %v", err)
	}
}

func cleanupZvol(t *testing.T, name string) {
	t.Cleanup(func() {
		_, _ = acctest.Client().CallJob(context.Background(), "pool.dataset.delete", name, map[string]any{"recursive": true})
	})
}

// TestAccZvol_deleteRefusedWithSnapshot: the destroy of a zvol with a snapshot
// fails naming it and deletes nothing; force_destroy = true is an in-place
// update, after which the destroy deletes the zvol and its snapshot.
func TestAccZvol_deleteRefusedWithSnapshot(t *testing.T) {
	name := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-zv-fd"))
	snap := name + "@keep"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t); cleanupZvol(t, name) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             checkPresence(false, name, snap),
		Steps: []resource.TestStep{
			{
				Config: zvolCfg(name, ""),
				Check:  resource.TestCheckResourceAttr("truenas_zvol.test", "force_destroy", "false"),
			},
			{
				PreConfig:   func() { snapshotZvol(t, name) },
				Config:      acctest.ProviderConfig(),
				ExpectError: regexp.MustCompile(`(?s)zvol "` + regexp.QuoteMeta(name) + `" has 1 snapshot.*` + regexp.QuoteMeta(snap) + `.*force_destroy`),
			},
			{
				Config: zvolCfg(name, "force_destroy = true"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("truenas_zvol.test", plancheck.ResourceActionUpdate),
				}},
				Check: resource.ComposeTestCheckFunc(
					checkPresence(true, name, snap),
					resource.TestCheckResourceAttr("truenas_zvol.test", "force_destroy", "true"),
				),
			},
			{
				Config: acctest.ProviderConfig(),
				Check:  checkPresence(false, name, snap),
			},
		},
	})
}

// TestAccZvol_forceDestroyAfterImport: import sets force_destroy to false; a
// config with force_destroy = true plans an in-place update, never a
// replacement, and the recorded value lets the destroy delete a snapshot.
func TestAccZvol_forceDestroyAfterImport(t *testing.T) {
	name := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-zv-fdi"))
	snap := name + "@keep"
	cfg := zvolCfg(name, "force_destroy = true")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t); cleanupZvol(t, name) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             checkPresence(false, name, snap),
		Steps: []resource.TestStep{
			{Config: cfg},
			{Config: acctest.ProviderConfig() + "\nremoved {\n  from = truenas_zvol.test\n  lifecycle {\n    destroy = false\n  }\n}\n"},
			{
				Config: cfg + fmt.Sprintf("\nimport {\n  to = truenas_zvol.test\n  id = %q\n}\n", name),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					acctest.ExpectNoReplace("truenas_zvol.test"),
					plancheck.ExpectResourceAction("truenas_zvol.test", plancheck.ResourceActionUpdate),
				}},
				Check: resource.TestCheckResourceAttr("truenas_zvol.test", "force_destroy", "true"),
			},
			{Config: cfg, ConfigPlanChecks: emptyPlanZ()},
			{
				PreConfig: func() { snapshotZvol(t, name) },
				Config:    acctest.ProviderConfig(),
				Check:     checkPresence(false, name, snap),
			},
		},
	})
}

// emptyPlanZ asserts the step plans no changes before apply.
func emptyPlanZ() resource.ConfigPlanChecks {
	return resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}
}

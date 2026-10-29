// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset_test

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

// The tests in this file guard the non-recursive delete (B-001) and
// force_destroy: a dataset with child datasets/zvols or snapshots is not
// deleted unless force_destroy = true, and the error names what is in the way.

// cleanupRecursive (called from PreCheck, so only when TF_ACC is set) deletes
// name recursively at the end of the test, so a failing step does not leave
// the dataset (and the children it was given out of band) behind.
func cleanupRecursive(t *testing.T, name string) {
	t.Cleanup(func() {
		_, _ = acctest.Client().CallJob(context.Background(), "pool.dataset.delete", name, map[string]any{"recursive": true})
	})
}

func mustCall(t *testing.T, method string, params ...any) {
	t.Helper()
	if _, err := acctest.Client().Call(context.Background(), method, params...); err != nil {
		t.Fatalf("%s: %v", method, err)
	}
}

// zfsExists reports whether a dataset/zvol (pool.dataset.query) or a snapshot
// (pool.snapshot.query, when name contains "@") exists.
func zfsExists(name string) (bool, error) {
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

func checkExists(names ...string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		for _, n := range names {
			ok, err := zfsExists(n)
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("%s was deleted, but the delete should have been refused", n)
			}
		}
		return nil
	}
}

func checkGone(names ...string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		for _, n := range names {
			ok, err := zfsExists(n)
			if err != nil {
				return err
			}
			if ok {
				return fmt.Errorf("%s still exists after force_destroy delete", n)
			}
		}
		return nil
	}
}

func datasetCfg(name, extra string) string {
	return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "test" {
  name = %q
  %s
}
`, name, extra)
}

func expectUpdate(addr string) resource.ConfigPlanChecks {
	return resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
		plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate),
	}}
}

// TestAccDataset_deleteRefusedWithUnmanagedChild: destroying a dataset that has
// a child dataset Terraform does not manage fails, names the child, and leaves
// both in place. Setting force_destroy = true is an in-place update, after
// which the destroy deletes both.
func TestAccDataset_deleteRefusedWithUnmanagedChild(t *testing.T) {
	name := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-ds-fd"))
	child := name + "/unmanaged"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t); cleanupRecursive(t, name) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             checkGone(name, child),
		Steps: []resource.TestStep{
			{
				Config: datasetCfg(name, ""),
				Check:  resource.TestCheckResourceAttr("truenas_dataset.test", "force_destroy", "false"),
			},
			{
				PreConfig:   func() { mustCall(t, "pool.dataset.create", map[string]any{"name": child}) },
				Config:      acctest.ProviderConfig(),
				ExpectError: regexp.MustCompile(`(?s)1 child dataset/zvol.*` + regexp.QuoteMeta(child) + `.*force_destroy`),
			},
			{
				// The refused destroy left the dataset in state and both on the
				// server; force_destroy is a state-only in-place update.
				Config:           datasetCfg(name, "force_destroy = true"),
				ConfigPlanChecks: expectUpdate("truenas_dataset.test"),
				Check: resource.ComposeTestCheckFunc(
					checkExists(name, child),
					resource.TestCheckResourceAttr("truenas_dataset.test", "force_destroy", "true"),
				),
			},
			{
				Config: acctest.ProviderConfig(),
				Check:  checkGone(name, child),
			},
		},
	})
}

// TestAccDataset_deleteRefusedWithSnapshot: a snapshot alone blocks the delete
// (it is not deleted without force_destroy), and the error names it. With
// force_destroy = true the destroy deletes the dataset with its snapshot and a
// child dataset.
func TestAccDataset_deleteRefusedWithSnapshot(t *testing.T) {
	name := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-ds-fds"))
	snap := name + "@keep"
	child := name + "/kid"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t); cleanupRecursive(t, name) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             checkGone(name, child, snap),
		Steps: []resource.TestStep{
			{Config: datasetCfg(name, "")},
			{
				PreConfig:   func() { mustCall(t, "pool.snapshot.create", map[string]any{"dataset": name, "name": "keep"}) },
				Config:      acctest.ProviderConfig(),
				ExpectError: regexp.MustCompile(`(?s)1 snapshot.*` + regexp.QuoteMeta(snap)),
			},
			{
				PreConfig:        func() { mustCall(t, "pool.dataset.create", map[string]any{"name": child}) },
				Config:           datasetCfg(name, "force_destroy = true"),
				ConfigPlanChecks: expectUpdate("truenas_dataset.test"),
				Check:            checkExists(name, snap, child),
			},
			{
				Config: acctest.ProviderConfig(),
				Check:  checkGone(name, child, snap),
			},
		},
	})
}

// TestAccDataset_forceDestroyAfterImport: import sets force_destroy to false, and
// a config with force_destroy = true then plans an in-place update, never a
// replacement. The recorded value takes effect: the destroy deletes the
// dataset together with an unmanaged child.
func TestAccDataset_forceDestroyAfterImport(t *testing.T) {
	name := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-ds-fdi"))
	child := name + "/kid"
	cfg := datasetCfg(name, "force_destroy = true")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t); cleanupRecursive(t, name) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             checkGone(name, child),
		Steps: []resource.TestStep{
			{Config: cfg},
			{Config: acctest.ProviderConfig() + "\nremoved {\n  from = truenas_dataset.test\n  lifecycle {\n    destroy = false\n  }\n}\n"},
			{
				Config: cfg + fmt.Sprintf("\nimport {\n  to = truenas_dataset.test\n  id = %q\n}\n", name),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					acctest.ExpectNoReplace("truenas_dataset.test"),
					acctest.ExpectOnlyAttrChanges("truenas_dataset.test", "force_destroy"),
					plancheck.ExpectResourceAction("truenas_dataset.test", plancheck.ResourceActionUpdate),
				}},
				Check: resource.TestCheckResourceAttr("truenas_dataset.test", "force_destroy", "true"),
			},
			{Config: cfg, ConfigPlanChecks: emptyPlan()},
			{
				PreConfig: func() { mustCall(t, "pool.dataset.create", map[string]any{"name": child}) },
				Config:    acctest.ProviderConfig(),
				Check:     checkGone(name, child),
			},
		},
	})
}

// TestAccDataset_volumeDeleteRefusedWithSnapshot: a type = "VOLUME" dataset goes
// through the same delete: a snapshot blocks it, force_destroy = true (a
// state-only update, so no pool.dataset.update is sent for the volume) lets it
// through.
func TestAccDataset_volumeDeleteRefusedWithSnapshot(t *testing.T) {
	name := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-ds-fdv"))
	snap := name + "@keep"
	vol := func(extra string) string {
		return datasetCfg(name, "type = \"VOLUME\"\n  volsize = 67108864\n  "+extra)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t); cleanupRecursive(t, name) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             checkGone(name, snap),
		Steps: []resource.TestStep{
			{Config: vol("")},
			{
				PreConfig:   func() { mustCall(t, "pool.snapshot.create", map[string]any{"dataset": name, "name": "keep"}) },
				Config:      acctest.ProviderConfig(),
				ExpectError: regexp.MustCompile(`(?s)dataset "` + regexp.QuoteMeta(name) + `" has 1 snapshot.*` + regexp.QuoteMeta(snap)),
			},
			{
				Config:           vol("force_destroy = true"),
				ConfigPlanChecks: expectUpdate("truenas_dataset.test"),
				Check:            checkExists(name, snap),
			},
			{
				Config: acctest.ProviderConfig(),
				Check:  checkGone(name, snap),
			},
		},
	})
}

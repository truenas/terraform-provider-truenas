// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset_test

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func importTestName(p string) string {
	return fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tfhunt-"+p))
}

func emptyPlan() resource.ConfigPlanChecks {
	return resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}
}

// importPersistNoReplace imports into the working state with an import block
// against the identical config. The import plan must not replace the dataset,
// and may change only the attributes in allowed: attributes the API does not
// return (share_type) or returns in a different letter case than the config
// (type/acltype/compression read back lower case). Terraform cannot plan a
// no-op for those, since a planned value must equal the configured one; the
// apply records them in state without an API call, and the next plan must be
// empty.
func importPersistNoReplace(addr, id, cfg string, allowed ...string) []resource.TestStep {
	// Forget the object (removed block, destroy=false), then import it.
	return []resource.TestStep{
		{
			Config: acctest.ProviderConfig() + fmt.Sprintf(`
removed {
  from = %s
  lifecycle {
    destroy = false
  }
}
`, addr),
		},
		{
			Config: cfg + fmt.Sprintf(`
import {
  to = %s
  id = %q
}
`, addr, id),
			ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
				acctest.ExpectNoReplace(addr),
				acctest.ExpectOnlyAttrChanges(addr, allowed...),
			}},
		},
		{Config: cfg, ConfigPlanChecks: emptyPlan()},
	}
}

// H4: import with share_type in config -> RequiresReplace null->"SMB" planned a
// destroy/recreate of the imported dataset. (B-002)
func TestAccDataset_importShareType(t *testing.T) {
	name := importTestName("st")
	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "test" {
  name       = %q
  share_type = "SMB"
}
`, name)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: append([]resource.TestStep{{Config: cfg}},
			importPersistNoReplace("truenas_dataset.test", name, cfg, "share_type")...),
	})
}

// H4b: the repo's own ImportReapplyNoop helper does NOT persist the imported
// state, so it passes on the exact config H4 fails on.
func TestAccDataset_importShareTypeViaHelper(t *testing.T) {
	name := importTestName("sth")
	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "test" {
  name       = %q
  share_type = "SMB"
}
`, name)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: append([]resource.TestStep{{Config: cfg}},
			acctest.ImportReapplyNoop("truenas_dataset.test", cfg, "share_type")...),
	})
}

// H5: documented "case-insensitive" type/acltype/compression in UPPER case;
// import lowercases them (preserveCase with null current), so the plan after
// import differs — and type/acltype were RequiresReplace. (B-002)
func TestAccDataset_importUppercase(t *testing.T) {
	name := importTestName("uc")
	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "test" {
  name        = %q
  type        = "FILESYSTEM"
  acltype     = "NFSV4"
  aclmode     = "PASSTHROUGH" # 25.10: NFSv4 under a POSIX pool needs it set
  compression = "LZ4"
}
`, name)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: append([]resource.TestStep{
			{Config: cfg},
			{Config: cfg, ConfigPlanChecks: emptyPlan()},
		}, importPersistNoReplace("truenas_dataset.test", name, cfg, "type", "acltype", "compression")...),
	})
}

// H9: a RequiresReplace change (acltype) on a parent dataset destroyed it with
// pool.dataset.delete recursive=true, silently wiping child datasets that are
// not managed by this resource. (B-001) The replacement must now fail with an
// error naming the child, the child must survive, and once the child is gone
// the same replacement goes through.
func TestAccDataset_replaceWipesChildren(t *testing.T) {
	name := importTestName("rep")
	child := name + "/keepme"
	// nfsv4 -> posix. aclmode is set explicitly so the configs create on every
	// version: 26.0+ requires DISCARD with POSIX (B-148), and 25.10 rejects an
	// NFSv4 dataset whose aclmode is inherited as DISCARD from a POSIX pool.
	cfg := func(acl string) string {
		aclmode := `aclmode = "PASSTHROUGH"`
		if acl == "posix" {
			aclmode = `aclmode = "DISCARD"`
		}
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "test" {
  name    = %q
  acltype = %q
  %s
}
`, name, acl, aclmode)
	}
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.PreCheck(t)
			t.Cleanup(func() {
				_, _ = acctest.Client().CallJob(context.Background(), "pool.dataset.delete", name, map[string]any{"recursive": true})
			})
		},
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg("nfsv4")},
			{
				PreConfig: func() {
					if _, err := acctest.Client().Call(context.Background(), "pool.dataset.create", map[string]any{"name": child}); err != nil {
						t.Fatalf("child create: %v", err)
					}
				},
				Config:      cfg("posix"),
				ExpectError: regexp.MustCompile(regexp.QuoteMeta(child)),
			},
			{
				PreConfig: func() {
					raw, err := acctest.Client().Call(context.Background(), "pool.dataset.query", [][]any{{"name", "=", child}})
					if err != nil {
						t.Fatalf("child query: %v", err)
					}
					if string(raw) == "[]" {
						t.Fatalf("replacing %s (acltype change) destroyed unmanaged child %s", name, child)
					}
					if _, err := acctest.Client().CallJob(context.Background(), "pool.dataset.delete", child, map[string]any{"recursive": false}); err != nil {
						t.Fatalf("child delete: %v", err)
					}
				},
				Config: cfg("posix"),
				Check:  resource.TestCheckResourceAttr("truenas_dataset.test", "acltype", "posix"),
			},
		},
	})
}

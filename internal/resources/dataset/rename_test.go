// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset_test

// Hunt finding F8: renaming a dataset in config used to be a replacement that
// destroyed the data. The ZFS guid is now the resource identity, so a rename
// is an in-place pool.dataset.rename: data, child datasets and snapshots move
// with it and the guid is unchanged. TrueNAS performs no safety checks on a
// rename, so the provider refuses one while shares, extents or tasks still
// point at the dataset unless allow_rename_in_use = true. The v1.6.1 destroy
// guard (a non-empty dataset needs force_destroy) is unchanged.

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

// snapshotCreate / snapshotExists use pool.snapshot.* (25.10/26.0) and fall
// back to zfs.snapshot.* (28.0 renamed the namespace).
func snapshotCreate(ctx context.Context, c *client.Client, dataset, name string) error {
	_, err := c.Call(ctx, "pool.snapshot.create", map[string]any{"dataset": dataset, "name": name})
	if err != nil && isMissingMethod(err) {
		_, err = c.Call(ctx, "zfs.snapshot.create", map[string]any{"dataset": dataset, "name": name})
	}
	return err
}

func snapshotExists(ctx context.Context, c *client.Client, full string) (bool, error) {
	raw, err := c.Call(ctx, "pool.snapshot.query", []any{[]any{"name", "=", full}})
	if err != nil && isMissingMethod(err) {
		raw, err = c.Call(ctx, "zfs.snapshot.query", []any{[]any{"name", "=", full}})
	}
	if err != nil {
		return false, err
	}
	return string(raw) != "[]", nil
}

func isMissingMethod(err error) bool {
	var apiErr *client.APIError
	return err != nil && ((errorsAs(err, &apiErr) && apiErr.Code == -32601) || regexp.MustCompile(`Method does not exist`).MatchString(err.Error()))
}

func errorsAs(err error, target **client.APIError) bool {
	e, ok := err.(*client.APIError)
	if ok {
		*target = e
	}
	return ok
}

// datasetExists reports whether pool.dataset.query knows the name.
func datasetExists(ctx context.Context, c *client.Client, name string) (bool, error) {
	raw, err := c.Call(ctx, "pool.dataset.query", []any{[]any{"id", "=", name}}, map[string]any{"extra": map[string]any{"retrieve_children": false}})
	if err != nil {
		return false, err
	}
	return string(raw) != "[]", nil
}

// stateAttr reads one attribute of a resource from the test state.
func stateAttr(s *terraform.State, addr, attr string) (string, error) {
	rs, ok := s.RootModule().Resources[addr]
	if !ok {
		return "", fmt.Errorf("%s not in state", addr)
	}
	v, ok := rs.Primary.Attributes[attr]
	if !ok {
		return "", fmt.Errorf("%s has no %s in state", addr, attr)
	}
	return v, nil
}

// TestAccDataset_renameInPlace renames a dataset that holds a file, a managed
// child dataset (whose own name changes with it), an unmanaged grandchild and a
// snapshot: everything moves, the guid and identity are unchanged, id and
// mountpoint follow the name, and the plan is empty afterwards.
func TestAccDataset_renameInPlace(t *testing.T) {
	acctest.PreCheck(t)
	base := acctest.TestPool() + "/" + acctest.RandName("tfaccren")
	renamed := base + "-renamed"
	cfg := func(name string) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "parent" {
  name          = %[1]q
  comments      = "rename test"
  force_destroy = true # unmanaged grandchild + snapshot
}
resource "truenas_dataset" "child" {
  name          = "%[1]s/child"
  comments      = "child" # own value: an unset comments reads the parent's inherited one (F12)
  force_destroy = true    # unmanaged grandchild
  depends_on    = [truenas_dataset.parent]
}
`, name)
	}
	c := acctest.Client()
	ctx := context.Background()
	var parentGuid, childGuid string
	// Known bytes: read back over ssh after the rename and compared, so the
	// test proves the data moved, not only that a file of that name exists.
	content := "precious data written before the rename " + acctest.RandName("payload") + "\n"
	plant := func(s *terraform.State) error {
		if _, err := c.Call(ctx, "filesystem.file_receive", "/mnt/"+base+"/keep.txt",
			base64.StdEncoding.EncodeToString([]byte(content)), map[string]any{"mode": 0o644}); err != nil {
			return err
		}
		if _, err := c.Call(ctx, "pool.dataset.create", map[string]any{"name": base + "/child/gc"}); err != nil {
			return err
		}
		if err := snapshotCreate(ctx, c, base, "keep"); err != nil {
			return err
		}
		var err error
		if parentGuid, err = stateAttr(s, "truenas_dataset.parent", "guid"); err != nil {
			return err
		}
		if childGuid, err = stateAttr(s, "truenas_dataset.child", "guid"); err != nil {
			return err
		}
		if parentGuid == "" || childGuid == "" || parentGuid == childGuid {
			return fmt.Errorf("guids not recorded: parent %q child %q", parentGuid, childGuid)
		}
		return nil
	}
	survived := func(s *terraform.State) error {
		if _, err := c.Call(ctx, "filesystem.stat", "/mnt/"+renamed+"/keep.txt"); err != nil {
			return fmt.Errorf("the file did not move with the rename: %w", err)
		}
		if got := acctest.SSH(t, "cat /mnt/"+renamed+"/keep.txt"); got != content {
			return fmt.Errorf("file content after rename %q, want %q", got, content)
		}
		if ok, err := datasetExists(ctx, c, renamed+"/child/gc"); err != nil || !ok {
			return fmt.Errorf("unmanaged grandchild did not move with the rename (exists=%v, err=%v)", ok, err)
		}
		if ok, err := snapshotExists(ctx, c, renamed+"@keep"); err != nil || !ok {
			return fmt.Errorf("snapshot did not move with the rename (exists=%v, err=%v)", ok, err)
		}
		if ok, err := datasetExists(ctx, c, base); err != nil || ok {
			return fmt.Errorf("old name %s still exists (exists=%v, err=%v)", base, ok, err)
		}
		for addr, want := range map[string]string{"truenas_dataset.parent": parentGuid, "truenas_dataset.child": childGuid} {
			got, err := stateAttr(s, addr, "guid")
			if err != nil {
				return err
			}
			if got != want {
				return fmt.Errorf("%s guid changed on rename: %s -> %s (a different ZFS object: the data would be gone)", addr, want, got)
			}
		}
		return nil
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg(base), Check: resource.ComposeTestCheckFunc(plant,
				resource.TestCheckResourceAttr("truenas_dataset.parent", "id", base),
				resource.TestCheckResourceAttr("truenas_dataset.parent", "mountpoint", "/mnt/"+base),
			)},
			{
				Config: cfg(renamed),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("truenas_dataset.parent", plancheck.ResourceActionUpdate),
					plancheck.ExpectResourceAction("truenas_dataset.child", plancheck.ResourceActionUpdate),
					// The new mountpoint is planned, not (known after apply), so
					// a consumer that uses it is not replaced needlessly.
					plancheck.ExpectKnownValue("truenas_dataset.parent", tfjsonpath.New("mountpoint"), knownvalue.StringExact("/mnt/"+renamed)),
					plancheck.ExpectKnownValue("truenas_dataset.parent", tfjsonpath.New("id"), knownvalue.StringExact(renamed)),
				}},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectIdentityValue("truenas_dataset.parent", tfjsonpath.New("guid"),
						knownvalue.StringFunc(func(v string) error {
							if v != parentGuid {
								return fmt.Errorf("identity guid %s, want %s", v, parentGuid)
							}
							return nil
						})),
					statecheck.ExpectIdentityValueMatchesState("truenas_dataset.child", tfjsonpath.New("guid")),
				},
				Check: resource.ComposeTestCheckFunc(survived,
					resource.TestCheckResourceAttr("truenas_dataset.parent", "id", renamed),
					resource.TestCheckResourceAttr("truenas_dataset.parent", "name", renamed),
					resource.TestCheckResourceAttr("truenas_dataset.parent", "mountpoint", "/mnt/"+renamed),
					resource.TestCheckResourceAttr("truenas_dataset.parent", "comments", "rename test"),
					resource.TestCheckResourceAttr("truenas_dataset.child", "name", renamed+"/child"),
					resource.TestCheckResourceAttr("truenas_dataset.child", "mountpoint", "/mnt/"+renamed+"/child"),
				),
			},
			{Config: cfg(renamed), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			// Rename back together with a property change in the same apply.
			{
				Config: acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "parent" {
  name          = %[1]q
  comments      = "renamed back"
  force_destroy = true
}
resource "truenas_dataset" "child" {
  name          = "%[1]s/child"
  comments      = "child"
  force_destroy = true
  depends_on    = [truenas_dataset.parent]
}
`, base),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_dataset.parent", "comments", "renamed back"),
					func(s *terraform.State) error {
						// parentGuid is only known at run time: compare in a closure.
						g, err := stateAttr(s, "truenas_dataset.parent", "guid")
						if err == nil && g != parentGuid {
							err = fmt.Errorf("guid after rename back %s, want %s", g, parentGuid)
						}
						return err
					},
					func(*terraform.State) error {
						if got := acctest.SSH(t, "cat /mnt/"+base+"/keep.txt"); got != content {
							return fmt.Errorf("file content after renaming back %q, want %q", got, content)
						}
						return nil
					},
				),
			},
		},
	})
}

// TestAccDataset_renameWithReplaceRefusedWhileNotEmpty is the combined case:
// the config changes name AND a replace-forcing attribute (acltype) with no
// force_destroy, so the plan is a replacement. The destroy half must be refused
// by the v1.6.1 guard and the original dataset must still exist under its old
// name with its file intact. Before the guard this test would have been red:
// the old dataset was destroyed with its contents and an empty one created.
func TestAccDataset_renameWithReplaceRefusedWhileNotEmpty(t *testing.T) {
	acctest.PreCheck(t)
	base := acctest.TestPool() + "/" + acctest.RandName("tfaccrenr")
	renamed := base + "-renamed"
	cfg := func(name, acltype string, force bool) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "d" {
  name          = %q
  acltype       = %q
  force_destroy = %t
}
`, name, acltype, force)
	}
	c := acctest.Client()
	ctx := context.Background()
	content := "must survive a refused replacement " + acctest.RandName("payload") + "\n"
	intact := func(*terraform.State) error {
		if ok, err := datasetExists(ctx, c, base); err != nil || !ok {
			return fmt.Errorf("original dataset %s is gone (exists=%v, err=%v): the refused replacement destroyed it", base, ok, err)
		}
		if ok, err := datasetExists(ctx, c, renamed); err != nil || ok {
			return fmt.Errorf("a dataset exists under the new name %s (exists=%v, err=%v) although the apply was refused", renamed, ok, err)
		}
		if got := acctest.SSH(t, "cat /mnt/"+base+"/keep.txt"); got != content {
			return fmt.Errorf("file content %q, want %q", got, content)
		}
		return nil
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg(base, "posix", false), Check: func(*terraform.State) error {
				_, err := c.Call(ctx, "filesystem.file_receive", "/mnt/"+base+"/keep.txt",
					base64.StdEncoding.EncodeToString([]byte(content)), map[string]any{"mode": 0o644})
				return err
			}},
			{
				Config: cfg(renamed, "nfsv4", false),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("truenas_dataset.d", plancheck.ResourceActionReplace),
				}},
				ExpectError: regexp.MustCompile(`Delete dataset refused`),
			},
			{Config: cfg(base, "posix", false), Check: intact,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{Config: cfg(base, "posix", true)},
		},
	})
}

// TestAccDataset_renameRefusedInUse: a rename is refused while an SMB share
// points at the dataset (TrueNAS would leave the share broken), allowed with
// allow_rename_in_use = true (the share is then stale), and a share that
// references the mountpoint plans its own replacement on the next apply.
func TestAccDataset_renameRefusedInUse(t *testing.T) {
	acctest.PreCheck(t)
	base := acctest.TestPool() + "/" + acctest.RandName("tfaccrenu")
	renamed := base + "-renamed"
	share := acctest.RandName("tfaccrenu")
	cfg := func(name, sharePath string, allow bool) string {
		allowLine := ""
		if allow {
			allowLine = "  allow_rename_in_use = true\n"
		}
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "d" {
  name = %q
%s}
resource "truenas_smb_share" "s" {
  name       = %q
  path       = %s
  depends_on = [truenas_dataset.d] # the path is a literal in the first steps
}
`, name, allowLine, share, sharePath)
	}
	c := acctest.Client()
	ctx := context.Background()
	sharePath := func() (string, error) {
		raw, err := c.Call(ctx, "sharing.smb.query", []any{[]any{"name", "=", share}})
		if err != nil {
			return "", err
		}
		var rows []struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(raw, &rows); err != nil || len(rows) != 1 {
			return "", fmt.Errorf("share %s: %d rows, %v", share, len(rows), err)
		}
		return rows[0].Path, nil
	}
	literal := fmt.Sprintf("%q", "/mnt/"+base)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg(base, literal, false)},
			// Refused: the error names the share; nothing is renamed.
			{
				Config:      cfg(renamed, literal, false),
				ExpectError: regexp.MustCompile(`(?s)Rename of dataset refused: it is in use.*SMB share "` + share + `"`),
			},
			{Config: cfg(base, literal, false), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
				Check: func(*terraform.State) error {
					if ok, err := datasetExists(ctx, c, base); err != nil || !ok {
						return fmt.Errorf("dataset %s missing after the refused rename (exists=%v, err=%v)", base, ok, err)
					}
					return nil
				}},
			// Allowed: renamed; TrueNAS leaves the share at the old path.
			{
				Config: cfg(renamed, literal, true),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("truenas_dataset.d", plancheck.ResourceActionUpdate),
					plancheck.ExpectResourceAction("truenas_smb_share.s", plancheck.ResourceActionNoop),
				}},
				Check: func(*terraform.State) error {
					p, err := sharePath()
					if err != nil {
						return err
					}
					if p != "/mnt/"+base {
						return fmt.Errorf("share path is %s; expected TrueNAS to leave it at the old path /mnt/%s (documentation claims so)", p, base)
					}
					return nil
				},
			},
			// Re-pointing the share at the dataset's mountpoint plans its
			// replacement (path forces a new share) and nothing on the dataset.
			{
				Config: cfg(renamed, "truenas_dataset.d.mountpoint", true),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("truenas_dataset.d", plancheck.ResourceActionNoop),
					plancheck.ExpectResourceAction("truenas_smb_share.s", plancheck.ResourceActionReplace),
				}},
				Check: func(*terraform.State) error {
					p, err := sharePath()
					if err != nil {
						return err
					}
					if p != "/mnt/"+renamed {
						return fmt.Errorf("share path %s, want /mnt/%s", p, renamed)
					}
					return nil
				},
			},
			{Config: cfg(renamed, "truenas_dataset.d.mountpoint", true), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}

// TestAccDataset_renameRefusedAcrossPools: ZFS cannot rename across pools, so
// the plan fails before anything is touched.
func TestAccDataset_renameRefusedAcrossPools(t *testing.T) {
	acctest.PreCheck(t)
	base := acctest.TestPool() + "/" + acctest.RandName("tfaccrenp")
	cfg := func(name string) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "d" {
  name = %q
}
`, name)
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg(base)},
			{Config: cfg("otherpool/" + acctest.RandName("tfaccrenp")), PlanOnly: true, ExpectError: regexp.MustCompile(`cannot be moved to another pool`)},
			{Config: cfg(base + "/inside"), PlanOnly: true, ExpectError: regexp.MustCompile(`cannot be renamed into itself`)},
		},
	})
}

// TestAccDataset_importByNameAndGuid: import by name (the id), by ZFS guid and
// by identity { guid } all land on the same dataset with an empty plan after.
func TestAccDataset_importByNameAndGuid(t *testing.T) {
	acctest.PreCheck(t)
	name := acctest.TestPool() + "/" + acctest.RandName("tfaccimpg")
	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "d" {
  name     = %q
  comments = "import by guid"
}
`, name)
	var guid string
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg, Check: func(s *terraform.State) error {
				var err error
				guid, err = stateAttr(s, "truenas_dataset.d", "guid")
				if err == nil && guid == "" {
					err = fmt.Errorf("guid empty in state")
				}
				return err
			}},
			{Config: cfg, ResourceName: "truenas_dataset.d", ImportState: true, ImportStateId: name, ImportStateVerify: true},
			{Config: cfg, ResourceName: "truenas_dataset.d", ImportState: true, ImportStateVerify: true,
				ImportStateIdFunc: func(*terraform.State) (string, error) { return guid, nil }},
			{Config: cfg, ResourceName: "truenas_dataset.d", ImportState: true, ImportStateKind: resource.ImportBlockWithResourceIdentity},
			{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}

// TestAccDataset_destroyRefusedWhileNotEmpty keeps the v1.6.1 destroy guard
// under test: a removed resource (or a replacement) that still holds files is
// refused unless force_destroy is set.
func TestAccDataset_destroyRefusedWhileNotEmpty(t *testing.T) {
	acctest.PreCheck(t)
	base := acctest.TestPool() + "/" + acctest.RandName("tfaccdne")
	cfg := func(force bool) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "d" {
  name          = %q
  force_destroy = %t
}
`, base, force)
	}
	c := acctest.Client()
	ctx := context.Background()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg(false), Check: func(*terraform.State) error {
				_, err := c.Call(ctx, "filesystem.file_receive", "/mnt/"+base+"/keep.txt",
					base64.StdEncoding.EncodeToString([]byte("precious")), map[string]any{"mode": 0o644})
				return err
			}},
			{Config: cfg(false), Destroy: true, ExpectError: regexp.MustCompile(`not empty`)},
			{Config: cfg(false), Check: func(*terraform.State) error {
				_, err := c.Call(ctx, "filesystem.stat", "/mnt/"+base+"/keep.txt")
				return err
			}},
			{Config: cfg(true)},
		},
	})
}

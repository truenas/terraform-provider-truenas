// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package provider_test

// TestDestructiveCalls_Guarded is the ratchet behind "this provider must be
// very resistant to destroying actual data". It parses every non-test Go file
// under internal/ and finds each call site of an API method that can destroy
// user data (or an option literal that turns a benign call destructive). Every
// such site must sit inside a function listed in destructiveCallSites with a
// reason naming the guard that stands in front of it. A new call site, or an
// old one that moved, fails this test until it is guarded and listed.
//
// This complements the per-resource guards: dataset/zvol (force_destroy +
// NonEmptyContents/ZvolHoldsData), pool (force_destroy + poolContents),
// snapshot (allow_recreate at plan time).

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// destructiveMethods are API methods whose success removes data or a data
// container. Keyed by the literal the provider passes to Call/CallJob.
var destructiveMethods = map[string]bool{
	"pool.export":              true, // with destroy=true erases the pool
	"pool.dataset.delete":      true,
	"pool.dataset.rename":      true, // no server-side safety checks; breaks dependants
	"pool.snapshot.delete":     true,
	"zfs.snapshot.delete":      true,
	"pool.snapshot.rename":     true,
	"pool.snapshot.rollback":   true,
	"zfs.resource.destroy":     true,
	"iscsi.extent.delete":      true, // remove=true deletes the backing file
	"nvmet.namespace.delete":   true, // remove=true deletes the backing file
	"vm.delete":                true, // zvols=true deletes the disks
	"vm.device.delete":         true, // zvol/raw_file=true delete the backing storage
	"app.delete":               true, // remove_ix_volumes=true deletes app data
	"container.delete":         true, // takes the container's root filesystem with it
	"boot.environment.destroy": true,
	"disk.wipe":                true,
	"disk.format":              true,
	"pool.remove":              true,
	"filesystem.setacl":        true, // recursive rewrites permissions on whole trees
	"filesystem.setperm":       true,
	"filesystem.chown":         true,
}

// destructiveOptions are option keys that make an otherwise benign call
// destroy data when set to true.
var destructiveOptions = map[string]bool{
	"destroy":           true, // pool.export
	"zvols":             true, // vm.delete
	"remove_ix_volumes": true, // app.delete
	"remove":            true, // iscsi.extent.delete, nvmet.namespace.delete
	"zvol":              true, // vm.device.delete
	"raw_file":          true, // vm.device.delete
	"recursive":         true, // pool.dataset.delete, pool.snapshot.delete (when true)
	"force":             true, // pool.dataset.rename, pool.snapshot.rename
}

// destructiveCallSites: "<package dir>.<enclosing function>" -> why it is safe.
// The function must contain, or be called only after, the guard named here.
var destructiveCallSites = map[string]string{
	"dataset.DeleteDataset":                                                "the only pool.dataset.delete call; recursive only when force_destroy, else refused while children/snapshots/contents exist (NonEmptyContents, ZvolHoldsData in the resources' Delete)",
	"dataset.Rename":                                                       "pool.dataset.rename with force (the API refuses every rename without it) and recursive=false; the only caller is dataset.PerformRename, which runs RenameBlockers first unless allow_rename_in_use",
	"pool.(*PoolResource).Delete":                                          "pool.export destroy=true only after poolContents is empty or force_destroy = true",
	"snapshot.(*SnapshotResource).Delete":                                  "a Terraform-managed snapshot is destroyed only on resource destroy; replacement is refused at plan time unless allow_recreate (ModifyPlan)",
	"iscsi_extent.(*ISCSIExtentResource).Delete":                           "remove=false: the backing file/zvol is kept",
	"nvmet_namespace.(*NVMetNamespaceResource).Delete":                     "no remove option: the backing file/zvol is kept",
	"vm.(*VMResource).Delete":                                              "zvols=false: disks are kept",
	"vm_device.(*VMDeviceResource).Delete":                                 "zvol=false, raw_file=false: backing storage is kept",
	"app.(*AppResource).Delete":                                            "remove_ix_volumes=false: app data is kept (images are removed)",
	"container.(*ContainerResource).Delete":                                "the container's own root filesystem goes with it, like a VM's config; disks attached from datasets are not touched",
	"boot_environment.(*BootEnvironmentResource).Delete":                   "refuses the active or activated boot environment",
	"filesystem_acl.(*FilesystemAclResource).applyACL":                     "setacl of the configured path; recursive/traverse only when the configuration says so (permissions, not data)",
	"filesystem_acl.(*FilesystemAclResource).Delete":                       "strips the ACL of the configured path on destroy (permissions, not data); documented on the resource",
	"filesystem_permissions.(*FilesystemPermissionsResource).applyAndStat": "setperm of the configured path; recursive/traverse only when the configuration says so (permissions, not data)",
	// Actions run only when the user triggers them (lifecycle action_trigger /
	// terraform apply -invoke); they are the explicit opt-in. Each passes the
	// API's own force flag through from configuration, never defaults it on.
	"dataset_rename.(*Action).Invoke":    "user-invoked action; force passed through from configuration",
	"snapshot_rename.(*Action).Invoke":   "user-invoked action; force passed through from configuration",
	"snapshot_rollback.(*Action).Invoke": "user-invoked action whose purpose is to discard data after the snapshot; the description says so",
}

func TestDestructiveCalls_Guarded(t *testing.T) {
	root := repoRootDir(t)
	var found []string // "pkg.func: method or option (file:line)"
	seen := map[string]bool{}
	err := filepath.WalkDir(filepath.Join(root, "internal"), func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		if strings.Contains(p, "/loadtest/") || strings.Contains(p, "/cmd/") {
			return nil // test-only tooling, not shipped in the provider binary
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			return err
		}
		pkg := filepath.Base(filepath.Dir(p))
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			name := pkg + "." + funcName(fn)
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.BasicLit:
					if x.Kind != token.STRING {
						return true
					}
					s := strings.Trim(x.Value, "`\"")
					if destructiveMethods[s] {
						seen[name] = true
						if _, ok := destructiveCallSites[name]; !ok {
							found = append(found, fmt.Sprintf("%s: method %q (%s)", name, s, fset.Position(x.Pos())))
						}
					}
				case *ast.KeyValueExpr:
					k, ok := x.Key.(*ast.BasicLit)
					if !ok || k.Kind != token.STRING {
						return true
					}
					key := strings.Trim(k.Value, "`\"")
					v, isIdent := x.Value.(*ast.Ident)
					if destructiveOptions[key] && isIdent && v.Name == "true" {
						seen[name] = true
						if _, ok := destructiveCallSites[name]; !ok {
							found = append(found, fmt.Sprintf("%s: option %q: true (%s)", name, key, fset.Position(x.Pos())))
						}
					}
				}
				return true
			})
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(found)
	for _, f := range found {
		t.Errorf("unguarded destructive call site: %s — put it behind an explicit opt-in (force_destroy / allow_*) and list it in destructiveCallSites with the guard", f)
	}
	for site, reason := range destructiveCallSites {
		if !seen[site] {
			t.Errorf("%s is listed in destructiveCallSites (%q) but makes no destructive call any more; drop the entry", site, reason)
		}
	}
}

func funcName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	recv := ""
	switch t := fn.Recv.List[0].Type.(type) {
	case *ast.StarExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			recv = "(*" + id.Name + ")"
		}
	case *ast.Ident:
		recv = t.Name
	}
	return recv + "." + fn.Name.Name
}

func repoRootDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

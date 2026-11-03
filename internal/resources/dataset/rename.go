// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset

// In-place rename for truenas_dataset and truenas_zvol (hunt finding F8).
//
// The ZFS guid is the resource identity: it survives pool.dataset.rename
// (verified identical on TrueNAS 25.10, 26.0 and 28.0), while the name — the
// Terraform id — changes. A rename is therefore an in-place update rather than
// a replacement, and the dataset keeps its data, child datasets and snapshots.
//
// pool.dataset.rename performs no safety checks ("No safety checks are
// performed when renaming ZFS resources"): it always needs force=true and it
// does not re-point SMB/NFS shares, iSCSI extents, NVMe-oF namespaces,
// snapshot/replication/rsync/cloud-sync tasks or VM disks at the old path. The
// provider does the check itself (RenameBlockers) and refuses the rename while
// anything references the dataset, unless allow_rename_in_use = true.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

// api is the slice of *client.Client the rename helpers use; a fake drives
// the unit tests.
type api interface {
	Call(ctx context.Context, method string, params ...any) (json.RawMessage, error)
	CallRead(ctx context.Context, method string, params ...any) (json.RawMessage, error)
}

var _ api = (*client.Client)(nil)

// IdentityVersion is the resource identity schema version of truenas_dataset
// and truenas_zvol: v0 (<= 1.6.1) was {id = name}, v1 is {guid}.
const IdentityVersion = 1

// IdentitySchema is the identity schema shared by truenas_dataset and
// truenas_zvol: the ZFS guid, which a rename does not change.
func IdentitySchema() identityschema.Schema {
	return identityschema.Schema{
		Version: IdentityVersion,
		Attributes: map[string]identityschema.Attribute{
			"guid": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "ZFS guid of the dataset (pool.dataset.query guid property). Stable across renames.",
			},
		},
	}
}

// identityModel is the v1 identity value.
type identityModel struct {
	Guid types.String `tfsdk:"guid"`
}

// priorIdentitySchema is identity schema v0: the dataset name under "id".
func priorIdentitySchema() identityschema.Schema {
	return identityschema.Schema{Attributes: map[string]identityschema.Attribute{
		"id": identityschema.StringAttribute{RequiredForImport: true},
	}}
}

// SetGuidIdentity writes the guid onto a resource identity.
func SetGuidIdentity(ctx context.Context, ri *tfsdk.ResourceIdentity, guid string) diag.Diagnostics {
	if ri == nil {
		return nil
	}
	return ri.SetAttribute(ctx, path.Root("guid"), guid)
}

// IdentityUpgraders upgrades identity v0 {id = name} written by <= 1.6.1 to
// v1 {guid}: the guid is looked up by name. A dataset that no longer exists
// gets an empty guid, which Read resolves by name and then removes from state.
func IdentityUpgraders(c func() *client.Client) map[int64]resource.IdentityUpgrader {
	prior := priorIdentitySchema()
	return map[int64]resource.IdentityUpgrader{
		0: {
			PriorSchema: &prior,
			IdentityUpgrader: func(ctx context.Context, req resource.UpgradeIdentityRequest, resp *resource.UpgradeIdentityResponse) {
				var name string
				if req.Identity != nil {
					resp.Diagnostics.Append(req.Identity.GetAttribute(ctx, path.Root("id"), &name)...)
				}
				if resp.Diagnostics.HasError() {
					return
				}
				guid := ""
				if name != "" && c() != nil {
					var d diag.Diagnostics
					guid, d = upgradeIdentityGuid(ctx, c(), name)
					resp.Diagnostics.Append(d...)
					if resp.Diagnostics.HasError() {
						return
					}
				}
				// Set, not SetAttribute: the response identity has no value yet.
				resp.Diagnostics.Append(resp.Identity.Set(ctx, identityModel{Guid: types.StringValue(guid)})...)
			},
		},
	}
}

// upgradeIdentityGuid looks up the guid for the identity upgrader: a dataset
// that no longer exists yields "" (Read then removes it from state); any other
// failure is an error, since a wrong identity must not be written.
func upgradeIdentityGuid(ctx context.Context, c api, name string) (string, diag.Diagnostics) {
	var diags diag.Diagnostics
	g, err := LookupGuid(ctx, c, name)
	if err != nil {
		if client.IsNotFound(err) {
			return "", diags
		}
		diags.AddError("Upgrade resource identity failed", fmt.Sprintf("looking up the ZFS guid of %q: %v", name, err))
		return "", diags
	}
	return g, diags
}

// guidRow is the row pool.dataset.query returns with extra.properties=["guid"].
type guidRow struct {
	Name string `json:"name"`
	Guid struct {
		Value string `json:"value"`
	} `json:"guid"`
}

// guidQueryOpts asks pool.dataset.query for the guid property only (the
// property list replaces the default set, so the rows carry little else).
func guidQueryOpts(flat bool) map[string]any {
	opts := map[string]any{"extra": map[string]any{"properties": []string{"guid"}, "retrieve_children": false}}
	if flat {
		opts["extra"].(map[string]any)["flat"] = true
	}
	return opts
}

// LookupGuid returns the ZFS guid of the dataset or zvol with this name. A
// missing dataset is reported with an error client.IsNotFound recognises.
func LookupGuid(ctx context.Context, c api, name string) (string, error) {
	raw, err := c.CallRead(ctx, "pool.dataset.query", [][]any{{"id", "=", name}}, guidQueryOpts(false))
	if err != nil {
		return "", err
	}
	var rows []guidRow
	if err := json.Unmarshal(raw, &rows); err != nil {
		return "", fmt.Errorf("parse pool.dataset.query: %w", err)
	}
	if len(rows) == 0 || rows[0].Guid.Value == "" {
		return "", notFound("dataset " + name)
	}
	return rows[0].Guid.Value, nil
}

// NameByGuid returns the current name of the dataset or zvol with this ZFS
// guid, following it across renames. A guid no dataset has is reported with an
// error client.IsNotFound recognises.
func NameByGuid(ctx context.Context, c api, guid string) (string, error) {
	raw, err := c.CallRead(ctx, "pool.dataset.query", [][]any{{"guid.value", "=", guid}}, guidQueryOpts(true))
	if err != nil {
		return "", err
	}
	var rows []guidRow
	if err := json.Unmarshal(raw, &rows); err != nil {
		return "", fmt.Errorf("parse pool.dataset.query: %w", err)
	}
	if len(rows) == 0 {
		return "", notFound("dataset with guid " + guid)
	}
	return rows[0].Name, nil
}

// GuidsByName returns name -> guid for every dataset matching filter (the
// list resources join it onto the full rows, which carry no guid).
func GuidsByName(ctx context.Context, c api, filter [][]any) (map[string]string, error) {
	raw, err := c.CallRead(ctx, "pool.dataset.query", filter, guidQueryOpts(true))
	if err != nil {
		return nil, err
	}
	var rows []guidRow
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("parse pool.dataset.query: %w", err)
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		out[r.Name] = r.Guid.Value
	}
	return out, nil
}

// notFound is a missing dataset/guid in the form client.IsNotFound matches.
func notFound(what string) error {
	return &client.APIError{Code: 2, ErrName: "ENOENT", Message: "[ENOENT] " + what + " does not exist"}
}

// IsGuid reports whether an import id is a ZFS guid rather than a dataset
// name: a guid is a decimal uint64, a name is pool[/path].
func IsGuid(id string) bool {
	if id == "" || strings.Contains(id, "/") {
		return false
	}
	_, err := strconv.ParseUint(id, 10, 64)
	return err == nil && len(id) >= 10
}

// ResolveImportID resolves an import id (`terraform import` / import { id }
// / import { identity = { guid } }) to the dataset's current name and guid.
// The id may be the name or the guid.
func ResolveImportID(ctx context.Context, c api, req resource.ImportStateRequest) (name, guid string, diags diag.Diagnostics) {
	id := req.ID
	if id == "" && req.Identity != nil {
		var g types.String
		diags.Append(req.Identity.GetAttribute(ctx, path.Root("guid"), &g)...)
		if diags.HasError() {
			return "", "", diags
		}
		id = g.ValueString()
	}
	if id == "" {
		diags.AddError("Missing import id", "Provide the dataset name or ZFS guid as the import id, or an identity block with a guid.")
		return "", "", diags
	}
	var err error
	if IsGuid(id) {
		guid = id
		name, err = NameByGuid(ctx, c, guid)
	} else {
		name = id
		guid, err = LookupGuid(ctx, c, name)
	}
	if err != nil {
		diags.AddError("Import failed", err.Error())
	}
	return name, guid, diags
}

// Rename renames a dataset or zvol in place with pool.dataset.rename. force is
// mandatory (the API refuses every rename without it) and recursive must stay
// false: it means "rename this snapshot on every descendant" and the API
// rejects it for a dataset ("recursive rename must be a snapshot"). Children
// and snapshots move with the dataset anyway, as in zfs rename.
func Rename(ctx context.Context, c api, oldName, newName string) error {
	_, err := c.Call(ctx, "pool.dataset.rename", oldName, map[string]any{
		"new_name":  newName,
		"force":     true,
		"recursive": false,
	})
	return err
}

// PoolOf is the pool component of a dataset name.
func PoolOf(name string) string {
	pool, _, _ := strings.Cut(name, "/")
	return pool
}

// CheckRenamePlan is the ModifyPlan check shared by truenas_dataset and
// truenas_zvol for a planned name change: ZFS cannot rename across pools, so a
// new name under another pool is an error at plan time rather than a failed
// apply. Returns the planned (old, new) names and whether the name changes.
func CheckRenamePlan(ctx context.Context, req resource.ModifyPlanRequest, kind string) (oldName, newName string, changed bool, diags diag.Diagnostics) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return "", "", false, diags
	}
	var prior, planned types.String
	diags.Append(req.State.GetAttribute(ctx, path.Root("name"), &prior)...)
	diags.Append(req.Plan.GetAttribute(ctx, path.Root("name"), &planned)...)
	if diags.HasError() || prior.IsNull() || planned.IsNull() || planned.IsUnknown() || prior.ValueString() == planned.ValueString() {
		return "", "", false, diags
	}
	oldName, newName = prior.ValueString(), planned.ValueString()
	if PoolOf(oldName) != PoolOf(newName) {
		diags.AddAttributeError(path.Root("name"), fmt.Sprintf("A %s cannot be moved to another pool", kind),
			fmt.Sprintf("%s is on pool %q and the new name %q is on pool %q. ZFS renames only within a pool; to move "+
				"the %s, replicate it to the other pool and import the copy, or recreate it with terraform apply -replace "+
				"(its data is lost unless copied first).", oldName, PoolOf(oldName), newName, PoolOf(newName), kind))
		return oldName, newName, true, diags
	}
	if strings.HasPrefix(newName, oldName+"/") {
		diags.AddAttributeError(path.Root("name"), fmt.Sprintf("A %s cannot be renamed into itself", kind),
			fmt.Sprintf("%q is a descendant of %q.", newName, oldName))
	}
	return oldName, newName, true, diags
}

// RenameBlocker is one server-side object that references a dataset about to
// be renamed and that TrueNAS would leave pointing at the old path.
type RenameBlocker struct {
	Kind string // "SMB share", "iSCSI extent", ...
	Name string // the object's own name or id
	Ref  string // the referencing value (path, disk, dataset)
}

func (b RenameBlocker) String() string { return fmt.Sprintf("%s %s (%s)", b.Kind, b.Name, b.Ref) }

// RenameBlockers lists everything on the server that references the dataset
// or zvol `name` (mountpoint `mountpoint`, "" for a zvol) or one of its
// descendants: SMB and NFS shares, iSCSI extents, NVMe-oF namespaces,
// periodic snapshot, replication, rsync and cloud sync tasks, and VM disks.
// pool.dataset.rename updates none of them, so each would break on rename.
// A query method the release does not have is skipped; any other failure is
// returned, since "could not check" must not pass as "nothing in use".
func RenameBlockers(ctx context.Context, c api, name, mountpoint string) ([]RenameBlocker, error) {
	underName := func(v string) bool { return v != "" && (v == name || strings.HasPrefix(v, name+"/")) }
	underPath := func(v string) bool {
		return mountpoint != "" && v != "" && (v == mountpoint || strings.HasPrefix(v, mountpoint+"/"))
	}
	underZvol := func(v string) bool { return underName(strings.TrimPrefix(v, "zvol/")) }
	underDev := func(v string) bool { return underName(strings.TrimPrefix(v, "/dev/zvol/")) }

	var out []RenameBlocker
	type probe struct {
		method string
		kind   string
		match  func(row map[string]any) (string, bool)
	}
	str := func(row map[string]any, k string) string { s, _ := row[k].(string); return s }
	label := func(row map[string]any) string {
		if n := str(row, "name"); n != "" {
			return fmt.Sprintf("%q", n)
		}
		return "id " + fmt.Sprint(row["id"])
	}
	probes := []probe{
		{"sharing.smb.query", "SMB share", func(r map[string]any) (string, bool) {
			p := str(r, "path")
			return "path " + p, underPath(p)
		}},
		{"sharing.nfs.query", "NFS share", func(r map[string]any) (string, bool) {
			p := str(r, "path")
			return "path " + p, underPath(p)
		}},
		{"iscsi.extent.query", "iSCSI extent", func(r map[string]any) (string, bool) {
			if d := str(r, "disk"); underZvol(d) || underName(str(r, "dataset")) {
				return "disk " + d, true
			}
			p := str(r, "path")
			return "path " + p, underPath(p)
		}},
		{"nvmet.namespace.query", "NVMe-oF namespace", func(r map[string]any) (string, bool) {
			p := str(r, "device_path")
			return "device_path " + p, underZvol(p) || underName(str(r, "dataset")) || underPath(p)
		}},
		{"pool.snapshottask.query", "periodic snapshot task", func(r map[string]any) (string, bool) {
			d := str(r, "dataset")
			return "dataset " + d, underName(d)
		}},
		{"replication.query", "replication task", func(r map[string]any) (string, bool) {
			if srcs, ok := r["source_datasets"].([]any); ok {
				for _, s := range srcs {
					if ss, _ := s.(string); underName(ss) {
						return "source_datasets " + ss, true
					}
				}
			}
			t := str(r, "target_dataset")
			return "target_dataset " + t, underName(t)
		}},
		{"rsynctask.query", "rsync task", func(r map[string]any) (string, bool) {
			p := str(r, "path")
			return "path " + p, underPath(p)
		}},
		{"cloudsync.query", "cloud sync task", func(r map[string]any) (string, bool) {
			p := str(r, "path")
			return "path " + p, underPath(p)
		}},
		{"cloud_backup.query", "cloud backup task", func(r map[string]any) (string, bool) {
			p := str(r, "path")
			return "path " + p, underPath(p)
		}},
		{"vm.device.query", "VM device", func(r map[string]any) (string, bool) {
			attrs, _ := r["attributes"].(map[string]any)
			p := str(attrs, "path")
			return fmt.Sprintf("vm %v, path %s", r["vm"], p), underDev(p) || underPath(p)
		}},
	}
	for _, p := range probes {
		raw, err := c.CallRead(ctx, p.method, []any{})
		if err != nil {
			if isMissingMethod(err) {
				continue
			}
			return nil, fmt.Errorf("%s: %w", p.method, err)
		}
		var rows []map[string]any
		if err := json.Unmarshal(raw, &rows); err != nil {
			return nil, fmt.Errorf("parse %s: %w", p.method, err)
		}
		for _, r := range rows {
			if ref, hit := p.match(r); hit {
				out = append(out, RenameBlocker{Kind: p.kind, Name: label(r), Ref: ref})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out, nil
}

// isMissingMethod reports a JSON-RPC "Method does not exist" (-32601): the
// release has no such service, so there is nothing of that kind to check.
func isMissingMethod(err error) bool {
	var apiErr *client.APIError
	if errors.As(err, &apiErr) && apiErr.Code == -32601 {
		return true
	}
	return strings.Contains(err.Error(), "Method does not exist")
}

// RenameRefusedError builds the diagnostic for a rename blocked by in-use
// references.
func RenameRefusedError(kind, oldName, newName string, blockers []RenameBlocker) diag.Diagnostic {
	lines := make([]string, 0, len(blockers))
	for _, b := range blockers {
		lines = append(lines, "  - "+b.String())
	}
	return diag.NewAttributeErrorDiagnostic(path.Root("name"),
		fmt.Sprintf("Rename of %s refused: it is in use", kind),
		fmt.Sprintf("%s cannot be renamed to %s while these still point at it; TrueNAS does not update them "+
			"and they would break:\n%s\nRe-point or remove them first, or set allow_rename_in_use = true to rename "+
			"anyway (Terraform-managed references plan their own update on the next apply; anything else must be "+
			"fixed by hand).", oldName, newName, strings.Join(lines, "\n")))
}

// PerformRename is the Update-time rename shared by truenas_dataset and
// truenas_zvol. guid identifies the object; its current server-side name may
// already differ from the state's (a parent renamed earlier in the same apply
// moved it), so the rename starts from the current name and is skipped when
// the object already has the planned name. Returns the name to use for the
// rest of the update.
func PerformRename(ctx context.Context, c api, kind, guid, stateName, planName, mountpoint string, allowInUse bool) (string, diag.Diagnostics) {
	var diags diag.Diagnostics
	current := stateName
	if guid != "" {
		cur, err := NameByGuid(ctx, c, guid)
		if err != nil {
			diags.AddError(fmt.Sprintf("Rename %s failed", kind), fmt.Sprintf("locating %s by guid %s: %v", stateName, guid, err))
			return "", diags
		}
		current = cur
	}
	if current == planName {
		return planName, diags // moved with its parent already
	}
	if !allowInUse {
		mp := mountpoint
		if mp != "" && current != stateName {
			mp = "/mnt/" + current
		}
		blockers, err := RenameBlockers(ctx, c, current, mp)
		if err != nil {
			diags.AddError(fmt.Sprintf("Rename %s failed", kind),
				fmt.Sprintf("could not check whether %s is in use: %v. Fix the error, or set allow_rename_in_use = true to skip the check.", current, err))
			return "", diags
		}
		if len(blockers) > 0 {
			diags.Append(RenameRefusedError(kind, current, planName, blockers))
			return "", diags
		}
	}
	if err := Rename(ctx, c, current, planName); err != nil {
		diags.AddError(fmt.Sprintf("Rename %s failed", kind), fmt.Sprintf("pool.dataset.rename %s -> %s: %v", current, planName, err))
		return "", diags
	}
	return planName, diags
}

// NameDescription is the schema description of `name` for truenas_dataset and
// truenas_zvol: it carries the rename warning, since a rename is applied in
// place and TrueNAS performs no safety checks.
func NameDescription(kind, example string) string {
	return fmt.Sprintf("Full %s path, e.g. %s. Changing it renames the %s in place (pool.dataset.rename): data, child "+
		"datasets and snapshots move with it and the ZFS guid is unchanged. WARNING: TrueNAS performs no safety checks on a "+
		"rename. SMB/NFS shares, iSCSI extents, NVMe-oF namespaces, snapshot, replication, rsync and cloud sync tasks and VM "+
		"disks that point at the old path are NOT updated and break until re-pointed; a zvol under a live iSCSI extent or "+
		"NVMe-oF namespace goes offline for its initiators. The provider refuses the rename while any of those reference the "+
		"%s unless allow_rename_in_use = true. Other Terraform resources that reference the %s's name or mountpoint plan their "+
		"own update (or replacement) in the same apply, so there is a window with a broken path. The pool component cannot change.",
		kind, example, kind, kind, kind)
}

// AllowRenameInUseAttribute is the allow_rename_in_use attribute shared by
// truenas_dataset and truenas_zvol. It only changes what an Update that renames
// does; it is never sent to the API and unset means false.
func AllowRenameInUseAttribute(kind string) schema.BoolAttribute {
	return schema.BoolAttribute{
		Optional: true,
		Description: fmt.Sprintf("When true, a rename proceeds even though SMB/NFS shares, iSCSI extents, NVMe-oF namespaces, "+
			"snapshot/replication/rsync/cloud sync tasks or VM disks still reference the %s or its descendants. Default false: "+
			"such a rename fails and lists them. TrueNAS does not re-point any of them; Terraform-managed references plan their "+
			"own update on the next apply, anything else must be fixed by hand.", kind),
	}
}

// ResolveByGuid locates a dataset or zvol for Read: by guid when state has one
// (following a rename made outside Terraform), else by the state name (state
// from <= 1.6.1, whose guid is then looked up). found is false when it no
// longer exists.
func ResolveByGuid(ctx context.Context, c api, stateGuid types.String, stateName string) (name, guid string, found bool, err error) {
	if !stateGuid.IsNull() && !stateGuid.IsUnknown() && stateGuid.ValueString() != "" {
		guid = stateGuid.ValueString()
		name, err = NameByGuid(ctx, c, guid)
	} else {
		name = stateName
		guid, err = LookupGuid(ctx, c, name)
	}
	if err != nil {
		if client.IsNotFound(err) {
			return "", "", false, nil
		}
		return "", "", false, err
	}
	return name, guid, true, nil
}

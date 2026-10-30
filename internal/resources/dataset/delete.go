// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

// maxListedBlockers caps how many child datasets/snapshots the delete error
// names before summarizing the rest as "and N more".
const maxListedBlockers = 10

// deleteSettleTimeout and deleteSettlePoll bound how long a non-recursive
// delete waits for blocking children/snapshots to disappear. Terraform
// destroys resources without a dependency edge in parallel, so a managed child
// can still exist when the parent's delete starts and vanish a moment later;
// only children that are still present after the wait are reported. Package
// vars (not consts) so a unit test can shrink them.
var (
	deleteSettleTimeout = 15 * time.Second
	deleteSettlePoll    = 2 * time.Second
)

// caller is the slice of *client.Client DeleteDataset needs; a fake drives the
// unit test.
type caller interface {
	Call(ctx context.Context, method string, params ...any) (json.RawMessage, error)
	CallJob(ctx context.Context, method string, params ...any) (json.RawMessage, error)
}

var _ caller = (*client.Client)(nil)

// ForceDestroyAttribute is the force_destroy attribute shared by
// truenas_dataset and truenas_zvol. It only changes what Delete does, so a
// change is an in-place, state-only update (no API call), and import sets it
// to false. kind ("dataset" or "zvol") is used in the description.
func ForceDestroyAttribute(kind string) schema.BoolAttribute {
	what := "child datasets, zvols and snapshots"
	if kind == "zvol" {
		what = "snapshots"
	}
	return schema.BoolAttribute{
		Optional: true,
		Computed: true,
		Default:  booldefault.StaticBool(false),
		Description: fmt.Sprintf("When true, destroying (or replacing) this %s also deletes all of its %s. "+
			"Default false: the delete is non-recursive, and while any of those exist it fails and lists them. "+
			"Changing it updates state only. It takes effect only once applied, so set it and apply before the "+
			"destroy or replacement that needs it. Import sets it to false.", kind, what),
	}
}

// ForceDestroyOrFalse returns v, or false when v is null or unknown: state
// written before force_destroy existed, an import, or a list result.
func ForceDestroyOrFalse(v types.Bool) types.Bool {
	if v.IsNull() || v.IsUnknown() {
		return types.BoolValue(false)
	}
	return v
}

// DeleteDataset deletes the dataset or zvol name. kind ("dataset" or "zvol")
// is used in the error text.
//
// With force = false (the default) the delete is non-recursive: when the
// dataset still has child datasets/zvols or snapshots it fails with an error
// that names them and says how to proceed, and nothing is deleted. The
// provider checks for them itself instead of relying on the server's
// ENOTEMPTY message, whose text differs between versions (25.10 reports only
// a snapshot name). With force = true it deletes recursively: the dataset
// with all of its children and snapshots.
//
// A dataset that is already gone is not an error.
func DeleteDataset(ctx context.Context, c caller, name, kind string, force bool) diag.Diagnostics {
	var diags diag.Diagnostics
	summary := fmt.Sprintf("Delete %s failed", kind)

	if force {
		_, err := c.CallJob(ctx, "pool.dataset.delete", name, map[string]any{"recursive": true})
		if err != nil && !client.IsNotFound(err) {
			diags.AddError(summary, err.Error())
		}
		return diags
	}

	deadline := time.Now().Add(deleteSettleTimeout)
	for {
		children, snaps, err := deleteBlockers(ctx, c, name)
		if err != nil {
			diags.AddError(summary, fmt.Sprintf("checking %s %q for child datasets and snapshots: %s", kind, name, err))
			return diags
		}
		var lastErr error
		if len(children) == 0 && len(snaps) == 0 {
			_, lastErr = c.CallJob(ctx, "pool.dataset.delete", name, map[string]any{"recursive": false})
			if lastErr == nil || client.IsNotFound(lastErr) {
				return diags
			}
			// The server refused: a child or snapshot may have appeared after the
			// check. Re-check before reporting.
		}
		if time.Now().After(deadline) {
			if len(children) > 0 || len(snaps) > 0 {
				diags.AddError(summary, blockersMessage(name, kind, children, snaps))
			} else {
				diags.AddError(summary, lastErr.Error())
			}
			return diags
		}
		select {
		case <-ctx.Done():
			diags.AddError(summary, ctx.Err().Error())
			return diags
		case <-time.After(deleteSettlePoll):
		}
	}
}

// deleteBlockers returns the descendant datasets/zvols ("name (TYPE)") and the
// snapshots of name that a non-recursive delete would refuse.
func deleteBlockers(ctx context.Context, c caller, name string) (children, snaps []string, err error) {
	raw, err := c.Call(ctx, "pool.dataset.query",
		[]any{[]any{"name", "^", name + "/"}},
		map[string]any{
			"select": []string{"name", "type"},
			"extra":  map[string]any{"flat": true, "retrieve_children": false, "properties": []string{}},
		})
	if err != nil {
		return nil, nil, err
	}
	var ds []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &ds); err != nil {
		return nil, nil, fmt.Errorf("parse pool.dataset.query: %w", err)
	}
	for _, d := range ds {
		children = append(children, fmt.Sprintf("%s (%s)", d.Name, d.Type))
	}

	raw, err = c.Call(ctx, "pool.snapshot.query",
		[]any{[]any{"dataset", "=", name}},
		map[string]any{"select": []string{"name"}})
	if err != nil {
		return nil, nil, err
	}
	var ss []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &ss); err != nil {
		return nil, nil, fmt.Errorf("parse pool.snapshot.query: %w", err)
	}
	for _, s := range ss {
		snaps = append(snaps, s.Name)
	}
	return children, snaps, nil
}

// blockersMessage builds the error for a non-recursive delete that children or
// snapshots block: what is in the way (up to maxListedBlockers), and the three
// ways to proceed.
func blockersMessage(name, kind string, children, snaps []string) string {
	var what []string
	if n := len(children); n > 0 {
		what = append(what, plural(n, "child dataset/zvol", "child datasets/zvols"))
	}
	if n := len(snaps); n > 0 {
		what = append(what, plural(n, "snapshot", "snapshots"))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s %q has %s, so it was not deleted:\n", kind, name, strings.Join(what, " and "))
	all := append(append([]string{}, children...), snaps...)
	for i, s := range all {
		if i == maxListedBlockers {
			fmt.Fprintf(&b, "  ... and %d more\n", len(all)-maxListedBlockers)
			break
		}
		fmt.Fprintf(&b, "  %s\n", s)
	}
	fmt.Fprintf(&b, "\nThe provider deletes a %s only when it has no child datasets, zvols or snapshots. To proceed, do one of:\n", kind)
	b.WriteString("  - Manage them in Terraform (truenas_dataset, truenas_zvol, truenas_snapshot) and make them depend on " +
		"this resource, by referencing its name (for example name = \"${truenas_dataset.parent.name}/child\") or with " +
		"depends_on, so Terraform destroys them first.\n")
	b.WriteString("  - Delete the children and snapshots yourself, then run the destroy again.\n")
	fmt.Fprintf(&b, "  - Set force_destroy = true on this %s and apply that change on its own first. "+
		"A later destroy or replacement then deletes the %s together with all of its child datasets, zvols and snapshots.", kind, kind)
	return b.String()
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

// NonEmptyContents lists up to three entries in the dataset's mountpoint
// (the .zfs control directory excluded). An empty result means the dataset
// holds no files. Errors (not mounted, locked, path missing) are returned to
// the caller, which treats "cannot tell" as "do not block".
func NonEmptyContents(ctx context.Context, c caller, mountpoint string) ([]string, error) {
	if mountpoint == "" {
		return nil, nil
	}
	raw, err := c.Call(ctx, "filesystem.listdir", mountpoint,
		[]any{[]any{"name", "!=", ".zfs"}},
		map[string]any{"select": []string{"name"}, "limit": 3})
	if err != nil {
		return nil, err
	}
	var entries []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("parse filesystem.listdir: %w", err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name)
	}
	return names, nil
}

// Fresh-zvol logicalreferenced baselines measured on 25.10, 26.0 and 28.0
// (independent of pool and volblocksize): dnode + metadata, and the extra
// encryption metadata for an encrypted volume. One written block adds at
// least volblocksize on top.
const (
	zvolEmptyLogicalReferenced          = 28672
	zvolEmptyLogicalReferencedEncrypted = 41472
)

// zvolHoldsData is the pure decision behind ZvolHoldsData.
func zvolHoldsData(logicalReferenced, volBlockSize int64, encrypted bool) bool {
	if volBlockSize <= 0 {
		volBlockSize = 16384
	}
	base := int64(zvolEmptyLogicalReferenced)
	if encrypted {
		base = zvolEmptyLogicalReferencedEncrypted
	}
	return logicalReferenced > base+volBlockSize/2
}

// ZvolHoldsData reports whether the volume has had data written to it, from
// ZFS logicalreferenced compared with the empty baseline for its encryption
// state. Errors mean "cannot tell"; callers do not block on them.
func ZvolHoldsData(ctx context.Context, c caller, name string) (bool, int64, error) {
	raw, err := c.Call(ctx, "pool.dataset.query",
		[]any{[]any{"name", "=", name}},
		map[string]any{"extra": map[string]any{"flat": true, "retrieve_children": false,
			// "encryption" (the ZFS property) must be requested for the API to
			// populate its derived top-level "encrypted" flag; with
			// extra.properties set it returns only what is asked for.
			"properties": []string{"logicalreferenced", "volblocksize", "encryption"}}})
	if err != nil {
		return false, 0, err
	}
	var rows []struct {
		LogicalReferenced struct {
			Parsed int64 `json:"parsed"`
		} `json:"logicalreferenced"`
		VolBlockSize struct {
			Parsed int64 `json:"parsed"`
		} `json:"volblocksize"`
		Encrypted bool `json:"encrypted"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil || len(rows) == 0 {
		return false, 0, fmt.Errorf("parse pool.dataset.query for %s: %v", name, err)
	}
	r := rows[0]
	return zvolHoldsData(r.LogicalReferenced.Parsed, r.VolBlockSize.Parsed, r.Encrypted), r.LogicalReferenced.Parsed, nil
}

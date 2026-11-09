// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset

// Clone-from-snapshot support shared by truenas_dataset and truenas_zvol:
// `origin` names a snapshot ("pool/golden@snap") and the resource is created
// with pool.snapshot.clone instead of pool.dataset.create. The clone shares
// the snapshot's blocks until it diverges and reads its origin back from the
// ZFS `origin` property, so an imported clone round-trips. origin is
// create-only: another origin is another disk, so it replaces the resource
// (guarded by force_destroy like every other replacement).

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// OriginAttribute is the `origin` attribute of truenas_dataset / truenas_zvol.
func OriginAttribute(kind string) schema.StringAttribute {
	return schema.StringAttribute{
		Optional: true,
		Computed: true,
		Description: fmt.Sprintf("Snapshot to clone this %[1]s from, as \"pool/dataset@snapshot\" (for example truenas_snapshot.golden.id). "+
			"When set, the %[1]s is created with pool.snapshot.clone and starts as a writable copy of the snapshot that shares its "+
			"blocks until it diverges; the configured properties are then applied with an update. Create-only: a different origin "+
			"is a different disk, so changing it replaces the %[1]s (the destroy half needs force_destroy = true). ZFS cannot destroy "+
			"a snapshot while clones of it exist, so reference the truenas_snapshot here and Terraform destroys the clones first. "+
			"Cannot be combined with the encryption inputs (a clone inherits the origin's encryption). Read back from the ZFS "+
			"origin property, so an imported clone round-trips; null for a %[1]s that is not a clone.", kind),
		// Stock RequiresReplace, not planmod.ReplaceFromKnown*: the API returns
		// the origin, so a null prior means "not a clone" and adding an origin
		// must replace (a ReplaceFromKnown would record it in place without
		// cloning anything).
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.UseStateForUnknown(),
			stringplanmodifier.RequiresReplace(),
		},
	}
}

// OriginValue maps the ZFS origin property (parsed form "pool/ds@snap", ""
// when the dataset is not a clone) to the attribute value.
func OriginValue(parsed string) types.String {
	if parsed == "" {
		return types.StringNull()
	}
	return types.StringValue(parsed)
}

// IsSet reports a known, non-null, non-empty string.
func isSet(v types.String) bool { return !v.IsNull() && !v.IsUnknown() && v.ValueString() != "" }

// OriginDataset is the dataset part of "pool/ds@snap".
func OriginDataset(origin string) string {
	ds, _, _ := strings.Cut(origin, "@")
	return ds
}

// CheckOriginConfig is the ValidateConfig check for origin: it must look like
// a snapshot and cannot be combined with the encryption inputs, which only
// pool.dataset.create accepts (a clone inherits the origin's encryption).
func CheckOriginConfig(origin, kind string, encryptionSet bool) diag.Diagnostics {
	var diags diag.Diagnostics
	if !strings.Contains(origin, "@") || strings.HasPrefix(origin, "@") || strings.HasSuffix(origin, "@") {
		diags.AddAttributeError(path.Root("origin"), "origin must name a snapshot",
			fmt.Sprintf("%q is not of the form pool/dataset@snapshot (truenas_snapshot.<name>.id has that form).", origin))
	}
	if encryptionSet {
		diags.AddAttributeError(path.Root("origin"), "origin cannot be combined with encryption settings",
			fmt.Sprintf("A cloned %s inherits the encryption of its origin snapshot; remove encryption, inherit_encryption, "+
				"encryption_algorithm, encryption_generate_key, encryption_passphrase and encryption_key, or clone nothing.", kind))
	}
	return diags
}

// CloneSnapshot creates dst from snapshot with pool.snapshot.clone (sync on
// 25.10, 26.0 and 28.0). Properties are applied afterwards by the caller with
// pool.dataset.update, so one code path sets them for both create modes.
func CloneSnapshot(ctx context.Context, c api, snapshot, dst string) error {
	_, err := c.Call(ctx, "pool.snapshot.clone", map[string]any{"snapshot": snapshot, "dataset_dst": dst})
	return err
}

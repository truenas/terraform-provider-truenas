// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

// RefreservationFollowsVolsize marks an unconfigured refreservation unknown
// when volsize changes. ZFS keeps a thick volume's refreservation equal to
// what its volsize needs and grows it with the volume, so the value
// UseStateForUnknown copied from state is stale once volsize changes; planning
// it unknown lets the update read back what ZFS set. A configured
// refreservation is left alone (it is sent with the new volsize). (B-038)
// List it after UseStateForUnknown.
func RefreservationFollowsVolsize() planmodifier.Int64 { return refreservationFollowsVolsize{} }

type refreservationFollowsVolsize struct{}

func (refreservationFollowsVolsize) Description(context.Context) string {
	return "Plans an unconfigured refreservation as unknown when volsize changes."
}

func (m refreservationFollowsVolsize) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (refreservationFollowsVolsize) PlanModifyInt64(ctx context.Context, req planmodifier.Int64Request, resp *planmodifier.Int64Response) {
	if !req.ConfigValue.IsNull() || req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}
	// The configured volsize, not the planned one: an unconfigured volsize
	// (a FILESYSTEM) is still unknown in the plan when this runs, before its
	// own UseStateForUnknown.
	var planned, prior types.Int64
	if d := req.Config.GetAttribute(ctx, path.Root("volsize"), &planned); d.HasError() || planned.IsNull() {
		return
	}
	if d := req.State.GetAttribute(ctx, path.Root("volsize"), &prior); d.HasError() {
		return
	}
	if planned.IsUnknown() || (!prior.IsNull() && planned.ValueInt64() != prior.ValueInt64()) {
		resp.PlanValue = types.Int64Unknown()
	}
}

// FixedEncryptionAlgorithm is the only algorithm TrueNAS 26.0 and later
// create: 26.0 accepts encryption_options.algorithm but ignores it (probed on
// 26.0.0-BETA.3: AES-128-CCM requested, AES-256-GCM created), and 27.0 removed
// it from the API.
const FixedEncryptionAlgorithm = "AES-256-GCM"

// CheckEncryptionAlgorithm is the ModifyPlan check shared by truenas_dataset
// and truenas_zvol: on TrueNAS 26.0+ an encryption_algorithm other than
// AES-256-GCM cannot be honored, so a plan that would create one fails at plan
// time. It used to be ignored (26.0) or dropped from the create payload (27.0+),
// which created an AES-256-GCM dataset and then failed the apply with an
// inconsistent result, leaving a tainted resource behind. (B-056) A value
// already in state (a dataset created before the server was upgraded) is not
// re-checked.
func CheckEncryptionAlgorithm(ctx context.Context, c *client.Client, req resource.ModifyPlanRequest) diag.Diagnostics {
	var diags diag.Diagnostics
	if c == nil || req.Plan.Raw.IsNull() {
		return diags
	}
	var cfg types.String
	if d := req.Config.GetAttribute(ctx, path.Root("encryption_algorithm"), &cfg); d.HasError() {
		return diags
	}
	if cfg.IsNull() || cfg.IsUnknown() || cfg.ValueString() == FixedEncryptionAlgorithm {
		return diags
	}
	if !req.State.Raw.IsNull() {
		var prior types.String
		if d := req.State.GetAttribute(ctx, path.Root("encryption_algorithm"), &prior); !d.HasError() &&
			!prior.IsNull() && prior.ValueString() == cfg.ValueString() {
			return diags
		}
	}
	ok, err := c.VersionAtLeast(ctx, 26, 0)
	if err != nil || !ok {
		return diags
	}
	diags.AddAttributeError(
		path.Root("encryption_algorithm"),
		"encryption_algorithm cannot be chosen on this TrueNAS release",
		"TrueNAS 26.0 and later always encrypt with "+FixedEncryptionAlgorithm+" (26.0 ignores the "+
			"algorithm, 27.0 removed it). Remove encryption_algorithm or set it to \""+FixedEncryptionAlgorithm+"\".",
	)
	return diags
}

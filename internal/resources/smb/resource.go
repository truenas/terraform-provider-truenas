// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package smb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/truenas/terraform-provider-truenas/internal/client"
	"github.com/truenas/terraform-provider-truenas/internal/listing"
)

var _ resource.Resource = &SMBShareResource{}
var _ resource.ResourceWithImportState = &SMBShareResource{}
var _ resource.ResourceWithIdentity = &SMBShareResource{}
var _ resource.ResourceWithValidateConfig = &SMBShareResource{}
var _ resource.ResourceWithModifyPlan = &SMBShareResource{}

// SMBShareResource implements the truenas_smb_share resource.
type SMBShareResource struct{ client *client.Client }

// NewResource returns a new SMBShareResource.
func NewResource() resource.Resource { return &SMBShareResource{} }

func (r *SMBShareResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_smb_share"
}

func (r *SMBShareResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *SMBShareResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = listing.IntIDIdentitySchema()
}

func (r *SMBShareResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data",
			fmt.Sprintf("expected *client.Client, got %T", req.ProviderData),
		)
		return
	}
	r.client = c
}

// ValidateConfig rejects, at plan time, config that would otherwise be
// silently ignored on apply:
//   - an options field that is not valid for the share's effective purpose
//     (apiPayload drops it; it is never sent and reads back null), e.g.
//     recyclebin on a TIMEMACHINE_SHARE;
//   - a flat legacy attribute (guestok, hostsallow, ...) on a non-LEGACY_SHARE
//     purpose: apiPayload only sends the flat attributes for LEGACY_SHARE, and
//     Read reports them as false/empty for every other purpose;
//   - a flat legacy attribute and its options.<key> twin both set: options
//     would win and the flat attribute would read back a different value.
//
// Validation is skipped when purpose or options are unknown (interpolated).
func (r *SMBShareResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var cfg SMBModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	purpose, bad, diags := cfg.invalidOptionKeys(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	validList := strings.Join(sortedValidOptions(purpose), ", ")
	for _, key := range bad {
		resp.Diagnostics.AddAttributeError(
			path.Root("options").AtName(key),
			"Option not valid for this purpose",
			fmt.Sprintf("options.%s is not valid for purpose %q and would be silently ignored on apply. "+
				"Remove it, or change purpose. Valid options for %s: %s.", key, purpose, purpose, validList),
		)
	}

	if cfg.Purpose.IsUnknown() {
		return
	}
	flat := cfg.configuredFlatLegacyKeys()
	if purpose != legacySharePurpose {
		for _, key := range flat {
			hint := "Remove it, or set purpose = \"LEGACY_SHARE\"."
			if smbOptionFieldsByPurpose[purpose][key] {
				hint = fmt.Sprintf("Set options.%s instead, or set purpose = \"LEGACY_SHARE\".", key)
			}
			resp.Diagnostics.AddAttributeError(
				path.Root(key),
				"Attribute only valid for LEGACY_SHARE",
				fmt.Sprintf("%s is a LEGACY_SHARE attribute and is not applied for purpose %q; it would be silently ignored. %s",
					key, purpose, hint),
			)
		}
		return
	}
	if cfg.Options.IsNull() || cfg.Options.IsUnknown() || len(flat) == 0 {
		return
	}
	var opt SMBOptionsModel
	resp.Diagnostics.Append(cfg.Options.As(ctx, &opt, basetypes.ObjectAsOptions{})...)
	if resp.Diagnostics.HasError() {
		return
	}
	inOptions := map[string]bool{}
	for _, key := range setOptionKeys(&opt) {
		inOptions[key] = true
	}
	for _, key := range flat {
		if inOptions[key] {
			resp.Diagnostics.AddAttributeError(
				path.Root(key),
				"Attribute set twice",
				fmt.Sprintf("%s and options.%s are the same share setting; set only one of them.", key, key),
			)
		}
	}
}

// ModifyPlan keeps the computed `options` and `vuid` in step with the inputs
// they are derived from (both are pinned from state by UseStateForUnknown):
//   - purpose change: the server returns the new purpose's options shape and
//     a vuid only for LEGACY_SHARE/TIMEMACHINE_SHARE, so the pinned values are
//     stale. When options is not configured, mark it unknown (only the flat
//     attributes and purpose are sent; the server applies the new purpose's
//     defaults); vuid is always marked unknown.
//   - same LEGACY_SHARE purpose, options not configured: copy the configured
//     flat attributes onto the pinned options so the payload (which must carry
//     the whole options object) sends the new values, and the plan shows them.
func (r *SMBShareResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() || req.State.Raw.IsNull() {
		return
	}
	var plan, state, cfg SMBModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	newPurpose := resolveEffectivePurpose(&plan)
	if newPurpose != state.Purpose.ValueString() {
		if cfg.Options.IsNull() {
			resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("options"), types.ObjectUnknown(smbOptionsAttrTypes))...)
		}
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("vuid"), types.StringUnknown())...)
		return
	}

	if newPurpose != legacySharePurpose || !cfg.Options.IsNull() || plan.Options.IsNull() || plan.Options.IsUnknown() {
		return
	}
	opts, changed, diags := plan.overlayFlatOnOptions(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || !changed {
		return
	}
	resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("options"), opts)...)
}

func (r *SMBShareResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan SMBModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.apiPayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.Call(ctx, "sharing.smb.create", payload)
	if err != nil {
		resp.Diagnostics.AddError("Create SMB share failed", err.Error())
		return
	}

	// Parse the create response to get the ID, then read back via get_instance.
	var created smbAPI
	if err := json.Unmarshal(raw, &created); err != nil {
		resp.Diagnostics.AddError("Parse create response", err.Error())
		return
	}

	raw, err = r.client.CallRead(ctx, "sharing.smb.get_instance", created.ID)
	if err != nil {
		resp.Diagnostics.AddError("Read-back after create failed", err.Error())
		return
	}

	var apiResp smbAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse get_instance response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueInt64())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *SMBShareResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state SMBModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "sharing.smb.get_instance", state.ID.ValueInt64())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read SMB share failed", err.Error())
		return
	}

	var apiResp smbAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, state.ID.ValueInt64())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *SMBShareResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan SMBModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Carry the existing state ID into the plan (plan.ID is Unknown until apply).
	var state SMBModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID

	payload, diags := plan.apiPayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Call(ctx, "sharing.smb.update", plan.ID.ValueInt64(), payload)
	if err != nil {
		resp.Diagnostics.AddError("Update SMB share failed", err.Error())
		return
	}

	raw, err := r.client.CallRead(ctx, "sharing.smb.get_instance", plan.ID.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Read-back after update failed", err.Error())
		return
	}

	var apiResp smbAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse get_instance response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueInt64())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *SMBShareResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state SMBModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.CallJob(ctx, "sharing.smb.delete", state.ID.ValueInt64())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete SMB share failed", err.Error())
	}
}

func (r *SMBShareResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, idDiags := listing.ImportInt64(ctx, req)
	resp.Diagnostics.Append(idDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "sharing.smb.get_instance", id)
	if err != nil {
		resp.Diagnostics.AddError("Import SMB share failed", err.Error())
		return
	}

	var apiResp smbAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse import response", err.Error())
		return
	}

	var state SMBModel
	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, id)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

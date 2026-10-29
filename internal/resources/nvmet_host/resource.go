// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_host

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
	"github.com/truenas/terraform-provider-truenas/internal/listing"
)

var _ resource.Resource = &NVMetHostResource{}
var _ resource.ResourceWithImportState = &NVMetHostResource{}
var _ resource.ResourceWithIdentity = &NVMetHostResource{}
var _ resource.ResourceWithModifyPlan = &NVMetHostResource{}

// NVMetHostResource implements the truenas_nvmet_host resource.
type NVMetHostResource struct{ client *client.Client }

// NewResource returns a new NVMetHostResource.
func NewResource() resource.Resource { return &NVMetHostResource{} }

func (r *NVMetHostResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_nvmet_host"
}

func (r *NVMetHostResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *NVMetHostResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = listing.IntIDIdentitySchema()
}

func (r *NVMetHostResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// applyDescriptionSupport handles the description field's version gate:
// the nvmet_host.create/update schemas gained it in TrueNAS 26.0, and older
// releases fail it with a generic "Extra inputs are not permitted". On a
// pre-26.0 server a non-empty description errors with a clear message; an
// empty one is silently stripped from the payload — description is
// Optional+Computed, so the 25.10 read-back's "" flows into later plans as
// a known (but meaningless) value and must not be resent. If the version
// probe itself fails, the payload is left alone and the server's own
// validation decides.
func (r *NVMetHostResource) applyDescriptionSupport(ctx context.Context, payload map[string]any, plan *NVMetHostModel, diags *diag.Diagnostics) {
	if _, present := payload["description"]; !present {
		return
	}
	ok, err := r.client.VersionAtLeast(ctx, 26, 0)
	if err != nil || ok {
		return
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() && plan.Description.ValueString() != "" {
		diags.AddAttributeError(
			path.Root("description"),
			"description requires TrueNAS 26.0 or later",
			"The nvmet_host description field does not exist on this TrueNAS release. Remove the attribute or upgrade the server.",
		)
		return
	}
	delete(payload, "description")
}

// ModifyPlan plans dhchap_key_set/dhchap_ctrl_key_set from whether the
// write-only keys are set in config. The keys never reach the plan, so
// without this removing a key from config planned nothing and the old key
// stayed active on the server; now the flag flips and Update clears it. The
// flags are read back from the server, so a key added or removed outside
// Terraform also shows as a change.
func (r *NVMetHostResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	var cfg NVMetHostModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	for attr, v := range map[string]types.String{"dhchap_key_set": cfg.DHChapKey, "dhchap_ctrl_key_set": cfg.DHChapCtrlKey} {
		planned := types.BoolUnknown()
		if !v.IsUnknown() {
			planned = types.BoolValue(configKey(v) != "")
		}
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root(attr), planned)...)
	}
}

func (r *NVMetHostResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan NVMetHostModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// write-only: the framework nulls WriteOnly attributes in req.Plan, so
	// the actual dhchap_key/dhchap_ctrl_key values are only available via
	// req.Config.
	var cfg NVMetHostModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.DHChapKey = cfg.DHChapKey
	plan.DHChapCtrlKey = cfg.DHChapCtrlKey

	payload, diags := plan.createPayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.applyDescriptionSupport(ctx, payload, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.Call(ctx, "nvmet.host.create", payload)
	if err != nil {
		resp.Diagnostics.AddError("Create NVMe-oF host failed", err.Error())
		return
	}

	// Parse create response to get the ID, then read back via get_instance.
	var created nvmetHostAPI
	if err := json.Unmarshal(raw, &created); err != nil {
		resp.Diagnostics.AddError("Parse create response", err.Error())
		return
	}

	raw, err = r.client.CallRead(ctx, "nvmet.host.get_instance", created.ID)
	if err != nil {
		resp.Diagnostics.AddError("Read-back after create failed", err.Error())
		return
	}

	var apiResp nvmetHostAPI
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

func (r *NVMetHostResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state NVMetHostModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "nvmet.host.get_instance", state.ID.ValueInt64())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read NVMe-oF host failed", err.Error())
		return
	}

	var apiResp nvmetHostAPI
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

func (r *NVMetHostResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan NVMetHostModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Carry the existing state ID into the plan (plan.ID is Unknown until apply).
	var state NVMetHostModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID

	// write-only: value lives in config, not plan.
	var cfg NVMetHostModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.DHChapKey = cfg.DHChapKey
	plan.DHChapCtrlKey = cfg.DHChapCtrlKey

	payload, diags := plan.updatePayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	omitUnrotatedKeys(payload, plan, state)
	r.applyDescriptionSupport(ctx, payload, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Call(ctx, "nvmet.host.update", plan.ID.ValueInt64(), payload)
	if err != nil {
		resp.Diagnostics.AddError("Update NVMe-oF host failed", err.Error())
		return
	}

	raw, err := r.client.CallRead(ctx, "nvmet.host.get_instance", plan.ID.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Read-back after update failed", err.Error())
		return
	}

	var apiResp nvmetHostAPI
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

func (r *NVMetHostResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state NVMetHostModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Call(ctx, "nvmet.host.delete", state.ID.ValueInt64())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete NVMe-oF host failed", err.Error())
	}
}

func (r *NVMetHostResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, idDiags := listing.ImportInt64(ctx, req)
	resp.Diagnostics.Append(idDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "nvmet.host.get_instance", id)
	if err != nil {
		resp.Diagnostics.AddError("Import NVMe-oF host failed", err.Error())
		return
	}

	var apiResp nvmetHostAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse import response", err.Error())
		return
	}

	var state NVMetHostModel
	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, id)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

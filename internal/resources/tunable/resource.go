// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package tunable

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
	"github.com/truenas/terraform-provider-truenas/internal/listing"
	"github.com/truenas/terraform-provider-truenas/internal/stateupgrade"
)

var _ resource.Resource = &TunableResource{}
var _ resource.ResourceWithUpgradeState = &TunableResource{}
var _ resource.ResourceWithImportState = &TunableResource{}
var _ resource.ResourceWithIdentity = &TunableResource{}

// TunableResource implements the truenas_tunable resource.
type TunableResource struct{ client *client.Client }

// NewResource returns a new TunableResource.
func NewResource() resource.Resource { return &TunableResource{} }

func (r *TunableResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tunable"
}

func (r *TunableResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *TunableResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = listing.IntIDIdentitySchema()
}

func (r *TunableResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *TunableResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan TunableModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.createPayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	// update_initramfs is write-only: read from config, never from plan or state.
	var cfg TunableModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !cfg.UpdateInitramfs.IsNull() && !cfg.UpdateInitramfs.IsUnknown() {
		payload["update_initramfs"] = cfg.UpdateInitramfs.ValueBool()
	}

	raw, err := r.client.CallJob(ctx, "tunable.create", payload)
	if err != nil {
		resp.Diagnostics.AddError("Create tunable failed", err.Error())
		return
	}

	// tunable.create may return either the created object or a bare int ID.
	apiResp, id, err := decodeCreateResult(raw)
	if err != nil {
		resp.Diagnostics.AddError("Parse create response", err.Error())
		return
	}

	if apiResp == nil {
		// Bare-int shape: read back full state via get_instance.
		raw2, err := r.client.CallRead(ctx, "tunable.get_instance", id)
		if err != nil {
			resp.Diagnostics.AddError("Read-back after create failed", err.Error())
			return
		}

		var got tunableAPI
		if err := json.Unmarshal(raw2, &got); err != nil {
			resp.Diagnostics.AddError("Parse get_instance response", err.Error())
			return
		}
		apiResp = &got
	}

	resp.Diagnostics.Append(responseToModel(ctx, apiResp, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueInt64())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *TunableResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state TunableModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "tunable.get_instance", state.ID.ValueInt64())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read tunable failed", err.Error())
		return
	}

	var apiResp tunableAPI
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

func (r *TunableResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan TunableModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Carry the existing state ID into the plan.
	var state TunableModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID

	payload, diags := plan.updatePayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	// update_initramfs is write-only: read from config, never from plan or state.
	var cfg TunableModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !cfg.UpdateInitramfs.IsNull() && !cfg.UpdateInitramfs.IsUnknown() {
		payload["update_initramfs"] = cfg.UpdateInitramfs.ValueBool()
	}

	_, err := r.client.CallJob(ctx, "tunable.update", plan.ID.ValueInt64(), payload)
	if err != nil {
		resp.Diagnostics.AddError("Update tunable failed", err.Error())
		return
	}

	raw, err := r.client.CallRead(ctx, "tunable.get_instance", plan.ID.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Read-back after update failed", err.Error())
		return
	}

	var apiResp tunableAPI
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

func (r *TunableResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state TunableModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.CallJob(ctx, "tunable.delete", state.ID.ValueInt64())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete tunable failed", err.Error())
	}
}

func (r *TunableResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, idDiags := listing.ImportInt64(ctx, req)
	resp.Diagnostics.Append(idDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "tunable.get_instance", id)
	if err != nil {
		resp.Diagnostics.AddError("Import tunable failed", err.Error())
		return
	}

	var apiResp tunableAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse import response", err.Error())
		return
	}

	var state TunableModel
	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, id)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// UpgradeState migrates state written by provider <= 1.5.24 (schema version
// 0), where update_initramfs was ordinary stored
// attribute. It is write-only since 1.6.0, and Terraform
// refuses a prior state that still carries a value for a write-only attribute
// ("Invalid resource state upgrade"), blocking plan, apply and destroy. The
// upgrader drops the stored value; the config still supplies it.
func (r *TunableResource) UpgradeState(context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{
		0: stateupgrade.DropAttrs("update_initramfs"),
	}
}

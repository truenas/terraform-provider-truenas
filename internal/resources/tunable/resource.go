// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package tunable

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &TunableResource{}
var _ resource.ResourceWithImportState = &TunableResource{}

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
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Import ID must be an integer", req.ID)
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
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

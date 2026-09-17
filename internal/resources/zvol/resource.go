// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package zvol

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &ZvolResource{}
var _ resource.ResourceWithImportState = &ZvolResource{}

type ZvolResource struct {
	client *client.Client
}

func NewResource() resource.Resource { return &ZvolResource{} }

func (r *ZvolResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_zvol"
}

func (r *ZvolResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *ZvolResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type",
			fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *ZvolResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ZvolModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Call(ctx, "pool.dataset.create", plan.apiPayload())
	if err != nil {
		resp.Diagnostics.AddError("Create zvol failed", err.Error())
		return
	}

	// Read back via get_instance for canonical state (create response omits some properties).
	raw, err := r.client.CallRead(ctx, "pool.dataset.get_instance", plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read after create failed", err.Error())
		return
	}

	var apiResp zvolAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse read response", err.Error())
		return
	}

	responseToModel(&apiResp, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ZvolResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ZvolModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "pool.dataset.get_instance", state.Name.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read zvol failed", err.Error())
		return
	}

	var apiResp zvolAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse read response", err.Error())
		return
	}

	responseToModel(&apiResp, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ZvolResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ZvolModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := plan.apiPayload()
	// name, type, volblocksize, and sparse are not updatable via update endpoint
	delete(payload, "name")
	delete(payload, "type")
	delete(payload, "volblocksize")
	delete(payload, "sparse")

	_, err := r.client.Call(ctx, "pool.dataset.update", plan.Name.ValueString(), payload)
	if err != nil {
		resp.Diagnostics.AddError("Update zvol failed", err.Error())
		return
	}

	// Re-read to get computed fields
	raw, err := r.client.CallRead(ctx, "pool.dataset.get_instance", plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read after update failed", err.Error())
		return
	}

	var apiResp zvolAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse update response", err.Error())
		return
	}

	responseToModel(&apiResp, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ZvolResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ZvolModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.CallJob(ctx, "pool.dataset.delete", state.Name.ValueString(),
		map[string]any{"recursive": false})
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete zvol failed", err.Error())
	}
}

func (r *ZvolResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	var state ZvolModel
	state.Name = types.StringValue(req.ID)
	state.ID = types.StringValue(req.ID)

	raw, err := r.client.CallRead(ctx, "pool.dataset.get_instance", req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Import zvol failed", err.Error())
		return
	}

	var apiResp zvolAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse import response", err.Error())
		return
	}

	responseToModel(&apiResp, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

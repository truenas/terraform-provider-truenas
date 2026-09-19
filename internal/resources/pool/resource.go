// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package pool

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &PoolResource{}
var _ resource.ResourceWithImportState = &PoolResource{}

// PoolResource manages a ZFS pool via the TrueNAS WebSocket API.
type PoolResource struct {
	client *client.Client
}

// NewResource returns a new PoolResource.
func NewResource() resource.Resource { return &PoolResource{} }

func (r *PoolResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_pool"
}

func (r *PoolResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *PoolResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data",
			fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *PoolResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan PoolModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.apiPayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallJob(ctx, "pool.create", payload)
	if err != nil {
		resp.Diagnostics.AddError("Create pool failed", err.Error())
		return
	}

	var apiResp poolAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse create response", err.Error())
		return
	}

	// autotrim is not accepted by pool.create (it is a pool.update field), so
	// apply it in a follow-up update when the user set it.
	if !plan.AutoTrim.IsNull() && !plan.AutoTrim.IsUnknown() {
		if _, err := r.client.CallJob(ctx, "pool.update", apiResp.ID,
			map[string]any{"autotrim": autotrimStr(plan.AutoTrim.ValueBool())}); err != nil {
			resp.Diagnostics.AddError("Set autotrim after pool create failed", err.Error())
			return
		}
		// Re-read so state reflects the applied autotrim.
		raw, err = r.client.CallRead(ctx, "pool.get_instance", apiResp.ID)
		if err != nil {
			resp.Diagnostics.AddError("Read-back after pool create failed", err.Error())
			return
		}
		if err := json.Unmarshal(raw, &apiResp); err != nil {
			resp.Diagnostics.AddError("Parse get_instance response", err.Error())
			return
		}
	}

	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *PoolResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state PoolModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "pool.get_instance", state.ID.ValueInt64())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read pool failed", err.Error())
		return
	}

	var apiResp poolAPI
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

func (r *PoolResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan PoolModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Only autotrim is updatable; topology and name are ForceNew.
	_, err := r.client.CallJob(ctx, "pool.update", plan.ID.ValueInt64(),
		map[string]any{"autotrim": autotrimStr(plan.AutoTrim.ValueBool())})
	if err != nil {
		resp.Diagnostics.AddError("Update pool failed", err.Error())
		return
	}

	// Re-read to get computed fields after update.
	raw, err := r.client.CallRead(ctx, "pool.get_instance", plan.ID.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Read after update failed", err.Error())
		return
	}

	var apiResp poolAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *PoolResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state PoolModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.CallJob(ctx, "pool.delete", state.ID.ValueInt64())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete pool failed", err.Error())
	}
}

func (r *PoolResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by pool name: query for pool with matching name, take first result.
	raw, err := r.client.CallRead(ctx, "pool.query", [][]any{{"name", "=", req.ID}})
	if err != nil {
		resp.Diagnostics.AddError("Import pool failed", err.Error())
		return
	}

	var pools []poolAPI
	if err := json.Unmarshal(raw, &pools); err != nil {
		resp.Diagnostics.AddError("Parse import response", err.Error())
		return
	}

	if len(pools) == 0 {
		resp.Diagnostics.AddError("Pool not found",
			fmt.Sprintf("no pool named %q found on TrueNAS", req.ID))
		return
	}

	var state PoolModel
	resp.Diagnostics.Append(responseToModel(ctx, &pools[0], &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

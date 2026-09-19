// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package periodic_snapshot

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &PeriodicSnapshotResource{}
var _ resource.ResourceWithImportState = &PeriodicSnapshotResource{}

// PeriodicSnapshotResource implements the truenas_periodic_snapshot_task resource.
type PeriodicSnapshotResource struct{ client *client.Client }

// NewResource returns a new instance of PeriodicSnapshotResource.
func NewResource() resource.Resource { return &PeriodicSnapshotResource{} }

func (r *PeriodicSnapshotResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_periodic_snapshot_task"
}

func (r *PeriodicSnapshotResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *PeriodicSnapshotResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *PeriodicSnapshotResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan PeriodicSnapshotModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.apiPayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallJob(ctx, "pool.snapshottask.create", payload)
	if err != nil {
		resp.Diagnostics.AddError("Create periodic snapshot task failed", err.Error())
		return
	}

	var created taskAPI
	if err := json.Unmarshal(raw, &created); err != nil {
		resp.Diagnostics.AddError("Parse create response", err.Error())
		return
	}

	raw, err = r.client.CallRead(ctx, "pool.snapshottask.get_instance", created.ID)
	if err != nil {
		resp.Diagnostics.AddError("Read-back after create failed", err.Error())
		return
	}

	var apiResp taskAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse read-back response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *PeriodicSnapshotResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state PeriodicSnapshotModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "pool.snapshottask.get_instance", state.ID.ValueInt64())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read periodic snapshot task failed", err.Error())
		return
	}

	var apiResp taskAPI
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

func (r *PeriodicSnapshotResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan PeriodicSnapshotModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.apiPayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := plan.ID.ValueInt64()
	_, err := r.client.CallJob(ctx, "pool.snapshottask.update", id, payload)
	if err != nil {
		resp.Diagnostics.AddError("Update periodic snapshot task failed", err.Error())
		return
	}

	raw, err := r.client.CallRead(ctx, "pool.snapshottask.get_instance", id)
	if err != nil {
		resp.Diagnostics.AddError("Read after update failed", err.Error())
		return
	}

	var apiResp taskAPI
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

func (r *PeriodicSnapshotResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state PeriodicSnapshotModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.CallJob(ctx, "pool.snapshottask.delete", state.ID.ValueInt64())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete periodic snapshot task failed", err.Error())
	}
}

func (r *PeriodicSnapshotResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Import ID must be an integer", req.ID)
		return
	}

	raw, err := r.client.CallRead(ctx, "pool.snapshottask.get_instance", id)
	if err != nil {
		resp.Diagnostics.AddError("Import periodic snapshot task failed", err.Error())
		return
	}

	var apiResp taskAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse import response", err.Error())
		return
	}

	var state PeriodicSnapshotModel
	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

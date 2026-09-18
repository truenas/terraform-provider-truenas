// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_subsys

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &NVMetSubsysResource{}
var _ resource.ResourceWithImportState = &NVMetSubsysResource{}

// NVMetSubsysResource implements the truenas_nvmet_subsys resource.
type NVMetSubsysResource struct{ client *client.Client }

// NewResource returns a new NVMetSubsysResource.
func NewResource() resource.Resource { return &NVMetSubsysResource{} }

func (r *NVMetSubsysResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_nvmet_subsys"
}

func (r *NVMetSubsysResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *NVMetSubsysResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *NVMetSubsysResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan NVMetSubsysModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.createPayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.Call(ctx, "nvmet.subsys.create", payload)
	if err != nil {
		resp.Diagnostics.AddError("Create NVMe-oF subsystem failed", err.Error())
		return
	}

	// Parse create response to get the ID, then read back via get_instance.
	var created nvmetSubsysAPI
	if err := json.Unmarshal(raw, &created); err != nil {
		resp.Diagnostics.AddError("Parse create response", err.Error())
		return
	}

	raw, err = r.client.CallRead(ctx, "nvmet.subsys.get_instance", created.ID)
	if err != nil {
		resp.Diagnostics.AddError("Read-back after create failed", err.Error())
		return
	}

	var apiResp nvmetSubsysAPI
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

func (r *NVMetSubsysResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state NVMetSubsysModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "nvmet.subsys.get_instance", state.ID.ValueInt64())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read NVMe-oF subsystem failed", err.Error())
		return
	}

	var apiResp nvmetSubsysAPI
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

func (r *NVMetSubsysResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan NVMetSubsysModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Carry the existing state ID into the plan (plan.ID is Unknown until apply).
	var state NVMetSubsysModel
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

	_, err := r.client.Call(ctx, "nvmet.subsys.update", plan.ID.ValueInt64(), payload)
	if err != nil {
		resp.Diagnostics.AddError("Update NVMe-oF subsystem failed", err.Error())
		return
	}

	raw, err := r.client.CallRead(ctx, "nvmet.subsys.get_instance", plan.ID.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Read-back after update failed", err.Error())
		return
	}

	var apiResp nvmetSubsysAPI
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

func (r *NVMetSubsysResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state NVMetSubsysModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Call(ctx, "nvmet.subsys.delete", state.ID.ValueInt64())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete NVMe-oF subsystem failed", err.Error())
	}
}

func (r *NVMetSubsysResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Import ID must be an integer", req.ID)
		return
	}

	raw, err := r.client.CallRead(ctx, "nvmet.subsys.get_instance", id)
	if err != nil {
		resp.Diagnostics.AddError("Import NVMe-oF subsystem failed", err.Error())
		return
	}

	var apiResp nvmetSubsysAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse import response", err.Error())
		return
	}

	var state NVMetSubsysModel
	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

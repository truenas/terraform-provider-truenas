// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nfs

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &NFSShareResource{}
var _ resource.ResourceWithImportState = &NFSShareResource{}

type NFSShareResource struct{ client *client.Client }

func NewResource() resource.Resource { return &NFSShareResource{} }

func (r *NFSShareResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_nfs_share"
}

func (r *NFSShareResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *NFSShareResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *NFSShareResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan NFSShareModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.Call(ctx, "sharing.nfs.create", plan.apiPayload())
	if err != nil {
		resp.Diagnostics.AddError("Create NFS share failed", err.Error())
		return
	}

	var apiResp apiResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse create response", err.Error())
		return
	}

	resp.Diagnostics.Append(r.responseToModel(ctx, &apiResp, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *NFSShareResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state NFSShareModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := strconv.ParseInt(state.ID.ValueString(), 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid ID", state.ID.ValueString())
		return
	}

	raw, err := r.client.CallRead(ctx, "sharing.nfs.get_instance", id)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read NFS share failed", err.Error())
		return
	}

	var apiResp apiResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse response", err.Error())
		return
	}

	resp.Diagnostics.Append(r.responseToModel(ctx, &apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *NFSShareResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan NFSShareModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := strconv.ParseInt(plan.ID.ValueString(), 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid NFS share ID", plan.ID.ValueString())
		return
	}
	_, err = r.client.Call(ctx, "sharing.nfs.update", id, plan.apiPayload())
	if err != nil {
		resp.Diagnostics.AddError("Update NFS share failed", err.Error())
		return
	}

	raw, err := r.client.CallRead(ctx, "sharing.nfs.get_instance", id)
	if err != nil {
		resp.Diagnostics.AddError("Read after update failed", err.Error())
		return
	}

	var apiResp apiResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse response", err.Error())
		return
	}

	resp.Diagnostics.Append(r.responseToModel(ctx, &apiResp, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *NFSShareResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state NFSShareModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := strconv.ParseInt(state.ID.ValueString(), 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid NFS share ID", state.ID.ValueString())
		return
	}
	_, err = r.client.Call(ctx, "sharing.nfs.delete", id)
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete NFS share failed", err.Error())
	}
}

func (r *NFSShareResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Import ID must be integer", req.ID)
		return
	}

	raw, err := r.client.CallRead(ctx, "sharing.nfs.get_instance", id)
	if err != nil {
		resp.Diagnostics.AddError("Import NFS share failed", err.Error())
		return
	}

	var apiResp apiResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse import response", err.Error())
		return
	}

	var state NFSShareModel
	resp.Diagnostics.Append(r.responseToModel(ctx, &apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *NFSShareResource) responseToModel(ctx context.Context, api *apiResponse, m *NFSShareModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.StringValue(strconv.FormatInt(api.ID, 10))
	m.Path = types.StringValue(api.Path)
	m.Comment = types.StringValue(api.Comment)
	m.Enabled = types.BoolValue(api.Enabled)
	m.ReadOnly = types.BoolValue(api.ReadOnly)
	m.MapRoot = types.StringValue(api.MapRoot)
	m.MapGroup = types.StringValue(api.MapGroup)
	m.MapAll = types.StringValue(api.MapAll)
	m.MapAllGr = types.StringValue(api.MapAllGr)
	m.ExposeSns = types.BoolValue(api.ExposeSn)

	security, dSec := types.ListValueFrom(ctx, types.StringType, api.Security)
	diags.Append(dSec...)
	m.Security = security

	networks, d := types.ListValueFrom(ctx, types.StringType, api.Networks)
	diags.Append(d...)
	m.Networks = networks
	hosts, d := types.ListValueFrom(ctx, types.StringType, api.Hosts)
	diags.Append(d...)
	m.Hosts = hosts
	return diags
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package webshare

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &WebshareResource{}
var _ resource.ResourceWithImportState = &WebshareResource{}

// WebshareResource implements the truenas_webshare resource.
type WebshareResource struct{ client *client.Client }

// NewResource returns a new WebshareResource.
func NewResource() resource.Resource { return &WebshareResource{} }

func (r *WebshareResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webshare"
}

func (r *WebshareResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *WebshareResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// checkVersion probes the target server's release and returns a clean error
// diagnostic if it is below the TrueNAS 26.0 floor the sharing.webshare
// namespace requires, before any sharing.webshare.* call is made. Every
// resource entry point (Create/Read/Update/Delete/ImportState) and the
// datasource's Read call this first.
func (r *WebshareResource) checkVersion(ctx context.Context) diag.Diagnostics {
	var diags diag.Diagnostics
	version, err := r.client.ServerVersion(ctx)
	if err != nil {
		diags.AddError("Version probe failed", err.Error())
		return diags
	}
	diags.Append(versionGateDiagnostics(version)...)
	return diags
}

// getInstance reads back a share by ID via sharing.webshare.get_instance
// (job:false, probed live).
func (r *WebshareResource) getInstance(ctx context.Context, id int64) (*webshareAPI, error) {
	raw, err := r.client.CallRead(ctx, "sharing.webshare.get_instance", id)
	if err != nil {
		return nil, err
	}
	var api webshareAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		return nil, err
	}
	return &api, nil
}

func (r *WebshareResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	resp.Diagnostics.Append(r.checkVersion(ctx)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan WebshareModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.Call(ctx, "sharing.webshare.create", plan.createPayload())
	if err != nil {
		resp.Diagnostics.AddError("Create Webshare share failed", err.Error())
		return
	}

	var created webshareAPI
	if err := json.Unmarshal(raw, &created); err != nil {
		resp.Diagnostics.AddError("Parse create response", err.Error())
		return
	}

	apiResp, err := r.getInstance(ctx, created.ID)
	if err != nil {
		resp.Diagnostics.AddError("Read-back after create failed", err.Error())
		return
	}

	responseToModel(apiResp, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *WebshareResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	resp.Diagnostics.Append(r.checkVersion(ctx)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state WebshareModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.getInstance(ctx, state.ID.ValueInt64())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read Webshare share failed", err.Error())
		return
	}

	responseToModel(apiResp, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *WebshareResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.Append(r.checkVersion(ctx)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan WebshareModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Carry the existing state ID into the plan (plan.ID is Unknown until apply).
	var state WebshareModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID

	_, err := r.client.Call(ctx, "sharing.webshare.update", plan.ID.ValueInt64(), plan.updatePayload())
	if err != nil {
		resp.Diagnostics.AddError("Update Webshare share failed", err.Error())
		return
	}

	apiResp, err := r.getInstance(ctx, plan.ID.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Read-back after update failed", err.Error())
		return
	}

	responseToModel(apiResp, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *WebshareResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.Append(r.checkVersion(ctx)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state WebshareModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// sharing.webshare.delete is job:false (probed live).
	_, err := r.client.Call(ctx, "sharing.webshare.delete", state.ID.ValueInt64())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete Webshare share failed", err.Error())
	}
}

func (r *WebshareResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(r.checkVersion(ctx)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Import ID must be an integer", req.ID)
		return
	}

	apiResp, err := r.getInstance(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Import Webshare share failed", err.Error())
		return
	}

	var state WebshareModel
	responseToModel(apiResp, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

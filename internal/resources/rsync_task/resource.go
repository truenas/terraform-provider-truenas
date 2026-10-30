// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package rsync_task

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
	"github.com/truenas/terraform-provider-truenas/internal/listing"
	"github.com/truenas/terraform-provider-truenas/internal/stateupgrade"
)

var _ resource.Resource = &RsyncTaskResource{}
var _ resource.ResourceWithUpgradeState = &RsyncTaskResource{}
var _ resource.ResourceWithImportState = &RsyncTaskResource{}
var _ resource.ResourceWithIdentity = &RsyncTaskResource{}

// RsyncTaskResource implements the truenas_rsync_task resource.
type RsyncTaskResource struct{ client *client.Client }

// NewResource returns a new instance of RsyncTaskResource.
func NewResource() resource.Resource { return &RsyncTaskResource{} }

func (r *RsyncTaskResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_rsync_task"
}

func (r *RsyncTaskResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *RsyncTaskResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = listing.IntIDIdentitySchema()
}

func (r *RsyncTaskResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// rsynctask.create/update/delete/query/get_instance are all non-job (sync)
// methods: probed via `core.get_methods`, every one of them reports
// "job": false. rsynctask.run (manual trigger) is the only job-type method
// in this namespace, and this resource never calls it.

func (r *RsyncTaskResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan RsyncTaskModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(copyDirectives(ctx, req.Config, &plan)...)
	payload, diags := plan.apiPayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.clearDirectives()

	raw, err := r.client.Call(ctx, "rsynctask.create", payload)
	if err != nil {
		resp.Diagnostics.AddError("Create rsync task failed", err.Error())
		return
	}

	var apiResp rsyncTaskAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse create response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueInt64())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *RsyncTaskResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state RsyncTaskModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "rsynctask.get_instance", state.ID.ValueInt64())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read rsync task failed", err.Error())
		return
	}

	var apiResp rsyncTaskAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse response", err.Error())
		return
	}

	// validate_rpath and ssh_keyscan are never returned by the API;
	// responseToModel leaves state's existing values for them untouched.
	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, state.ID.ValueInt64())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *RsyncTaskResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan RsyncTaskModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state RsyncTaskModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(copyDirectives(ctx, req.Config, &plan)...)
	payload, diags := plan.apiPayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.clearDirectives()

	raw, err := r.client.Call(ctx, "rsynctask.update", state.ID.ValueInt64(), payload)
	if err != nil {
		resp.Diagnostics.AddError("Update rsync task failed", err.Error())
		return
	}

	var apiResp rsyncTaskAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueInt64())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *RsyncTaskResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state RsyncTaskModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Call(ctx, "rsynctask.delete", state.ID.ValueInt64())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete rsync task failed", err.Error())
	}
}

func (r *RsyncTaskResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, idDiags := listing.ImportInt64(ctx, req)
	resp.Diagnostics.Append(idDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "rsynctask.get_instance", id)
	if err != nil {
		resp.Diagnostics.AddError("Import rsync task failed", err.Error())
		return
	}

	var apiResp rsyncTaskAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse import response", err.Error())
		return
	}

	var state RsyncTaskModel
	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// validate_rpath and ssh_keyscan are write-only; they stay null.
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, id)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// UpgradeState migrates state written by provider <= 1.5.24 (schema version
// 0), where validate_rpath, ssh_keyscan were ordinary stored
// attributes. They are write-only since 1.6.0, and Terraform
// refuses a prior state that still carries a value for a write-only attribute
// ("Invalid resource state upgrade"), blocking plan, apply and destroy. The
// upgrader drops the stored values; the config still supplies them.
func (r *RsyncTaskResource) UpgradeState(context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{
		0: stateupgrade.DropAttrs("validate_rpath", "ssh_keyscan"),
	}
}

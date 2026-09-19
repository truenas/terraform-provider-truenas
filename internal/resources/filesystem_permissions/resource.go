// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package filesystem_permissions

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &FilesystemPermissionsResource{}
var _ resource.ResourceWithImportState = &FilesystemPermissionsResource{}

// FilesystemPermissionsResource implements the
// truenas_filesystem_permissions resource: a declarative wrapper around
// the imperative filesystem.setperm action, keyed by path rather than a
// TrueNAS-assigned id.
type FilesystemPermissionsResource struct{ client *client.Client }

// NewResource returns a new FilesystemPermissionsResource.
func NewResource() resource.Resource { return &FilesystemPermissionsResource{} }

func (r *FilesystemPermissionsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_filesystem_permissions"
}

func (r *FilesystemPermissionsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *FilesystemPermissionsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// filesystem.setperm is job:true (probed live via core.get_methods and a
// real CallJob round trip) - dispatched via CallJob. filesystem.stat is
// job:false (plain synchronous Call).

// applyAndStat calls filesystem.setperm (only when the model actually
// specifies mode/uid/gid — an all-omitted model is a read-only "adopt
// whatever's already there" configuration, so no write call is made) and
// then always reads back the current state via filesystem.stat.
func (r *FilesystemPermissionsResource) applyAndStat(ctx context.Context, m *FilesystemPermissionsModel) (*fsStatAPI, error) {
	if m.hasWrite() {
		if _, err := r.client.CallJob(ctx, "filesystem.setperm", m.setpermPayload()); err != nil {
			return nil, fmt.Errorf("filesystem.setperm: %w", err)
		}
	}

	raw, err := r.client.CallRead(ctx, "filesystem.stat", m.Path.ValueString())
	if err != nil {
		return nil, fmt.Errorf("filesystem.stat: %w", err)
	}
	var api fsStatAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		return nil, fmt.Errorf("parse filesystem.stat response: %w", err)
	}
	return &api, nil
}

func (r *FilesystemPermissionsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan FilesystemPermissionsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	api, err := r.applyAndStat(ctx, &plan)
	if err != nil {
		resp.Diagnostics.AddError("Create filesystem permissions failed", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(api, plan.Path.ValueString(), &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *FilesystemPermissionsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state FilesystemPermissionsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "filesystem.stat", state.Path.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read filesystem permissions failed", err.Error())
		return
	}

	var api fsStatAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		resp.Diagnostics.AddError("Parse filesystem.stat response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(&api, state.Path.ValueString(), &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *FilesystemPermissionsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan FilesystemPermissionsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	api, err := r.applyAndStat(ctx, &plan)
	if err != nil {
		resp.Diagnostics.AddError("Update filesystem permissions failed", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(api, plan.Path.ValueString(), &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete makes NO client calls: a filesystem path's mode/uid/gid are not
// an object TrueNAS creates or destroys on this resource's behalf, so
// removing it from Terraform state must never revert them. See
// deleteWarningDiagnostics and the resource schema's Description.
func (r *FilesystemPermissionsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state FilesystemPermissionsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(deleteWarningDiagnostics(state.Path.ValueString())...)
}

func (r *FilesystemPermissionsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by path (e.g. "terraform import truenas_filesystem_permissions.example /mnt/tank/mydata").
	// recursive/traverse are apply-time-only options with nothing on the
	// wire to recover them from; they're left null and must be ignored via
	// ImportStateVerifyIgnore.
	raw, err := r.client.CallRead(ctx, "filesystem.stat", req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Import filesystem permissions failed", err.Error())
		return
	}

	var api fsStatAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		resp.Diagnostics.AddError("Parse filesystem.stat response", err.Error())
		return
	}

	var state FilesystemPermissionsModel
	resp.Diagnostics.Append(responseToModel(&api, req.ID, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_extent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
	"github.com/truenas/terraform-provider-truenas/internal/listing"
)

var _ resource.Resource = &ISCSIExtentResource{}
var _ resource.ResourceWithImportState = &ISCSIExtentResource{}
var _ resource.ResourceWithIdentity = &ISCSIExtentResource{}
var _ resource.ResourceWithValidateConfig = &ISCSIExtentResource{}

// ISCSIExtentResource implements the truenas_iscsi_extent resource.
type ISCSIExtentResource struct{ client *client.Client }

// NewResource returns a new ISCSIExtentResource.
func NewResource() resource.Resource { return &ISCSIExtentResource{} }

func (r *ISCSIExtentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iscsi_extent"
}

func (r *ISCSIExtentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *ISCSIExtentResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = listing.IntIDIdentitySchema()
}

func (r *ISCSIExtentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ISCSIExtentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ISCSIExtentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.apiPayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := callWaitingForDevice(ctx, r.client, "iscsi.extent.create", payload)
	if err != nil {
		resp.Diagnostics.AddError("Create iSCSI extent failed", err.Error())
		return
	}

	// Parse create response to get the ID, then read back via get_instance.
	var created extentAPI
	if err := json.Unmarshal(raw, &created); err != nil {
		resp.Diagnostics.AddError("Parse create response", err.Error())
		return
	}

	raw, err = r.client.CallRead(ctx, "iscsi.extent.get_instance", created.ID)
	if err != nil {
		resp.Diagnostics.AddError("Read-back after create failed", err.Error())
		return
	}

	var apiResp extentAPI
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

func (r *ISCSIExtentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ISCSIExtentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "iscsi.extent.get_instance", state.ID.ValueInt64())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read iSCSI extent failed", err.Error())
		return
	}

	var apiResp extentAPI
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

func (r *ISCSIExtentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ISCSIExtentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Carry the existing state ID into the plan (plan.ID is Unknown until apply).
	var state ISCSIExtentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID

	payload, diags := plan.apiPayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := callWaitingForDevice(ctx, r.client, "iscsi.extent.update", plan.ID.ValueInt64(), payload)
	if err != nil {
		resp.Diagnostics.AddError("Update iSCSI extent failed", err.Error())
		return
	}

	raw, err := r.client.CallRead(ctx, "iscsi.extent.get_instance", plan.ID.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Read-back after update failed", err.Error())
		return
	}

	var apiResp extentAPI
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

func (r *ISCSIExtentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ISCSIExtentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// TrueNAS 26.0: delete takes positional booleans (id, remove, force), not an
	// options object (verified live). remove=false keeps file-backed data.
	_, err := r.client.Call(ctx, "iscsi.extent.delete", state.ID.ValueInt64(), false, false)
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete iSCSI extent failed", err.Error())
		return
	}

	// ro = true made TrueNAS set readonly=on on the backing zvol, and the
	// server's delete does not undo it, so the zvol would stay read-only
	// after the extent is gone. Reset it to inherit. A failure here is a
	// warning: the extent itself is deleted.
	if state.ReadOnly.ValueBool() && state.Type.ValueString() == "DISK" {
		if name, ok := zvolName(state.Disk.ValueString()); ok {
			if _, err := r.client.Call(ctx, "pool.dataset.update", name, map[string]any{"readonly": "INHERIT"}); err != nil && !client.IsNotFound(err) {
				resp.Diagnostics.AddWarning("Could not reset zvol readonly property",
					fmt.Sprintf("The extent was deleted, but resetting readonly to inherit on zvol %q failed: %s. "+
						"TrueNAS set readonly=on on it because the extent had ro = true.", name, err))
			}
		}
	}
}

// ValidateConfig rejects disk/path combinations TrueNAS would refuse or
// silently rewrite: a DISK extent needs disk and stores it as path, and a
// FILE extent needs path and has no disk.
func (r *ISCSIExtentResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var cfg ISCSIExtentModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() || cfg.Type.IsUnknown() || cfg.Type.IsNull() {
		return
	}
	switch cfg.Type.ValueString() {
	case "DISK":
		if cfg.Disk.IsNull() || (!cfg.Disk.IsUnknown() && cfg.Disk.ValueString() == "") {
			resp.Diagnostics.AddAttributeError(path.Root("disk"), "disk is required for a DISK extent",
				"Set disk to the zvol path, for example \"zvol/${truenas_zvol.x.name}\".")
		}
		if !cfg.Path.IsNull() && !cfg.Path.IsUnknown() && !cfg.Disk.IsUnknown() && cfg.Path.ValueString() != cfg.Disk.ValueString() {
			resp.Diagnostics.AddAttributeError(path.Root("path"), "path must be unset for a DISK extent",
				"TrueNAS stores a DISK extent's zvol in path, so path always reads back as the disk value. Remove path.")
		}
	case "FILE":
		if cfg.Path.IsNull() || (!cfg.Path.IsUnknown() && cfg.Path.ValueString() == "") {
			resp.Diagnostics.AddAttributeError(path.Root("path"), "path is required for a FILE extent",
				"Set path to the backing file, for example \"${truenas_dataset.x.mountpoint}/extent.img\".")
		}
		if !cfg.Disk.IsNull() && !cfg.Disk.IsUnknown() && cfg.Disk.ValueString() != "" {
			resp.Diagnostics.AddAttributeError(path.Root("disk"), "disk must be unset for a FILE extent",
				"TrueNAS ignores disk for FILE extents and reports it as empty. Remove disk.")
		}
	}
}

func (r *ISCSIExtentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, idDiags := listing.ImportInt64(ctx, req)
	resp.Diagnostics.Append(idDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "iscsi.extent.get_instance", id)
	if err != nil {
		resp.Diagnostics.AddError("Import iSCSI extent failed", err.Error())
		return
	}

	var apiResp extentAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse import response", err.Error())
		return
	}

	var state ISCSIExtentModel
	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, id)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

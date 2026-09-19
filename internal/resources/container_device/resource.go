// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package container_device

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

var _ resource.Resource = &ContainerDeviceResource{}
var _ resource.ResourceWithImportState = &ContainerDeviceResource{}

// ContainerDeviceResource implements the truenas_container_device resource.
type ContainerDeviceResource struct{ client *client.Client }

// NewResource returns a new ContainerDeviceResource.
func NewResource() resource.Resource { return &ContainerDeviceResource{} }

func (r *ContainerDeviceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_container_device"
}

func (r *ContainerDeviceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *ContainerDeviceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
// diagnostic if it is below the TrueNAS 26.0 floor the container.device
// namespace requires, before any container.device.* call is made. Every
// resource entry point (Create/Read/Update/Delete/ImportState) calls this
// first — Delete included, since a version gate that only covers
// Create/Read/Update would let Delete reach the wire with a raw API error
// on an unsupported release instead of the same clean diagnostic.
func (r *ContainerDeviceResource) checkVersion(ctx context.Context) diag.Diagnostics {
	var diags diag.Diagnostics
	version, err := r.client.ServerVersion(ctx)
	if err != nil {
		diags.AddError("Version probe failed", err.Error())
		return diags
	}
	diags.Append(versionGateDiagnostics(version)...)
	return diags
}

// getInstance reads back a container device by ID via
// container.device.get_instance (job:false, probed live).
func (r *ContainerDeviceResource) getInstance(ctx context.Context, id int64) (*containerDeviceAPI, error) {
	raw, err := r.client.CallRead(ctx, "container.device.get_instance", id)
	if err != nil {
		return nil, err
	}
	var api containerDeviceAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		return nil, err
	}
	return &api, nil
}

func (r *ContainerDeviceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	resp.Diagnostics.Append(r.checkVersion(ctx)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan ContainerDeviceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.createPayload()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// container.device.create is job:false (probed live).
	raw, err := r.client.Call(ctx, "container.device.create", payload)
	if err != nil {
		resp.Diagnostics.AddError("Create container device failed", err.Error())
		return
	}

	// Parse only the ID from the create response, then read back via
	// get_instance to pick up server-assigned defaults (e.g. an
	// auto-generated NIC "mac", or FILESYSTEM's own quirky "target"
	// default — see schema.go's Description).
	var created containerDeviceAPI
	if err := json.Unmarshal(raw, &created); err != nil {
		resp.Diagnostics.AddError("Parse create response", err.Error())
		return
	}

	apiResp, err := r.getInstance(ctx, created.ID)
	if err != nil {
		resp.Diagnostics.AddError("Read-back after create failed", err.Error())
		return
	}

	// Keep the plan's Attributes JSON string in state (write-what-you-said):
	// the API may normalize/add default keys.
	responseToModel(apiResp, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ContainerDeviceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	resp.Diagnostics.Append(r.checkVersion(ctx)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state ContainerDeviceModel
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
		resp.Diagnostics.AddError("Read container device failed", err.Error())
		return
	}

	responseToModel(apiResp, &state)

	// Drift-aware attributes handling: only overwrite state Attributes when
	// a user-set key's value differs in the API. API-added default keys are
	// not considered drift.
	var stateAttrs map[string]any
	if err := json.Unmarshal([]byte(state.Attributes.ValueString()), &stateAttrs); err != nil {
		resp.Diagnostics.AddError("Parse state attributes JSON", err.Error())
		return
	}
	if attributesDrifted(stateAttrs, apiResp.Attributes) {
		attrJSON, diags := apiAttributesJSON(apiResp)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		state.Attributes = types.StringValue(attrJSON)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ContainerDeviceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.Append(r.checkVersion(ctx)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan ContainerDeviceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Carry the existing state ID into the plan.
	var state ContainerDeviceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID

	payload, diags := plan.updatePayload()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// container.device.update is job:false (probed live).
	_, err := r.client.Call(ctx, "container.device.update", plan.ID.ValueInt64(), payload)
	if err != nil {
		resp.Diagnostics.AddError("Update container device failed", err.Error())
		return
	}

	apiResp, err := r.getInstance(ctx, plan.ID.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Read-back after update failed", err.Error())
		return
	}

	// Keep the plan's Attributes JSON string in state (write-what-you-said).
	responseToModel(apiResp, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ContainerDeviceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.Append(r.checkVersion(ctx)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state ContainerDeviceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// container.device.delete is job:false (probed live). Its "options"
	// arg (force/raw_file/zvol) defaults to all-false server-side (probed
	// via core.get_methods), matching this call's intent exactly, so it is
	// omitted rather than resent explicitly.
	_, err := r.client.Call(ctx, "container.device.delete", state.ID.ValueInt64())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete container device failed", err.Error())
	}
}

func (r *ContainerDeviceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
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
		resp.Diagnostics.AddError("Import container device failed", err.Error())
		return
	}

	var state ContainerDeviceModel
	responseToModel(apiResp, &state)
	attrJSON, diags := apiAttributesJSON(apiResp)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Attributes = types.StringValue(attrJSON)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

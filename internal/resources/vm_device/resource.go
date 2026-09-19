// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm_device

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &VMDeviceResource{}
var _ resource.ResourceWithImportState = &VMDeviceResource{}

// VMDeviceResource implements the truenas_vm_device resource.
type VMDeviceResource struct{ client *client.Client }

// NewResource returns a new VMDeviceResource.
func NewResource() resource.Resource { return &VMDeviceResource{} }

func (r *VMDeviceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vm_device"
}

func (r *VMDeviceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *VMDeviceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *VMDeviceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan VMDeviceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.createPayload()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.Call(ctx, "vm.device.create", payload)
	if err != nil {
		resp.Diagnostics.AddError("Create VM device failed", err.Error())
		return
	}

	// Parse only the ID from the create response, then read back via
	// get_instance to pick up server-assigned defaults (e.g. order).
	var created deviceAPI
	if err := json.Unmarshal(raw, &created); err != nil {
		resp.Diagnostics.AddError("Parse create response", err.Error())
		return
	}

	raw, err = r.client.CallRead(ctx, "vm.device.get_instance", created.ID)
	if err != nil {
		resp.Diagnostics.AddError("Read-back after create failed", err.Error())
		return
	}

	var apiResp deviceAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse get_instance response", err.Error())
		return
	}

	// Keep the plan's Attributes JSON string in state (write-what-you-said):
	// the API may normalize/add default keys.
	responseToModel(&apiResp, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *VMDeviceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state VMDeviceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "vm.device.get_instance", state.ID.ValueInt64())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read VM device failed", err.Error())
		return
	}

	var apiResp deviceAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse response", err.Error())
		return
	}

	responseToModel(&apiResp, &state)

	// Drift-aware attributes handling: only overwrite state Attributes when
	// a user-set key's value differs in the API. API-added default keys are
	// not considered drift.
	var stateAttrs map[string]any
	if err := json.Unmarshal([]byte(state.Attributes.ValueString()), &stateAttrs); err != nil {
		resp.Diagnostics.AddError("Parse state attributes JSON", err.Error())
		return
	}
	if attributesDrifted(stateAttrs, apiResp.Attributes) {
		attrJSON, diags := apiAttributesJSON(&apiResp)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		state.Attributes = types.StringValue(attrJSON)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *VMDeviceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan VMDeviceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Carry the existing state ID into the plan.
	var state VMDeviceModel
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

	_, err := r.client.Call(ctx, "vm.device.update", plan.ID.ValueInt64(), payload)
	if err != nil {
		resp.Diagnostics.AddError("Update VM device failed", err.Error())
		return
	}

	raw, err := r.client.CallRead(ctx, "vm.device.get_instance", plan.ID.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Read-back after update failed", err.Error())
		return
	}

	var apiResp deviceAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse get_instance response", err.Error())
		return
	}

	// Keep the plan's Attributes JSON string in state (write-what-you-said).
	responseToModel(&apiResp, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *VMDeviceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state VMDeviceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Call(ctx, "vm.device.delete", state.ID.ValueInt64(), map[string]any{
		"force":    false,
		"raw_file": false,
		"zvol":     false,
	})
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete VM device failed", err.Error())
	}
}

func (r *VMDeviceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Import ID must be an integer", req.ID)
		return
	}

	raw, err := r.client.CallRead(ctx, "vm.device.get_instance", id)
	if err != nil {
		resp.Diagnostics.AddError("Import VM device failed", err.Error())
		return
	}

	var apiResp deviceAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse import response", err.Error())
		return
	}

	var state VMDeviceModel
	responseToModel(&apiResp, &state)
	attrJSON, diags := apiAttributesJSON(&apiResp)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Attributes = types.StringValue(attrJSON)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

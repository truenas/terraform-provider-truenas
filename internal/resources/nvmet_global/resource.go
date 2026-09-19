// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_global

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &NVMeTGlobalResource{}
var _ resource.ResourceWithImportState = &NVMeTGlobalResource{}

// NVMeTGlobalResource implements the truenas_nvmet_global singleton resource.
type NVMeTGlobalResource struct{ client *client.Client }

// NewResource returns a new NVMeTGlobalResource.
func NewResource() resource.Resource { return &NVMeTGlobalResource{} }

func (r *NVMeTGlobalResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_nvmet_global"
}

func (r *NVMeTGlobalResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *NVMeTGlobalResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// fetchConfig calls nvmet.global.config and unmarshals the response.
func (r *NVMeTGlobalResource) fetchConfig(ctx context.Context) (*nvmetGlobalAPI, error) {
	raw, err := r.client.CallRead(ctx, "nvmet.global.config")
	if err != nil {
		return nil, err
	}
	var api nvmetGlobalAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		return nil, err
	}
	return &api, nil
}

func (r *NVMeTGlobalResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan NVMeTGlobalModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.updatePayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.Call(ctx, "nvmet.global.update", payload); err != nil {
		resp.Diagnostics.AddError("Create NVMe-oF global configuration failed", err.Error())
		return
	}

	api, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read-back after create failed", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, api, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *NVMeTGlobalResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state NVMeTGlobalModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// This is a singleton: nvmet.global.config always exists, so Read never
	// removes the resource from state (there is no "not found" case).
	api, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read NVMe-oF global configuration failed", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *NVMeTGlobalResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan NVMeTGlobalModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.updatePayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.Call(ctx, "nvmet.global.update", payload); err != nil {
		resp.Diagnostics.AddError("Update NVMe-oF global configuration failed", err.Error())
		return
	}

	api, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read-back after update failed", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, api, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete makes NO client calls: the NVMe-oF global configuration serves live
// storage, so removing this resource from Terraform state must never alter
// the box's NVMe-oF service configuration. It only emits a warning
// diagnostic; Terraform itself drops the resource from state once Delete
// returns.
func (r *NVMeTGlobalResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.Append(deleteWarningDiagnostics()...)
}

func (r *NVMeTGlobalResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Any import ID is accepted and normalized to the fixed singleton ID; all
	// other fields are left unset so the subsequent Read call populates them
	// directly from nvmet.global.config.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), nvmetGlobalResourceID)...)
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package resilver_config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &ResilverConfigResource{}
var _ resource.ResourceWithImportState = &ResilverConfigResource{}

// ResilverConfigResource implements the truenas_resilver_config singleton
// resource.
type ResilverConfigResource struct{ client *client.Client }

// NewResource returns a new ResilverConfigResource.
func NewResource() resource.Resource { return &ResilverConfigResource{} }

func (r *ResilverConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_resilver_config"
}

func (r *ResilverConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *ResilverConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// fetchConfig calls pool.resilver.config and unmarshals the response.
func (r *ResilverConfigResource) fetchConfig(ctx context.Context) (*resilverConfigAPI, error) {
	raw, err := r.client.CallRead(ctx, "pool.resilver.config")
	if err != nil {
		return nil, err
	}
	var api resilverConfigAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		return nil, err
	}
	return &api, nil
}

func (r *ResilverConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ResilverConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.updatePayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.Call(ctx, "pool.resilver.update", payload); err != nil {
		resp.Diagnostics.AddError("Create resilver configuration failed", err.Error())
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

func (r *ResilverConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ResilverConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// This is a singleton: pool.resilver.config always exists, so Read never
	// removes the resource from state (there is no "not found" case).
	api, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read resilver configuration failed", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ResilverConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ResilverConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.updatePayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.Call(ctx, "pool.resilver.update", payload); err != nil {
		resp.Diagnostics.AddError("Update resilver configuration failed", err.Error())
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

// Delete makes NO client calls: the resilver schedule is system-critical
// pool maintenance configuration, so removing this resource from Terraform
// state must never reset the box's resilver schedule. It only emits a
// warning diagnostic; Terraform itself drops the resource from state once
// Delete returns.
func (r *ResilverConfigResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.Append(deleteWarningDiagnostics()...)
}

func (r *ResilverConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Any import ID is accepted and normalized to the fixed singleton ID; all
	// other fields are left unset so the subsequent Read call populates them
	// directly from pool.resilver.config.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), resilverConfigResourceID)...)
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package ups_config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &UPSConfigResource{}
var _ resource.ResourceWithImportState = &UPSConfigResource{}

// UPSConfigResource implements the truenas_ups_config singleton resource.
type UPSConfigResource struct{ client *client.Client }

// NewResource returns a new UPSConfigResource.
func NewResource() resource.Resource { return &UPSConfigResource{} }

func (r *UPSConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ups_config"
}

func (r *UPSConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *UPSConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// fetchConfig calls ups.config and unmarshals the response.
func (r *UPSConfigResource) fetchConfig(ctx context.Context) (*upsConfigAPI, error) {
	raw, err := r.client.CallRead(ctx, "ups.config")
	if err != nil {
		return nil, err
	}
	var api upsConfigAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		return nil, err
	}
	return &api, nil
}

func (r *UPSConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan UPSConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// write-only: the framework nulls WriteOnly attributes in req.Plan, so
	// the actual monpwd value is only available via req.Config.
	var cfg UPSConfigModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.MonPwd = cfg.MonPwd

	// ups.update requires several fields (at minimum port and driver) on
	// every call, so the live config is fetched first and merged with the
	// plan's known values: this lets a config that sets only one field
	// (e.g. description) still send a complete, valid payload.
	live, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read current UPS configuration failed", err.Error())
		return
	}

	if _, err := r.client.Call(ctx, "ups.update", mergedPayload(live, &plan)); err != nil {
		resp.Diagnostics.AddError("Create UPS configuration failed", err.Error())
		return
	}

	api, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read-back after create failed", err.Error())
		return
	}

	// responseToModel never touches plan.MonPwd (write-only), so the plan's
	// secret value — whatever the user set, or null if they didn't — is
	// preserved in state as-is.
	resp.Diagnostics.Append(responseToModel(api, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *UPSConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state UPSConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// This is a singleton: ups.config always exists, so Read never removes
	// the resource from state (there is no "not found" case).
	api, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read UPS configuration failed", err.Error())
		return
	}

	// responseToModel never touches state.MonPwd (write-only), so the
	// existing state's secret value is preserved unchanged.
	resp.Diagnostics.Append(responseToModel(api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *UPSConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan UPSConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// write-only: value lives in config, not plan.
	var cfg UPSConfigModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.MonPwd = cfg.MonPwd

	// See Create: ups.update requires several fields on every call, so the
	// live config is fetched first and merged with the plan's known values.
	live, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read current UPS configuration failed", err.Error())
		return
	}

	if _, err := r.client.Call(ctx, "ups.update", mergedPayload(live, &plan)); err != nil {
		resp.Diagnostics.AddError("Update UPS configuration failed", err.Error())
		return
	}

	api, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read-back after update failed", err.Error())
		return
	}

	// responseToModel never touches plan.MonPwd (write-only), so the plan's
	// secret value is preserved in state as-is.
	resp.Diagnostics.Append(responseToModel(api, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete makes NO client calls: UPS settings are system-critical power
// management configuration, so removing this resource from Terraform state
// must never reset the box's UPS configuration. It only emits a warning
// diagnostic; Terraform itself drops the resource from state once Delete
// returns.
func (r *UPSConfigResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.Append(deleteWarningDiagnostics()...)
}

func (r *UPSConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Any import ID is accepted and normalized to the fixed singleton ID; all
	// other fields are left unset so the subsequent Read call populates them
	// directly from ups.config.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), upsConfigResourceID)...)
}

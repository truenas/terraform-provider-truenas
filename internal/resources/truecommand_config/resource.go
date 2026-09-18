// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package truecommand_config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &TrueCommandConfigResource{}
var _ resource.ResourceWithImportState = &TrueCommandConfigResource{}

// TrueCommandConfigResource implements the truenas_truecommand_config
// singleton resource. No version gate is needed: truecommand.config/
// truecommand.update are present, with an identical shape, on both probed
// releases (25.10.4 HA and 26.0) — see model.go's doc comments for the
// probe evidence.
type TrueCommandConfigResource struct{ client *client.Client }

// NewResource returns a new TrueCommandConfigResource.
func NewResource() resource.Resource { return &TrueCommandConfigResource{} }

func (r *TrueCommandConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_truecommand_config"
}

func (r *TrueCommandConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *TrueCommandConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// fetchConfig calls truecommand.config and unmarshals the response.
func (r *TrueCommandConfigResource) fetchConfig(ctx context.Context) (*truecommandConfigAPI, error) {
	raw, err := r.client.CallRead(ctx, "truecommand.config")
	if err != nil {
		return nil, err
	}
	var api truecommandConfigAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		return nil, err
	}
	return &api, nil
}

// applyUpdate calls truecommand.update. Probed job:false on both releases,
// so a plain synchronous r.client.Call is sufficient (no CallJob polling
// needed). payload MUST come from TrueCommandConfigModel.updatePayload —
// see its doc comment for the safety contract this depends on.
func (r *TrueCommandConfigResource) applyUpdate(ctx context.Context, payload map[string]any) error {
	_, err := r.client.Call(ctx, "truecommand.update", payload)
	return err
}

func (r *TrueCommandConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan TrueCommandConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build the payload from req.Config, NOT plan — see updatePayload's doc
	// comment for why sourcing from the plan would risk resending "enabled"
	// even when the user never configured it.
	var config TrueCommandConfigModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.applyUpdate(ctx, config.updatePayload()); err != nil {
		resp.Diagnostics.AddError("Create TrueCommand configuration failed", err.Error())
		return
	}

	api, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read-back after create failed", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(api, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *TrueCommandConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state TrueCommandConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// This is a singleton: truecommand.config always exists, so Read never
	// removes the resource from state — there is no "not found" case.
	api, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read TrueCommand configuration failed", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *TrueCommandConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan TrueCommandConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build the payload from req.Config, NOT plan — see updatePayload's doc
	// comment (both "enabled" and "api_key" carry UseStateForUnknown, so
	// req.Plan would echo prior state for anything left unset in HCL).
	var config TrueCommandConfigModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.applyUpdate(ctx, config.updatePayload()); err != nil {
		resp.Diagnostics.AddError("Update TrueCommand configuration failed", err.Error())
		return
	}

	api, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read-back after update failed", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(api, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete makes NO client calls: truecommand.config governs whether this
// system is connected to a TrueCommand instance, so removing this resource
// from Terraform state must never disable (or otherwise change) an active
// connection or clear a stored api_key. It only emits a warning diagnostic;
// Terraform itself drops the resource from state once Delete returns.
func (r *TrueCommandConfigResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.Append(deleteWarningDiagnostics()...)
}

func (r *TrueCommandConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Any import ID is accepted and normalized to the fixed singleton ID;
	// all other fields are left unset so the subsequent Read call populates
	// them directly from truecommand.config.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), trueCommandConfigResourceID)...)
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package tn_connect_config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &TnConnectConfigResource{}
var _ resource.ResourceWithImportState = &TnConnectConfigResource{}

// TnConnectConfigResource implements the truenas_tn_connect_config singleton
// resource. No version gate is needed: tn_connect.config/tn_connect.update
// are present on both probed releases (25.10 and 26.0) — see model.go's
// doc comments for the probe evidence and how the differing per-release
// field shapes are handled.
type TnConnectConfigResource struct{ client *client.Client }

// NewResource returns a new TnConnectConfigResource.
func NewResource() resource.Resource { return &TnConnectConfigResource{} }

func (r *TnConnectConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tn_connect_config"
}

func (r *TnConnectConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *TnConnectConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// fetchConfig calls tn_connect.config and unmarshals the response.
func (r *TnConnectConfigResource) fetchConfig(ctx context.Context) (*tnConnectConfigAPI, error) {
	raw, err := r.client.CallRead(ctx, "tn_connect.config")
	if err != nil {
		return nil, err
	}
	var api tnConnectConfigAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		return nil, err
	}
	return &api, nil
}

// applyUpdate calls tn_connect.update. Probed job:false on both releases,
// so a plain synchronous r.client.Call is sufficient (no CallJob polling
// needed). payload MUST come from TnConnectConfigModel.updatePayload — see
// its doc comment for the safety contract this depends on.
func (r *TnConnectConfigResource) applyUpdate(ctx context.Context, payload map[string]any) error {
	_, err := r.client.Call(ctx, "tn_connect.update", payload)
	return err
}

func (r *TnConnectConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan TnConnectConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build the payload from req.Config, NOT plan: see updatePayload's doc
	// comment for why sourcing from the plan would risk resending "enabled"
	// even when the user never configured it — the one thing this resource
	// must never do.
	var config TnConnectConfigModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// A practitioner who never sets "enabled" in HCL produces an empty
	// payload (see updatePayload's doc comment); skip the tn_connect.update
	// call entirely in that case rather than sending an unprobed {} update —
	// see needsUpdateCall's doc comment.
	payload := config.updatePayload()
	if needsUpdateCall(payload) {
		if err := r.applyUpdate(ctx, payload); err != nil {
			resp.Diagnostics.AddError("Create TrueNAS Connect configuration failed", err.Error())
			return
		}
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

func (r *TnConnectConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state TnConnectConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// This is a singleton: tn_connect.config always exists, so Read never
	// removes the resource from state — there is no "not found" case.
	api, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read TrueNAS Connect configuration failed", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *TnConnectConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan TnConnectConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build the payload from req.Config, NOT plan. "enabled" is
	// Optional+Computed with UseStateForUnknown, so on Update the *plan*
	// value is NOT null for a user who left it unset in HCL — the modifier
	// echoes the prior state's value into the plan. Sourcing from req.Plan
	// here would resend "enabled" on every Update even when the user never
	// configured it, which for this specific field would mean silently
	// re-asserting whatever enrollment state happens to already be in
	// Terraform state. req.Config stays null for anything the user did not
	// set in HCL regardless of plan modifiers, so it is the only correct
	// and safe source. See updatePayload's doc comment for the full
	// safety rationale.
	var config TnConnectConfigModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// A practitioner who never sets "enabled" in HCL produces an empty
	// payload (see updatePayload's doc comment); skip the tn_connect.update
	// call entirely in that case rather than sending an unprobed {} update —
	// see needsUpdateCall's doc comment.
	payload := config.updatePayload()
	if needsUpdateCall(payload) {
		if err := r.applyUpdate(ctx, payload); err != nil {
			resp.Diagnostics.AddError("Update TrueNAS Connect configuration failed", err.Error())
			return
		}
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

// Delete makes NO client calls: tn_connect.config governs whether this
// system is enrolled with the TrueNAS Connect cloud service, so removing
// this resource from Terraform state must never disable (or otherwise
// change) an active enrollment. It only emits a warning diagnostic;
// Terraform itself drops the resource from state once Delete returns.
func (r *TnConnectConfigResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.Append(deleteWarningDiagnostics()...)
}

func (r *TnConnectConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Any import ID is accepted and normalized to the fixed singleton ID;
	// all other fields are left unset so the subsequent Read call populates
	// them directly from tn_connect.config.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), tnConnectConfigResourceID)...)
}

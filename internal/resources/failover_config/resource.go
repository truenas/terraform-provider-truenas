// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package failover_config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &FailoverConfigResource{}
var _ resource.ResourceWithImportState = &FailoverConfigResource{}

// FailoverConfigResource implements the truenas_failover_config singleton
// resource. No version or license gate is needed: failover.config/
// failover.update are present and readable on both probed releases,
// including a box that isn't licensed for Enterprise HA — see model.go's
// doc comments for the probe evidence.
type FailoverConfigResource struct{ client *client.Client }

// NewResource returns a new FailoverConfigResource.
func NewResource() resource.Resource { return &FailoverConfigResource{} }

func (r *FailoverConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_failover_config"
}

func (r *FailoverConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *FailoverConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// fetchConfig calls failover.config and unmarshals the response.
func (r *FailoverConfigResource) fetchConfig(ctx context.Context) (*failoverConfigAPI, error) {
	raw, err := r.client.CallRead(ctx, "failover.config")
	if err != nil {
		return nil, err
	}
	var api failoverConfigAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		return nil, err
	}
	return &api, nil
}

// applyUpdate calls failover.update. Probed job:false on both releases, so
// a plain synchronous r.client.Call is sufficient (no CallJob polling
// needed). payload MUST come from FailoverConfigModel.updatePayload — see
// its doc comment for the safety contract this depends on.
func (r *FailoverConfigResource) applyUpdate(ctx context.Context, payload map[string]any) error {
	_, err := r.client.Call(ctx, "failover.update", payload)
	return err
}

func (r *FailoverConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan FailoverConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build the payload from req.Config, NOT plan: see updatePayload's doc
	// comment for why sourcing from the plan would risk resending
	// "disabled"/"master" even when the user never configured them — the
	// one thing this resource must never do.
	var config FailoverConfigModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.applyUpdate(ctx, config.updatePayload()); err != nil {
		resp.Diagnostics.AddError("Create failover configuration failed", err.Error())
		return
	}

	api, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read-back after create failed", err.Error())
		return
	}

	responseToModel(api, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *FailoverConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state FailoverConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// This is a singleton: failover.config always exists, so Read never
	// removes the resource from state — there is no "not found" case.
	api, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read failover configuration failed", err.Error())
		return
	}

	responseToModel(api, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *FailoverConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan FailoverConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build the payload from req.Config, NOT plan. Each field is
	// Optional+Computed with UseStateForUnknown, so on Update the *plan*
	// value is NOT null for a user who left it unset in HCL — the modifier
	// echoes the prior state's value into the plan. Sourcing from req.Plan
	// here would resend "disabled"/"master" on every Update even when the
	// user never configured them, which for these specific fields would
	// mean silently re-asserting whatever HA state happens to already be
	// in Terraform state. req.Config stays null for anything the user did
	// not set in HCL regardless of plan modifiers, so it is the only
	// correct and safe source. See updatePayload's doc comment for the
	// full safety rationale.
	var config FailoverConfigModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.applyUpdate(ctx, config.updatePayload()); err != nil {
		resp.Diagnostics.AddError("Update failover configuration failed", err.Error())
		return
	}

	api, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read-back after update failed", err.Error())
		return
	}

	responseToModel(api, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete makes NO client calls: failover.config governs whether this HA
// system is administratively disabled and which node is master, so
// removing this resource from Terraform state must never change either.
// It only emits a warning diagnostic; Terraform itself drops the resource
// from state once Delete returns.
func (r *FailoverConfigResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.Append(deleteWarningDiagnostics()...)
}

func (r *FailoverConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Any import ID is accepted and normalized to the fixed singleton ID;
	// all other fields are left unset so the subsequent Read call
	// populates them directly from failover.config.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), failoverConfigResourceID)...)
}

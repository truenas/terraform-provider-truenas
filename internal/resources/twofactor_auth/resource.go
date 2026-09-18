// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package twofactor_auth

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &TwoFactorAuthResource{}
var _ resource.ResourceWithImportState = &TwoFactorAuthResource{}

// TwoFactorAuthResource implements the truenas_twofactor_auth singleton
// resource.
type TwoFactorAuthResource struct{ client *client.Client }

// NewResource returns a new TwoFactorAuthResource.
func NewResource() resource.Resource { return &TwoFactorAuthResource{} }

func (r *TwoFactorAuthResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_twofactor_auth"
}

func (r *TwoFactorAuthResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *TwoFactorAuthResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// fetchConfig calls auth.twofactor.config and unmarshals the response.
func (r *TwoFactorAuthResource) fetchConfig(ctx context.Context) (*twoFactorAuthAPI, error) {
	raw, err := r.client.CallRead(ctx, "auth.twofactor.config")
	if err != nil {
		return nil, err
	}
	var api twoFactorAuthAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		return nil, err
	}
	return &api, nil
}

func (r *TwoFactorAuthResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan TwoFactorAuthModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build the payload from req.Config, NOT plan: see updatePayload's doc
	// comment for why sourcing from the plan would resend prior-state
	// echoes for fields the user never configured.
	var config TwoFactorAuthModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := config.updatePayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.Call(ctx, "auth.twofactor.update", payload); err != nil {
		resp.Diagnostics.AddError("Create two-factor authentication configuration failed", err.Error())
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

func (r *TwoFactorAuthResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state TwoFactorAuthModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// This is a singleton: auth.twofactor.config always exists, so Read
	// never removes the resource from state (there is no "not found" case).
	api, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read two-factor authentication configuration failed", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *TwoFactorAuthResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan TwoFactorAuthModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build the payload from req.Config, NOT plan. "enabled", "window", and
	// "services" are Optional+Computed with UseStateForUnknown plan
	// modifiers, so on Update the *plan* value for any of them left unset
	// in the user's config is NOT null — the modifier silently repopulates
	// it with the prior state's value. Sourcing from req.Plan here would
	// therefore resend a field's last-known value on every Update even
	// when the user never configured it, which both contradicts this
	// resource's "unconfigured fields are never sent" contract and creates
	// a revert race if that field changed on the box between the last read
	// and this update. req.Config stays null for anything the user did not
	// set in HCL regardless of plan modifiers, so it is the only correct
	// source for payload inclusion. See updatePayload's doc comment.
	var config TwoFactorAuthModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := config.updatePayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.Call(ctx, "auth.twofactor.update", payload); err != nil {
		resp.Diagnostics.AddError("Update two-factor authentication configuration failed", err.Error())
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

// Delete makes NO client calls: two-factor authentication is
// security-critical, system-wide configuration, so removing this resource
// from Terraform state must never disable 2FA (or otherwise change it) on
// the box. It only emits a warning diagnostic; Terraform itself drops the
// resource from state once Delete returns.
func (r *TwoFactorAuthResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.Append(deleteWarningDiagnostics()...)
}

func (r *TwoFactorAuthResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Any import ID is accepted and normalized to the fixed singleton ID; all
	// other fields are left unset so the subsequent Read call populates them
	// directly from auth.twofactor.config.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), twoFactorAuthResourceID)...)
}

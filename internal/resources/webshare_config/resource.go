// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package webshare_config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &WebshareConfigResource{}
var _ resource.ResourceWithImportState = &WebshareConfigResource{}

// WebshareConfigResource implements the truenas_webshare_config singleton
// resource.
type WebshareConfigResource struct{ client *client.Client }

// NewResource returns a new WebshareConfigResource.
func NewResource() resource.Resource { return &WebshareConfigResource{} }

func (r *WebshareConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webshare_config"
}

func (r *WebshareConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *WebshareConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
// diagnostic if it is below the TrueNAS 26.0 floor webshare.config/
// webshare.update require, before any webshare.* call is made. Every
// resource entry point (Create/Read/Update) and the datasource's Read call
// this first.
func (r *WebshareConfigResource) checkVersion(ctx context.Context) diag.Diagnostics {
	var diags diag.Diagnostics
	version, err := r.client.ServerVersion(ctx)
	if err != nil {
		diags.AddError("Version probe failed", err.Error())
		return diags
	}
	diags.Append(versionGateDiagnostics(version)...)
	return diags
}

// fetchConfig calls webshare.config and unmarshals the response.
func (r *WebshareConfigResource) fetchConfig(ctx context.Context) (*webshareConfigAPI, error) {
	raw, err := r.client.CallRead(ctx, "webshare.config")
	if err != nil {
		return nil, err
	}
	var api webshareConfigAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		return nil, err
	}
	return &api, nil
}

// applyUpdate calls webshare.update. Probed job:false on TrueNAS 26.0, so a
// plain synchronous r.client.Call is sufficient (no CallJob polling needed).
func (r *WebshareConfigResource) applyUpdate(ctx context.Context, payload map[string]any) error {
	_, err := r.client.Call(ctx, "webshare.update", payload)
	return err
}

func (r *WebshareConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	resp.Diagnostics.Append(r.checkVersion(ctx)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan WebshareConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build the payload from req.Config, NOT plan: see updatePayload's doc
	// comment for why sourcing from the plan would resend prior-state
	// echoes for fields the user never configured.
	var config WebshareConfigModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := config.updatePayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.applyUpdate(ctx, payload); err != nil {
		resp.Diagnostics.AddError("Create Webshare configuration failed", err.Error())
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

func (r *WebshareConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	resp.Diagnostics.Append(r.checkVersion(ctx)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state WebshareConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// This is a singleton: webshare.config always exists (on TrueNAS 26.0+,
	// see checkVersion above), so Read never removes the resource from
	// state — there is no "not found" case.
	api, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read Webshare configuration failed", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *WebshareConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.Append(r.checkVersion(ctx)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan WebshareConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build the payload from req.Config, NOT plan. "bindip", "search",
	// "passkey", and "groups" are Optional+Computed with UseStateForUnknown
	// plan modifiers, so on Update the *plan* value for any of them left
	// unset in the user's config is NOT null — the modifier silently
	// repopulates it with the prior state's value. Sourcing from req.Plan
	// here would therefore resend a field's last-known value on every
	// Update even when the user never configured it, both contradicting
	// this resource's "unconfigured fields are never sent" contract and
	// creating a revert race if that field changed on the box between the
	// last read and this update. req.Config stays null for anything the
	// user did not set in HCL regardless of plan modifiers, so it is the
	// only correct source for payload inclusion. See updatePayload's doc
	// comment.
	var config WebshareConfigModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := config.updatePayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.applyUpdate(ctx, payload); err != nil {
		resp.Diagnostics.AddError("Update Webshare configuration failed", err.Error())
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

// Delete makes NO client calls: webshare.config governs the box's Webshare
// authentication mode, allowed groups, and bind addresses, so removing this
// resource from Terraform state must never reset that live configuration.
// It only emits a warning diagnostic; Terraform itself drops the resource
// from state once Delete returns. It does not version-gate: there is
// nothing to fail against a pre-26.0 server since it never calls the API.
func (r *WebshareConfigResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.Append(deleteWarningDiagnostics()...)
}

func (r *WebshareConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Any import ID is accepted and normalized to the fixed singleton ID; all
	// other fields are left unset so the subsequent Read call (which
	// version-gates itself, see Read above) populates them directly from
	// webshare.config.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), webshareConfigResourceID)...)
}

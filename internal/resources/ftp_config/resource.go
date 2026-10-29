// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package ftp_config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
	"github.com/truenas/terraform-provider-truenas/internal/listing"
)

var _ resource.Resource = &FTPConfigResource{}
var _ resource.ResourceWithImportState = &FTPConfigResource{}
var _ resource.ResourceWithIdentity = &FTPConfigResource{}
var _ resource.ResourceWithValidateConfig = &FTPConfigResource{}
var _ resource.ResourceWithModifyPlan = &FTPConfigResource{}

// FTPConfigResource implements the truenas_ftp_config singleton resource.
type FTPConfigResource struct{ client *client.Client }

// NewResource returns a new FTPConfigResource.
func NewResource() resource.Resource { return &FTPConfigResource{} }

func (r *FTPConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ftp_config"
}

func (r *FTPConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *FTPConfigResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = listing.StringIDIdentitySchema()
}

func (r *FTPConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// fetchConfig calls ftp.config and unmarshals the response.
func (r *FTPConfigResource) fetchConfig(ctx context.Context) (*ftpConfigAPI, error) {
	raw, err := r.client.CallRead(ctx, "ftp.config")
	if err != nil {
		return nil, err
	}
	var api ftpConfigAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		return nil, err
	}
	return &api, nil
}

// ValidateConfig rejects a non-empty anonpath unless onlyanonymous is set
// to true in the same configuration. ftp.update stores anonpath only for
// anonymous-only FTP and silently replaces it with null otherwise, so the
// apply failed with "inconsistent result after apply" (B-072).
func (r *FTPConfigResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var cfg FTPConfigModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() || cfg.AnonPath.IsNull() || cfg.AnonPath.IsUnknown() ||
		cfg.AnonPath.ValueString() == "" || cfg.OnlyAnonymous.IsUnknown() {
		return
	}
	if cfg.OnlyAnonymous.IsNull() || !cfg.OnlyAnonymous.ValueBool() {
		resp.Diagnostics.AddAttributeError(path.Root("anonpath"),
			"anonpath requires onlyanonymous = true",
			"TrueNAS keeps anonpath only when anonymous-only FTP is enabled and discards it otherwise. "+
				"Set onlyanonymous = true in the same configuration, or remove anonpath.")
	}
}

// ModifyPlan plans anonpath as "" when onlyanonymous is planned false and the
// configuration does not set anonpath: the server clears it then, so the
// value UseStateForUnknown carried from state would not survive the apply.
func (r *FTPConfigResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	var plan, cfg FTPConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() || !cfg.AnonPath.IsNull() || plan.OnlyAnonymous.IsUnknown() ||
		plan.OnlyAnonymous.ValueBool() || plan.AnonPath.IsUnknown() || plan.AnonPath.ValueString() == "" {
		return
	}
	resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("anonpath"), types.StringValue(""))...)
}

func (r *FTPConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan FTPConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := plan.updatePayload()

	if _, err := r.client.Call(ctx, "ftp.update", payload); err != nil {
		resp.Diagnostics.AddError("Create FTP configuration failed", err.Error())
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
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, ftpConfigResourceID)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *FTPConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state FTPConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// This is a singleton: ftp.config always exists, so Read never removes
	// the resource from state (there is no "not found" case).
	api, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read FTP configuration failed", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, ftpConfigResourceID)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *FTPConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan FTPConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := plan.updatePayload()

	if _, err := r.client.Call(ctx, "ftp.update", payload); err != nil {
		resp.Diagnostics.AddError("Update FTP configuration failed", err.Error())
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
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, ftpConfigResourceID)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete makes NO client calls: removing this resource from Terraform state
// must never rewrite the box's FTP configuration. It only emits a warning
// diagnostic; Terraform itself drops the resource from state once Delete
// returns.
func (r *FTPConfigResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.Append(deleteWarningDiagnostics()...)
}

func (r *FTPConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Any import ID is accepted and normalized to the fixed singleton ID; all
	// other fields are left unset so the subsequent Read call populates them
	// directly from ftp.config.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), ftpConfigResourceID)...)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, ftpConfigResourceID)...)
}

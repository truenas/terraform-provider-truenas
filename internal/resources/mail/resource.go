// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package mail

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &MailResource{}
var _ resource.ResourceWithImportState = &MailResource{}

// MailResource implements the truenas_mail singleton resource.
type MailResource struct{ client *client.Client }

// NewResource returns a new MailResource.
func NewResource() resource.Resource { return &MailResource{} }

func (r *MailResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mail"
}

func (r *MailResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *MailResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// fetchConfig calls mail.config and unmarshals the response.
func (r *MailResource) fetchConfig(ctx context.Context) (*mailAPI, error) {
	raw, err := r.client.CallRead(ctx, "mail.config")
	if err != nil {
		return nil, err
	}
	var api mailAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		return nil, err
	}
	return &api, nil
}

func (r *MailResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan MailModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// write-only: the framework nulls WriteOnly attributes in req.Plan, so
	// the actual pass value is only available via req.Config.
	var cfg MailModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.Pass = cfg.Pass

	// mail.update requires several fields (at minimum fromemail) on every
	// call, so the live config is fetched first and merged with the plan's
	// known values: this lets a config that sets only one field (e.g.
	// fromname) still send a complete, valid payload.
	live, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read current mail configuration failed", err.Error())
		return
	}

	if _, err := r.client.Call(ctx, "mail.update", mergedPayload(live, &plan)); err != nil {
		resp.Diagnostics.AddError("Create mail configuration failed", err.Error())
		return
	}

	api, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read-back after create failed", err.Error())
		return
	}

	// responseToModel never touches plan.Pass (write-only), so the plan's
	// pass value — whatever the user set, or null if they didn't — is
	// preserved in state as-is.
	resp.Diagnostics.Append(responseToModel(api, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *MailResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state MailModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// This is a singleton: mail.config always exists, so Read never removes
	// the resource from state (there is no "not found" case).
	api, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read mail configuration failed", err.Error())
		return
	}

	// responseToModel never touches state.Pass (write-only), so the
	// existing state's pass value is preserved unchanged.
	resp.Diagnostics.Append(responseToModel(api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *MailResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan MailModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// write-only: value lives in config, not plan.
	var cfg MailModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.Pass = cfg.Pass

	// See Create: mail.update requires several fields on every call, so the
	// live config is fetched first and merged with the plan's known values.
	live, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read current mail configuration failed", err.Error())
		return
	}

	if _, err := r.client.Call(ctx, "mail.update", mergedPayload(live, &plan)); err != nil {
		resp.Diagnostics.AddError("Update mail configuration failed", err.Error())
		return
	}

	api, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read-back after update failed", err.Error())
		return
	}

	// responseToModel never touches plan.Pass (write-only), so the plan's
	// pass value is preserved in state as-is.
	resp.Diagnostics.Append(responseToModel(api, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete makes NO client calls: mail settings are system-critical, so
// removing this resource from Terraform state must never blank out the
// box's mail configuration. It only emits a warning diagnostic; Terraform
// itself drops the resource from state once Delete returns.
func (r *MailResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.Append(deleteWarningDiagnostics()...)
}

func (r *MailResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Any import ID is accepted and normalized to the fixed singleton ID;
	// all other fields are left unset so the subsequent Read call populates
	// them directly from mail.config.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), mailResourceID)...)
}

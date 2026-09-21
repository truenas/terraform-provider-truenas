// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package alert_policy

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &AlertPolicyResource{}
var _ resource.ResourceWithImportState = &AlertPolicyResource{}

// AlertPolicyResource implements the truenas_alert_policy singleton resource.
type AlertPolicyResource struct{ client *client.Client }

// NewResource returns a new AlertPolicyResource.
func NewResource() resource.Resource { return &AlertPolicyResource{} }

func (r *AlertPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_alert_policy"
}

func (r *AlertPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *AlertPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// fetchConfig calls alertclasses.config and unmarshals the response.
func (r *AlertPolicyResource) fetchConfig(ctx context.Context) (*alertClassesAPI, error) {
	raw, err := r.client.CallRead(ctx, "alertclasses.config")
	if err != nil {
		return nil, err
	}
	var api alertClassesAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		return nil, err
	}
	return &api, nil
}

func (r *AlertPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan AlertPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.updatePayload()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.Call(ctx, "alertclasses.update", payload); err != nil {
		resp.Diagnostics.AddError("Create alert policy failed", err.Error())
		return
	}

	api, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read-back after create failed", err.Error())
		return
	}

	// Keep the plan's classes JSON string in state when it deep-equals the
	// API response (write-what-you-said); otherwise adopt the API's JSON.
	resp.Diagnostics.Append(applyAPIToModel(api, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AlertPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state AlertPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// This is a singleton: alertclasses.config always exists, so Read never
	// removes the resource from state (there is no "not found" case).
	api, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read alert policy failed", err.Error())
		return
	}

	resp.Diagnostics.Append(applyAPIToModel(api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *AlertPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan AlertPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.updatePayload()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.Call(ctx, "alertclasses.update", payload); err != nil {
		resp.Diagnostics.AddError("Update alert policy failed", err.Error())
		return
	}

	api, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read-back after update failed", err.Error())
		return
	}

	resp.Diagnostics.Append(applyAPIToModel(api, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AlertPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Singleton: there is nothing to delete on TrueNAS. Instead, reset the
	// classes object back to "{}" (all classes revert to their defaults),
	// then Terraform drops the resource from state.
	if _, err := r.client.Call(ctx, "alertclasses.update", deletePayload()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Failed to reset alert policy", err.Error())
	}
}

func (r *AlertPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Any import ID is accepted and normalized to the fixed singleton ID;
	// Classes is left unset so the subsequent Read call populates it
	// directly from alertclasses.config.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), alertPolicyResourceID)...)
}

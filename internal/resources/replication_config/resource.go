// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package replication_config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &ReplicationConfigResource{}
var _ resource.ResourceWithImportState = &ReplicationConfigResource{}

// ReplicationConfigResource implements the truenas_replication_config
// singleton resource.
type ReplicationConfigResource struct{ client *client.Client }

// NewResource returns a new ReplicationConfigResource.
func NewResource() resource.Resource { return &ReplicationConfigResource{} }

func (r *ReplicationConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_replication_config"
}

func (r *ReplicationConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *ReplicationConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// fetchConfig calls replication.config.config and unmarshals the response.
func (r *ReplicationConfigResource) fetchConfig(ctx context.Context) (*replicationConfigAPI, error) {
	raw, err := r.client.CallRead(ctx, "replication.config.config")
	if err != nil {
		return nil, err
	}
	var api replicationConfigAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		return nil, err
	}
	return &api, nil
}

func (r *ReplicationConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ReplicationConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.updatePayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.Call(ctx, "replication.config.update", payload); err != nil {
		resp.Diagnostics.AddError("Create replication configuration failed", err.Error())
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

func (r *ReplicationConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ReplicationConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// This is a singleton: replication.config.config always exists, so Read
	// never removes the resource from state (there is no "not found" case).
	api, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read replication configuration failed", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ReplicationConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ReplicationConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.updatePayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.Call(ctx, "replication.config.update", payload); err != nil {
		resp.Diagnostics.AddError("Update replication configuration failed", err.Error())
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

// Delete makes NO client calls: the replication configuration controls
// system-wide replication task concurrency, so removing this resource from
// Terraform state must never rewrite the box's configuration. It only emits
// a warning diagnostic; Terraform itself drops the resource from state once
// Delete returns.
func (r *ReplicationConfigResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.Append(deleteWarningDiagnostics()...)
}

func (r *ReplicationConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Any import ID is accepted and normalized to the fixed singleton ID; all
	// other fields are left unset so the subsequent Read call populates them
	// directly from replication.config.config.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), replicationConfigResourceID)...)
}

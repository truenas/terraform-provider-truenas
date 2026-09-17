// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package system_dataset

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &SystemDatasetResource{}
var _ resource.ResourceWithImportState = &SystemDatasetResource{}

// SystemDatasetResource implements the truenas_system_dataset singleton
// resource.
type SystemDatasetResource struct{ client *client.Client }

// NewResource returns a new SystemDatasetResource.
func NewResource() resource.Resource { return &SystemDatasetResource{} }

func (r *SystemDatasetResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_system_dataset"
}

func (r *SystemDatasetResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *SystemDatasetResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// fetchConfig calls systemdataset.config and unmarshals the response.
func (r *SystemDatasetResource) fetchConfig(ctx context.Context) (*systemDatasetAPI, error) {
	raw, err := r.client.CallRead(ctx, "systemdataset.config")
	if err != nil {
		return nil, err
	}
	var api systemDatasetAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		return nil, err
	}
	return &api, nil
}

// applyUpdate calls systemdataset.update via CallJob: moving the system
// dataset between pools is a long-running operation, so this is the only
// method in this provider (among the singletons in this plan) that must go
// through the job-polling path rather than a plain synchronous Call.
func (r *SystemDatasetResource) applyUpdate(ctx context.Context, payload map[string]any) error {
	_, err := r.client.CallJob(ctx, "systemdataset.update", payload)
	return err
}

func (r *SystemDatasetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan SystemDatasetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.updatePayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.applyUpdate(ctx, payload); err != nil {
		resp.Diagnostics.AddError("Create system dataset configuration failed", err.Error())
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

func (r *SystemDatasetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state SystemDatasetModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// This is a singleton: systemdataset.config always exists, so Read
	// never removes the resource from state (there is no "not found" case).
	api, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read system dataset configuration failed", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *SystemDatasetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan SystemDatasetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.updatePayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.applyUpdate(ctx, payload); err != nil {
		resp.Diagnostics.AddError("Update system dataset configuration failed", err.Error())
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

// Delete makes NO client calls: the system dataset holds core system state
// (logs, reporting, syslog, samba4 data), so removing this resource from
// Terraform state must never rewrite the box's configuration or trigger a
// pool migration. It only emits a warning diagnostic; Terraform itself drops
// the resource from state once Delete returns.
func (r *SystemDatasetResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.Append(deleteWarningDiagnostics()...)
}

func (r *SystemDatasetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Any import ID is accepted and normalized to the fixed singleton ID; all
	// other fields are left unset so the subsequent Read call populates them
	// directly from systemdataset.config.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), systemDatasetResourceID)...)
}

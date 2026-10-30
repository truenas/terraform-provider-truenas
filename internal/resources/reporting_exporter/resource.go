// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package reporting_exporter

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/truenas/terraform-provider-truenas/internal/planmod"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
	"github.com/truenas/terraform-provider-truenas/internal/listing"
)

var _ resource.Resource = &ReportingExporterResource{}
var _ resource.ResourceWithImportState = &ReportingExporterResource{}
var _ resource.ResourceWithIdentity = &ReportingExporterResource{}

// ReportingExporterResource implements the truenas_reporting_exporter resource.
type ReportingExporterResource struct{ client *client.Client }

// NewResource returns a new ReportingExporterResource.
func NewResource() resource.Resource { return &ReportingExporterResource{} }

func (r *ReportingExporterResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_reporting_exporter"
}

func (r *ReportingExporterResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *ReportingExporterResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = listing.IntIDIdentitySchema()
}

func (r *ReportingExporterResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// reporting.exporters.create/update/delete/query/get_instance are all
// non-job (sync) methods: probed via core.get_methods, every one of them
// reports "job": false.

func (r *ReportingExporterResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ReportingExporterModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.apiPayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.Call(ctx, "reporting.exporters.create", payload)
	if err != nil {
		resp.Diagnostics.AddError("Create reporting exporter failed", err.Error())
		return
	}

	var apiResp reportingExporterAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse create response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueInt64())...)
	// Remember which attributes the configuration owns (planmod.ResetWhenUnset*);
	// plan-time private state is discarded on create, so record it here.
	resp.Diagnostics.Append(planmod.RecordManaged(ctx, req.Config, resp.Private, "attributes.prefix", "attributes.update_every", "attributes.buffer_on_failures", "attributes.send_names_instead_of_ids", "attributes.matching_charts")...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ReportingExporterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ReportingExporterModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "reporting.exporters.get_instance", state.ID.ValueInt64())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read reporting exporter failed", err.Error())
		return
	}

	var apiResp reportingExporterAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, state.ID.ValueInt64())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ReportingExporterResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ReportingExporterModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state ReportingExporterModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.apiPayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.Call(ctx, "reporting.exporters.update", state.ID.ValueInt64(), payload)
	if err != nil {
		resp.Diagnostics.AddError("Update reporting exporter failed", err.Error())
		return
	}

	var apiResp reportingExporterAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueInt64())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ReportingExporterResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ReportingExporterModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Call(ctx, "reporting.exporters.delete", state.ID.ValueInt64())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete reporting exporter failed", err.Error())
	}
}

func (r *ReportingExporterResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, idDiags := listing.ImportInt64(ctx, req)
	resp.Diagnostics.Append(idDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "reporting.exporters.get_instance", id)
	if err != nil {
		resp.Diagnostics.AddError("Import reporting exporter failed", err.Error())
		return
	}

	var apiResp reportingExporterAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse import response", err.Error())
		return
	}

	var state ReportingExporterModel
	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, id)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package service

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

var _ resource.Resource = &ServiceResource{}
var _ resource.ResourceWithImportState = &ServiceResource{}
var _ resource.ResourceWithIdentity = &ServiceResource{}

// ServiceResource manages a single TrueNAS service.
type ServiceResource struct{ client *client.Client }

// NewResource returns a new ServiceResource.
func NewResource() resource.Resource { return &ServiceResource{} }

func (r *ServiceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service"
}

func (r *ServiceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *ServiceResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = listing.StringIDIdentitySchema()
}

func (r *ServiceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return
	}
	r.client = c
}

// lookupByName queries the service by service name string.
func (r *ServiceResource) lookupByName(ctx context.Context, name string) (*serviceAPI, error) {
	raw, err := r.client.CallRead(ctx, "service.query", [][]any{{"service", "=", name}})
	if err != nil {
		return nil, err
	}
	var results []serviceAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, &client.APIError{Code: 2, Message: "service not found: " + name}
	}
	return &results[0], nil
}

// responseToModel maps a serviceAPI struct into a ServiceModel.
func responseToModel(api *serviceAPI, m *ServiceModel) {
	m.ID = types.StringValue(api.Service)
	m.Name = types.StringValue(api.Service)
	m.Enabled = types.BoolValue(api.Enable)
	m.Running = types.BoolValue(api.State == "RUNNING")
}

// applyRunningState starts or stops the service to reach the desired state.
func (r *ServiceResource) applyRunningState(ctx context.Context, name string, desired bool) error {
	// service.control exists on every supported release; service.start/stop
	// take a required options argument on 26.0 and fail with a TypeError when
	// called with the name alone. silent=false makes a failed start/stop an
	// error instead of a quiet `false` result.
	verb := "STOP"
	if desired {
		verb = "START"
	}
	_, err := r.client.CallJob(ctx, "service.control", verb, name, map[string]any{"silent": false})
	return err
}

// applyEnabled sends service.update {"enable": ...} when the planned value
// is known and differs from the service's current setting. An unset enabled
// (unknown in the plan of a create) is left alone: sending ValueBool() of an
// unknown value disabled the service's autostart on every create that did
// not set enabled.
func (r *ServiceResource) applyEnabled(ctx context.Context, svc *serviceAPI, enabled types.Bool) error {
	if enabled.IsNull() || enabled.IsUnknown() || enabled.ValueBool() == svc.Enable {
		return nil
	}
	_, err := r.client.Call(ctx, "service.update", svc.ID, map[string]any{"enable": enabled.ValueBool()})
	return err
}

func (r *ServiceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ServiceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	svc, err := r.lookupByName(ctx, plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Service lookup failed", err.Error())
		return
	}

	if err := r.applyEnabled(ctx, svc, plan.Enabled); err != nil {
		resp.Diagnostics.AddError("Failed to update service", err.Error())
		return
	}

	if !plan.Running.IsNull() && !plan.Running.IsUnknown() {
		desired := plan.Running.ValueBool()
		current := svc.State == "RUNNING"
		if desired != current {
			if err := r.applyRunningState(ctx, plan.Name.ValueString(), desired); err != nil {
				resp.Diagnostics.AddError("Failed to change service state", err.Error())
				return
			}
		}
	}

	svc2, err := r.lookupByName(ctx, plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read-back failed", err.Error())
		return
	}
	responseToModel(svc2, &plan)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueString())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ServiceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ServiceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	svc, err := r.lookupByName(ctx, state.Name.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read service failed", err.Error())
		return
	}

	responseToModel(svc, &state)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, state.ID.ValueString())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ServiceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ServiceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	svc, err := r.lookupByName(ctx, plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Service lookup failed", err.Error())
		return
	}

	if err := r.applyEnabled(ctx, svc, plan.Enabled); err != nil {
		resp.Diagnostics.AddError("Failed to update service", err.Error())
		return
	}

	if !plan.Running.IsNull() && !plan.Running.IsUnknown() {
		desired := plan.Running.ValueBool()
		current := svc.State == "RUNNING"
		if desired != current {
			if err := r.applyRunningState(ctx, plan.Name.ValueString(), desired); err != nil {
				resp.Diagnostics.AddError("Failed to change service state", err.Error())
				return
			}
		}
	}

	svc2, err := r.lookupByName(ctx, plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read-back failed", err.Error())
		return
	}
	responseToModel(svc2, &plan)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueString())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete makes NO client calls: it only removes the service from Terraform
// state and leaves it enabled/running exactly as it is. TrueNAS services are
// never created or deleted, and stopping one as a side effect of removing it
// from the configuration (or of a name change, which replaces the resource)
// is destructive: removing truenas_service "ssh" used to disable and stop
// SSH, and ignored any error doing so (B-106).
func (r *ServiceResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.AddWarning(
		"Service left in place",
		"The service was removed from Terraform state only; its enabled and running state on TrueNAS are unchanged.",
	)
}

func (r *ServiceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, idDiags := listing.ImportString(ctx, req)
	resp.Diagnostics.Append(idDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), id)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, id)...)
}

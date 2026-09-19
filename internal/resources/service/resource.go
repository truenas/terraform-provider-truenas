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
)

var _ resource.Resource = &ServiceResource{}
var _ resource.ResourceWithImportState = &ServiceResource{}

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
	if desired {
		_, err := r.client.CallJob(ctx, "service.start", name)
		return err
	}
	_, err := r.client.CallJob(ctx, "service.stop", name)
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

	_, err = r.client.Call(ctx, "service.update", svc.ID, map[string]any{"enable": plan.Enabled.ValueBool()})
	if err != nil {
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

	_, err = r.client.Call(ctx, "service.update", svc.ID, map[string]any{"enable": plan.Enabled.ValueBool()})
	if err != nil {
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
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ServiceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ServiceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	svc, err := r.lookupByName(ctx, state.Name.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Failed to look up service", err.Error())
		return
	}
	// Best-effort: disable autostart and stop the service. Errors are ignored
	// because the Terraform resource is being removed from state regardless.
	r.client.Call(ctx, "service.update", svc.ID, map[string]any{"enable": false}) //nolint:errcheck
	if svc.State == "RUNNING" {
		r.client.CallJob(ctx, "service.stop", state.Name.ValueString()) //nolint:errcheck
	}
}

func (r *ServiceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}

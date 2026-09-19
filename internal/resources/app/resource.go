// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &AppResource{}
var _ resource.ResourceWithImportState = &AppResource{}

// AppResource manages a single TrueNAS app (Docker-based, TrueNAS 24.10+).
type AppResource struct{ client *client.Client }

// NewResource returns a new AppResource.
func NewResource() resource.Resource { return &AppResource{} }

func (r *AppResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app"
}

func (r *AppResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *AppResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// getInstance fetches a single app by name via app.get_instance (sync call).
func (r *AppResource) getInstance(ctx context.Context, name string) (*appAPI, error) {
	raw, err := r.client.CallRead(ctx, "app.get_instance", name)
	if err != nil {
		return nil, err
	}
	var api appAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		return nil, err
	}
	return &api, nil
}

func (r *AppResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan AppModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.createPayload()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.CallJob(ctx, "app.create", payload); err != nil {
		resp.Diagnostics.AddError("Failed to create app", err.Error())
		return
	}

	name := plan.Name.ValueString()
	api, err := r.getInstance(ctx, name)
	if err != nil {
		if client.IsNotFound(err) {
			resp.Diagnostics.AddError("App create read-back failed", fmt.Sprintf("app %q was not found after creation (create may have failed silently): %s", name, err.Error()))
			return
		}
		resp.Diagnostics.AddError("Read-back failed", err.Error())
		return
	}

	// If the plan explicitly requests the app be stopped, and it is
	// currently running (apps typically start automatically on create),
	// stop it and re-read.
	if !plan.Running.IsNull() && !plan.Running.IsUnknown() && !plan.Running.ValueBool() && api.State == "RUNNING" {
		if _, err := r.client.CallJob(ctx, "app.stop", name); err != nil {
			resp.Diagnostics.AddError("Failed to stop app", err.Error())
			return
		}
		api, err = r.getInstance(ctx, name)
		if err != nil {
			resp.Diagnostics.AddError("Read-back failed", err.Error())
			return
		}
	}

	// If the plan explicitly requests the app be running, and it is not
	// currently running, start it and re-read.
	if !plan.Running.IsNull() && !plan.Running.IsUnknown() && plan.Running.ValueBool() && api.State != "RUNNING" {
		if _, err := r.client.CallJob(ctx, "app.start", name); err != nil {
			resp.Diagnostics.AddError("Failed to start app", err.Error())
			return
		}
		api, err = r.getInstance(ctx, name)
		if err != nil {
			resp.Diagnostics.AddError("Read-back failed", err.Error())
			return
		}
	}

	// values/custom_compose_config_string/catalog_app are never echoed back
	// by the API (write-only). responseToModel does not touch them, so the
	// plan's values are preserved in state as-is.
	responseToModel(api, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AppResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state AppModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Use Name (not ID) for the lookup: after ImportState only "name" is
	// populated in state, so relying on ID here would break import.
	// ID and Name always carry the same value for this resource.
	api, err := r.getInstance(ctx, state.Name.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read app failed", err.Error())
		return
	}

	// Do NOT overwrite Values/ComposeYAML/CatalogApp from the API response:
	// the API never echoes these back, so we must preserve whatever is
	// already in state.
	responseToModel(api, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *AppResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan AppModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state AppModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := plan.Name.ValueString()

	// version changed -> app.upgrade (must run before app.update / read-back,
	// otherwise the read-back overwrites plan.Version with the old value and
	// Terraform aborts with "Provider produced inconsistent result after
	// apply").
	if needsUpgrade(&plan, &state) {
		if _, err := r.client.CallJob(ctx, "app.upgrade", name, map[string]any{
			"app_version": plan.Version.ValueString(),
		}); err != nil {
			resp.Diagnostics.AddError("Failed to upgrade app", err.Error())
			return
		}
	}

	// values or compose config changed -> app.update
	if !plan.Values.Equal(state.Values) || !plan.ComposeYAML.Equal(state.ComposeYAML) {
		updatePayload, diags := plan.updatePayload()
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		if len(updatePayload) > 0 {
			if _, err := r.client.CallJob(ctx, "app.update", name, updatePayload); err != nil {
				resp.Diagnostics.AddError("Failed to update app", err.Error())
				return
			}
		}
	}

	// running state changed -> app.start / app.stop
	if !plan.Running.IsNull() && !plan.Running.IsUnknown() && !plan.Running.Equal(state.Running) {
		if plan.Running.ValueBool() {
			if _, err := r.client.CallJob(ctx, "app.start", name); err != nil {
				resp.Diagnostics.AddError("Failed to start app", err.Error())
				return
			}
		} else {
			if _, err := r.client.CallJob(ctx, "app.stop", name); err != nil {
				resp.Diagnostics.AddError("Failed to stop app", err.Error())
				return
			}
		}
	}

	api, err := r.getInstance(ctx, name)
	if err != nil {
		resp.Diagnostics.AddError("Read-back failed", err.Error())
		return
	}

	responseToModel(api, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AppResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state AppModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.CallJob(ctx, "app.delete", state.ID.ValueString(), map[string]any{
		"remove_images":     true,
		"remove_ix_volumes": false,
	})
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Failed to delete app", err.Error())
		return
	}
}

func (r *AppResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}

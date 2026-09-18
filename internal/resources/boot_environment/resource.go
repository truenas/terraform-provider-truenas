// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package boot_environment

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &BootEnvironmentResource{}
var _ resource.ResourceWithImportState = &BootEnvironmentResource{}

// BootEnvironmentResource manages a TrueNAS boot environment via clone.
type BootEnvironmentResource struct{ client *client.Client }

// NewResource returns a new BootEnvironmentResource.
func NewResource() resource.Resource { return &BootEnvironmentResource{} }

func (r *BootEnvironmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_boot_environment"
}

func (r *BootEnvironmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *BootEnvironmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// clonePayload builds the args for boot.environment.clone: cloning "source"
// into a new boot environment named "target".
func clonePayload(source, target string) map[string]any {
	return map[string]any{"id": source, "target": target}
}

// keepPayload builds the args for boot.environment.keep.
func keepPayload(name string, value bool) map[string]any {
	return map[string]any{"id": name, "value": value}
}

// activatePayload builds the args for boot.environment.activate.
func activatePayload(name string) map[string]any {
	return map[string]any{"id": name}
}

// destroyPayload builds the args for boot.environment.destroy.
func destroyPayload(name string) map[string]any {
	return map[string]any{"id": name}
}

// deleteBlocked reports whether state describes a boot environment that must
// not be destroyed: one that is currently active (booted) or activated
// (will be booted next).
func deleteBlocked(state *BootEnvironmentModel) bool {
	return state.Active.ValueBool() || state.Activated.ValueBool()
}

// lookupByName queries a boot environment by name.
func (r *BootEnvironmentResource) lookupByName(ctx context.Context, name string) (*bootEnvAPI, error) {
	raw, err := r.client.CallRead(ctx, "boot.environment.query", [][]any{{"id", "=", name}})
	if err != nil {
		return nil, err
	}
	var results []bootEnvAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, &client.APIError{Code: 2, Message: "boot environment not found: " + name}
	}
	return &results[0], nil
}

func (r *BootEnvironmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan BootEnvironmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := plan.Name.ValueString()
	source := plan.Source.ValueString()

	if _, err := r.client.Call(ctx, "boot.environment.clone", clonePayload(source, name)); err != nil {
		resp.Diagnostics.AddError("Failed to clone boot environment", err.Error())
		return
	}

	if !plan.Keep.IsNull() && !plan.Keep.IsUnknown() {
		if _, err := r.client.Call(ctx, "boot.environment.keep", keepPayload(name, plan.Keep.ValueBool())); err != nil {
			resp.Diagnostics.AddError("Failed to set boot environment keep flag", err.Error())
			return
		}
	}

	if !plan.Activated.IsNull() && !plan.Activated.IsUnknown() && plan.Activated.ValueBool() {
		if _, err := r.client.Call(ctx, "boot.environment.activate", activatePayload(name)); err != nil {
			resp.Diagnostics.AddError("Failed to activate boot environment", err.Error())
			return
		}
	}

	api, err := r.lookupByName(ctx, name)
	if err != nil {
		resp.Diagnostics.AddError("Read-back failed", err.Error())
		return
	}
	responseToModel(api, &plan)
	// plan.Source already carries the configured value; responseToModel never
	// touches it, so it is preserved as-is.
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *BootEnvironmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state BootEnvironmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Use Name (not ID) for the lookup: after ImportState only "name" is
	// populated in state, so relying on ID here would break import. ID and
	// Name always carry the same value for this resource.
	api, err := r.lookupByName(ctx, state.Name.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read boot environment failed", err.Error())
		return
	}

	responseToModel(api, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *BootEnvironmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan BootEnvironmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var state BootEnvironmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// "name" is RequiresReplace, so plan.Name always equals state.Name here.
	name := plan.Name.ValueString()

	if !plan.Keep.IsNull() && !plan.Keep.IsUnknown() && !plan.Keep.Equal(state.Keep) {
		if _, err := r.client.Call(ctx, "boot.environment.keep", keepPayload(name, plan.Keep.ValueBool())); err != nil {
			resp.Diagnostics.AddError("Failed to set boot environment keep flag", err.Error())
			return
		}
	}

	if !plan.Activated.IsNull() && !plan.Activated.IsUnknown() {
		switch planActivationChange(state.Activated.ValueBool(), plan.Activated.ValueBool()) {
		case activationActivate:
			if _, err := r.client.Call(ctx, "boot.environment.activate", activatePayload(name)); err != nil {
				resp.Diagnostics.AddError("Failed to activate boot environment", err.Error())
				return
			}
		case activationUnsupportedDeactivate:
			resp.Diagnostics.AddError(
				"Cannot deactivate boot environment",
				fmt.Sprintf(
					"Boot environment %q cannot be deactivated directly; the TrueNAS API has no "+
						"deactivate operation. To deactivate it, activate a different boot environment "+
						"instead (its activation implicitly deactivates this one).",
					name,
				),
			)
			return
		case activationNoChange:
			// nothing to do
		}
	}

	api, err := r.lookupByName(ctx, name)
	if err != nil {
		resp.Diagnostics.AddError("Read-back failed", err.Error())
		return
	}
	responseToModel(api, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *BootEnvironmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state BootEnvironmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if deleteBlocked(&state) {
		resp.Diagnostics.AddError(
			"Cannot destroy active or activated boot environment",
			fmt.Sprintf(
				"Boot environment %q is currently active or activated and cannot be destroyed. "+
					"Activate a different boot environment first, then destroy this one.",
				state.ID.ValueString(),
			),
		)
		return
	}

	_, err := r.client.Call(ctx, "boot.environment.destroy", destroyPayload(state.ID.ValueString()))
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Failed to destroy boot environment", err.Error())
	}
}

func (r *BootEnvironmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("name"), req, resp)
}

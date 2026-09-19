// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package lxc_config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &LXCConfigResource{}
var _ resource.ResourceWithImportState = &LXCConfigResource{}

// LXCConfigResource implements the truenas_lxc_config singleton resource.
type LXCConfigResource struct{ client *client.Client }

// NewResource returns a new LXCConfigResource.
func NewResource() resource.Resource { return &LXCConfigResource{} }

func (r *LXCConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_lxc_config"
}

func (r *LXCConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *LXCConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// checkVersion probes the target server's release and returns a clean error
// diagnostic if it is below the TrueNAS 26.0 floor lxc.config/lxc.update
// require (see versionGateDiagnostics), before any lxc.* call is made. Every
// resource entry point (Create/Read/Update) and the datasource's Read call
// this first.
func (r *LXCConfigResource) checkVersion(ctx context.Context) diag.Diagnostics {
	var diags diag.Diagnostics
	version, err := r.client.ServerVersion(ctx)
	if err != nil {
		diags.AddError("Version probe failed", err.Error())
		return diags
	}
	diags.Append(versionGateDiagnostics(version)...)
	return diags
}

// fetchConfig calls lxc.config and unmarshals the response.
func (r *LXCConfigResource) fetchConfig(ctx context.Context) (*lxcConfigAPI, error) {
	raw, err := r.client.CallRead(ctx, "lxc.config")
	if err != nil {
		return nil, err
	}
	var api lxcConfigAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		return nil, err
	}
	return &api, nil
}

// applyUpdate calls lxc.update. Probed job:false on TrueNAS 26.0, so a plain
// synchronous r.client.Call is sufficient (no CallJob polling needed).
func (r *LXCConfigResource) applyUpdate(ctx context.Context, payload map[string]any) error {
	_, err := r.client.Call(ctx, "lxc.update", payload)
	return err
}

// buildUpdatePayload assembles the model updatePayload's CALLERS MUST
// contract requires: start from plan (req.Plan-sourced), then overwrite
// ONLY V4Network/V6Network with the corresponding req.Config values.
// PreferredPool/Bridge are deliberately left as-is from plan — see
// updatePayload's doc comment for why their three-way clear/leave-
// unchanged/set semantics depend on Plan sourcing and would break under
// Config sourcing.
func buildUpdatePayload(plan, config *LXCConfigModel) map[string]any {
	merged := *plan
	merged.V4Network = config.V4Network
	merged.V6Network = config.V6Network
	return merged.updatePayload()
}

func (r *LXCConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	resp.Diagnostics.Append(r.checkVersion(ctx)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan LXCConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var config LXCConfigModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.applyUpdate(ctx, buildUpdatePayload(&plan, &config)); err != nil {
		resp.Diagnostics.AddError("Create LXC configuration failed", err.Error())
		return
	}

	api, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read-back after create failed", err.Error())
		return
	}

	responseToModel(api, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *LXCConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	resp.Diagnostics.Append(r.checkVersion(ctx)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state LXCConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// This is a singleton: lxc.config always exists (on TrueNAS 26.0+, see
	// checkVersion above), so Read never removes the resource from state —
	// there is no "not found" case.
	api, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read LXC configuration failed", err.Error())
		return
	}

	responseToModel(api, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *LXCConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.Append(r.checkVersion(ctx)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan LXCConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var config LXCConfigModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.applyUpdate(ctx, buildUpdatePayload(&plan, &config)); err != nil {
		resp.Diagnostics.AddError("Update LXC configuration failed", err.Error())
		return
	}

	api, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read-back after update failed", err.Error())
		return
	}

	responseToModel(api, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete makes NO client calls: the LXC pool/bridge/network configuration
// underpins any running LXC-backed instances, so removing this resource
// from Terraform state must never reset or reconfigure the box's LXC setup.
// It only emits a warning diagnostic; Terraform itself drops the resource
// from state once Delete returns. It does not version-gate: there is
// nothing to fail against a pre-26.0 server since it never calls the API.
func (r *LXCConfigResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.Append(deleteWarningDiagnostics()...)
}

func (r *LXCConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Any import ID is accepted and normalized to the fixed singleton ID; all
	// other fields are left unset so the subsequent Read call (which
	// version-gates itself, see Read above) populates them directly from
	// lxc.config.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), lxcConfigResourceID)...)
}

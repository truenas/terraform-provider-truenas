// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nfs_config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &NFSConfigResource{}
var _ resource.ResourceWithImportState = &NFSConfigResource{}
var _ resource.ResourceWithModifyPlan = &NFSConfigResource{}

// NFSConfigResource implements the truenas_nfs_config singleton resource.
type NFSConfigResource struct{ client *client.Client }

// NewResource returns a new NFSConfigResource.
func NewResource() resource.Resource { return &NFSConfigResource{} }

func (r *NFSConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_nfs_config"
}

func (r *NFSConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *NFSConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// fetchConfig calls nfs.config and unmarshals the response.
func (r *NFSConfigResource) fetchConfig(ctx context.Context) (*nfsConfigAPI, error) {
	raw, err := r.client.CallRead(ctx, "nfs.config")
	if err != nil {
		return nil, err
	}
	var api nfsConfigAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		return nil, err
	}
	return &api, nil
}

func (r *NFSConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan NFSConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.updatePayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.Call(ctx, "nfs.update", payload); err != nil {
		resp.Diagnostics.AddError("Create NFS configuration failed", err.Error())
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

func (r *NFSConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state NFSConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// This is a singleton: nfs.config always exists, so Read never removes
	// the resource from state (there is no "not found" case).
	api, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read NFS configuration failed", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *NFSConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan NFSConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.updatePayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.Call(ctx, "nfs.update", payload); err != nil {
		resp.Diagnostics.AddError("Update NFS configuration failed", err.Error())
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

// Delete makes NO client calls: the NFS service configuration is
// system-critical (existing NFS exports and clients may depend on it), so
// removing this resource from Terraform state must never rewrite the box's
// NFS configuration. It only emits a warning diagnostic; Terraform itself
// drops the resource from state once Delete returns.
func (r *NFSConfigResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.Append(deleteWarningDiagnostics()...)
}

// ModifyPlan marks the three server-mutable computed attributes
// (managed_nfsd, v4_krb_enabled, keytab_has_nfs_spn) unknown whenever this
// is an update (both the prior state and the new plan are non-null).
// TrueNAS may flip these as a side effect of unrelated field changes — e.g.
// changing v4_domain can flip managed_nfsd — and since they're
// Computed-only with UseStateForUnknown, the plan would otherwise carry the
// prior state's value forward as "known", causing Terraform to error with
// an inconsistent-result-after-apply if the server's actual post-update
// value differs. Marking them unknown here lets the post-update Read fill
// in whatever value the server actually produced.
//
// Create (state is null) and destroy (plan is null) are left untouched:
// there is no prior state to carry forward on create, and no plan to modify
// on destroy.
func (r *NFSConfigResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() || req.State.Raw.IsNull() {
		return
	}

	// Only mark the trio unknown when a writable field is actually changing.
	// Marking them unconditionally would make every plan non-empty
	// ("known after apply" on a no-op plan).
	var plan, state NFSConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	changed := !plan.Servers.Equal(state.Servers) ||
		!plan.AllowNonroot.Equal(state.AllowNonroot) ||
		!plan.Protocols.Equal(state.Protocols) ||
		!plan.V4Domain.Equal(state.V4Domain) ||
		!plan.V4Krb.Equal(state.V4Krb) ||
		!plan.BindIP.Equal(state.BindIP) ||
		!plan.MountdPort.Equal(state.MountdPort) ||
		!plan.RPCStatdPort.Equal(state.RPCStatdPort) ||
		!plan.RPCLockdPort.Equal(state.RPCLockdPort) ||
		!plan.MountdLog.Equal(state.MountdLog) ||
		!plan.StatdLockdLog.Equal(state.StatdLockdLog) ||
		!plan.UserdManageGids.Equal(state.UserdManageGids) ||
		!plan.RDMA.Equal(state.RDMA)
	if !changed {
		return
	}

	resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("managed_nfsd"), types.BoolUnknown())...)
	resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("v4_krb_enabled"), types.BoolUnknown())...)
	resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("keytab_has_nfs_spn"), types.BoolUnknown())...)
}

func (r *NFSConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Any import ID is accepted and normalized to the fixed singleton ID; all
	// other fields are left unset so the subsequent Read call populates them
	// directly from nfs.config.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), nfsConfigResourceID)...)
}

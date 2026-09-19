// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package audit_config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &AuditConfigResource{}
var _ resource.ResourceWithImportState = &AuditConfigResource{}

// AuditConfigResource implements the truenas_audit_config singleton
// resource.
type AuditConfigResource struct{ client *client.Client }

// NewResource returns a new AuditConfigResource.
func NewResource() resource.Resource { return &AuditConfigResource{} }

func (r *AuditConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_audit_config"
}

func (r *AuditConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *AuditConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// fetchConfig calls audit.config and unmarshals the response.
func (r *AuditConfigResource) fetchConfig(ctx context.Context) (*auditConfigAPI, error) {
	raw, err := r.client.CallRead(ctx, "audit.config")
	if err != nil {
		return nil, err
	}
	var api auditConfigAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		return nil, err
	}
	return &api, nil
}

func (r *AuditConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan AuditConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build the payload from req.Config, NOT plan: see updatePayload's doc
	// comment for why sourcing from the plan would resend prior-state
	// echoes for fields the user never configured.
	var config AuditConfigModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.Call(ctx, "audit.update", config.updatePayload()); err != nil {
		resp.Diagnostics.AddError("Create audit configuration failed", err.Error())
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

func (r *AuditConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state AuditConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// This is a singleton: audit.config always exists, so Read never removes
	// the resource from state (there is no "not found" case).
	api, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read audit configuration failed", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *AuditConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan AuditConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build the payload from req.Config, NOT plan. See updatePayload's doc
	// comment: all five writable fields are Optional+Computed with
	// UseStateForUnknown plan modifiers, so sourcing from req.Plan would
	// resend a field's last-known value on every Update even when the user
	// never configured it. req.Config stays null for anything the user did
	// not set in HCL regardless of plan modifiers, so it is the only
	// correct source for payload inclusion.
	var config AuditConfigModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.Call(ctx, "audit.update", config.updatePayload()); err != nil {
		resp.Diagnostics.AddError("Update audit configuration failed", err.Error())
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

// Delete makes NO client calls: audit retention/quota configuration is
// system-critical, so removing this resource from Terraform state must
// never reset the box's audit settings. It only emits a warning diagnostic;
// Terraform itself drops the resource from state once Delete returns.
func (r *AuditConfigResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.Append(deleteWarningDiagnostics()...)
}

func (r *AuditConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Any import ID is accepted and normalized to the fixed singleton ID; all
	// other fields are left unset so the subsequent Read call populates them
	// directly from audit.config.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), auditConfigResourceID)...)
}

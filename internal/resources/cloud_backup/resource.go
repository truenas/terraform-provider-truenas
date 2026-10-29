// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cloud_backup

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
	"github.com/truenas/terraform-provider-truenas/internal/listing"
)

var _ resource.Resource = &CloudBackupResource{}
var _ resource.ResourceWithImportState = &CloudBackupResource{}
var _ resource.ResourceWithIdentity = &CloudBackupResource{}
var _ resource.ResourceWithConfigValidators = &CloudBackupResource{}

// ConfigValidators enforces that exactly one of password / password_wo is set:
// one is required (restic needs a password) and setting both is ambiguous. (#36)
func (r *CloudBackupResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(
			path.MatchRoot("password"),
			path.MatchRoot("password_wo"),
		),
	}
}

// injectWriteOnlyPassword overrides the payload password with the write-only
// password_wo read from config, when set. Write-only values are absent from the
// model, so they must be read from req.Config.
func injectWriteOnlyPassword(payload map[string]any, cfg *CloudBackupModel) {
	if !cfg.PasswordWO.IsNull() && !cfg.PasswordWO.IsUnknown() && cfg.PasswordWO.ValueString() != "" {
		payload["password"] = cfg.PasswordWO.ValueString()
	}
}

// CloudBackupResource implements the truenas_cloud_backup resource.
type CloudBackupResource struct{ client *client.Client }

// NewResource returns a new CloudBackupResource.
func NewResource() resource.Resource { return &CloudBackupResource{} }

func (r *CloudBackupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_backup"
}

func (r *CloudBackupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *CloudBackupResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = listing.IntIDIdentitySchema()
}

func (r *CloudBackupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// cloud_backup.create/query/update/delete/get_instance are all job:false
// (synchronous), probed via core.get_methods; only cloud_backup.sync,
// cloud_backup.restore, and cloud_backup.delete_snapshot are job:true, and
// this resource never calls any of them.

func (r *CloudBackupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan CloudBackupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.apiPayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	var cfg CloudBackupModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	injectWriteOnlyPassword(payload, &cfg)

	raw, err := r.client.Call(ctx, "cloud_backup.create", payload)
	if err != nil {
		resp.Diagnostics.AddError("Create cloud backup task failed", err.Error())
		return
	}

	var created cloudBackupAPI
	if err := json.Unmarshal(raw, &created); err != nil {
		resp.Diagnostics.AddError("Parse create response", err.Error())
		return
	}

	raw, err = r.client.CallRead(ctx, "cloud_backup.get_instance", created.ID)
	if err != nil {
		resp.Diagnostics.AddError("Read-back after create failed", err.Error())
		return
	}

	var apiResp cloudBackupAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse get_instance response", err.Error())
		return
	}

	// Keep the plan's Attributes JSON string in state (write-what-you-said):
	// the API may normalize/add default keys.
	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueInt64())...)
	resp.Diagnostics.Append(setImported(ctx, resp.Private, false)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *CloudBackupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state CloudBackupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "cloud_backup.get_instance", state.ID.ValueInt64())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read cloud backup task failed", err.Error())
		return
	}

	var apiResp cloudBackupAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Drift-aware attributes handling: only overwrite state Attributes when
	// a user-set key's value differs in the API. API-added default keys are
	// not considered drift.
	var stateAttrs map[string]any
	if err := json.Unmarshal([]byte(state.Attributes.ValueString()), &stateAttrs); err != nil {
		resp.Diagnostics.AddError("Parse state attributes JSON", err.Error())
		return
	}
	if attributesDrifted(stateAttrs, apiResp.Attributes) {
		attrJSON, diags := apiAttributesJSON(&apiResp)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		state.Attributes = types.StringValue(attrJSON)
	}

	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, state.ID.ValueInt64())...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *CloudBackupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan CloudBackupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Carry the existing state ID into the plan.
	var state CloudBackupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID

	payload, diags := plan.updatePayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	var cfg CloudBackupModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// The write-only password is sent only when password_wo_version changed;
	// cloud_backup.update keeps the stored password when it is absent. (B-141)
	if !plan.PasswordWOVersion.Equal(state.PasswordWOVersion) {
		injectWriteOnlyPassword(payload, &cfg)
	}

	_, err := r.client.Call(ctx, "cloud_backup.update", plan.ID.ValueInt64(), payload)
	if err != nil {
		resp.Diagnostics.AddError("Update cloud backup task failed", err.Error())
		return
	}

	raw, err := r.client.CallRead(ctx, "cloud_backup.get_instance", plan.ID.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Read-back after update failed", err.Error())
		return
	}

	var apiResp cloudBackupAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse get_instance response", err.Error())
		return
	}

	// Keep the plan's Attributes JSON string in state (write-what-you-said).
	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, plan.ID.ValueInt64())...)
	resp.Diagnostics.Append(setImported(ctx, resp.Private, false)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *CloudBackupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state CloudBackupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Call(ctx, "cloud_backup.delete", state.ID.ValueInt64())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete cloud backup task failed", err.Error())
	}
}

func (r *CloudBackupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, idDiags := listing.ImportInt64(ctx, req)
	resp.Diagnostics.Append(idDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "cloud_backup.get_instance", id)
	if err != nil {
		resp.Diagnostics.AddError("Import cloud backup task failed", err.Error())
		return
	}

	var apiResp cloudBackupAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse import response", err.Error())
		return
	}

	var state CloudBackupModel
	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	attrJSON, diags := apiAttributesJSON(&apiResp)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Attributes = types.StringValue(attrJSON)

	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, id)...)
	resp.Diagnostics.Append(setImported(ctx, resp.Private, true)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acl_template

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &AclTemplateResource{}
var _ resource.ResourceWithImportState = &AclTemplateResource{}

// AclTemplateResource implements the truenas_acl_template resource.
type AclTemplateResource struct{ client *client.Client }

// NewResource returns a new AclTemplateResource.
func NewResource() resource.Resource { return &AclTemplateResource{} }

func (r *AclTemplateResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_acl_template"
}

func (r *AclTemplateResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *AclTemplateResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// filesystem.acltemplate.create/query/get_instance/update/delete are all
// job:false (synchronous), probed via core.get_methods and confirmed by a
// live create/query/get_instance/update/delete round trip.

func (r *AclTemplateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan AclTemplateModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.apiPayload()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.Call(ctx, "filesystem.acltemplate.create", payload)
	if err != nil {
		resp.Diagnostics.AddError("Create ACL template failed", err.Error())
		return
	}

	var apiResp aclTemplateAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse create response", err.Error())
		return
	}

	// Keep the plan's exact "acl" JSON text in state (write-what-you-said):
	// the server may normalize cosmetic details (e.g. an omitted "id"
	// becomes an explicit -1). See aclDrifted / responseToModel.
	resp.Diagnostics.Append(responseToModel(&apiResp, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AclTemplateResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state AclTemplateModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "filesystem.acltemplate.get_instance", state.ID.ValueInt64())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read ACL template failed", err.Error())
		return
	}

	var apiResp aclTemplateAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(&apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	drifted, diags := aclDrifted(state.ACL.ValueString(), apiResp.ACL)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if drifted {
		aclJSON, d := canonicalACLJSON(apiResp.ACL)
		resp.Diagnostics.Append(d...)
		if resp.Diagnostics.HasError() {
			return
		}
		state.ACL = types.StringValue(aclJSON)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *AclTemplateResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan AclTemplateModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state AclTemplateModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID

	payload, diags := plan.apiPayload()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.Call(ctx, "filesystem.acltemplate.update", plan.ID.ValueInt64(), payload)
	if err != nil {
		resp.Diagnostics.AddError("Update ACL template failed", err.Error())
		return
	}

	var apiResp aclTemplateAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse update response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(&apiResp, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AclTemplateResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state AclTemplateModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Call(ctx, "filesystem.acltemplate.delete", state.ID.ValueInt64())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete ACL template failed", err.Error())
	}
}

func (r *AclTemplateResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Import ID must be an integer", req.ID)
		return
	}

	raw, err := r.client.CallRead(ctx, "filesystem.acltemplate.get_instance", id)
	if err != nil {
		resp.Diagnostics.AddError("Import ACL template failed", err.Error())
		return
	}

	var apiResp aclTemplateAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse import response", err.Error())
		return
	}

	var state AclTemplateModel
	resp.Diagnostics.Append(responseToModel(&apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	aclJSON, diags := canonicalACLJSON(apiResp.ACL)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.ACL = types.StringValue(aclJSON)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

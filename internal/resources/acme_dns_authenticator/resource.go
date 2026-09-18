// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acme_dns_authenticator

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &AcmeDnsAuthenticatorResource{}
var _ resource.ResourceWithImportState = &AcmeDnsAuthenticatorResource{}

// AcmeDnsAuthenticatorResource implements the truenas_acme_dns_authenticator resource.
type AcmeDnsAuthenticatorResource struct{ client *client.Client }

// NewResource returns a new instance of AcmeDnsAuthenticatorResource.
func NewResource() resource.Resource { return &AcmeDnsAuthenticatorResource{} }

func (r *AcmeDnsAuthenticatorResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_acme_dns_authenticator"
}

func (r *AcmeDnsAuthenticatorResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *AcmeDnsAuthenticatorResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// acme.dns.authenticator.create/update/delete/get_instance/query are all
// job:false (probed live via core.get_methods, confirmed identical on
// TrueNAS 25.10/26.0) — plain synchronous calls, no CallJob needed.

func (r *AcmeDnsAuthenticatorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan AcmeDnsAuthenticatorModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.createPayload()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.Call(ctx, "acme.dns.authenticator.create", payload)
	if err != nil {
		resp.Diagnostics.AddError("Create ACME DNS authenticator failed", err.Error())
		return
	}

	var apiResp acmeDnsAuthenticatorAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse create response", err.Error())
		return
	}

	// Keep the plan's attributes JSON string in state (write-what-you-said):
	// the API may echo back a normalized/expanded attributes document.
	responseToModel(&apiResp, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AcmeDnsAuthenticatorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state AcmeDnsAuthenticatorModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "acme.dns.authenticator.get_instance", state.ID.ValueInt64())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read ACME DNS authenticator failed", err.Error())
		return
	}

	var apiResp acmeDnsAuthenticatorAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse response", err.Error())
		return
	}

	responseToModel(&apiResp, &state)

	// Drift-aware attributes handling (mirrors alert_service/resource.go):
	// only overwrite the state string when a user-set key's value differs
	// in the API response. API-added keys are not considered drift. If the
	// state string is empty/null (as on import), populate it directly from
	// the API JSON.
	if state.Attributes.IsNull() || state.Attributes.ValueString() == "" {
		attrsJSON, diags := apiAttributesJSON(&apiResp)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		state.Attributes = types.StringValue(attrsJSON)
	} else {
		var stateAttrs map[string]any
		if err := json.Unmarshal([]byte(state.Attributes.ValueString()), &stateAttrs); err != nil {
			resp.Diagnostics.AddError("Parse state attributes JSON", err.Error())
			return
		}
		if attributesDrifted(stateAttrs, apiResp.Attributes) {
			attrsJSON, diags := apiAttributesJSON(&apiResp)
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}
			state.Attributes = types.StringValue(attrsJSON)
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *AcmeDnsAuthenticatorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan AcmeDnsAuthenticatorModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state AcmeDnsAuthenticatorModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID

	payload, diags := plan.updatePayload()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.Call(ctx, "acme.dns.authenticator.update", plan.ID.ValueInt64(), payload)
	if err != nil {
		resp.Diagnostics.AddError("Update ACME DNS authenticator failed", err.Error())
		return
	}

	var apiResp acmeDnsAuthenticatorAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse response", err.Error())
		return
	}

	// Keep the plan's attributes JSON string in state (write-what-you-said).
	responseToModel(&apiResp, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AcmeDnsAuthenticatorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state AcmeDnsAuthenticatorModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Call(ctx, "acme.dns.authenticator.delete", state.ID.ValueInt64())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete ACME DNS authenticator failed", err.Error())
	}
}

func (r *AcmeDnsAuthenticatorResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Import ID must be an integer", req.ID)
		return
	}

	raw, err := r.client.CallRead(ctx, "acme.dns.authenticator.get_instance", id)
	if err != nil {
		resp.Diagnostics.AddError("Import ACME DNS authenticator failed", err.Error())
		return
	}

	var apiResp acmeDnsAuthenticatorAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse import response", err.Error())
		return
	}

	var state AcmeDnsAuthenticatorModel
	responseToModel(&apiResp, &state)
	attrsJSON, diags := apiAttributesJSON(&apiResp)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Attributes = types.StringValue(attrsJSON)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

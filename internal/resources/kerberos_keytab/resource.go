// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package kerberos_keytab

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &KerberosKeytabResource{}
var _ resource.ResourceWithImportState = &KerberosKeytabResource{}

// KerberosKeytabResource implements the truenas_kerberos_keytab resource.
type KerberosKeytabResource struct{ client *client.Client }

// NewResource returns a new instance of KerberosKeytabResource.
func NewResource() resource.Resource { return &KerberosKeytabResource{} }

func (r *KerberosKeytabResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kerberos_keytab"
}

func (r *KerberosKeytabResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *KerberosKeytabResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// kerberos.keytab.create/update/delete/get_instance/query are all non-job
// (sync) methods, and "file" round-trips intact on every read path — see
// model.go's kerberosKeytabAPI doc comment for the full probe writeup.

func (r *KerberosKeytabResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan KerberosKeytabModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.Call(ctx, "kerberos.keytab.create", plan.apiPayload())
	if err != nil {
		resp.Diagnostics.AddError("Create Kerberos keytab failed", err.Error())
		return
	}

	var apiResp kerberosKeytabAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse create response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(&apiResp, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *KerberosKeytabResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state KerberosKeytabModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "kerberos.keytab.get_instance", state.ID.ValueInt64())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read Kerberos keytab failed", err.Error())
		return
	}

	var apiResp kerberosKeytabAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(&apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *KerberosKeytabResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan KerberosKeytabModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state KerberosKeytabModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.Call(ctx, "kerberos.keytab.update", state.ID.ValueInt64(), plan.apiPayload())
	if err != nil {
		resp.Diagnostics.AddError("Update Kerberos keytab failed", err.Error())
		return
	}

	var apiResp kerberosKeytabAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(&apiResp, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *KerberosKeytabResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state KerberosKeytabModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Call(ctx, "kerberos.keytab.delete", state.ID.ValueInt64())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete Kerberos keytab failed", err.Error())
	}
}

func (r *KerberosKeytabResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Import ID must be an integer", req.ID)
		return
	}

	raw, err := r.client.CallRead(ctx, "kerberos.keytab.get_instance", id)
	if err != nil {
		resp.Diagnostics.AddError("Import Kerberos keytab failed", err.Error())
		return
	}

	var apiResp kerberosKeytabAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse import response", err.Error())
		return
	}

	var state KerberosKeytabModel
	resp.Diagnostics.Append(responseToModel(&apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

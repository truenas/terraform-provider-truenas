// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package kerberos_realm

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &KerberosRealmResource{}
var _ resource.ResourceWithImportState = &KerberosRealmResource{}

// KerberosRealmResource implements the truenas_kerberos_realm resource.
type KerberosRealmResource struct{ client *client.Client }

// NewResource returns a new instance of KerberosRealmResource.
func NewResource() resource.Resource { return &KerberosRealmResource{} }

func (r *KerberosRealmResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kerberos_realm"
}

func (r *KerberosRealmResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *KerberosRealmResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// kerberos.realm.create/update/delete/get_instance/query are all non-job
// (sync) methods: probed via `core.get_methods`, every one of them reports
// "job": false. kerberos.realm.create was also probed directly (a throwaway
// realm "TF-PROBE.LAN" with kdc ["192.0.2.88"], a reserved/unreachable
// TEST-NET-1 address) and returned in ~200ms — it does NOT contact the KDC
// to validate reachability, so acceptance tests may safely use synthetic,
// unreachable KDC addresses.

func (r *KerberosRealmResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan KerberosRealmModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.apiPayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.Call(ctx, "kerberos.realm.create", payload)
	if err != nil {
		resp.Diagnostics.AddError("Create Kerberos realm failed", err.Error())
		return
	}

	var apiResp kerberosRealmAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse create response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *KerberosRealmResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state KerberosRealmModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "kerberos.realm.get_instance", state.ID.ValueInt64())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read Kerberos realm failed", err.Error())
		return
	}

	var apiResp kerberosRealmAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *KerberosRealmResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan KerberosRealmModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state KerberosRealmModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.apiPayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.Call(ctx, "kerberos.realm.update", state.ID.ValueInt64(), payload)
	if err != nil {
		resp.Diagnostics.AddError("Update Kerberos realm failed", err.Error())
		return
	}

	var apiResp kerberosRealmAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *KerberosRealmResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state KerberosRealmModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Call(ctx, "kerberos.realm.delete", state.ID.ValueInt64())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete Kerberos realm failed", err.Error())
	}
}

func (r *KerberosRealmResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Import ID must be an integer", req.ID)
		return
	}

	raw, err := r.client.CallRead(ctx, "kerberos.realm.get_instance", id)
	if err != nil {
		resp.Diagnostics.AddError("Import Kerberos realm failed", err.Error())
		return
	}

	var apiResp kerberosRealmAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse import response", err.Error())
		return
	}

	var state KerberosRealmModel
	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

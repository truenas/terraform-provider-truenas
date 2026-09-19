// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package snmp_config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &SNMPConfigResource{}
var _ resource.ResourceWithImportState = &SNMPConfigResource{}

// SNMPConfigResource implements the truenas_snmp_config singleton resource.
type SNMPConfigResource struct{ client *client.Client }

// NewResource returns a new SNMPConfigResource.
func NewResource() resource.Resource { return &SNMPConfigResource{} }

func (r *SNMPConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_snmp_config"
}

func (r *SNMPConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *SNMPConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// fetchConfig calls snmp.config and unmarshals the response.
func (r *SNMPConfigResource) fetchConfig(ctx context.Context) (*snmpConfigAPI, error) {
	raw, err := r.client.CallRead(ctx, "snmp.config")
	if err != nil {
		return nil, err
	}
	var api snmpConfigAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		return nil, err
	}
	return &api, nil
}

func (r *SNMPConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan SNMPConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// write-only: the framework nulls WriteOnly attributes in req.Plan, so
	// the actual v3_password/v3_privpassphrase values are only available via
	// req.Config.
	var cfg SNMPConfigModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.V3Password = cfg.V3Password
	plan.V3PrivPassphrase = cfg.V3PrivPassphrase

	if _, err := r.client.Call(ctx, "snmp.update", plan.updatePayload()); err != nil {
		resp.Diagnostics.AddError("Create SNMP configuration failed", err.Error())
		return
	}

	api, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read-back after create failed", err.Error())
		return
	}

	// responseToModel never touches plan.V3Password/plan.V3PrivPassphrase
	// (write-only), so the plan's secret values — whatever the user set, or
	// null if they didn't — are preserved in state as-is.
	resp.Diagnostics.Append(responseToModel(api, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *SNMPConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state SNMPConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// This is a singleton: snmp.config always exists, so Read never removes
	// the resource from state (there is no "not found" case).
	api, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read SNMP configuration failed", err.Error())
		return
	}

	// responseToModel never touches state.V3Password/state.V3PrivPassphrase
	// (write-only), so the existing state's secret values are preserved
	// unchanged.
	resp.Diagnostics.Append(responseToModel(api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *SNMPConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan SNMPConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// write-only: value lives in config, not plan.
	var cfg SNMPConfigModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.V3Password = cfg.V3Password
	plan.V3PrivPassphrase = cfg.V3PrivPassphrase

	if _, err := r.client.Call(ctx, "snmp.update", plan.updatePayload()); err != nil {
		resp.Diagnostics.AddError("Update SNMP configuration failed", err.Error())
		return
	}

	api, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read-back after update failed", err.Error())
		return
	}

	// responseToModel never touches plan.V3Password/plan.V3PrivPassphrase
	// (write-only), so the plan's secret values are preserved in state as-is.
	resp.Diagnostics.Append(responseToModel(api, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete makes NO client calls: SNMP settings are system-critical monitoring
// configuration, so removing this resource from Terraform state must never
// reset the box's SNMP configuration. It only emits a warning diagnostic;
// Terraform itself drops the resource from state once Delete returns.
func (r *SNMPConfigResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.Append(deleteWarningDiagnostics()...)
}

func (r *SNMPConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Any import ID is accepted and normalized to the fixed singleton ID; all
	// other fields are left unset so the subsequent Read call populates them
	// directly from snmp.config.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), snmpConfigResourceID)...)
}

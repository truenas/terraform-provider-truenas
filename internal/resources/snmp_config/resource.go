// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package snmp_config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
	"github.com/truenas/terraform-provider-truenas/internal/listing"
)

var _ resource.Resource = &SNMPConfigResource{}
var _ resource.ResourceWithImportState = &SNMPConfigResource{}
var _ resource.ResourceWithIdentity = &SNMPConfigResource{}

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

func (r *SNMPConfigResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = listing.StringIDIdentitySchema()
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

	payload := plan.updatePayload()
	// Inject the write-only community string from config (absent from the model).
	if !cfg.CommunityWO.IsNull() && !cfg.CommunityWO.IsUnknown() && cfg.CommunityWO.ValueString() != "" {
		payload["community"] = cfg.CommunityWO.ValueString()
	}
	r.gatePost2800(ctx, payload, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := r.client.Call(ctx, "snmp.update", payload); err != nil {
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
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, snmpConfigResourceID)...)
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
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, snmpConfigResourceID)...)
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

	var state SNMPConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := plan.updatePayload()
	// Inject the write-only community string from config (absent from the
	// model), only when community_wo_version changed: snmp.update keeps the
	// stored community when it is absent. (B-141)
	if !plan.CommunityWOVersion.Equal(state.CommunityWOVersion) &&
		!cfg.CommunityWO.IsNull() && !cfg.CommunityWO.IsUnknown() && cfg.CommunityWO.ValueString() != "" {
		payload["community"] = cfg.CommunityWO.ValueString()
	}
	r.gatePost2800(ctx, payload, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := r.client.Call(ctx, "snmp.update", payload); err != nil {
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
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, snmpConfigResourceID)...)
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
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, snmpConfigResourceID)...)
}

// gatePost2800 removes fields TrueNAS 28.0 dropped from snmp.update and reports
// a clear error if the configuration set one. loglevel was removed in 28.0
// (the frozen v27_0_0 API model still declares it; v28_0_0 drops it and maps
// it to a fixed level for older clients); sending it fails with "Extra
// inputs are not permitted". The gate sat at 27.0 before, which wrongly
// rejected loglevel on 27.x (B-130).
func (r *SNMPConfigResource) gatePost2800(ctx context.Context, payload map[string]any, diags *diag.Diagnostics) {
	if _, ok := payload["loglevel"]; !ok {
		return
	}
	if ok, err := r.client.VersionAtLeast(ctx, loglevelRemovedMajor, 0); err == nil && ok {
		delete(payload, "loglevel")
		diags.AddAttributeError(
			path.Root("loglevel"),
			"loglevel is not supported on TrueNAS 28.0 or later",
			"snmp.update on this TrueNAS release does not accept the \"loglevel\" field (it was removed in TrueNAS 28.0). Remove the attribute or target a pre-28.0 server.",
		)
	}
}

// loglevelRemovedMajor is the first TrueNAS major release whose snmp.update
// no longer accepts loglevel.
const loglevelRemovedMajor = 28

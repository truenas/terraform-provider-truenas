// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package smb_config

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &SMBConfigResource{}
var _ resource.ResourceWithImportState = &SMBConfigResource{}

// SMBConfigResource implements the truenas_smb_config singleton resource.
type SMBConfigResource struct{ client *client.Client }

// NewResource returns a new SMBConfigResource.
func NewResource() resource.Resource { return &SMBConfigResource{} }

func (r *SMBConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_smb_config"
}

func (r *SMBConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *SMBConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// fetchConfig calls smb.config and unmarshals the response.
func (r *SMBConfigResource) fetchConfig(ctx context.Context) (*smbConfigAPI, error) {
	raw, err := r.client.CallRead(ctx, "smb.config")
	if err != nil {
		return nil, err
	}
	var api smbConfigAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		return nil, err
	}
	return &api, nil
}

// applyPost2600FieldsSupport folds "stateful_failover", "minimum_protocol",
// and "search_protocols" into payload when the user explicitly set them in
// HCL, then strips whichever were added (with a clear apply-time error) if
// the target server is below the TrueNAS 26.0 floor smb.update requires for
// all three.
//
// This gate fails CLOSED: it strips the fields unless the version probe
// both succeeds and reports >= 26.0. These three fields exist only on the
// 26.0+ minority of targets, so treating a probe error as "supported"
// (docker_config's fail-open approach) would send them to every pre-26.0
// box and always fail with smb.update's generic "Extra inputs are not
// permitted" — reintroducing the bug this gate exists to fix. Fail-closed
// instead costs a genuinely-26.0 box the field on the one apply that hit a
// transient probe error (recoverable on retry), which is the safer trade
// given pre-26.0 is the overwhelming majority of installed targets.
//
// cfg MUST be the practitioner's raw Config model, not the resolved Plan:
// all three fields are Optional+Computed with UseStateForUnknown, so once
// they've ever been read from a live smb.config they carry a known
// (non-null) value in every subsequent plan even when the user never wrote
// them in their .tf file — Plan alone cannot distinguish "user explicitly
// (re-)configured this" from "framework carried the previous known value
// forward." Config never does this carrying-forward: it is null unless the
// practitioner actually wrote the attribute, so gating on Config is what
// keeps this resource usable on a pre-26.0 target that never sets any of
// the three (the overwhelmingly common case) while still catching a
// practitioner who explicitly tries to set one. Mirrors
// system_advanced's applyNvidiaSupport.
//
// smb.update's accepts-schema genuinely does not include any of these three
// fields below TrueNAS 26.0 — probed live (go run against a temp probe binary
// calling core.get_methods) against a 25.10.3.1 VM (192.168.1.249) and a
// 25.10.4 HA pair member (10.220.16.188), neither of which lists any of the
// three in smb.update's accepts schema, versus a 26.0.0-BETA.2 box
// (192.168.1.68) which lists all three. See post2600FieldsSupported in
// model.go for the pure version comparison this wraps.
func (r *SMBConfigResource) applyPost2600FieldsSupport(ctx context.Context, payload map[string]any, cfg *SMBConfigModel, diags *diag.Diagnostics) {
	var gated []string

	if !cfg.StatefulFailover.IsNull() && !cfg.StatefulFailover.IsUnknown() {
		payload["stateful_failover"] = cfg.StatefulFailover.ValueBool()
		gated = append(gated, "stateful_failover")
	}
	if !cfg.MinimumProtocol.IsNull() && !cfg.MinimumProtocol.IsUnknown() {
		payload["minimum_protocol"] = cfg.MinimumProtocol.ValueString()
		gated = append(gated, "minimum_protocol")
	}
	if !cfg.SearchProtocols.IsNull() && !cfg.SearchProtocols.IsUnknown() {
		var v []string
		diags.Append(cfg.SearchProtocols.ElementsAs(ctx, &v, false)...)
		if v == nil {
			v = []string{}
		}
		payload["search_protocols"] = v
		gated = append(gated, "search_protocols")
	}
	if len(gated) == 0 {
		return
	}

	version, err := r.client.ServerVersion(ctx)
	if err == nil && post2600FieldsSupported(version) {
		return
	}
	for _, attr := range gated {
		delete(payload, attr)
		diags.AddAttributeError(
			path.Root(attr),
			attr+" requires TrueNAS 26.0 or later",
			"smb.update on this TrueNAS release does not accept the \""+attr+"\" field (it was added in TrueNAS 26.0). Remove the attribute from your configuration or target a TrueNAS 26.0+ server.",
		)
	}
}

func (r *SMBConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan SMBConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var cfg SMBConfigModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.updatePayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.applyPost2600FieldsSupport(ctx, payload, &cfg, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.Call(ctx, "smb.update", payload); err != nil {
		resp.Diagnostics.AddError("Create SMB configuration failed", err.Error())
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

func (r *SMBConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state SMBConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// This is a singleton: smb.config always exists, so Read never removes
	// the resource from state (there is no "not found" case).
	api, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read SMB configuration failed", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *SMBConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan SMBConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var cfg SMBConfigModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.updatePayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.applyPost2600FieldsSupport(ctx, payload, &cfg, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.Call(ctx, "smb.update", payload); err != nil {
		resp.Diagnostics.AddError("Update SMB configuration failed", err.Error())
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

// Delete makes NO client calls: the SMB service configuration is
// system-critical (shares and domain membership may depend on it), so
// removing this resource from Terraform state must never rewrite the box's
// SMB configuration. It only emits a warning diagnostic; Terraform itself
// drops the resource from state once Delete returns.
func (r *SMBConfigResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.Append(deleteWarningDiagnostics()...)
}

func (r *SMBConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Any import ID is accepted and normalized to the fixed singleton ID; all
	// other fields are left unset so the subsequent Read call populates them
	// directly from smb.config.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), smbConfigResourceID)...)
}

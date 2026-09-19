// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package system_advanced

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &SystemAdvancedResource{}
var _ resource.ResourceWithImportState = &SystemAdvancedResource{}

// SystemAdvancedResource implements the truenas_system_advanced singleton
// resource.
type SystemAdvancedResource struct{ client *client.Client }

// NewResource returns a new SystemAdvancedResource.
func NewResource() resource.Resource { return &SystemAdvancedResource{} }

func (r *SystemAdvancedResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_system_advanced"
}

func (r *SystemAdvancedResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *SystemAdvancedResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// fetchConfig calls system.advanced.config and unmarshals the response.
func (r *SystemAdvancedResource) fetchConfig(ctx context.Context) (*systemAdvancedAPI, error) {
	raw, err := r.client.CallRead(ctx, "system.advanced.config")
	if err != nil {
		return nil, err
	}
	var api systemAdvancedAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		return nil, err
	}
	return &api, nil
}

// applyNvidiaSupport folds "nvidia" into payload when the user explicitly
// set it in HCL, then strips it (with a clear apply-time error) if the
// target server is below the TrueNAS 26.0 floor system.advanced.update
// requires for this field.
//
// This gate fails CLOSED: it strips "nvidia" unless the version probe both
// succeeds and reports >= 26.0. "nvidia" exists only on the 26.0+ minority
// of targets, so treating a probe error as "supported" (docker_config's
// fail-open approach, correct there because it gates a field valid on the
// <26.0 majority release) would send "nvidia" to every pre-26.0 box and
// always fail with system.advanced.update's generic "Extra inputs are not
// permitted" — reintroducing the bug this gate exists to fix. Fail-closed
// instead costs a genuinely-26.0 box the field on the one apply that hit a
// transient probe error (recoverable on retry), which is the safer trade
// given pre-26.0 is the overwhelming majority of installed targets.
//
// configNvidia MUST come from the practitioner's raw Config, not the
// resolved Plan: "nvidia" is Optional+Computed with UseStateForUnknown, so
// once it's ever been read from a live system.advanced.config it carries a
// known (non-null) value in every subsequent plan even when the user never
// wrote "nvidia = ..." in their .tf file — Plan alone cannot distinguish
// "user explicitly (re-)configured this" from "framework carried the
// previous known value forward." Config never does this carrying-forward:
// it is null unless the practitioner actually wrote the attribute, so
// gating on Config is what keeps this resource usable on a pre-26.0 target
// that never sets "nvidia" at all (the overwhelmingly common case) while
// still catching a practitioner who explicitly tries to set it there. This
// mirrors docker_config's applyNvidiaSupport, gating in the opposite
// direction (a field that doesn't exist YET, rather than one that stopped
// existing).
//
// system.advanced.update's accepts-schema genuinely does not include
// "nvidia" below TrueNAS 26.0 — probed live (go run against a temp probe
// binary calling core.get_methods) against a 25.10.3.1 VM (192.168.1.249)
// and a 25.10.4 HA pair member (10.220.16.188), neither of which lists
// "nvidia" in system.advanced.update's accepts schema, versus a
// 26.0.0-BETA.2 box (192.168.1.68) which does. See nvidiaSupported in
// model.go for the pure version comparison this wraps.
func (r *SystemAdvancedResource) applyNvidiaSupport(ctx context.Context, payload map[string]any, configNvidia types.Bool, diags *diag.Diagnostics) {
	if configNvidia.IsNull() || configNvidia.IsUnknown() {
		return
	}
	payload["nvidia"] = configNvidia.ValueBool()

	version, err := r.client.ServerVersion(ctx)
	if err == nil && nvidiaSupported(version) {
		return
	}
	delete(payload, "nvidia")
	diags.AddAttributeError(
		path.Root("nvidia"),
		"nvidia requires TrueNAS 26.0 or later",
		"system.advanced.update on this TrueNAS release does not accept the \"nvidia\" field (it was added in TrueNAS 26.0). Remove the attribute from your configuration or target a TrueNAS 26.0+ server.",
	)
}

func (r *SystemAdvancedResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan SystemAdvancedModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// write-only: the framework nulls WriteOnly attributes in req.Plan, so
	// the actual sed_passwd value is only available via req.Config.
	var cfg SystemAdvancedModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.SedPasswd = cfg.SedPasswd

	payload, diags := plan.updatePayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.applyNvidiaSupport(ctx, payload, cfg.Nvidia, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.Call(ctx, "system.advanced.update", payload); err != nil {
		resp.Diagnostics.AddError("Create system advanced configuration failed", err.Error())
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

func (r *SystemAdvancedResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state SystemAdvancedModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// This is a singleton: system.advanced.config always exists, so Read
	// never removes the resource from state (there is no "not found" case).
	api, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read system advanced configuration failed", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *SystemAdvancedResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan SystemAdvancedModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// write-only: value lives in config, not plan.
	var cfg SystemAdvancedModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.SedPasswd = cfg.SedPasswd

	payload, diags := plan.updatePayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.applyNvidiaSupport(ctx, payload, cfg.Nvidia, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.Call(ctx, "system.advanced.update", payload); err != nil {
		resp.Diagnostics.AddError("Update system advanced configuration failed", err.Error())
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

// Delete makes NO client calls: the system advanced configuration controls
// syslog, console, and kernel/hardware behavior, so removing this resource
// from Terraform state must never rewrite the box's configuration. It only
// emits a warning diagnostic; Terraform itself drops the resource from state
// once Delete returns.
func (r *SystemAdvancedResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.Append(deleteWarningDiagnostics()...)
}

func (r *SystemAdvancedResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Any import ID is accepted and normalized to the fixed singleton ID; all
	// other fields are left unset so the subsequent Read call populates them
	// directly from system.advanced.config.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), systemAdvancedResourceID)...)
}

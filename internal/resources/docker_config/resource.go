// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package docker_config

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

var _ resource.Resource = &DockerConfigResource{}
var _ resource.ResourceWithImportState = &DockerConfigResource{}

// DockerConfigResource implements the truenas_docker_config singleton
// resource.
type DockerConfigResource struct{ client *client.Client }

// NewResource returns a new DockerConfigResource.
func NewResource() resource.Resource { return &DockerConfigResource{} }

func (r *DockerConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_docker_config"
}

func (r *DockerConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *DockerConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// fetchConfig calls docker.config and unmarshals the response.
func (r *DockerConfigResource) fetchConfig(ctx context.Context) (*dockerConfigAPI, error) {
	raw, err := r.client.CallRead(ctx, "docker.config")
	if err != nil {
		return nil, err
	}
	var api dockerConfigAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		return nil, err
	}
	return &api, nil
}

// applyNvidiaSupport folds "nvidia" into payload when the user explicitly
// set it in HCL, then strips it (with a clear apply-time error) if the
// target server is TrueNAS 26.0+.
//
// configNvidia MUST come from the practitioner's raw Config, not the
// resolved Plan: "nvidia" is Optional+Computed with UseStateForUnknown, so
// once it's ever been read from a live docker.config it carries a known
// (non-null) value in every subsequent plan even when the user never wrote
// "nvidia = ..." in their .tf file — Plan alone cannot distinguish "user
// explicitly (re-)configured this" from "framework carried the previous
// known value forward." Config never does this carrying-forward: it is
// null unless the practitioner actually wrote the attribute, so gating on
// Config is what keeps this resource usable on a TrueNAS 26.0+ target that
// never sets "nvidia" at all (the overwhelmingly common case) while still
// catching a practitioner who explicitly tries to set it there.
//
// Docker.config's response genuinely includes "nvidia" with a real boolean
// on BOTH probed releases (probed live, see task-1-report.md) — TrueNAS
// 26.0+ only dropped it from docker.update's accepted fields, not from
// docker.config's response — so this is a write-side-only restriction, not
// a "field doesn't exist" one; see model.go's dockerConfigAPI doc comment.
func (r *DockerConfigResource) applyNvidiaSupport(ctx context.Context, payload map[string]any, configNvidia types.Bool, diags *diag.Diagnostics) {
	if configNvidia.IsNull() || configNvidia.IsUnknown() {
		return
	}
	payload["nvidia"] = configNvidia.ValueBool()

	ok, err := r.client.VersionAtLeast(ctx, 26, 0)
	if err != nil || !ok {
		return
	}
	delete(payload, "nvidia")
	diags.AddAttributeError(
		path.Root("nvidia"),
		"nvidia is read-only on TrueNAS 26.0 and later",
		"docker.update on TrueNAS 26.0+ no longer accepts the \"nvidia\" field (docker.config still reports its current value, but it can no longer be changed through this API). Remove the attribute from your configuration or target a TrueNAS 25.10 (or earlier) server.",
	)
}

// buildUpdatePayload determines the target release's registry_mirrors wire
// shape via a live version probe, then builds the full docker.update
// payload from the plan (see DockerConfigModel.updatePayload for why the
// shape-selection itself is a plain parameter rather than living inside
// the pure payload builder). configNvidia is the practitioner's raw
// Config value for "nvidia" (see applyNvidiaSupport for why Plan can't be
// used here).
func (r *DockerConfigResource) buildUpdatePayload(ctx context.Context, plan *DockerConfigModel, configNvidia types.Bool) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	unified, err := r.client.VersionAtLeast(ctx, 26, 0)
	if err != nil {
		diags.AddError("Version probe failed", err.Error())
		return nil, diags
	}

	payload, d := plan.updatePayload(ctx, unified)
	diags.Append(d...)
	if diags.HasError() {
		return nil, diags
	}

	r.applyNvidiaSupport(ctx, payload, configNvidia, &diags)
	if diags.HasError() {
		return nil, diags
	}

	return payload, diags
}

// applyUpdate calls docker.update via CallJob (probed job:true — updating
// Docker's pool/dataset is a long-running migration operation).
func (r *DockerConfigResource) applyUpdate(ctx context.Context, payload map[string]any) error {
	_, err := r.client.CallJob(ctx, "docker.update", payload)
	return err
}

func (r *DockerConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DockerConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var config DockerConfigModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := r.buildUpdatePayload(ctx, &plan, config.Nvidia)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.applyUpdate(ctx, payload); err != nil {
		resp.Diagnostics.AddError("Create Docker configuration failed", err.Error())
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

func (r *DockerConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DockerConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// This is a singleton: docker.config always exists, so Read never
	// removes the resource from state (there is no "not found" case).
	api, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read Docker configuration failed", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *DockerConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan DockerConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var config DockerConfigModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := r.buildUpdatePayload(ctx, &plan, config.Nvidia)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.applyUpdate(ctx, payload); err != nil {
		resp.Diagnostics.AddError("Update Docker configuration failed", err.Error())
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

// Delete makes NO client calls: the Docker pool/network configuration
// underpins every running application, so removing this resource from
// Terraform state must never reset or reconfigure the box's Docker setup.
// It only emits a warning diagnostic; Terraform itself drops the resource
// from state once Delete returns.
func (r *DockerConfigResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.Append(deleteWarningDiagnostics()...)
}

func (r *DockerConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Any import ID is accepted and normalized to the fixed singleton ID; all
	// other fields are left unset so the subsequent Read call populates them
	// directly from docker.config.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), dockerConfigResourceID)...)
}

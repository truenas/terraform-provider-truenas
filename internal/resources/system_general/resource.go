// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package system_general

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
	"github.com/truenas/terraform-provider-truenas/internal/listing"
)

var _ resource.Resource = &SystemGeneralResource{}
var _ resource.ResourceWithImportState = &SystemGeneralResource{}
var _ resource.ResourceWithIdentity = &SystemGeneralResource{}

// SystemGeneralResource implements the truenas_system_general singleton
// resource.
type SystemGeneralResource struct{ client *client.Client }

// NewResource returns a new SystemGeneralResource.
func NewResource() resource.Resource { return &SystemGeneralResource{} }

func (r *SystemGeneralResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_system_general"
}

func (r *SystemGeneralResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *SystemGeneralResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = listing.StringIDIdentitySchema()
}

func (r *SystemGeneralResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// fetchConfig calls system.general.config and unmarshals the response.
func (r *SystemGeneralResource) fetchConfig(ctx context.Context) (*systemGeneralAPI, error) {
	raw, err := r.client.CallRead(ctx, "system.general.config")
	if err != nil {
		return nil, err
	}
	var api systemGeneralAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		return nil, err
	}
	return &api, nil
}

// uiRestartDelay is the ui_restart_delay (seconds) sent with a change to a
// web server setting: long enough for the update's response to reach the
// provider before nginx restarts and drops every HTTP/websocket connection.
// uiRestartTimeout bounds how long the provider waits for the restart job to
// finish and the API to answer again; uiRestartPoll is the poll interval.
var (
	uiRestartDelay   = 2 * time.Second
	uiRestartTimeout = 120 * time.Second
	uiRestartPoll    = 2 * time.Second
)

// applyUpdate sends the configured settings (config must come from
// req.Config; see updatePayload) to system.general.update. When the update
// changes a web server setting it also asks TrueNAS to restart the web
// server, because system.general.update only saves those settings (B-027),
// then waits for that restart to finish. An empty payload makes no call.
func (r *SystemGeneralResource) applyUpdate(ctx context.Context, config *SystemGeneralModel, diags *diag.Diagnostics) error {
	payload, d := config.updatePayload(ctx)
	diags.Append(d...)
	if diags.HasError() || len(payload) == 0 {
		return nil
	}
	current, err := r.fetchConfig(ctx)
	if err != nil {
		return fmt.Errorf("read current configuration: %w", err)
	}
	restart := webServerSettingsChanged(current, payload)
	var lastJob int64
	if restart {
		if lastJob, err = r.latestJobID(ctx); err != nil {
			return fmt.Errorf("read job list: %w", err)
		}
		payload["ui_restart_delay"] = int64(uiRestartDelay / time.Second)
	}
	if _, err := r.client.Call(ctx, "system.general.update", payload); err != nil {
		return err
	}
	if !restart {
		return nil
	}
	return r.awaitWebServerRestart(ctx, lastJob)
}

// restartJob is the subset of a core.get_jobs entry awaitWebServerRestart
// reads.
type restartJob struct {
	ID        int64   `json:"id"`
	Arguments []any   `json:"arguments"`
	State     string  `json:"state"`
	Error     *string `json:"error"`
}

// latestJobID returns the id of the newest middleware job, so the web
// server restart job scheduled after it can be told apart from older ones.
func (r *SystemGeneralResource) latestJobID(ctx context.Context) (int64, error) {
	raw, err := r.client.CallRead(ctx, "core.get_jobs", []any{}, map[string]any{"order_by": []string{"-id"}, "limit": 1})
	if err != nil {
		return 0, err
	}
	var jobs []restartJob
	if err := json.Unmarshal(raw, &jobs); err != nil {
		return 0, err
	}
	if len(jobs) == 0 {
		return 0, nil
	}
	return jobs[0].ID, nil
}

// awaitWebServerRestart waits for the service.control RESTART http job that
// ui_restart_delay schedules (newer than afterJob) to finish. The restart
// drops this provider's connection; CallRead re-dials it. A failed restart
// job is an error, and so is not seeing the job finish within
// uiRestartTimeout (for example because the change moved the address or port
// the provider connects to).
func (r *SystemGeneralResource) awaitWebServerRestart(ctx context.Context, afterJob int64) error {
	deadline := time.Now().Add(uiRestartTimeout)
	filter := []any{[]any{"id", ">", afterJob}, []any{"method", "=", "service.control"}}
	var lastErr error
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(uiRestartPoll):
		}
		raw, err := r.client.CallRead(ctx, "core.get_jobs", filter)
		if err == nil {
			var jobs []restartJob
			if err = json.Unmarshal(raw, &jobs); err == nil {
				for _, j := range jobs {
					if len(j.Arguments) < 2 || j.Arguments[0] != "RESTART" || j.Arguments[1] != "http" {
						continue
					}
					switch j.State {
					case "SUCCESS":
						return nil
					case "FAILED", "ABORTED":
						msg := j.State
						if j.Error != nil {
							msg = *j.Error
						}
						return fmt.Errorf("the web UI settings were saved, but restarting the web server to apply them failed: %s", msg)
					}
				}
			}
		}
		if err != nil {
			lastErr = err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("the web UI settings were saved and a web server restart was requested to apply "+
				"them, but the restart was not seen to finish within %s (last error: %v). If the change moved "+
				"the address or port the provider connects to, update the provider endpoint and apply "+
				"again", uiRestartTimeout, lastErr)
		}
	}
}

func (r *SystemGeneralResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan SystemGeneralModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var config SystemGeneralModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.applyUpdate(ctx, &config, &resp.Diagnostics); err != nil {
		resp.Diagnostics.AddError("Create system general configuration failed", err.Error())
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
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, systemGeneralResourceID)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *SystemGeneralResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state SystemGeneralModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// This is a singleton: system.general.config always exists, so Read
	// never removes the resource from state (there is no "not found" case).
	api, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read system general configuration failed", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, systemGeneralResourceID)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *SystemGeneralResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan SystemGeneralModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var config SystemGeneralModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.applyUpdate(ctx, &config, &resp.Diagnostics); err != nil {
		resp.Diagnostics.AddError("Update system general configuration failed", err.Error())
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
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, systemGeneralResourceID)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete makes NO client calls: the system general configuration controls
// management UI access, so removing this resource from Terraform state must
// never rewrite the box's configuration. It only emits a warning
// diagnostic; Terraform itself drops the resource from state once Delete
// returns.
func (r *SystemGeneralResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.Append(deleteWarningDiagnostics()...)
}

func (r *SystemGeneralResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Any import ID is accepted and normalized to the fixed singleton ID; all
	// other fields are left unset so the subsequent Read call populates them
	// directly from system.general.config.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), systemGeneralResourceID)...)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, systemGeneralResourceID)...)
}

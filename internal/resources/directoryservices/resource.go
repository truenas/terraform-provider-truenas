// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package directoryservices

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &DirectoryServicesResource{}
var _ resource.ResourceWithImportState = &DirectoryServicesResource{}

// dsStatusPollInterval is the interval between directoryservices.status
// polls after an enabling update. It's a package variable (not a const) so
// tests could shrink it, mirroring client.jobPollInterval.
var dsStatusPollInterval = 5 * time.Second

// dsStatusPollTimeout bounds how long Create/Update wait for
// directoryservices.status to report HEALTHY after an enabling
// directoryservices.update job succeeds. The update job itself completing
// only means the join RPC returned; winbind/sssd health can take longer to
// settle, per the task-8 brief.
const dsStatusPollTimeout = 5 * time.Minute

// DirectoryServicesResource implements the truenas_directoryservices
// singleton resource.
type DirectoryServicesResource struct{ client *client.Client }

// NewResource returns a new DirectoryServicesResource.
func NewResource() resource.Resource { return &DirectoryServicesResource{} }

func (r *DirectoryServicesResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_directoryservices"
}

func (r *DirectoryServicesResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *DirectoryServicesResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// fetchConfig calls directoryservices.config and unmarshals the response.
func (r *DirectoryServicesResource) fetchConfig(ctx context.Context) (*directoryServicesAPI, error) {
	raw, err := r.client.CallRead(ctx, "directoryservices.config")
	if err != nil {
		return nil, err
	}
	var api directoryServicesAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		return nil, err
	}
	return &api, nil
}

// fetchStatus calls directoryservices.status and unmarshals the response.
func (r *DirectoryServicesResource) fetchStatus(ctx context.Context) (*directoryServicesStatusAPI, error) {
	raw, err := r.client.CallRead(ctx, "directoryservices.status")
	if err != nil {
		return nil, err
	}
	var st directoryServicesStatusAPI
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

// waitForHealthy polls directoryservices.status until it reports HEALTHY,
// FAULTED, or dsStatusPollTimeout elapses. It's called after an enabling
// directoryservices.update job succeeds: the job completing only means the
// join RPC itself returned, not that winbind/sssd has finished settling
// into a healthy state.
func (r *DirectoryServicesResource) waitForHealthy(ctx context.Context) error {
	deadline := time.Now().Add(dsStatusPollTimeout)

	for {
		st, err := r.fetchStatus(ctx)
		if err != nil {
			return fmt.Errorf("polling directoryservices.status: %w", err)
		}

		status := "unknown"
		if st.Status != nil {
			status = *st.Status
		}

		switch status {
		case "HEALTHY":
			return nil
		case "FAULTED":
			msg := "(no status_msg)"
			if st.StatusMsg != nil && *st.StatusMsg != "" {
				msg = *st.StatusMsg
			}
			return fmt.Errorf("directory service status is FAULTED: %s", msg)
		}

		if time.Now().After(deadline) {
			msg := ""
			if st.StatusMsg != nil && *st.StatusMsg != "" {
				msg = fmt.Sprintf(": %s", *st.StatusMsg)
			}
			return fmt.Errorf("timed out after %s waiting for directoryservices.status to report HEALTHY "+
				"(last status: %s%s)", dsStatusPollTimeout, status, msg)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(dsStatusPollInterval):
		}
	}
}

// resetStaleServiceType clears ALL existing directory services
// configuration via directoryservices.update's "nuke" shape (enable=false,
// service_type=null, configuration=null, credential=null). Create/Update
// call this first whenever needsServiceTypeReset (model.go) reports that
// plan is about to switch the singleton to a different service_type than
// whatever is currently persisted (e.g. a previous ACTIVEDIRECTORY/IPA join
// being re-pointed at LDAP).
//
// This works around a confirmed live TrueNAS 25.10/26.0 middleware defect
// (probed against the disposable test VM during Task 3 of the LDAP/IPA
// plan): directoryservices.update is SUPPOSED to handle a service_type
// switch by calling its own internal directoryservices.reset() — see that
// method's "Configuration from a different service type should not by
// default carry over to the new service_type" comment — but reset() only
// mutates the on-disk datastore row directly, and the SAME update() call's
// own later commit (compress(new) + datastore.update) still writes back
// whatever kerberos_realm/credential value was already in memory before
// reset() ran, because compress() can resolve and set a kerberos_realm id
// but has no path to explicitly null one back out (unlike its sibling
// extend(), which does). Confirmed live: switching this box directly from
// a disabled ACTIVEDIRECTORY join to LDAP left a stale kerberos_realm in
// place even though the request omitted it entirely, and
// directoryservices_/ldap_join_mixin.py's _ldap_activate unconditionally
// calls kerberos.start() whenever kerberos_realm is truthy — which fails
// with "[EFAULT] Directory services are not configured to use kerberos
// credentials" (the LDAP_PLAIN credential isn't Kerberos-based) even
// though the LDAP bind itself succeeds and directoryservices.status
// reports HEALTHY moments later. The same stale config would also make
// existingKerberosPrincipal wrongly resend the OLD machine-account
// credential instead of the new LDAP_PLAIN one. The one code path in
// directoryservices.update where its own reset() call is NOT immediately
// undone is the full "nuke" shape (new['service_type'] is None), so that's
// what this sends as a preliminary step; the caller re-fetches
// directoryservices.config afterward to build the real payload against a
// genuinely clean baseline.
func (r *DirectoryServicesResource) resetStaleServiceType(ctx context.Context) error {
	_, err := r.client.CallJob(ctx, "directoryservices.update", map[string]any{
		"enable":        false,
		"service_type":  nil,
		"configuration": nil,
		"credential":    nil,
	})
	return err
}

func (r *DirectoryServicesResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DirectoryServicesModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// write-only: the framework nulls WriteOnly attributes (credential.password)
	// in req.Plan, so the actual password value (if any) is only available via
	// req.Config.
	var cfg DirectoryServicesModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.Credential = cfg.Credential

	// Fetch whatever directoryservices.config already holds (normally
	// nothing, for a brand new join) so trusted_domains can be round-tripped
	// verbatim if directory services somehow already have a persisted AD
	// configuration (e.g. joined previously outside Terraform) — see
	// buildADConfigPayload's doc comment. idmap itself is explicitly modeled
	// (see ActiveDirectoryConfigModel) and no longer needs this verbatim
	// round-trip.
	existing, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read directory services configuration failed", err.Error())
		return
	}

	// Switching this singleton to a different service_type than whatever is
	// currently persisted needs a preliminary clear — see
	// resetStaleServiceType's doc comment.
	if plan.Enable.ValueBool() && needsServiceTypeReset(&plan, existing) {
		if err := r.resetStaleServiceType(ctx); err != nil {
			resp.Diagnostics.AddError("Clearing stale directory services configuration failed", err.Error())
			return
		}
		existing, err = r.fetchConfig(ctx)
		if err != nil {
			resp.Diagnostics.AddError("Read directory services configuration failed", err.Error())
			return
		}
	}

	payload, diags := plan.updatePayload(ctx, existing)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// directoryservices.update is job:true — joining a domain can take a
	// while.
	if _, err := r.client.CallJob(ctx, "directoryservices.update", payload); err != nil {
		resp.Diagnostics.AddError("Create directory services configuration failed", err.Error())
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
	// Always persist state once the update job itself has succeeded, even if
	// the health poll below fails: the join already happened on the domain
	// (computer account, DNS records, ...), so Terraform must keep tracking
	// it rather than leaving it orphaned outside of state.
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)

	if plan.Enable.ValueBool() {
		if err := r.waitForHealthy(ctx); err != nil {
			resp.Diagnostics.AddError("Directory service did not become healthy", err.Error())
		}
	}
}

func (r *DirectoryServicesResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DirectoryServicesModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// This is a singleton: directoryservices.config always exists, so Read
	// never removes the resource from state (there is no "not found" case).
	api, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read directory services configuration failed", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, api, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *DirectoryServicesResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan DirectoryServicesModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// write-only: value lives in config, not plan.
	var cfg DirectoryServicesModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.Credential = cfg.Credential

	// Fetch the currently-persisted configuration so trusted_domains (not
	// modeled in Terraform state) can be round-tripped verbatim — see
	// buildADConfigPayload's doc comment for why TrueNAS requires this
	// whenever plan.Enable is true, even for an update that only bumps
	// "timeout". idmap itself is explicitly modeled (see
	// ActiveDirectoryConfigModel) and no longer needs this verbatim
	// round-trip.
	existing, err := r.fetchConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Read directory services configuration failed", err.Error())
		return
	}

	// Switching this singleton to a different service_type than whatever is
	// currently persisted needs a preliminary clear — see
	// resetStaleServiceType's doc comment.
	if plan.Enable.ValueBool() && needsServiceTypeReset(&plan, existing) {
		if err := r.resetStaleServiceType(ctx); err != nil {
			resp.Diagnostics.AddError("Clearing stale directory services configuration failed", err.Error())
			return
		}
		existing, err = r.fetchConfig(ctx)
		if err != nil {
			resp.Diagnostics.AddError("Read directory services configuration failed", err.Error())
			return
		}
	}

	payload, diags := plan.updatePayload(ctx, existing)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.CallJob(ctx, "directoryservices.update", payload); err != nil {
		resp.Diagnostics.AddError("Update directory services configuration failed", err.Error())
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

	if plan.Enable.ValueBool() {
		if err := r.waitForHealthy(ctx); err != nil {
			resp.Diagnostics.AddError("Directory service did not become healthy", err.Error())
		}
	}
}

// Delete calls directoryservices.update with ONLY enable=false — it NEVER
// calls directoryservices.leave. Leaving the domain requires a domain
// administrator credential that is not necessarily available (or desirable
// to hold) at destroy time, and would remove the TrueNAS computer account
// from the domain controller. Destroying this resource therefore only
// disables directory services locally; the domain join itself (and its
// swapped-in machine-account credential — see
// model.go's existingKerberosPrincipal) is left in place and can be
// re-enabled later by recreating the resource with enable=true and no
// credential.
func (r *DirectoryServicesResource) Delete(ctx context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	if _, err := r.client.CallJob(ctx, "directoryservices.update", map[string]any{"enable": false}); err != nil {
		resp.Diagnostics.AddError("Disable directory services failed", err.Error())
	}
}

func (r *DirectoryServicesResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Any import ID is accepted and normalized to the fixed singleton ID; all
	// other fields are left unset so the subsequent Read call populates them
	// directly from directoryservices.config. "credential" is left unset:
	// it is write-only and was never held by TrueNAS in a readable form.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), directoryServicesResourceID)...)
}

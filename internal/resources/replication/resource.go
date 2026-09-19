// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package replication

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &ReplicationResource{}
var _ resource.ResourceWithImportState = &ReplicationResource{}
var _ resource.ResourceWithValidateConfig = &ReplicationResource{}

// ReplicationResource implements the truenas_replication_task resource.
type ReplicationResource struct{ client *client.Client }

// NewResource returns a new ReplicationResource.
func NewResource() resource.Resource { return &ReplicationResource{} }

func (r *ReplicationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_replication_task"
}

func (r *ReplicationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *ReplicationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// nameRegexConflict reports whether config sets name_regex together with
// naming_schema and/or also_include_naming_schema: the API rejects that
// combination, so ValidateConfig surfaces it as a config-time error rather
// than a Create/Update API failure. Split out from ValidateConfig so the
// boolean logic can be unit-tested without the full plugin-framework config
// harness.
func nameRegexConflict(config *ReplicationModel) bool {
	hasRegex := !config.NameRegex.IsNull() && !config.NameRegex.IsUnknown() && config.NameRegex.ValueString() != ""
	hasSchema := !config.NamingSchema.IsNull() && !config.NamingSchema.IsUnknown() && len(config.NamingSchema.Elements()) > 0
	hasAlso := !config.AlsoIncludeNamingSchema.IsNull() && !config.AlsoIncludeNamingSchema.IsUnknown() && len(config.AlsoIncludeNamingSchema.Elements()) > 0

	return hasRegex && (hasSchema || hasAlso)
}

// transportOrDefault resolves the effective transport for preflight
// purposes: the config's own value when known, "LOCAL" (the schema
// default applied at plan time — ValidateConfig runs against the raw
// config, before that default lands, so an omitted transport must be
// treated as its eventual default here too) when the config omits it
// (null), or "" when it isn't knowable yet (unknown — e.g. interpolated
// from another resource's not-yet-applied attribute). Callers treat ""
// as "can't conclude, don't flag an error", the same treatment
// nameRegexConflict gives unknown naming_schema/also_include_naming_schema
// values.
func transportOrDefault(config *ReplicationModel) string {
	if config.Transport.IsUnknown() {
		return ""
	}
	if config.Transport.IsNull() {
		return "LOCAL"
	}
	return config.Transport.ValueString()
}

// sshCredentialsSet reports whether config sets a non-zero ssh_credentials,
// treating null/0 as unset (0 is the sentinel model.go's
// responseToModel/apiPayload use for "no ssh_credentials", matching the
// pre-existing LOCAL-transport convention). Callers must check
// IsUnknown() themselves where that distinction matters.
func sshCredentialsSet(config *ReplicationModel) bool {
	return !config.SSHCredentials.IsNull() && !config.SSHCredentials.IsUnknown() && config.SSHCredentials.ValueInt64() != 0
}

// transportSSHCredentialsMismatch enforces the pairing the API requires:
// transport = "SSH" needs ssh_credentials (replication.create rejects SSH
// without it), and transport = "LOCAL" must not carry one (there is no
// remote system to authenticate to). Returns "" when the config is
// consistent (including when either value isn't knowable yet — e.g. the
// acceptance test's `ssh_credentials = truenas_keychain_ssh_connection.test.id`
// is Unknown during ValidateConfig, before that resource is created).
func transportSSHCredentialsMismatch(config *ReplicationModel) string {
	if config.SSHCredentials.IsUnknown() {
		return ""
	}
	transport := transportOrDefault(config)
	if transport == "" {
		return ""
	}
	hasCred := sshCredentialsSet(config)
	switch transport {
	case "SSH", "SSH+NETCAT":
		if !hasCred {
			return fmt.Sprintf("transport = %q requires ssh_credentials to be set (the id of a "+
				"truenas_keychain_ssh_connection credential).", transport)
		}
	case "LOCAL":
		if hasCred {
			return "transport = \"LOCAL\" (the default) does not accept ssh_credentials; either unset " +
				"ssh_credentials or set transport = \"SSH\" or \"SSH+NETCAT\"."
		}
	}
	return ""
}

// netcatSet reports whether any netcat_* attribute carries a known value.
func netcatSet(config *ReplicationModel) bool {
	return (!config.NetcatActiveSide.IsNull() && !config.NetcatActiveSide.IsUnknown()) ||
		(!config.NetcatActiveSideListenAddress.IsNull() && !config.NetcatActiveSideListenAddress.IsUnknown()) ||
		(!config.NetcatActiveSidePortMin.IsNull() && !config.NetcatActiveSidePortMin.IsUnknown()) ||
		(!config.NetcatActiveSidePortMax.IsNull() && !config.NetcatActiveSidePortMax.IsUnknown()) ||
		(!config.NetcatPassiveSideConnectAddress.IsNull() && !config.NetcatPassiveSideConnectAddress.IsUnknown())
}

// netcatFieldsMismatch enforces the SSH+NETCAT pairing: the netcat_* fields
// are valid only for transport = "SSH+NETCAT", and that transport requires
// netcat_active_side. Returns "" when consistent (including when transport
// isn't knowable yet).
func netcatFieldsMismatch(config *ReplicationModel) string {
	transport := transportOrDefault(config)
	if transport == "" {
		return ""
	}
	if transport != "SSH+NETCAT" {
		if netcatSet(config) {
			return "the netcat_active_side, netcat_active_side_listen_address, " +
				"netcat_active_side_port_min, netcat_active_side_port_max, and " +
				"netcat_passive_side_connect_address attributes are only valid for transport = \"SSH+NETCAT\"."
		}
		return ""
	}
	// transport == SSH+NETCAT: netcat_active_side is required (unless not yet knowable).
	if config.NetcatActiveSide.IsUnknown() {
		return ""
	}
	if config.NetcatActiveSide.IsNull() {
		return "transport = \"SSH+NETCAT\" requires netcat_active_side to be set (LOCAL or REMOTE)."
	}
	return ""
}

// sshOnlyFieldsWithoutSSH reports whether compression or speed_limit are
// set while transport isn't "SSH" — both are SSH-stream-only per the API
// schema (probed live: their descriptions read "Available only for SSH
// transport"), so LOCAL replication must not carry them. Returns "" when
// the config is consistent, including when transport isn't knowable yet.
func sshOnlyFieldsWithoutSSH(config *ReplicationModel) string {
	transport := transportOrDefault(config)
	if transport == "" || transport == "SSH" {
		return ""
	}
	hasCompression := !config.Compression.IsNull() && !config.Compression.IsUnknown()
	hasSpeedLimit := !config.SpeedLimit.IsNull() && !config.SpeedLimit.IsUnknown()
	if hasCompression || hasSpeedLimit {
		return "compression and speed_limit are only valid for transport = \"SSH\"."
	}
	return ""
}

// ValidateConfig enforces config-time invariants the API only checks at
// Create/Update time: name_regex is mutually exclusive with naming_schema
// and also_include_naming_schema; transport = "SSH" requires
// ssh_credentials while transport = "LOCAL" forbids it; compression and
// speed_limit are SSH-only.
func (r *ReplicationResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config ReplicationModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if nameRegexConflict(&config) {
		resp.Diagnostics.AddError(
			"Conflicting attributes",
			"name_regex is mutually exclusive with naming_schema and also_include_naming_schema.",
		)
	}

	if msg := transportSSHCredentialsMismatch(&config); msg != "" {
		resp.Diagnostics.AddError("Invalid ssh_credentials for transport", msg)
	}

	if msg := sshOnlyFieldsWithoutSSH(&config); msg != "" {
		resp.Diagnostics.AddError("SSH-only attribute set", msg)
	}

	if msg := netcatFieldsMismatch(&config); msg != "" {
		resp.Diagnostics.AddError("Invalid netcat configuration for transport", msg)
	}
}

func (r *ReplicationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ReplicationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.apiPayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.Call(ctx, "replication.create", payload)
	if err != nil {
		resp.Diagnostics.AddError("Create replication task failed", err.Error())
		return
	}

	// Parse only the ID from the create response, then read back via get_instance.
	var created replicationAPI
	if err := json.Unmarshal(raw, &created); err != nil {
		resp.Diagnostics.AddError("Parse create response", err.Error())
		return
	}

	raw, err = r.client.CallRead(ctx, "replication.get_instance", created.ID)
	if err != nil {
		resp.Diagnostics.AddError("Read-back after create failed", err.Error())
		return
	}

	var apiResp replicationAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse get_instance response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ReplicationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ReplicationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "replication.get_instance", state.ID.ValueInt64())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read replication task failed", err.Error())
		return
	}

	var apiResp replicationAPI
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

func (r *ReplicationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ReplicationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Carry the existing state ID into the plan.
	var state ReplicationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID

	payload, diags := plan.apiPayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Call(ctx, "replication.update", plan.ID.ValueInt64(), payload)
	if err != nil {
		resp.Diagnostics.AddError("Update replication task failed", err.Error())
		return
	}

	raw, err := r.client.CallRead(ctx, "replication.get_instance", plan.ID.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Read-back after update failed", err.Error())
		return
	}

	var apiResp replicationAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse get_instance response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ReplicationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ReplicationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Call(ctx, "replication.delete", state.ID.ValueInt64())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete replication task failed", err.Error())
	}
}

func (r *ReplicationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Import ID must be an integer", req.ID)
		return
	}

	raw, err := r.client.CallRead(ctx, "replication.get_instance", id)
	if err != nil {
		resp.Diagnostics.AddError("Import replication task failed", err.Error())
		return
	}

	var apiResp replicationAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse import response", err.Error())
		return
	}

	var state ReplicationModel
	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

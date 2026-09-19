// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package keychain_ssh_keypair

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &KeychainSSHKeyPairResource{}
var _ resource.ResourceWithImportState = &KeychainSSHKeyPairResource{}

// KeychainSSHKeyPairResource implements the truenas_keychain_ssh_keypair
// resource.
type KeychainSSHKeyPairResource struct{ client *client.Client }

// NewResource returns a new instance of KeychainSSHKeyPairResource.
func NewResource() resource.Resource { return &KeychainSSHKeyPairResource{} }

func (r *KeychainSSHKeyPairResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_keychain_ssh_keypair"
}

func (r *KeychainSSHKeyPairResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *KeychainSSHKeyPairResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// keychaincredential.create/update/delete/get_instance/query and
// generate_ssh_key_pair are all non-job (sync) methods (probed via
// core.get_methods: every one reports "job": false).

// isConcreteTrue reports whether a plan bool is a known, non-null value of
// true — the same "not Null and not Unknown" shape certificate.go's isSet
// helper uses for strings, applied here to distinguish "user set
// generate=true" from "left unset" (which plans as Unknown for this
// Optional+Computed attribute on Create, since there is no prior state).
func isConcreteTrue(v types.Bool) bool {
	return !v.IsNull() && !v.IsUnknown() && v.ValueBool()
}

// isSet reports whether a string attribute was given a concrete, non-empty
// value in the plan, matching certificate.go's preflight helper.
func isSet(v types.String) bool {
	return !v.IsNull() && !v.IsUnknown() && v.ValueString() != ""
}

func (r *KeychainSSHKeyPairResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan KeychainSSHKeyPairModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	generate := isConcreteTrue(plan.Generate)
	privateKeySet := isSet(plan.PrivateKey)

	switch {
	case generate && privateKeySet:
		resp.Diagnostics.AddError(
			"Conflicting configuration",
			"Exactly one of \"generate\" (true) or \"private_key\" must be set, not both.",
		)
		return
	case !generate && !privateKeySet:
		resp.Diagnostics.AddError(
			"Missing configuration",
			"Exactly one of \"generate\" (true) or \"private_key\" must be set.",
		)
		return
	}

	// Resolve "generate" to a concrete value now: it is Optional+Computed,
	// so an unset config plans as Unknown, which cannot be persisted to
	// state as-is.
	plan.Generate = types.BoolValue(generate)

	var privateKey, publicKey string
	if generate {
		raw, err := r.client.Call(ctx, "keychaincredential.generate_ssh_key_pair")
		if err != nil {
			resp.Diagnostics.AddError("Generate SSH key pair failed", err.Error())
			return
		}
		var gen generatedKeyPair
		if err := json.Unmarshal(raw, &gen); err != nil {
			resp.Diagnostics.AddError("Parse generate_ssh_key_pair response", err.Error())
			return
		}
		privateKey, publicKey = gen.PrivateKey, gen.PublicKey
	} else {
		privateKey = plan.PrivateKey.ValueString()
		// publicKey left empty: TrueNAS derives it server-side (see
		// createPayload's doc comment).
	}

	payload := createPayload(plan.Name.ValueString(), privateKey, publicKey)

	raw, err := r.client.Call(ctx, "keychaincredential.create", payload)
	if err != nil {
		resp.Diagnostics.AddError("Create keychain SSH key pair failed", err.Error())
		return
	}

	var apiResp keychainSSHKeyPairAPI
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

func (r *KeychainSSHKeyPairResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state KeychainSSHKeyPairModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.CallRead(ctx, "keychaincredential.get_instance", state.ID.ValueInt64())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read keychain SSH key pair failed", err.Error())
		return
	}

	var apiResp keychainSSHKeyPairAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse response", err.Error())
		return
	}
	if apiResp.Type != keychainCredentialType {
		resp.Diagnostics.AddError(
			"Unexpected keychain credential type",
			fmt.Sprintf("id %d is a %q keychain credential, not %q — it was not created by (and cannot be "+
				"managed as) truenas_keychain_ssh_keypair", state.ID.ValueInt64(), apiResp.Type, keychainCredentialType),
		)
		return
	}

	resp.Diagnostics.Append(responseToModel(&apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *KeychainSSHKeyPairResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan KeychainSSHKeyPairModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state KeychainSSHKeyPairModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.Call(ctx, "keychaincredential.update", state.ID.ValueInt64(), plan.updatePayload())
	if err != nil {
		resp.Diagnostics.AddError("Update keychain SSH key pair failed", err.Error())
		return
	}

	var apiResp keychainSSHKeyPairAPI
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

func (r *KeychainSSHKeyPairResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state KeychainSSHKeyPairModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Call(ctx, "keychaincredential.delete", state.ID.ValueInt64())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete keychain SSH key pair failed", err.Error())
	}
}

func (r *KeychainSSHKeyPairResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Import ID must be an integer", req.ID)
		return
	}

	raw, err := r.client.CallRead(ctx, "keychaincredential.get_instance", id)
	if err != nil {
		resp.Diagnostics.AddError("Import keychain SSH key pair failed", err.Error())
		return
	}

	var apiResp keychainSSHKeyPairAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse import response", err.Error())
		return
	}
	if apiResp.Type != keychainCredentialType {
		resp.Diagnostics.AddError(
			"Unexpected keychain credential type",
			fmt.Sprintf("id %d is a %q keychain credential, not %q — it cannot be imported as "+
				"truenas_keychain_ssh_keypair", id, apiResp.Type, keychainCredentialType),
		)
		return
	}

	var state KeychainSSHKeyPairModel
	// "generate" is never knowable on import — TrueNAS never records
	// whether a key pair's private_key was user-supplied or
	// server-generated. Left explicitly null; acceptance tests must set
	// ImportStateVerifyIgnore: ["generate"].
	state.Generate = types.BoolNull()
	resp.Diagnostics.Append(responseToModel(&apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

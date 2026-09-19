// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package keychain_ssh_keypair

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &KeychainSSHKeyPairDataSource{}

// KeychainSSHKeyPairDataSource implements the truenas_keychain_ssh_keypair
// data source.
type KeychainSSHKeyPairDataSource struct{ client *client.Client }

// NewDataSource returns a new instance of KeychainSSHKeyPairDataSource.
func NewDataSource() datasource.DataSource { return &KeychainSSHKeyPairDataSource{} }

func (d *KeychainSSHKeyPairDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_keychain_ssh_keypair"
}

func (d *KeychainSSHKeyPairDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS keychain SSH key pair credential (keychaincredential.* with " +
			"type=SSH_KEY_PAIR) by name.",
		Attributes: map[string]dschema.Attribute{
			"id":   dschema.Int64Attribute{Computed: true, Description: "Numeric identifier of the keychain credential."},
			"name": dschema.StringAttribute{Required: true, Description: "Name of the keychain credential to look up."},
			"private_key": dschema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "SSH private key in OpenSSH PEM format.",
			},
			"public_key": dschema.StringAttribute{
				Computed:    true,
				Description: "SSH public key in OpenSSH authorized_keys format.",
			},
		},
	}
}

func (d *KeychainSSHKeyPairDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *KeychainSSHKeyPairDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state KeychainSSHKeyPairDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := d.client.CallRead(ctx, "keychaincredential.query",
		[]any{[]any{"name", "=", state.Name.ValueString()}})
	if err != nil {
		resp.Diagnostics.AddError("Query keychain credentials failed", err.Error())
		return
	}

	var results []keychainSSHKeyPairAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse query response", err.Error())
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"Keychain SSH key pair not found",
			fmt.Sprintf("no keychain credential found with name %q", state.Name.ValueString()),
		)
		return
	}
	if results[0].Type != keychainCredentialType {
		resp.Diagnostics.AddError(
			"Unexpected keychain credential type",
			fmt.Sprintf("keychain credential %q is a %q credential, not %q",
				state.Name.ValueString(), results[0].Type, keychainCredentialType),
		)
		return
	}

	resp.Diagnostics.Append(responseToDataSourceModel(&results[0], &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

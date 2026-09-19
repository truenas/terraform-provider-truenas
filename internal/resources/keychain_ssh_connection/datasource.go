// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package keychain_ssh_connection

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ datasource.DataSource = &KeychainSSHConnectionDataSource{}

// KeychainSSHConnectionDataSource implements the
// truenas_keychain_ssh_connection data source.
type KeychainSSHConnectionDataSource struct{ client *client.Client }

// NewDataSource returns a new instance of KeychainSSHConnectionDataSource.
func NewDataSource() datasource.DataSource { return &KeychainSSHConnectionDataSource{} }

func (d *KeychainSSHConnectionDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_keychain_ssh_connection"
}

func (d *KeychainSSHConnectionDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dschema.Schema{
		Description: "Fetches a TrueNAS keychain SSH connection credential (keychaincredential.* with " +
			"type=SSH_CREDENTIALS) by name.",
		Attributes: map[string]dschema.Attribute{
			"id":   dschema.Int64Attribute{Computed: true, Description: "Numeric identifier of the keychain credential."},
			"name": dschema.StringAttribute{Required: true, Description: "Name of the keychain credential to look up."},
			"host": dschema.StringAttribute{Computed: true, Description: "SSH server hostname or IP address."},
			"port": dschema.Int64Attribute{Computed: true, Description: "SSH server port number."},
			"username": dschema.StringAttribute{
				Computed:    true,
				Description: "SSH username for authentication.",
			},
			"private_key_id": dschema.Int64Attribute{
				Computed:    true,
				Description: "Numeric id of the truenas_keychain_ssh_keypair used to authenticate.",
			},
			"remote_host_key": dschema.StringAttribute{
				Computed:    true,
				Description: "The remote host's SSH public key(s), one per line.",
			},
			"connect_timeout": dschema.Int64Attribute{
				Computed:    true,
				Description: "Connection timeout in seconds for SSH connections.",
			},
		},
	}
}

func (d *KeychainSSHConnectionDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *KeychainSSHConnectionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state KeychainSSHConnectionDataSourceModel
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

	var results []keychainSSHConnectionAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		resp.Diagnostics.AddError("Parse query response", err.Error())
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"Keychain SSH connection not found",
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

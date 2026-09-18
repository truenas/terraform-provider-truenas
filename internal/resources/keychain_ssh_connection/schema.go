// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package keychain_ssh_connection

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages an SSH connection credential stored in the TrueNAS keychain " +
			"(keychaincredential.* with type=SSH_CREDENTIALS) — the credential type consumed by SSH-transport " +
			"truenas_replication. \"private_key_id\" references a truenas_keychain_ssh_keypair credential's id " +
			"(NOT the raw key material); \"remote_host_key\" is the target host's public key, discoverable via " +
			"keychaincredential.remote_ssh_host_key_scan. keychaincredential.create does not itself validate " +
			"SSH connectivity or authentication (probed live: a connection was created successfully " +
			"referencing a key that was never authorized on the target host) — this resource stores the " +
			"connection's configuration, it does not verify the credential actually works. Every field is " +
			"updatable in place: keychaincredential.update requires resending the complete \"attributes\" " +
			"object, so this resource's Update always sends every attribute (not a partial merge).",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Numeric identifier of the keychain credential.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Human-readable name for the keychain credential. Updatable in place.",
			},
			"host": schema.StringAttribute{
				Required:    true,
				Description: "SSH server hostname or IP address. Updatable in place.",
			},
			"port": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "SSH server port number. Defaults to 22. Updatable in place.",
				Validators:  []validator.Int64{int64validator.Between(1, 65535)},
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"username": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "SSH username for authentication. Defaults to \"root\". Updatable in place.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"private_key_id": schema.Int64Attribute{
				Required: true,
				Description: "Numeric id of a truenas_keychain_ssh_keypair (keychaincredential of type " +
					"SSH_KEY_PAIR) to authenticate with. Updatable in place.",
			},
			"remote_host_key": schema.StringAttribute{
				Required: true,
				Description: "The remote host's SSH public key(s), one per line, as returned by " +
					"keychaincredential.remote_ssh_host_key_scan (which this provider does not call " +
					"automatically — obtain the value via that method, e.g. a data source or provider script, " +
					"and pass it in here). Not validated against the live host at create/update time. Updatable " +
					"in place.",
			},
			"connect_timeout": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Connection timeout in seconds for SSH connections. Defaults to 10. Updatable in place.",
				Validators:  []validator.Int64{int64validator.AtLeast(1)},
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

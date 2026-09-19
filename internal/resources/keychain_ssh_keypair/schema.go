// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package keychain_ssh_keypair

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages an SSH key pair stored in the TrueNAS keychain (keychaincredential.* with " +
			"type=SSH_KEY_PAIR) — the credential type consumed by truenas_keychain_ssh_connection's " +
			"\"private_key_id\" and by SSH-transport truenas_replication. Exactly one of \"private_key\" or " +
			"\"generate\" must be set: either supply your own OpenSSH-format private key, or set " +
			"generate=true to have TrueNAS generate a fresh RSA key pair via " +
			"keychaincredential.generate_ssh_key_pair. Both paths are immutable after creation (changing " +
			"either forces replacement) — keychaincredential.update accepts a full replacement \"attributes\" " +
			"object in principle, but this resource never sends one, treating the key material itself as " +
			"create-once. Only \"name\" is updatable in place.",
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
			"generate": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Description: "Set to true to have TrueNAS generate a new RSA SSH key pair (via " +
					"keychaincredential.generate_ssh_key_pair) instead of supplying \"private_key\" directly. " +
					"Exactly one of \"generate\" or \"private_key\" must be set. Not itself an API field — this " +
					"is a provider-side switch controlling which keychaincredential.create path is taken; " +
					"Create resolves it to a concrete true/false and stores that, and it is otherwise never " +
					"read back from the API (its state value is simply preserved on refresh), so it always " +
					"reads as null after `terraform import`. Immutable.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
					boolplanmodifier.RequiresReplace(),
				},
			},
			"private_key": schema.StringAttribute{
				Optional:  true,
				Computed:  true,
				Sensitive: true,
				Description: "SSH private key in OpenSSH PEM format. Supply this directly (with \"generate\" " +
					"left false/unset) to import an existing key, or leave it unset with generate=true to have " +
					"TrueNAS generate one — either way the resulting value (yours or the generated one) is read " +
					"back here. Marked Sensitive (kept out of plan/apply output and logs), but — unlike a " +
					"WriteOnly attribute — it IS stored in state and IS read back on refresh/import: " +
					"keychaincredential.get_instance/query return it byte-for-byte intact (probed live: no " +
					"masking anywhere in the create/get_instance/query/update responses, unlike " +
					"truenas_certificate's job-result masking). Immutable.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"public_key": schema.StringAttribute{
				Computed: true,
				Description: "SSH public key in OpenSSH authorized_keys format, in \"<algorithm> <base64> " +
					"[comment]\" form. Always server-derived — TrueNAS computes it from \"private_key\" " +
					"automatically (probed live: creating a key pair with only private_key set, no public_key, " +
					"correctly echoes back the matching derived public key), so this attribute never accepts " +
					"input and is purely a read-back. Immutable.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

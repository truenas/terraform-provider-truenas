// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package api_key

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages a TrueNAS API key (api_key): a credential belonging to a local user, used to " +
			"authenticate to the TrueNAS API in place of a password. Key rotation (the API's `reset` flag) is not " +
			"exposed by this resource — taint and recreate the resource to rotate a key.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Numeric identifier of the API key.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Human-readable name for the API key.",
			},
			"username": schema.StringAttribute{
				Required: true,
				Description: "Local username that owns this API key. Immutable after creation — api_key.update " +
					"rejects any attempt to change it, so changing this attribute forces replacement.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"expires_at": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "RFC3339 timestamp after which the key expires (e.g. \"2030-01-01T00:00:00Z\"); " +
					"null/unset means the key never expires. The provider always reads this back as a UTC, " +
					"'Z'-suffixed RFC3339 string, so configure it that way to avoid a permanent diff against a " +
					"differently-formatted (but equivalent) input.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"key": schema.StringAttribute{
				Computed:  true,
				Sensitive: true,
				Description: "The plaintext API key value. TrueNAS returns this only once, at creation time; it is " +
					"stored in state from that response and is never re-read from the API afterward (reads and " +
					"updates always carry the existing state value forward unchanged). Not verifiable on import — " +
					"see the resource's import documentation.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				Computed:    true,
				Description: "RFC3339 UTC timestamp of when the API key was created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"local": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether this API key is for local system use only.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"revoked": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the API key has been revoked and is no longer valid.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

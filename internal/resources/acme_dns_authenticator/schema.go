// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acme_dns_authenticator

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/truenas/terraform-provider-truenas/internal/normalize"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages a TrueNAS ACME DNS authenticator (acme.dns.authenticator): stored " +
			"credentials/config for a DNS provider, used to satisfy ACME DNS-01 challenges (referenced by ID from " +
			"a truenas_certificate's \"dns_mapping\" when create_type is CERTIFICATE_CREATE_ACME). " +
			"acme.dns.authenticator.authenticator_schemas returns five discriminated variants (probed live, " +
			"identical on TrueNAS 25.10/26.0): cloudflare, digitalocean, OVH, route53, and shell — see " +
			"\"attributes\" below for why that variant count means attributes is a free-form JSON document " +
			"rather than a typed nested block.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Numeric identifier of the ACME DNS authenticator.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Human-readable name for the DNS authenticator. Updatable in place.",
			},
			"attributes": schema.StringAttribute{
				Required:   true,
				Sensitive:  true,
				CustomType: normalize.JSONSubsetType{},
				PlanModifiers: []planmodifier.String{
					keepImportedIfEquivalent{},
				},
				Description: "JSON document of DNS provider credentials/config. Must include \"authenticator\" " +
					"(one of: cloudflare, digitalocean, OVH, route53, shell) plus that variant's own fields — " +
					"e.g. {\"authenticator\":\"route53\",\"access_key_id\":\"...\",\"secret_access_key\":\"...\"}. " +
					"Marked Sensitive: probed live, credentials are NOT masked on read-back (create/get_instance/" +
					"query all return them in cleartext), so — unlike a WriteOnly attribute — this value IS " +
					"stored in state and IS read back on refresh/import. acme.dns.authenticator.create does not " +
					"validate credentials against the actual DNS provider (probed live: a syntactically valid " +
					"but fake cloudflare api_token was accepted without any outbound call failing); the \"shell\" " +
					"variant is the one exception, performing a local check that \"script\" is an existing file " +
					"under a pool mount point. Updatable in place (a full replace, not a partial patch — " +
					"acme.dns.authenticator.update takes the same {name, attributes} shape as create). Generated configuration (`terraform plan -generate-config-out`) has this as null, because Terraform never writes sensitive or write-only values there; fill it in before applying.",
			},
			"attributes_secrets_wo": schema.StringAttribute{
				Optional:  true,
				WriteOnly: true,
				Sensitive: true,
				Description: "Write-only JSON object of secret attributes (e.g. the DNS provider API " +
					"token) merged over attributes when sending to TrueNAS. Never stored in state, and not " +
					"read back on refresh. Requires attributes_secrets_wo_version.",
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.MatchRoot("attributes_secrets_wo_version")),
				},
			},
			"attributes_secrets_wo_version": schema.Int64Attribute{
				Optional: true,
				Description: "Version trigger for attributes_secrets_wo. Bump to re-send a changed " +
					"write-only secret overlay. The overlay is sent on create and when this integer changes; " +
					"other updates send the secret values TrueNAS already has (the API needs the whole object), so a " +
					"changed secret is applied only with a version bump. Required when attributes_secrets_wo is set.",
				Validators: []validator.Int64{
					int64validator.AlsoRequires(path.MatchRoot("attributes_secrets_wo")),
				},
			},
		},
	}
}

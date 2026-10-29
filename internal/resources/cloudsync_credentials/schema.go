// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cloudsync_credentials

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages TrueNAS cloud sync credentials.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Numeric cloud sync credentials ID assigned by TrueNAS.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the cloud sync credentials.",
			},
			"provider_config": schema.StringAttribute{
				Required:      true,
				Sensitive:     true,
				Description:   "JSON document of provider settings. Must include \"type\" (e.g. S3, B2, GOOGLE_CLOUD_STORAGE, STORJ_IX). May contain secrets (access keys); use provider_secrets_wo to keep secret keys out of state. Compared as JSON: any spelling works, keys the server adds are not a change, and `url`/`endpoint` compare as URLs (host case, default port and a trailing slash are ignored). Generated configuration (`terraform plan -generate-config-out`) has this as null, because Terraform never writes sensitive or write-only values there; fill it in before applying.",
				PlanModifiers: []planmodifier.String{keepImportedIfEquivalent{}},
			},
			"provider_secrets_wo": schema.StringAttribute{
				Optional:  true,
				WriteOnly: true,
				Sensitive: true,
				Description: "Write-only JSON object of secret provider settings (e.g. an S3 access " +
					"key/secret) merged over provider_config when sending to TrueNAS. Never stored in " +
					"state, and not read back on refresh. Requires provider_secrets_wo_version.",
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.MatchRoot("provider_secrets_wo_version")),
				},
			},
			"provider_secrets_wo_version": schema.Int64Attribute{
				Optional: true,
				Description: "Version trigger for provider_secrets_wo. Bump to re-send a changed " +
					"write-only secret overlay. The overlay is sent on create and when this integer changes; " +
					"other updates send the secret values TrueNAS already has (the API needs the whole object), so a " +
					"changed secret is applied only with a version bump. Required when provider_secrets_wo is set.",
				Validators: []validator.Int64{
					int64validator.AlsoRequires(path.MatchRoot("provider_secrets_wo")),
				},
			},
		},
	}
}

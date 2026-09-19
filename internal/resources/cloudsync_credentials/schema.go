// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cloudsync_credentials

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
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
				Required:    true,
				Sensitive:   true,
				Description: "JSON document of provider settings. Must include \"type\" (e.g. S3, B2, GOOGLE_CLOUD_STORAGE, STORJ_IX).",
			},
		},
	}
}

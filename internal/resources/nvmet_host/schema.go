// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_host

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages an NVMe-oF host (initiator) on TrueNAS.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Numeric NVMe-oF host ID assigned by TrueNAS.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"hostnqn": schema.StringAttribute{
				Required:    true,
				Description: "NVMe Qualified Name (NQN) of the host (initiator).",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Free-form description of the host.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"dhchap_key": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				WriteOnly: true,
				Description: "DH-CHAP key used by this host to authenticate to a subsystem. " +
					"Write-only: never stored in Terraform state. Requires Terraform >= 1.11.",
			},
			"dhchap_ctrl_key": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				WriteOnly: true,
				Description: "DH-CHAP controller key used for bidirectional authentication. " +
					"Write-only: never stored in Terraform state. Requires Terraform >= 1.11.",
			},
			"dhchap_dhgroup": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "DH-CHAP Diffie-Hellman group used for bidirectional authentication.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"dhchap_hash": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "DH-CHAP hash algorithm. One of: SHA-256, SHA-384, SHA-512.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

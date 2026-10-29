// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_host

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
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
				Description: "DH-CHAP key used by this host to authenticate to a subsystem. Removing it from " +
					"config turns DH-CHAP off for the host. To rotate it, change the key and bump dhchap_key_wo_version. " +
					"Write-only: never stored in Terraform state. Requires Terraform >= 1.11.",
			},
			"dhchap_ctrl_key": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				WriteOnly: true,
				Description: "DH-CHAP controller key used for bidirectional authentication. Needs dhchap_key. " +
					"Removing it from config turns bidirectional authentication off. To rotate it, change the key and " +
					"bump dhchap_ctrl_key_wo_version. " +
					"Write-only: never stored in Terraform state. Requires Terraform >= 1.11.",
			},
			"dhchap_dhgroup": schema.StringAttribute{
				Validators:  []validator.String{stringvalidator.OneOf("", "2048-BIT", "3072-BIT", "4096-BIT", "6144-BIT", "8192-BIT")},
				Optional:    true,
				Computed:    true,
				Description: "DH-CHAP Diffie-Hellman group used for bidirectional authentication. Set to \"\" to clear it.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"dhchap_key_wo_version": schema.Int64Attribute{
				Optional: true,
				Description: "Rotation trigger for the write-only dhchap_key. A write-only value is absent from state, " +
					"so Terraform cannot detect that dhchap_key changed. Bump this integer whenever dhchap_key changes " +
					"to make Terraform re-send it. dhchap_key is sent on create, when it is added or removed, and otherwise only " +
					"when this integer changes; other updates leave the stored key alone. Optional.",
			},
			"dhchap_ctrl_key_wo_version": schema.Int64Attribute{
				Optional: true,
				Description: "Rotation trigger for the write-only dhchap_ctrl_key. A write-only value is absent from state, " +
					"so Terraform cannot detect that dhchap_ctrl_key changed. Bump this integer whenever dhchap_ctrl_key " +
					"changes to make Terraform re-send it. dhchap_ctrl_key is sent on create, when it is added or removed, and otherwise only " +
					"when this integer changes; other updates leave the stored key alone. Optional.",
			},
			"dhchap_key_set": schema.BoolAttribute{
				Computed: true,
				Description: "Whether TrueNAS has a DH-CHAP key for this host. Planned from whether dhchap_key is " +
					"set in config, so adding or removing the key (in config or outside Terraform) shows as a change.",
			},
			"dhchap_ctrl_key_set": schema.BoolAttribute{
				Computed: true,
				Description: "Whether TrueNAS has a DH-CHAP controller key for this host. Planned from whether " +
					"dhchap_ctrl_key is set in config.",
			},
			"dhchap_hash": schema.StringAttribute{
				Validators:  []validator.String{stringvalidator.OneOf("SHA-256", "SHA-384", "SHA-512")},
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

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_extent

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages an iSCSI extent on TrueNAS.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Numeric iSCSI extent ID assigned by TrueNAS.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:      true,
				Description:   "Name of the iSCSI extent.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"type": schema.StringAttribute{
				Required:    true,
				Description: "Extent type: DISK or FILE.",
			},
			"disk": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Zvol path for DISK type extents (e.g. zvol/tank/myvol).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"path": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "File path for FILE type extents.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"filesize": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Size of a FILE type extent, in bytes (0 = unset; ignored for DISK extents).",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"comment": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Optional description for the extent.",
			},
			"blocksize": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Block size in bytes: 512, 1024, 2048, or 4096.",
			},
			"pblocksize": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Report physical block size to initiators.",
			},
			"avail_threshold": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Availability threshold percentage (0 = unset).",
			},
			"insecure_tpc": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Allow target-to-target XCOPY (insecure third-party copy).",
			},
			"xen": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Enable Xen initiator compatibility mode.",
			},
			"ro": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Export extent as read-only.",
			},
			"rpm": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Reported disk RPM: SSD, 5400, 7200, 10000, or 15000.",
			},
			"enabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Enable this extent.",
			},
			// Computed-only (server-generated)
			"naa": schema.StringAttribute{
				Computed:    true,
				Description: "Network Address Authority identifier assigned by TrueNAS.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"serial": schema.StringAttribute{
				Computed:    true,
				Description: "Serial number assigned by TrueNAS.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"product_id": schema.StringAttribute{
				Computed:    true,
				Description: "Product ID string reported to initiators.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"vendor": schema.StringAttribute{
				Computed:    true,
				Description: "Vendor string reported to initiators.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"locked": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the extent is currently locked.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

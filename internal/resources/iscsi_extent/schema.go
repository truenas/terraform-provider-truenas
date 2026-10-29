// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_extent

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
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
				Required: true,
				Description: "Name of the iSCSI extent. Renamed in place: the extent keeps its serial, NAA and " +
					"target mappings.",
			},
			"type": schema.StringAttribute{
				Validators:  []validator.String{stringvalidator.OneOf("DISK", "FILE")},
				Required:    true,
				Description: "Extent type: DISK or FILE.",
			},
			"disk": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Zvol path for DISK type extents (e.g. zvol/tank/myvol). Required for DISK, " +
					"must be unset for FILE (TrueNAS reports \"\" for FILE extents).",
				PlanModifiers: []planmodifier.String{
					diskFollowsType{},
				},
			},
			"path": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "File path for FILE type extents. Required for FILE. For DISK extents TrueNAS " +
					"reports the zvol path here (the same value as disk), so leave it unset.",
				PlanModifiers: []planmodifier.String{
					pathFollowsDisk{},
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
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"blocksize": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Block size in bytes: 512, 1024, 2048, or 4096.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"pblocksize": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Report physical block size to initiators.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"avail_threshold": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Availability threshold percentage (0 = unset). Left out of config, the server value is kept.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"insecure_tpc": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Allow target-to-target XCOPY (insecure third-party copy).",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"xen": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Enable Xen initiator compatibility mode.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"ro": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Description: "Export extent as read-only. TrueNAS also sets the backing zvol's ZFS readonly " +
					"property to match (on for true, off for false) on every create and update of a DISK extent, " +
					"so do not manage truenas_zvol.readonly for that zvol as well. When an extent with ro = true " +
					"is destroyed, the provider resets the zvol's readonly property to inherit, since TrueNAS " +
					"would otherwise leave the zvol read-only.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"rpm": schema.StringAttribute{
				Validators:  []validator.String{stringvalidator.OneOf("10000", "15000", "5400", "7200", "SSD", "UNKNOWN")},
				Optional:    true,
				Computed:    true,
				Description: "Reported disk RPM: SSD, 5400, 7200, 10000, or 15000.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"enabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Enable this extent.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
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

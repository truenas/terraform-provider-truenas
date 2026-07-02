package smb

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages an SMB share on TrueNAS SCALE.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Numeric SMB share ID assigned by TrueNAS.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"path": schema.StringAttribute{
				Required:    true,
				Description: "Absolute path to the directory being shared (e.g. /mnt/tank/share).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name":    schema.StringAttribute{Required: true, Description: "NetBIOS share name."},
			"comment": schema.StringAttribute{Optional: true, Computed: true, Description: "Optional share description."},
			"ro": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Export share as read-only.",
			},
			"browsable": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Allow share to appear in Windows network browsing.",
			},
			"recyclebin": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Enable Windows Recycle Bin behaviour.",
			},
			"guestok": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Allow unauthenticated (guest) access.",
			},
			"hostsallow": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "List of hosts/IPs allowed to connect.",
			},
			"hostsdeny": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "List of hosts/IPs denied from connecting.",
			},
			"abe": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Enable Access-Based Enumeration.",
			},
			"acl": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Enable ACL support on this share.",
			},
			"durablehandle": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Enable SMB2 durable handles.",
			},
			"streams": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Enable alternate data streams.",
			},
			"timemachine": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Advertise share as a Time Machine target.",
			},
			"timemachine_quota": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Per-machine Time Machine quota in GiB (0 = unlimited).",
			},
			"enabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Enable this share.",
			},
			"home": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Use this share as user home-directory share.",
			},
			"purpose": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Purpose preset: NO_PRESET, DEFAULT_SHARE, ENHANCED_TIMEMACHINE, MULTI_PROTOCOL_AFP, MULTI_PROTOCOL_NFS, PRIVATE_DATASETS, WORM_DROPBOX.",
			},
			// Computed-only (server-generated)
			"vuid": schema.StringAttribute{
				Computed:    true,
				Description: "Vendor unique identifier assigned by TrueNAS.",
			},
			"locked": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the share path is currently locked.",
			},
		},
	}
}

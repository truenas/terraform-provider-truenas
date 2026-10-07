// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package smb

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages an SMB share on TrueNAS.",
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
				Description: "Export share as read-only. Sent to TrueNAS 26.0 as the top-level `readonly` field (renamed from `ro` on the wire; the Terraform attribute name is unchanged for backward compatibility).",
			},
			"browsable": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Allow share to appear in Windows network browsing. Sent as the top-level `browsable` field.",
			},
			"recyclebin": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Enable Windows Recycle Bin behaviour. On TrueNAS 26.0 this only applies when the share's `purpose` is (or defaults to) LEGACY_SHARE; it is sent nested under `options` and is not sent (or read back) for any other purpose.",
			},
			"guestok": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Allow unauthenticated (guest) access. LEGACY_SHARE-only on TrueNAS 26.0; see `recyclebin` for details on the options mapping.",
			},
			"hostsallow": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "List of hosts/IPs allowed to connect. LEGACY_SHARE-only on TrueNAS 26.0; see `recyclebin` for details on the options mapping.",
			},
			"hostsdeny": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "List of hosts/IPs denied from connecting. LEGACY_SHARE-only on TrueNAS 26.0; see `recyclebin` for details on the options mapping.",
			},
			"abe": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Enable Access-Based Enumeration. Sent to TrueNAS 26.0 as the top-level `access_based_share_enumeration` field (renamed from `abe` on the wire; the Terraform attribute name is unchanged for backward compatibility).",
			},
			"acl": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Enable ACL support on this share. LEGACY_SHARE-only on TrueNAS 26.0; see `recyclebin` for details on the options mapping.",
			},
			"durablehandle": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Enable SMB2 durable handles. LEGACY_SHARE-only on TrueNAS 26.0; see `recyclebin` for details on the options mapping.",
			},
			"streams": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Enable alternate data streams. LEGACY_SHARE-only on TrueNAS 26.0; see `recyclebin` for details on the options mapping.",
			},
			"timemachine": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Advertise share as a Time Machine target. LEGACY_SHARE-only on TrueNAS 26.0; see `recyclebin` for details on the options mapping.",
			},
			"timemachine_quota": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Per-machine Time Machine quota in GiB (0 = unlimited). LEGACY_SHARE-only on TrueNAS 26.0; see `recyclebin` for details on the options mapping.",
			},
			"enabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Enable this share.",
			},
			"home": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Use this share as user home-directory share. LEGACY_SHARE-only on TrueNAS 26.0; see `recyclebin` for details on the options mapping.",
			},
			"purpose": schema.StringAttribute{
				Validators:  []validator.String{stringvalidator.OneOf("DEFAULT_SHARE", "EXTERNAL_SHARE", "FCP_SHARE", "LEGACY_SHARE", "MULTIPROTOCOL_SHARE", "PRIVATE_DATASETS_SHARE", "TIMEMACHINE_SHARE", "TIME_LOCKED_SHARE", "VEEAM_REPOSITORY_SHARE")},
				Optional:    true,
				Computed:    true,
				Description: "Purpose preset. One of: DEFAULT_SHARE, LEGACY_SHARE, TIMEMACHINE_SHARE, MULTIPROTOCOL_SHARE, TIME_LOCKED_SHARE, PRIVATE_DATASETS_SHARE, EXTERNAL_SHARE, VEEAM_REPOSITORY_SHARE, FCP_SHARE. On TrueNAS 26.0 this drives a discriminated `options` object on the wire. When left unset (or set to an unrecognized value), the provider defaults to LEGACY_SHARE so this resource's flat legacy attributes (recyclebin, hostsallow, hostsdeny, guestok, streams, durablehandle, home, acl, timemachine, timemachine_quota) continue to work as before. Setting purpose to any other enum value switches the share to that purpose's variant defaults server-side and stops sending the legacy attributes.",
			},
			"options": schema.SingleNestedAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Purpose-specific SMB options (the discriminated `options` object on TrueNAS 26.0+). Only the fields valid for the share's `purpose` are sent; the rest read back null. This is the recommended surface for all purposes — for LEGACY_SHARE the flat top-level attributes still work and are used as a fallback when the matching option here is unset.",
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
				Attributes: map[string]schema.Attribute{
					"recyclebin":            schema.BoolAttribute{Optional: true, Computed: true, Description: "LEGACY_SHARE: move deleted files to a .recycle directory."},
					"path_suffix":           schema.StringAttribute{Optional: true, Computed: true, Description: "LEGACY_SHARE: path suffix template (e.g. %D/%U)."},
					"hostsallow":            schema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType, Description: "Hosts/subnets allowed to connect. Valid for all purposes except EXTERNAL_SHARE."},
					"hostsdeny":             schema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType, Description: "Hosts/subnets denied. Valid for all purposes except EXTERNAL_SHARE."},
					"guestok":               schema.BoolAttribute{Optional: true, Computed: true, Description: "LEGACY_SHARE: allow guest access."},
					"streams":               schema.BoolAttribute{Optional: true, Computed: true, Description: "LEGACY_SHARE: enable alternate data streams."},
					"durablehandle":         schema.BoolAttribute{Optional: true, Computed: true, Description: "LEGACY_SHARE: enable SMB2 durable handles."},
					"shadowcopy":            schema.BoolAttribute{Optional: true, Computed: true, Description: "LEGACY_SHARE: expose ZFS snapshots as VSS shadow copies."},
					"fsrvp":                 schema.BoolAttribute{Optional: true, Computed: true, Description: "LEGACY_SHARE: enable File Server Remote VSS Protocol."},
					"home":                  schema.BoolAttribute{Optional: true, Computed: true, Description: "LEGACY_SHARE: use as a home-directory share."},
					"acl":                   schema.BoolAttribute{Optional: true, Computed: true, Description: "LEGACY_SHARE: enable SMB ACL support."},
					"afp":                   schema.BoolAttribute{Optional: true, Computed: true, Description: "LEGACY_SHARE: share was migrated from AFP (compatibility)."},
					"timemachine":           schema.BoolAttribute{Optional: true, Computed: true, Description: "LEGACY_SHARE: advertise as a Time Machine target."},
					"timemachine_quota":     schema.Int64Attribute{Optional: true, Computed: true, Description: "LEGACY_SHARE/TIMEMACHINE_SHARE: per-machine Time Machine quota (0 = unlimited)."},
					"aapl_name_mangling":    schema.BoolAttribute{Optional: true, Computed: true, Description: "macOS-style name mangling. Valid for DEFAULT/LEGACY/MULTIPROTOCOL/TIME_LOCKED/PRIVATE_DATASETS/FCP."},
					"vuid":                  schema.StringAttribute{Optional: true, Computed: true, Description: "Vendor unique identifier (LEGACY_SHARE/TIMEMACHINE_SHARE); server-generated."},
					"auxsmbconf":            schema.StringAttribute{Optional: true, Computed: true, Description: "LEGACY_SHARE: raw auxiliary smb.conf parameters."},
					"auto_snapshot":         schema.BoolAttribute{Optional: true, Computed: true, Description: "TIMEMACHINE_SHARE: take a ZFS snapshot on each backup."},
					"auto_dataset_creation": schema.BoolAttribute{Optional: true, Computed: true, Description: "TIMEMACHINE_SHARE: create a per-connecting-client ZFS sub-dataset (replaces Multi-user Time Machine)."},
					"dataset_naming_schema": schema.StringAttribute{Optional: true, Computed: true, Description: "TIMEMACHINE_SHARE/PRIVATE_DATASETS_SHARE: naming schema for auto-created datasets."},
					"grace_period":          schema.Int64Attribute{Optional: true, Computed: true, Description: "TIME_LOCKED_SHARE: grace period in seconds before locking."},
					"auto_quota":            schema.Int64Attribute{Optional: true, Computed: true, Description: "PRIVATE_DATASETS_SHARE: per-dataset auto quota (GiB; 0 = unlimited)."},
					"remote_path":           schema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType, Description: "EXTERNAL_SHARE: remote DFS target path(s)."},
				},
			},
			"audit": schema.SingleNestedAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Per-share audit logging configuration.",
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
				Attributes: map[string]schema.Attribute{
					"enable": schema.BoolAttribute{
						Optional:    true,
						Computed:    true,
						Description: "Enable auditing for this share. Cannot be enabled if the SMB service minimum_protocol is SMB1.",
					},
					"watch_list": schema.ListAttribute{
						Optional:    true,
						Computed:    true,
						ElementType: types.StringType,
						Description: "Group names to audit. Empty means audit all groups.",
					},
					"ignore_list": schema.ListAttribute{
						Optional:    true,
						Computed:    true,
						ElementType: types.StringType,
						Description: "Group names to exclude from auditing.",
					},
				},
			},
			// Computed-only (server-generated)
			"vuid": schema.StringAttribute{
				Computed:    true,
				Description: "Vendor unique identifier assigned by TrueNAS. On TrueNAS 26.0 this is only populated (and only meaningful) when `purpose` is LEGACY_SHARE, where it lives nested under `options.vuid` on the wire.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"locked": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the share path is currently locked.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

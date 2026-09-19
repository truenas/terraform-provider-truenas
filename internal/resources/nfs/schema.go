// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nfs

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages an NFS share on TrueNAS.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"path": schema.StringAttribute{
				Required:    true,
				Description: "Absolute path to the directory being shared.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"comment": schema.StringAttribute{Optional: true, Computed: true},
			"enabled": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
			},
			"ro": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Export as read-only.",
			},
			"networks": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				Description: "List of allowed networks in CIDR notation.",
				ElementType: types.StringType,
			},
			"hosts": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				Description: "List of allowed hosts (IP or hostname).",
				ElementType: types.StringType,
			},
			"maproot_user": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Map root user to this user.",
			},
			"maproot_group": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Map root group to this group. Mutually exclusive with mapall_group.",
			},
			"mapall_user": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Map all client users to this user. Mutually exclusive with maproot_user.",
			},
			"mapall_group": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Map all client groups to this group. Mutually exclusive with maproot_group.",
			},
			"security": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "NFS security flavors for the export, in order of preference: SYS, KRB5, KRB5I, KRB5P.",
				Validators: []validator.List{
					listvalidator.ValueStringsAre(stringvalidator.OneOf("SYS", "KRB5", "KRB5I", "KRB5P")),
				},
			},
			"expose_snapshots": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Description: "Enterprise feature: expose the ZFS snapshot directory for the export. The export " +
					"path must be the root directory of a ZFS dataset.",
			},
		},
	}
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nfs

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/normalize"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages an NFS share on TrueNAS. Every optional attribute has the server's default " +
			"when omitted, so removing an attribute from the configuration resets it on the share.",
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
			"comment": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
				Description: "Share comment. Removing it from the configuration clears it.",
			},
			"enabled": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
			},
			"ro": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Export as read-only. Defaults to `false`.",
			},
			"networks": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				Description: "List of allowed networks in CIDR notation. TrueNAS stores the network address (host bits cleared) and a bare address as /32 (/128), so 192.168.1.10/24 equals 192.168.1.0/24 and 10.1.2.3 equals 10.1.2.3/32. Empty or omitted allows every network.",
				ElementType: normalize.NetworkType{},
				Default:     listdefault.StaticValue(types.ListValueMust(normalize.NetworkType{}, []attr.Value{})),
				PlanModifiers: []planmodifier.List{
					normalize.KeepStateIfEquivalentList(types.ListType{ElemType: normalize.NetworkType{}}),
				},
			},
			"hosts": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				Description: "List of allowed hosts (IP or hostname). Empty or omitted allows every host.",
				ElementType: types.StringType,
				Default:     listdefault.StaticValue(types.ListValueMust(types.StringType, []attr.Value{})),
			},
			"maproot_user": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
				Description: "Map root user to this user.",
			},
			"maproot_group": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
				Description: "Map root group to this group. Mutually exclusive with mapall_group.",
			},
			"mapall_user": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
				Description: "Map all client users to this user. Mutually exclusive with maproot_user.",
			},
			"mapall_group": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
				Description: "Map all client groups to this group. Mutually exclusive with maproot_group.",
			},
			"security": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Default:     listdefault.StaticValue(types.ListValueMust(types.StringType, []attr.Value{})),
				Description: "NFS security flavors for the export, in order of preference: SYS, KRB5, KRB5I, KRB5P. Empty or omitted uses the server's default flavors.",
				Validators: []validator.List{
					listvalidator.ValueStringsAre(stringvalidator.OneOf("SYS", "KRB5", "KRB5I", "KRB5P")),
				},
			},
			"expose_snapshots": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
				Description: "Enterprise feature: expose the ZFS snapshot directory for the export. The export " +
					"path must be the root directory of a ZFS dataset. Defaults to `false`.",
			},
		},
	}
}

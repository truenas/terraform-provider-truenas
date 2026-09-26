// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset

import (
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// inheritDescription ends the description of every source-aware property.
const inheritDescription = " Set to INHERIT (case-insensitive) to state explicitly that the " +
	"property is inherited from the parent dataset. When the property is not set on this " +
	"dataset itself (inherited, default or received), state holds INHERIT. Omitting " +
	"the attribute on create leaves the property inherited; removing it from the " +
	"configuration later keeps the last applied value rather than reverting to " +
	"inherited."

// specialSmallBlockSizeValidators admit what special_small_block_size, a
// string so that it can hold INHERIT, may be set to. A number in HCL, e.g.
// special_small_block_size = 16384, converts to "16384" and passes.
var specialSmallBlockSizeValidators = []validator.String{
	stringvalidator.RegexMatches(regexp.MustCompile(`^(?i:INHERIT)$|^[0-9]+$`),
		"must be a non-negative decimal integer or INHERIT"),
}

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages a ZFS dataset (filesystem or volume) on TrueNAS.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Dataset name (used as Terraform ID).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Full dataset path, e.g. tank/mydata.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"type": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Dataset type: FILESYSTEM (default) or VOLUME. Case-insensitive.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"compression": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Compression algorithm. Case-insensitive: lz4, zstd, off, etc.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"acltype": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "ACL type: posix, nfsv4, or off. Case-insensitive.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"share_type": schema.StringAttribute{
				Optional:    true,
				Description: "Optimised share type: UNIX or WINDOWS (write-only, not returned by API).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"comments": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Human-readable description stored as org.freenas:description.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"quota": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Quota in bytes (0 = unlimited).",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"refquota": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Referenced quota in bytes (0 = unlimited).",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"reservation": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Reserved space in bytes.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"volsize": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Volume size in bytes. Required for type=VOLUME.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"special_small_block_size": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Threshold in bytes below which blocks are written to a " +
					"pool's special allocation class vdev (ZFS special_small_blocks), " +
					"as a decimal integer, or INHERIT. 0 disables the behaviour. Must be " +
					"0 or a power of two no larger than the dataset's record size. A " +
					"number in the configuration (special_small_block_size = 16384) is " +
					"accepted and stored as the string \"16384\"." + inheritDescription +
					" A size set on the dataset itself cannot be changed to INHERIT: " +
					"the plan fails.",
				Validators: specialSmallBlockSizeValidators,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					keepLocalSpecialSmallBlockSize{},
				},
			},
			"mountpoint": schema.StringAttribute{
				Computed:    true,
				Description: "Dataset mountpoint path.",
			},
			"encrypted": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the dataset is encrypted.",
			},
			"pool": schema.StringAttribute{
				Computed:    true,
				Description: "Name of the pool containing this dataset.",
			},
		},
	}
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package filesystem_permissions

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Declaratively sets UNIX mode/owner/group on an existing filesystem path via " +
			"filesystem.setperm (probed live: job:true). This is a wrapper around an imperative action, not " +
			"an object TrueNAS itself tracks as a resource — every path on the box already has SOME mode/uid/" +
			"gid, whether or not Terraform manages it. Consequently `terraform destroy` does NOT revert or " +
			"reset anything: it only removes the resource from Terraform state (with a warning diagnostic) " +
			"and leaves the path's permissions exactly as last applied. Read drift-checks mode/uid/gid " +
			"against filesystem.stat on every refresh, so out-of-band changes (chmod/chown, another Terraform " +
			"config, the UI) are detected as a plan diff like any other attribute.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Same as \"path\" — this resource is keyed by filesystem path, not a TrueNAS-assigned id.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"path": schema.StringAttribute{
				Required:    true,
				Description: "Absolute filesystem path (e.g. \"/mnt/tank/mydata\") to set permissions on. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"mode": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "UNIX permission bits in octal string form (e.g. \"0750\"), as accepted by " +
					"filesystem.setperm's \"mode\" parameter (probed live: a string, not an int). Read back " +
					"from filesystem.stat's \"mode\" field, which is the full stat(2) st_mode (file-type bits " +
					"included, e.g. a directory reads as decimal 16872 for 0750) — this provider masks it to " +
					"the low 12 bits and re-encodes as a 4-digit octal string before storing it, so state " +
					"always reflects just the permission bits. Omit to leave the path's existing mode " +
					"unchanged (filesystem.setperm treats a null mode as \"leave unchanged\").",
				Validators: []validator.String{stringvalidator.RegexMatches(
					modeRegexp(), "must be 3 or 4 octal digits, e.g. \"0750\" or \"750\"",
				)},
			},
			"uid": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Description: "Numeric user ID to set as owner. Read back from filesystem.stat's \"uid\" " +
					"field. Omit to leave the path's existing owner unchanged.",
				Validators: []validator.Int64{int64validator.Between(-1, 2147483647)},
			},
			"gid": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Description: "Numeric group ID to set as group owner. Read back from filesystem.stat's " +
					"\"gid\" field. Omit to leave the path's existing group owner unchanged.",
				Validators: []validator.Int64{int64validator.Between(-1, 2147483647)},
			},
			"recursive": schema.BoolAttribute{
				Optional: true,
				Description: "Apply mode/uid/gid recursively to everything under \"path\" (filesystem." +
					"setperm's \"options.recursive\"), limited to the same dataset. This is an apply-time " +
					"instruction, not a queryable property of the path — TrueNAS has nothing to read it back " +
					"from, so it is not Computed, is never echoed into state, and must be ignored on import " +
					"(ImportStateVerifyIgnore). Defaults to false.",
			},
			"traverse": schema.BoolAttribute{
				Optional: true,
				Description: "When \"recursive\" is set, also cross into child dataset boundaries " +
					"(filesystem.setperm's \"options.traverse\"). Same apply-time-only caveats as " +
					"\"recursive\" apply here. Defaults to false.",
			},
		},
	}
}

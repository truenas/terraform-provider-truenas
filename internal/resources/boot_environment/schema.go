// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package boot_environment

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages a TrueNAS boot environment. Boot environments are always created " +
			"by cloning an existing one; there is no plain create.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Description:   "Boot environment name (used as Terraform ID).",
			},
			"name": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "Name of the new boot environment (clone target).",
			},
			"source": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "Name of the existing boot environment to clone from.",
			},
			"activated": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
				Description: "Whether this boot environment is activated (will be booted next). " +
					"Setting this to true activates the boot environment. Setting it back to false " +
					"is not supported by the API; activate a different boot environment instead.",
			},
			"keep": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
				Description:   "Whether this boot environment is protected from automatic pruning.",
			},
			"dataset": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Description:   "ZFS dataset backing this boot environment.",
			},
			// active and used_bytes are server-mutable (they change whenever
			// any boot environment is activated, or when the pool's usage
			// shifts) and therefore intentionally carry no
			// UseStateForUnknown plan modifier: Terraform must always accept
			// the freshly read server value rather than preserving stale
			// prior state.
			"active": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether this boot environment is currently booted.",
			},
			"used_bytes": schema.Int64Attribute{
				Computed:    true,
				Description: "Space used by this boot environment, in bytes.",
			},
		},
	}
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package catalog_config

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages TrueNAS's app catalog preferences (catalog.config): which trains " +
			"(e.g. \"stable\", \"community\", \"enterprise\") are preferred when browsing and installing " +
			"applications. This is a singleton resource — there is exactly one app catalog per TrueNAS system, " +
			"so it is never created or deleted on TrueNAS; Terraform create/update calls catalog.update " +
			"(probed job:false on both TrueNAS 25.10 and 26.0 — a plain synchronous call, unlike docker.update), " +
			"and Terraform delete only removes the resource from state (the catalog configuration is left in " +
			"place)." +
			"\n\n" +
			"\"label\" and \"location\" are read-only: catalog.update's accepts schema is {preferred_trains} " +
			"only (probed live, identical on both releases), so this resource can display but never change them.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Fixed identifier for this singleton resource: always \"catalog_config\".",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"label": schema.StringAttribute{
				Computed: true,
				Description: "Read-only: the catalog's identifier/label (e.g. \"TRUENAS\"). Not settable via " +
					"catalog.update.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"location": schema.StringAttribute{
				Computed: true,
				Description: "Read-only: the git repository URL or local filesystem path backing the catalog. " +
					"Not settable via catalog.update.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"preferred_trains": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "Train names (e.g. \"stable\", \"community\", \"enterprise\") preferred when " +
					"browsing and installing applications from this catalog.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

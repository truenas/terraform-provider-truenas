// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package app

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages an application (Docker-based) on TrueNAS 24.10+.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Description:   "App name (used as Terraform ID).",
			},
			"name": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "Application name (unique per system).",
			},
			"catalog_app": schema.StringAttribute{
				Optional:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "Catalog app to install (e.g. \"plex\"). Omit for custom apps.",
			},
			"train": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()},
				Description:   "Catalog train: stable, community, enterprise.",
			},
			"version": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Description:   "App version to install (default \"latest\").",
			},
			"values": schema.StringAttribute{
				Optional:    true,
				Description: "JSON document of app configuration values (write-only; not read back).",
			},
			"custom_app": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.RequiresReplace(), boolplanmodifier.UseStateForUnknown()},
				Description:   "True for custom (compose-based) apps.",
			},
			"custom_compose_config_string": schema.StringAttribute{
				Optional:    true,
				Description: "Docker compose YAML for custom apps (write-only; not read back).",
			},
			"running": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
				Description:   "Whether the app should be running. Set false to stop.",
			},
			"state": schema.StringAttribute{
				Computed:    true,
				Description: "Current app state (RUNNING, STOPPED, DEPLOYING, ...).",
			},
			"human_version":     schema.StringAttribute{Computed: true},
			"upgrade_available": schema.BoolAttribute{Computed: true},
		},
	}
}

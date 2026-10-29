// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package app

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
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
				Description: "App version to install (default \"latest\"). Changing it to a specific version " +
					"upgrades the app. \"latest\" installs the newest catalog version at create time and is kept " +
					"as written in state (it does not upgrade the app later; the installed version is in " +
					"human_version).",
			},
			"values": schema.StringAttribute{
				Optional: true,
				Description: "JSON document of app configuration values. On read it is reconciled " +
					"from the live app config, projected onto the keys you set, so configuration drift " +
					"in those keys (e.g. a change made in the UI) is detected. Chart defaults you did not " +
					"set and server-managed ix_* keys are not reported as drift.",
			},
			"custom_app": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.RequiresReplace(), boolplanmodifier.UseStateForUnknown()},
				Description:   "True for custom (compose-based) apps.",
			},
			"custom_compose_config_string": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				Description: "Docker Compose YAML for custom apps. On read it is reconciled from the " +
					"live app configuration, so drift (an edit made in the UI or via the API) is detected; " +
					"formatting, comments, and key order are compared semantically and are not reported as drift. " +
					"Marked sensitive: the Compose document may contain secrets (environment values), so it is not " +
					"shown in plan or state output.",
			},
			"custom_compose_config_string_wo": schema.StringAttribute{
				Optional:  true,
				WriteOnly: true,
				Sensitive: true,
				Description: "Write-only overlay (JSON or YAML) deep-merged into custom_compose_config_string " +
					"when sending, for secret parts of the Compose (e.g. nested service environment values). " +
					"Read from configuration and never stored in state; on refresh the live Compose is projected " +
					"onto only the keys in custom_compose_config_string, so these secret keys are not read back. " +
					"Requires custom_compose_config_string_wo_version.",
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.MatchRoot("custom_compose_config_string_wo_version")),
				},
			},
			"custom_compose_config_string_wo_version": schema.Int64Attribute{
				Optional: true,
				Description: "Version trigger for custom_compose_config_string_wo; bump to re-send a rotated " +
					"overlay (write-only values are absent from state so a change cannot be detected otherwise). " +
					"Required when custom_compose_config_string_wo is set.",
				Validators: []validator.Int64{
					int64validator.AlsoRequires(path.MatchRoot("custom_compose_config_string_wo")),
				},
			},
			"running": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
				Description:   "Whether the app should be running. Set false to stop.",
			},
			"state": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{keepSettledState{}},
				Description:   "Current app state (RUNNING, STOPPED, DEPLOYING, ...).",
			},
			"human_version": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{keepVersionDerivedString{}},
				Description:   "Installed app version as shown in the UI (app version and catalog version).",
			},
			"upgrade_available": schema.BoolAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.Bool{keepVersionDerivedBool{}},
				Description:   "Whether a newer catalog version is available.",
			},
		},
	}
}

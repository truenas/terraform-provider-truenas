// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package snmp_config

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/truenas/terraform-provider-truenas/internal/normalize"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages the TrueNAS SNMP service configuration. This is a singleton resource — " +
			"there is exactly one SNMP configuration per TrueNAS system, so it is never created or deleted on " +
			"TrueNAS; Terraform create/update calls snmp.update, and Terraform delete only removes the resource " +
			"from state (the configuration is left in place). v3_password and v3_privpassphrase are write-only: " +
			"they are never read back from TrueNAS and are not stored in state.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Fixed identifier for this singleton resource: always \"snmp_config\".",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"community": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Sensitive:     true,
				Description:   "SNMP community string. Stored in state (read back unmasked from TrueNAS). Use community_wo to keep it out of state.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Validators:    []validator.String{stringvalidator.ConflictsWith(path.MatchRoot("community_wo"))},
			},
			"community_wo": schema.StringAttribute{
				Optional:  true,
				WriteOnly: true,
				Sensitive: true,
				Description: "Write-only alternative to community: the SNMP community string read from " +
					"configuration and never stored in state, and not read back on refresh. Requires " +
					"community_wo_version; conflicts with community.",
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.MatchRoot("community_wo_version")),
				},
			},
			"community_wo_version": schema.Int64Attribute{
				Optional: true,
				Description: "Version trigger for community_wo. Bump this integer to re-send a changed " +
					"write-only community string (a write-only value is absent from state, so its rotation " +
					"cannot be detected automatically). community_wo is sent on create and, after that, only " +
					"when this integer changes; other updates leave the stored community alone. Required when " +
					"community_wo is set.",
				Validators: []validator.Int64{
					int64validator.AlsoRequires(path.MatchRoot("community_wo")),
				},
			},
			"contact": schema.StringAttribute{
				Optional:   true,
				Computed:   true,
				CustomType: normalize.EmailType{},
				Description: "Contact information for the SNMP administrator (an e-mail address on TrueNAS 28.0, " +
					"which stores its domain in lower case; that compares equal to the configured spelling).",
				PlanModifiers: []planmodifier.String{
					normalize.KeepStateIfEquivalent(normalize.EmailType{}),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"location": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Physical location of the system, exposed via SNMP.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"loglevel": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Description:   "SNMP daemon syslog level.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"options": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Extra snmpd.conf options, appended verbatim.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"traps": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether SNMP traps are enabled.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"zilstat": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether zilstat reporting is enabled.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"v3": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether SNMPv3 support is enabled.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"v3_username": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "SNMPv3 username.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"v3_authtype": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "SNMPv3 authentication type: one of \"\" (none), MD5, SHA.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"v3_password": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				WriteOnly: true,
				Description: "SNMPv3 authentication password (never read back from TrueNAS). " +
					"Write-only: never stored in Terraform state. Requires Terraform >= 1.11.",
			},
			"v3_privproto": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "SNMPv3 privacy protocol: one of AES, DES, or null/empty for none.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"v3_privpassphrase": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				WriteOnly: true,
				Description: "SNMPv3 privacy passphrase (never read back from TrueNAS). " +
					"Write-only: never stored in Terraform state. Requires Terraform >= 1.11.",
			},
		},
	}
}

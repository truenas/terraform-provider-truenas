// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package smb_share_acl

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/truenas/terraform-provider-truenas/internal/normalize"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages the share-level Access Control List of an existing SMB share via " +
			"sharing.smb.setacl / getacl (both probed live on TrueNAS 27.0 as job:false plain calls), " +
			"keyed by share_name rather than a TrueNAS-assigned id. This is the SMB \"Share ACL\" surface " +
			"(who may connect to the share and at what level), distinct from the filesystem ACL on the " +
			"share's path (truenas_filesystem_acl) and from the share's own settings (truenas_smb_share).\n\n" +
			"Each entry names its principal by exactly one of ae_who_sid or ae_who_id. The server stores " +
			"the SID and resolves the other fields (e.g. ae_who_sid \"S-1-1-0\" reads back with ae_who_str " +
			"\"everyone@\"); all three are kept in state, so an imported share ACL plans no change whichever " +
			"selector the configuration uses.\n\n" +
			"Delete semantics: `terraform destroy` resets the share ACL to the TrueNAS default of a single " +
			"\"everyone@ FULL ALLOWED\" entry (sharing.smb.setacl's documented default), not to \"no ACL\" " +
			"- an SMB share always has a share ACL. A warning diagnostic is emitted since this mutates the " +
			"share outside Terraform's usual forget-state convention.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Same as \"share_name\" - this resource is keyed by SMB share name, not a numeric id.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"share_name": schema.StringAttribute{
				Required: true,
				Description: "Name of the existing SMB share (truenas_smb_share.name) whose share ACL this " +
					"resource manages. Changing it forces a new resource (it targets a different share).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"share_acl": schema.ListNestedAttribute{
				Required: true,
				Description: "Ordered list of share ACL entries. Order is significant (DENIED entries are " +
					"typically listed before ALLOWED ones). At least one entry is required.",
				Validators: []validator.List{listvalidator.SizeAtLeast(1)},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"ae_perm": schema.StringAttribute{
							Required: true,
							Description: "Permission level: FULL (read, write, execute, delete, write-ACL, " +
								"change-owner), CHANGE (read, write, execute, delete), or READ (read, execute). " +
								"The API also reports CUSTOM when a share ACL was edited outside TrueNAS's " +
								"supported means; CUSTOM cannot be set (sharing.smb.setacl rejects it, verified " +
								"live), so it is not an accepted value here - if a share drifts to CUSTOM, reset " +
								"the entry to FULL, CHANGE, or READ.",
							Validators: []validator.String{
								stringvalidator.OneOf("FULL", "CHANGE", "READ"),
							},
						},
						"ae_type": schema.StringAttribute{
							Required:    true,
							Description: "Whether this entry grants (ALLOWED) or denies (DENIED) the permission.",
							Validators: []validator.String{
								stringvalidator.OneOf("ALLOWED", "DENIED"),
							},
						},
						"ae_who_sid": schema.StringAttribute{
							Optional: true,
							Computed: true,
							Description: "Principal by SID (e.g. \"S-1-1-0\" for everyone). Specify exactly one " +
								"of ae_who_sid / ae_who_id per entry. When the entry uses ae_who_id, this is the " +
								"SID the server resolved. Compared case-insensitively (the server stores SIDs upper-case).",
							CustomType: normalize.CaseInsensitiveType{},
							PlanModifiers: []planmodifier.String{
								normalize.KeepStateIfEquivalent(normalize.CaseInsensitiveType{}),
							},
						},
						"ae_who_id": schema.SingleNestedAttribute{
							Optional: true,
							Computed: true,
							Description: "Principal by Unix ID. Specify exactly one of ae_who_sid / ae_who_id per " +
								"entry. The user or group must be an SMB account. When the entry uses ae_who_sid, " +
								"this is the Unix ID the SID maps to (null if it maps to none, e.g. everyone).",
							Attributes: map[string]schema.Attribute{
								"id_type": schema.StringAttribute{
									Required:    true,
									Description: "USER (id is a UID) or GROUP (id is a GID).",
									Validators: []validator.String{
										stringvalidator.OneOf("USER", "GROUP"),
									},
								},
								"id": schema.Int64Attribute{
									Required:    true,
									Description: "Numeric Unix UID or GID, per id_type.",
								},
							},
						},
						"ae_who_str": schema.StringAttribute{
							Computed: true,
							Description: "Principal user or group name the server resolved (e.g. \"everyone@\"). " +
								"Read-only: sharing.smb.setacl cannot resolve a name, so use ae_who_sid or ae_who_id.",
						},
					},
				},
			},
		},
	}
}

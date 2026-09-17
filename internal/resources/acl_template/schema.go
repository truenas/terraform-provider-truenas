// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acl_template

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages a filesystem ACL template (filesystem.acltemplate.*) on TrueNAS: a " +
			"reusable, named set of NFS4 or POSIX1E access control entries that can be applied to a path " +
			"(e.g. from the UI's \"Manage ACL\" workflow, or as a starting point for truenas_filesystem_acl). " +
			"TrueNAS ships 9 builtin templates (NFS4_OPEN, NFS4_RESTRICTED, NFS4_HOME, NFS4_DOMAIN_HOME, " +
			"POSIX_OPEN, POSIX_RESTRICTED, POSIX_HOME, NFS4_ADMIN, POSIX_ADMIN, probed live) that this " +
			"resource never targets: it only ever creates/updates/deletes templates by the id it itself " +
			"created (or one explicitly imported), never touching a builtin's id unless a user deliberately " +
			"imports one.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Numeric identifier of the ACL template.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Human-readable name for the ACL template. Updatable in place.",
			},
			"acltype": schema.StringAttribute{
				Required: true,
				Description: "ACL type this template provides: \"NFS4\" (ZFS/NFSv4-style ACEs, the default " +
					"acltype on TrueNAS datasets) or \"POSIX1E\" (POSIX draft ACLs). Updatable in place " +
					"(filesystem.acltemplate.update accepts it), but changing it requires the \"acl\" entries " +
					"to match the new type's shape in the same apply.",
				Validators: []validator.String{stringvalidator.OneOf("NFS4", "POSIX1E")},
			},
			"acl": schema.StringAttribute{
				Required: true,
				Description: "JSON array of Access Control Entries, in the exact shape " +
					"filesystem.acltemplate.create/update accept (probed live via core.get_methods). For " +
					"acltype \"NFS4\", each entry is " +
					"{\"tag\": \"owner@\"|\"group@\"|\"everyone@\"|\"USER\"|\"GROUP\", \"type\": \"ALLOW\"|\"DENY\", " +
					"\"perms\": {\"BASIC\": \"FULL_CONTROL\"|\"MODIFY\"|\"READ\"|\"TRAVERSE\"} or an object of " +
					"individual boolean permission flags (READ_DATA, WRITE_DATA, ..., SYNCHRONIZE), " +
					"\"flags\": {\"BASIC\": \"INHERIT\"|\"NOINHERIT\"} or an object of individual boolean " +
					"inheritance flags (FILE_INHERIT, DIRECTORY_INHERIT, NO_PROPAGATE_INHERIT, INHERIT_ONLY, " +
					"INHERITED), plus optional \"id\" (uid/gid, required when tag is USER/GROUP) and \"who\" " +
					"(username/group name, alternative to id when tag is USER/GROUP). For acltype \"POSIX1E\", " +
					"each entry is {\"tag\": \"USER_OBJ\"|\"GROUP_OBJ\"|\"OTHER\"|\"MASK\"|\"USER\"|\"GROUP\", " +
					"\"perms\": {\"READ\": bool, \"WRITE\": bool, \"EXECUTE\": bool}, \"default\": bool (whether " +
					"this ACE applies to newly created child objects), plus optional \"id\"/\"who\". On read, " +
					"an \"id\" of -1 or null (the server's two observed sentinels, probed live, for entries " +
					"where tag isn't USER/GROUP) is normalized away rather than causing perpetual drift; the " +
					"exact JSON text is otherwise preserved as configured (write-what-you-said) unless the " +
					"server-side content genuinely changes.",
			},
			"comment": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Optional descriptive comment about the template's purpose. Defaults to an empty string.",
			},
			"builtin": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether this is one of TrueNAS's built-in system templates. Always false for a template this resource creates.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

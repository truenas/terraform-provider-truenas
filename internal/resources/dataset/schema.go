// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// zfsEnumAttr builds an Optional+Computed string attribute for a source-aware
// ZFS enum property (coverage audit): values are the uppercase ZFS forms, it reads back
// null when the property is inherited/default, and it cannot be reverted to
// inherited by removing it from config (see zfsprops.go localString).
func zfsEnumAttr(desc string, values ...string) schema.StringAttribute {
	return schema.StringAttribute{
		Optional:      true,
		Computed:      true,
		Description:   desc,
		Validators:    []validator.String{stringvalidator.OneOf(append(append([]string{}, values...), "INHERIT")...)},
		PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
	}
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
				Description: "Compression algorithm. Case-insensitive: lz4, zstd, off, inherit, etc.",
				Validators:  []validator.String{stringvalidator.OneOfCaseInsensitive("ON", "OFF", "LZ4", "GZIP", "GZIP-1", "GZIP-9", "ZSTD", "ZSTD-FAST", "ZLE", "LZJB", "ZSTD-1", "ZSTD-2", "ZSTD-3", "ZSTD-4", "ZSTD-5", "ZSTD-6", "ZSTD-7", "ZSTD-8", "ZSTD-9", "ZSTD-10", "ZSTD-11", "ZSTD-12", "ZSTD-13", "ZSTD-14", "ZSTD-15", "ZSTD-16", "ZSTD-17", "ZSTD-18", "ZSTD-19", "ZSTD-FAST-1", "ZSTD-FAST-2", "ZSTD-FAST-3", "ZSTD-FAST-4", "ZSTD-FAST-5", "ZSTD-FAST-6", "ZSTD-FAST-7", "ZSTD-FAST-8", "ZSTD-FAST-9", "ZSTD-FAST-10", "ZSTD-FAST-20", "ZSTD-FAST-30", "ZSTD-FAST-40", "ZSTD-FAST-50", "ZSTD-FAST-60", "ZSTD-FAST-70", "ZSTD-FAST-80", "ZSTD-FAST-90", "ZSTD-FAST-100", "ZSTD-FAST-500", "ZSTD-FAST-1000", "INHERIT")},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"acltype": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "ACL type: posix, nfsv4, off, or inherit. Case-insensitive.",
				Validators:  []validator.String{stringvalidator.OneOfCaseInsensitive("OFF", "NFSV4", "POSIX", "INHERIT")},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"share_type": schema.StringAttribute{
				Optional:    true,
				Description: "Optimised share-type preset applied at creation: GENERIC, SMB, MULTIPROTOCOL, NFS, or APPS (write-only, not returned by the API). Create-only.",
				Validators:  []validator.String{stringvalidator.OneOfCaseInsensitive("GENERIC", "MULTIPROTOCOL", "NFS", "SMB", "APPS")},
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
			// --- Source-aware ZFS tuning properties (coverage audit) ---
			// Each is Optional+Computed and reads back null when the property is
			// inherited from the parent or left at its ZFS default (only a value
			// set LOCAL on this dataset is recorded). Consequence: reverting a
			// locally-set value to inherited cannot be done by removing it from
			// the configuration — change it out of band and refresh.
			"aclmode": zfsEnumAttr("ACL inheritance mode: PASSTHROUGH, RESTRICTED, or DISCARD. Null (unset) inherits from the parent.",
				"PASSTHROUGH", "RESTRICTED", "DISCARD"),
			"atime":    zfsEnumAttr("Update access time on read: ON or OFF. Null inherits.", "ON", "OFF"),
			"exec":     zfsEnumAttr("Allow executing files: ON or OFF. Null inherits.", "ON", "OFF"),
			"readonly": zfsEnumAttr("Mount read-only: ON or OFF. Null inherits.", "ON", "OFF"),
			"sync": zfsEnumAttr("Sync write behaviour: STANDARD, ALWAYS, or DISABLED. Null inherits.",
				"STANDARD", "ALWAYS", "DISABLED"),
			"checksum": zfsEnumAttr("Checksum algorithm: ON, OFF, FLETCHER2, FLETCHER4, SHA256, SHA512, SKEIN, EDONR, or BLAKE3. Null inherits.",
				"ON", "OFF", "FLETCHER2", "FLETCHER4", "SHA256", "SHA512", "SKEIN", "EDONR", "BLAKE3"),
			"snapdir": zfsEnumAttr("Visibility of the .zfs/snapshot directory: VISIBLE, HIDDEN, or DISABLED. Null inherits.",
				"VISIBLE", "HIDDEN", "DISABLED"),
			"dedup": zfsEnumAttr("Deduplication (the ZFS `deduplication` property): ON, VERIFY, or OFF. Null inherits. Named `dedup` to match truenas_zvol.",
				"ON", "VERIFY", "OFF"),
			"recordsize": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Suggested block size for files, e.g. \"128K\" or \"1M\". Null (unset) inherits from the parent. Use the ZFS form (uppercase suffix) to avoid drift.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"copies": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Number of copies of each block (1-3). Null (unset) inherits from the parent.",
				Validators:  []validator.Int64{int64validator.Between(1, 3)},
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"special_small_block_size": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Threshold in bytes below which blocks are written to a pool's special allocation-class vdev; 0 disables it. Null (unset) inherits from the parent.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"refreservation": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Referenced reservation in bytes (space guaranteed to this dataset, excluding descendants/snapshots). Null (unset) inherits.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"xattr": schema.StringAttribute{
				Computed:    true,
				Description: "ZFS extended-attribute storage mode: SA (system-attribute), ON/DIR (directory-based), or OFF. Read-only — TrueNAS does not expose xattr in the writable create/update API, so it is set at dataset creation or inherited and only surfaced here for reading and drift-awareness.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"mountpoint": schema.StringAttribute{
				Computed: true,
				// Derived from name, which is RequiresReplace, so an in-place
				// update cannot change it. Keep the known value instead of
				// planning (known after apply), which would otherwise replace
				// consumers that use it as a RequiresReplace path. (#27)
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Description: "Dataset mountpoint path.",
			},
			"encrypted": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the dataset is encrypted.",
			},
			"encryption": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Enable ZFS encryption on this dataset at creation. Create-only: changing it recreates the dataset. Cannot be combined with inherit_encryption = true (the parent determines encryption).",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
					boolplanmodifier.RequiresReplace(),
				},
			},
			"inherit_encryption": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Description: "Whether this dataset inherits its encryption from the parent dataset " +
					"rather than owning its own key. Create-only. Computed: reconciled from the " +
					"dataset's encryption root on read, so it reflects reality even when left unset.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
					replaceIfChangedFromKnown(),
				},
			},
			"encryption_algorithm": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Encryption algorithm, e.g. \"AES-256-GCM\". Create-only.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"encryption_generate_key": schema.BoolAttribute{
				Optional:    true,
				Description: "Automatically generate the encryption key (key-based encryption). Create-only.",
				PlanModifiers: []planmodifier.Bool{
					replaceIfChangedFromKnown(),
				},
			},
			"encryption_passphrase": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				WriteOnly:   true,
				Description: "Passphrase for passphrase-based encryption (minimum 8 characters). Write-only: never stored in state. Create-only.",
			},
			"encryption_key": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				WriteOnly:   true,
				Description: "64-character hex key for key-based encryption. Write-only: never stored in state. Create-only.",
			},
			"key_format": schema.StringAttribute{
				Computed:    true,
				Description: "Encryption key format: PASSPHRASE or HEX (null when not encrypted).",
			},
			"locked": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the encrypted dataset is currently locked.",
			},
			"pool": schema.StringAttribute{
				Computed: true,
				// Derived from name (RequiresReplace); keep the known value on
				// update rather than planning (known after apply). (#27)
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Description: "Name of the pool containing this dataset.",
			},
		},
	}
}

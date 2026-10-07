// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package zvol

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
// ZFS enum property (coverage audit): uppercase values, reads back null when inherited/
// default, and cannot be reverted to inherited by removing it from config.
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
		Description: "Manages a ZFS volume (zvol/block device) on TrueNAS.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Zvol name (used as Terraform ID).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Full zvol path, e.g. tank/myvol.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"volsize": schema.Int64Attribute{
				Required:    true,
				Description: "Volume size in bytes.",
			},
			"volblocksize": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Block size in bytes (512, 1024, 2048, 4096, 8192, 16384, 32768, 65536, 131072). Set at create time only.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
					int64planmodifier.RequiresReplace(),
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
			"sync": zfsEnumAttr("Sync write behaviour: STANDARD, ALWAYS, or DISABLED. INHERIT inherits from the parent.",
				"STANDARD", "ALWAYS", "DISABLED"),
			"dedup": zfsEnumAttr("Deduplication: ON, VERIFY, or OFF. INHERIT inherits from the parent.",
				"ON", "VERIFY", "OFF"),
			"sparse": schema.BoolAttribute{
				Optional:    true,
				Description: "Sparse provisioning (write-only; not returned by API).",
			},
			"comments": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Human-readable description.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			// --- Source-aware ZFS tuning properties applicable to volumes (coverage audit) ---
			// Optional+Computed; read back null when inherited/default. Reverting a
			// locally-set value to inherited cannot be done by removing it from
			// config — change it out of band and refresh.
			"checksum": zfsEnumAttr("Checksum algorithm: ON, OFF, FLETCHER2, FLETCHER4, SHA256, SHA512, SKEIN, EDONR, or BLAKE3. Null inherits.",
				"ON", "OFF", "FLETCHER2", "FLETCHER4", "SHA256", "SHA512", "SKEIN", "EDONR", "BLAKE3"),
			"readonly": zfsEnumAttr("Mount read-only: ON or OFF. Null inherits.", "ON", "OFF"),
			"snapdev": zfsEnumAttr("Visibility of the volume's snapshot device nodes: VISIBLE or HIDDEN. Null inherits.",
				"VISIBLE", "HIDDEN"),
			"copies": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Description:   "Number of copies of each block (1-3). Null (unset) inherits from the parent.",
				Validators:    []validator.Int64{int64validator.Between(1, 3)},
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"reservation": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Description:   "Reserved space in bytes (guaranteed to this volume including snapshots). Null (unset) inherits.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"refreservation": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Description:   "Referenced reservation in bytes (space guaranteed to the volume, excluding snapshots). Null (unset) inherits.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"pool": schema.StringAttribute{
				Computed: true,
				// Derived from name (RequiresReplace), so it never changes on an
				// in-place update; keep the known value instead of planning "known
				// after apply" (parity with truenas_dataset, #27).
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Description: "Name of the pool containing this zvol.",
			},
			"encrypted": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the zvol is encrypted.",
			},
			"encryption": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Enable ZFS encryption on this zvol at creation. Create-only: changing it recreates the zvol. Cannot be combined with inherit_encryption = true (the parent determines encryption).",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
					boolplanmodifier.RequiresReplace(),
				},
			},
			"inherit_encryption": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Description: "Whether this zvol inherits its encryption from the parent dataset rather " +
					"than owning its own key. Create-only. Computed: reconciled from the zvol's encryption " +
					"root on read, so it reflects reality even when left unset.",
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
				Description: "Whether the encrypted zvol is currently locked.",
			},
		},
	}
}

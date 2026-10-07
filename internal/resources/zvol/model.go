// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package zvol

import (
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ZvolModel is the Terraform state/plan model for a TrueNAS zvol.
type ZvolModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	VolSize      types.Int64  `tfsdk:"volsize"`
	VolBlockSize types.Int64  `tfsdk:"volblocksize"`
	Compression  types.String `tfsdk:"compression"`
	Sync         types.String `tfsdk:"sync"`
	Dedup        types.String `tfsdk:"dedup"`
	Sparse       types.Bool   `tfsdk:"sparse"`
	Comments     types.String `tfsdk:"comments"`
	Pool         types.String `tfsdk:"pool"`
	Encrypted    types.Bool   `tfsdk:"encrypted"`

	// Encryption (all create-only; changing any recreates the zvol). Mirrors
	// truenas_dataset. encryption_passphrase / encryption_key are write-only:
	// read from config, never stored in state.
	Encryption            types.Bool   `tfsdk:"encryption"`
	InheritEncryption     types.Bool   `tfsdk:"inherit_encryption"`
	EncryptionAlgorithm   types.String `tfsdk:"encryption_algorithm"`
	EncryptionGenerateKey types.Bool   `tfsdk:"encryption_generate_key"`
	EncryptionPassphrase  types.String `tfsdk:"encryption_passphrase"`
	EncryptionKey         types.String `tfsdk:"encryption_key"`
	KeyFormat             types.String `tfsdk:"key_format"`
	Locked                types.Bool   `tfsdk:"locked"`

	// Source-aware ZFS tuning properties applicable to volumes (coverage audit). Each
	// reads back null when inherited/default rather than set LOCAL, so an
	// inherited value is never carried into state and re-sent. See zfsprops.go.
	Checksum       types.String `tfsdk:"checksum"`
	ReadOnly       types.String `tfsdk:"readonly"`
	Snapdev        types.String `tfsdk:"snapdev"`
	Copies         types.Int64  `tfsdk:"copies"`
	Reservation    types.Int64  `tfsdk:"reservation"`
	RefReservation types.Int64  `tfsdk:"refreservation"`
}

// zvolAPI matches the flat JSON structure returned by pool.dataset.get_instance for zvols.
type zvolAPI struct {
	Name      string `json:"name"`
	Pool      string `json:"pool"`
	Encrypted bool   `json:"encrypted"`
	Locked    bool   `json:"locked"`
	// EncryptionRoot owns the key this zvol uses: equal to Name when set locally,
	// an ancestor when inherited, null when not encrypted. Reconciles
	// inherit_encryption on read (#31/#32).
	EncryptionRoot *string `json:"encryption_root"`

	EncryptionAlgorithm struct {
		Value *string `json:"value"`
	} `json:"encryption_algorithm"`
	KeyFormat struct {
		Value *string `json:"value"`
	} `json:"key_format"`

	Compression struct {
		Parsed string `json:"parsed"`
		Source string `json:"source"` // LOCAL, INHERITED, DEFAULT, RECEIVED
	} `json:"compression"`

	// sync/dedup are read source-aware from the "value" field (upper case, e.g.
	// ALWAYS/ON), not the lower-case "parsed" field, so an imported zvol
	// round-trips against an upper-case config. The API key for dedup is
	// "deduplication", not "dedup".
	SyncP  zfsSourced `json:"sync"`
	DedupP zfsSourced `json:"deduplication"`

	VolSize struct {
		Parsed int64 `json:"parsed"`
	} `json:"volsize"`

	VolBlockSize struct {
		Parsed int64 `json:"parsed"`
	} `json:"volblocksize"`

	// Source-aware ZFS tuning properties (coverage audit); see zfsprops.go.
	ChecksumP  zfsSourced `json:"checksum"`
	ReadOnlyP  zfsSourced `json:"readonly"`
	SnapdevP   zfsSourced `json:"snapdev"`
	CopiesP    zfsSourced `json:"copies"`
	ReservP    zfsSourced `json:"reservation"`
	RefReservP zfsSourced `json:"refreservation"`

	UserProperties struct {
		Comments struct {
			Value string `json:"value"`
		} `json:"comments"`
	} `json:"user_properties"`
}

// apiPayload converts the model to the create/update JSON payload.
// volblocksizeStr converts a byte count to the string enum pool.dataset.create
// accepts for volblocksize. Returns "" for a value that is not a valid size.
func volblocksizeStr(b int64) string {
	switch b {
	case 512:
		return "512"
	case 1024:
		return "1K"
	case 2048:
		return "2K"
	case 4096:
		return "4K"
	case 8192:
		return "8K"
	case 16384:
		return "16K"
	case 32768:
		return "32K"
	case 65536:
		return "64K"
	case 131072:
		return "128K"
	}
	return ""
}

func (m *ZvolModel) apiPayload() map[string]any {
	p := map[string]any{
		"name":    m.Name.ValueString(),
		"type":    "VOLUME",
		"volsize": m.VolSize.ValueInt64(),
	}
	if !m.VolBlockSize.IsNull() && !m.VolBlockSize.IsUnknown() && m.VolBlockSize.ValueInt64() != 0 {
		// pool.dataset.create wants volblocksize as a string enum ("512", "1K",
		// … "128K"), not the raw byte count the schema models it as.
		if s := volblocksizeStr(m.VolBlockSize.ValueInt64()); s != "" {
			p["volblocksize"] = s
		}
	}
	if !m.Compression.IsNull() && !m.Compression.IsUnknown() {
		p["compression"] = strings.ToUpper(m.Compression.ValueString())
	}
	if !m.Sync.IsNull() && !m.Sync.IsUnknown() {
		p["sync"] = strings.ToUpper(m.Sync.ValueString())
	}
	if !m.Dedup.IsNull() && !m.Dedup.IsUnknown() {
		p["deduplication"] = strings.ToUpper(m.Dedup.ValueString())
	}
	if !m.Sparse.IsNull() && !m.Sparse.IsUnknown() {
		p["sparse"] = m.Sparse.ValueBool()
	}
	if !m.Comments.IsNull() && !m.Comments.IsUnknown() {
		p["comments"] = m.Comments.ValueString()
	}

	// Source-aware ZFS tuning properties (coverage audit). Only sent when set; an
	// inherited property reads back null so it never reaches the payload.
	putEnum(p, "checksum", m.Checksum)
	putEnum(p, "readonly", m.ReadOnly)
	putEnum(p, "snapdev", m.Snapdev)
	putInt(p, "copies", m.Copies)
	putInt(p, "reservation", m.Reservation)
	putInt(p, "refreservation", m.RefReservation)

	// Encryption (create-only). Write-only passphrase/key are injected from
	// req.Config by Create, not from the model (they are null in state).
	if !m.Encryption.IsNull() && !m.Encryption.IsUnknown() {
		p["encryption"] = m.Encryption.ValueBool()
	}
	if !m.InheritEncryption.IsNull() && !m.InheritEncryption.IsUnknown() {
		p["inherit_encryption"] = m.InheritEncryption.ValueBool()
	}
	if eo := m.encryptionOptions(); len(eo) > 0 {
		p["encryption_options"] = eo
	}
	return p
}

// encryptionOptions builds the non-secret encryption_options from the model.
// The write-only passphrase/key are added separately by Create from req.Config.
func (m *ZvolModel) encryptionOptions() map[string]any {
	eo := map[string]any{}
	if !m.EncryptionAlgorithm.IsNull() && !m.EncryptionAlgorithm.IsUnknown() && m.EncryptionAlgorithm.ValueString() != "" {
		eo["algorithm"] = m.EncryptionAlgorithm.ValueString()
	}
	if !m.EncryptionGenerateKey.IsNull() && !m.EncryptionGenerateKey.IsUnknown() {
		eo["generate_key"] = m.EncryptionGenerateKey.ValueBool()
	}
	return eo
}

// injectEncryptionSecrets adds the write-only encryption_passphrase /
// encryption_key from config into the create payload's encryption_options.
func injectEncryptionSecrets(payload map[string]any, cfg *ZvolModel) {
	eo, _ := payload["encryption_options"].(map[string]any)
	if eo == nil {
		eo = map[string]any{}
	}
	if !cfg.EncryptionPassphrase.IsNull() && !cfg.EncryptionPassphrase.IsUnknown() && cfg.EncryptionPassphrase.ValueString() != "" {
		eo["passphrase"] = cfg.EncryptionPassphrase.ValueString()
	}
	if !cfg.EncryptionKey.IsNull() && !cfg.EncryptionKey.IsUnknown() && cfg.EncryptionKey.ValueString() != "" {
		eo["key"] = cfg.EncryptionKey.ValueString()
	}
	if len(eo) > 0 {
		payload["encryption_options"] = eo
	}
}

// responseToModel populates m from the API response. Sparse is write-only
// (not returned by the API) so the plan/state value is preserved as-is.
func responseToModel(api *zvolAPI, m *ZvolModel) {
	m.ID = types.StringValue(api.Name)
	m.Name = types.StringValue(api.Name)
	m.Pool = types.StringValue(api.Pool)
	m.Encrypted = types.BoolValue(api.Encrypted)
	m.Encryption = types.BoolValue(api.Encrypted)
	m.Locked = types.BoolValue(api.Locked)
	if api.EncryptionAlgorithm.Value != nil && *api.EncryptionAlgorithm.Value != "" {
		m.EncryptionAlgorithm = types.StringValue(*api.EncryptionAlgorithm.Value)
	} else {
		m.EncryptionAlgorithm = types.StringNull()
	}
	if api.KeyFormat.Value != nil && *api.KeyFormat.Value != "" {
		m.KeyFormat = types.StringValue(*api.KeyFormat.Value)
	} else {
		m.KeyFormat = types.StringNull()
	}
	// inherit_encryption reconciled from encryption_root (owned by an ancestor =>
	// inherited); Computed so it round-trips even when unset (#31/#32). The
	// write-only passphrase/key and encryption_generate_key are not returned;
	// their config/plan values are preserved.
	m.InheritEncryption = types.BoolValue(api.Encrypted && api.EncryptionRoot != nil && *api.EncryptionRoot != api.Name)
	m.VolSize = types.Int64Value(api.VolSize.Parsed)
	m.VolBlockSize = types.Int64Value(api.VolBlockSize.Parsed)
	// Compression is source-aware: when it is not set LOCAL on this zvol the
	// value is inherited (or the ZFS default), so report "INHERIT" rather than
	// the resolved value (e.g. "lz4"). This lets compression = "inherit" round-
	// trip instead of failing with an inconsistent result (planned "inherit",
	// applied "lz4"). preserveCase keeps the user's casing of either the real
	// algorithm (local) or the inherit sentinel. (#38)
	if api.Compression.Source == "LOCAL" {
		m.Compression = preserveCompression(m.Compression, api.Compression.Parsed)
	} else {
		m.Compression = preserveCase(m.Compression, "INHERIT")
	}
	m.Sync = localStringOrInherit(api.SyncP)
	m.Dedup = localStringOrInherit(api.DedupP)
	m.Comments = types.StringValue(api.UserProperties.Comments.Value)
	// Sparse is write-only (not in API response); preserve plan/state value.

	// Source-aware ZFS tuning properties (coverage audit): recorded only when set LOCAL.
	m.Checksum = localStringOrInherit(api.ChecksumP)
	m.ReadOnly = localStringOrInherit(api.ReadOnlyP)
	m.Snapdev = localStringOrInherit(api.SnapdevP)
	m.Copies = localInt(api.CopiesP)
	m.Reservation = localInt(api.ReservP)
	m.RefReservation = localInt(api.RefReservP)
}

// preserveCase returns current if it matches apiVal case-insensitively
// (preserving the user's chosen casing), or the lower-cased apiVal otherwise.
// Only compression uses it: the API reports compression in lower case ("lz4"),
// which matches the conventional config form.
func preserveCase(current types.String, apiVal string) types.String {
	if current.IsNull() || current.IsUnknown() {
		return types.StringValue(strings.ToLower(apiVal))
	}
	if strings.EqualFold(current.ValueString(), apiVal) {
		return current
	}
	return types.StringValue(strings.ToLower(apiVal))
}

// compressionCanon folds a compression value to the form ZFS actually stores,
// so an input alias the API canonicalizes away still round-trips. ZFS treats
// zstd-fast-1 as plain zstd-fast (level 1 is the default fast level) and reports
// it back as "zstd-fast"; without folding, compression = "ZSTD-FAST-1" would
// read back "zstd-fast" and fail with an inconsistent result. Every other level
// (zstd-fast-10, zstd-5, gzip-9) is stored verbatim. Case-insensitive.
func compressionCanon(v string) string {
	s := strings.ToLower(strings.TrimSpace(v))
	if s == "zstd-fast-1" {
		return "zstd-fast"
	}
	return s
}

// preserveCompression is preserveCase for the compression property: current is
// kept when it is the same compression as apiVal under ZFS's own folding
// (compressionCanon), not merely equal-fold, so an alias like ZSTD-FAST-1 that
// the API reports back as ZSTD-FAST still round-trips. Real drift (a different
// algorithm) still overwrites state with the lower-cased API value.
func preserveCompression(current types.String, apiVal string) types.String {
	if current.IsNull() || current.IsUnknown() {
		return types.StringValue(strings.ToLower(apiVal))
	}
	if compressionCanon(current.ValueString()) == compressionCanon(apiVal) {
		return current
	}
	return types.StringValue(strings.ToLower(apiVal))
}

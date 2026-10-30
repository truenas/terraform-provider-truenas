// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset

import (
	"reflect"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/normalize"
)

// DatasetModel maps to the TrueNAS pool.dataset API fields.
type DatasetModel struct {
	ID types.String `tfsdk:"id"`

	// Required
	Name types.String `tfsdk:"name"`

	// Optional with TrueNAS defaults
	Type        types.String `tfsdk:"type"`
	Compression types.String `tfsdk:"compression"`
	AClType     types.String `tfsdk:"acltype"`
	ShareType   types.String `tfsdk:"share_type"`
	Comments    types.String `tfsdk:"comments"`
	Quota       types.Int64  `tfsdk:"quota"`
	RefQuota    types.Int64  `tfsdk:"refquota"`
	Reservation types.Int64  `tfsdk:"reservation"`
	VolSize     types.Int64  `tfsdk:"volsize"`

	// Source-aware ZFS tuning properties (coverage audit). Each reads back null when the
	// property is inherited/default rather than set on this dataset, so an
	// inherited value is never carried into state and re-sent. Enum names match
	// the ZFS property; sync/dedup match truenas_zvol's existing attributes.
	// See zfsprops.go (localString/localInt).
	ACLMode               types.String              `tfsdk:"aclmode"`
	ATime                 types.String              `tfsdk:"atime"`
	Exec                  types.String              `tfsdk:"exec"`
	ReadOnly              types.String              `tfsdk:"readonly"`
	Sync                  types.String              `tfsdk:"sync"`
	Checksum              types.String              `tfsdk:"checksum"`
	Snapdir               types.String              `tfsdk:"snapdir"`
	Dedup                 types.String              `tfsdk:"dedup"` // API field "deduplication"
	RecordSize            normalize.CaseInsensitive `tfsdk:"recordsize"`
	Copies                types.Int64               `tfsdk:"copies"`
	SpecialSmallBlockSize types.Int64               `tfsdk:"special_small_block_size"`
	RefReservation        types.Int64               `tfsdk:"refreservation"`

	// XAttr is read-only: ZFS extended-attribute storage mode is returned by
	// get_instance but is NOT in the writable create/update API (checked live
	// on 25.10 and 27.0), so it is exposed for reading only, not set.
	XAttr types.String `tfsdk:"xattr"`

	// Encryption (all create-only; changing any of these recreates the dataset).
	// encryption_passphrase / encryption_key are write-only: read from config,
	// never stored in state.
	Encryption            types.Bool   `tfsdk:"encryption"`
	InheritEncryption     types.Bool   `tfsdk:"inherit_encryption"`
	EncryptionAlgorithm   types.String `tfsdk:"encryption_algorithm"`
	EncryptionGenerateKey types.Bool   `tfsdk:"encryption_generate_key"`
	EncryptionPassphrase  types.String `tfsdk:"encryption_passphrase"`
	EncryptionKey         types.String `tfsdk:"encryption_key"`

	// Computed
	MountPoint types.String `tfsdk:"mountpoint"`
	Encrypted  types.Bool   `tfsdk:"encrypted"`
	KeyFormat  types.String `tfsdk:"key_format"`
	Locked     types.Bool   `tfsdk:"locked"`
	Pool       types.String `tfsdk:"pool"`
}

// datasetResourceModel is the truenas_dataset resource model: DatasetModel
// (shared with the data source and the list resource) plus force_destroy, which
// only the resource has.
type datasetResourceModel struct {
	DatasetModel
	ForceDestroy types.Bool `tfsdk:"force_destroy"`
}

// apiPayload converts the model to the create/update JSON payload.
func (m *DatasetModel) apiPayload() map[string]any {
	p := map[string]any{"name": m.Name.ValueString()}
	if !m.Type.IsNull() && !m.Type.IsUnknown() {
		p["type"] = strings.ToUpper(m.Type.ValueString())
	}
	if !m.Compression.IsNull() && !m.Compression.IsUnknown() {
		p["compression"] = compressionAPI(m.Compression.ValueString())
	}
	if !m.AClType.IsNull() && !m.AClType.IsUnknown() {
		p["acltype"] = strings.ToUpper(m.AClType.ValueString())
	}
	if !m.ShareType.IsNull() && !m.ShareType.IsUnknown() {
		p["share_type"] = strings.ToUpper(m.ShareType.ValueString())
	}
	if !m.Comments.IsNull() && !m.Comments.IsUnknown() {
		p["comments"] = m.Comments.ValueString()
	}
	if !m.Quota.IsNull() && !m.Quota.IsUnknown() {
		p["quota"] = m.Quota.ValueInt64()
	}
	if !m.RefQuota.IsNull() && !m.RefQuota.IsUnknown() {
		p["refquota"] = m.RefQuota.ValueInt64()
	}
	if !m.Reservation.IsNull() && !m.Reservation.IsUnknown() {
		p["reservation"] = m.Reservation.ValueInt64()
	}
	// volsize only applies to VOLUME datasets; pool.dataset.update rejects
	// "volsize" outright for FILESYSTEM datasets (TrueNAS API error code
	// 22: 'volsize'). VolSize is Computed in the schema and reads back as 0
	// for FILESYSTEM datasets, so a plain null/unknown guard isn't enough -
	// state carries a known-but-zero value into every later plan. Only
	// include it when it's a real, known, non-zero size.
	if !m.VolSize.IsNull() && !m.VolSize.IsUnknown() && m.VolSize.ValueInt64() != 0 {
		p["volsize"] = m.VolSize.ValueInt64()
	}

	// Source-aware ZFS tuning properties (coverage audit). Only sent when set in config
	// or carried LOCAL in state; an inherited property reads back null (see
	// responseToModel), so it never reaches the payload and an apply cannot
	// convert an inherited property into a local one.
	putEnum(p, "aclmode", m.ACLMode)
	putEnum(p, "atime", m.ATime)
	putEnum(p, "exec", m.Exec)
	putEnum(p, "readonly", m.ReadOnly)
	putEnum(p, "sync", m.Sync)
	putEnum(p, "checksum", m.Checksum)
	putEnum(p, "snapdir", m.Snapdir)
	putEnum(p, "deduplication", m.Dedup)
	// recordsize goes out in upper case ("1m" -> "1M"): 25.10 and 26.0 reject
	// the lower-case suffix that 28.0 accepts and stores in upper case. (B-126)
	if !m.RecordSize.IsNull() && !m.RecordSize.IsUnknown() {
		p["recordsize"] = strings.ToUpper(m.RecordSize.ValueString())
	}
	putInt(p, "copies", m.Copies)
	putInt(p, "special_small_block_size", m.SpecialSmallBlockSize)
	putInt(p, "refreservation", m.RefReservation)

	// Encryption (create-only). The write-only passphrase/key are injected from
	// req.Config by the resource's Create, not from the model (they are null in
	// state); see encryptionOptions.
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
func (m *DatasetModel) encryptionOptions() map[string]any {
	eo := map[string]any{}
	if !m.EncryptionAlgorithm.IsNull() && !m.EncryptionAlgorithm.IsUnknown() && m.EncryptionAlgorithm.ValueString() != "" {
		eo["algorithm"] = m.EncryptionAlgorithm.ValueString()
	}
	if !m.EncryptionGenerateKey.IsNull() && !m.EncryptionGenerateKey.IsUnknown() {
		eo["generate_key"] = m.EncryptionGenerateKey.ValueBool()
	}
	return eo
}

// createAPIPayload is apiPayload plus the create-only defaults the provider
// fills in when the configuration leaves them unset.
//
//   - inherit_encryption = false when encryption is set: the server defaults
//     inherit_encryption to true, so encryption = false under an encrypted
//     parent silently created an encrypted child (inconsistent result), and
//     encryption = true failed with "Must be disabled when encryption is
//     enabled". An explicit encryption value means "this dataset decides".
//     (B-042)
//   - aclmode = DISCARD when acltype is POSIX or OFF: TrueNAS 28.0 sets this
//     itself, while 25.10/26.0 reject a POSIX/OFF dataset under a non-DISCARD
//     parent ("Must be set to DISCARD when acltype is POSIX or OFF"). Sending
//     it gives the same result on every release. (B-148)
func (m *DatasetModel) createAPIPayload() map[string]any {
	p := m.apiPayload()
	if _, set := p["encryption"]; set {
		if _, inh := p["inherit_encryption"]; !inh {
			p["inherit_encryption"] = false
		}
	}
	if at, _ := p["acltype"].(string); at == "POSIX" || at == "OFF" {
		if _, set := p["aclmode"]; !set {
			p["aclmode"] = "DISCARD"
		}
	}
	return p
}

// updatableFields is apiPayload without the keys pool.dataset.update rejects
// or that must never be resent: "name" (the dataset id is the update
// method's first positional arg), "type" (create-only: "[EINVAL] data.type:
// Extra inputs are not permitted"), the create-only encryption keys,
// share_type (write-only, RequiresReplace; #25) and acltype (RequiresReplace;
// resending it is not a no-op — pool.dataset.update also writes
// aclmode/aclinherit=DISCARD for POSIX/OFF, turning an inherited aclmode
// into a local one; #26).
func (m *DatasetModel) updatableFields() map[string]any {
	p := m.apiPayload()
	delete(p, "name")
	delete(p, "type")
	delete(p, "encryption")
	delete(p, "inherit_encryption")
	delete(p, "encryption_options")
	delete(p, "share_type")
	delete(p, "acltype")
	return p
}

// updateAPIPayload converts the model to the pool.dataset.update payload.
// With a prior state it sends only what changed (see StripUnchanged); with
// nil it returns every updatable field.
func (m *DatasetModel) updateAPIPayload(prior *DatasetModel) map[string]any {
	p := m.updatableFields()
	if prior != nil {
		StripUnchanged(p, prior.updatableFields())
		KeepThick(p, m.RefReservation, prior.RefReservation, prior.VolSize)
	}
	return p
}

// StripUnchanged removes from an update payload every key whose value equals
// the value prior state would send, so an update writes only what changed.
//
// Every Optional+Computed attribute is carried into the plan from state by
// UseStateForUnknown, so a payload built from the plan holds every property
// the last read returned. Resending an unchanged value is not a no-op: it
// turns a DEFAULT/INHERITED property LOCAL (an untouched unlimited quota
// became a local quota of 0, #61; an absent description became a LOCAL empty
// org.freenas:description, #62 and B-039), and it re-applies a value ZFS
// itself maintains (a thick volume's refreservation, which ZFS grows with
// volsize; resending the old value left a grown zvol partly thin, B-038).
//
// One exception: when volsize changes, a configured refreservation is kept
// even if unchanged, so ZFS's automatic refreservation adjustment cannot
// override the configured value.
func StripUnchanged(p, prior map[string]any) {
	volsizeChanged := false
	if v, ok := p["volsize"]; ok && !reflect.DeepEqual(v, prior["volsize"]) {
		volsizeChanged = true
	}
	for k, v := range p {
		pv, ok := prior[k]
		if !ok || !reflect.DeepEqual(v, pv) {
			continue
		}
		if k == "refreservation" && volsizeChanged {
			continue
		}
		delete(p, k)
	}
}

// KeepThick keeps a thick volume thick when volsize grows and refreservation
// is not configured (planned unknown, see RefreservationFollowsVolsize).
// TrueNAS 25.10 and 26.0 create a thick volume with refreservation equal to
// volsize, which ZFS does not adjust when the volume grows, so the grown volume
// was left partly thin; when the prior refreservation equals the prior volsize
// the new volsize is sent as the refreservation too. (28.0 creates it with the
// ZFS-computed reservation, which ZFS grows by itself; that value never equals
// volsize, so nothing is added there.) (B-038)
func KeepThick(p map[string]any, planned, priorRefRes, priorVolSize types.Int64) {
	vs, grows := p["volsize"]
	if !grows || !planned.IsUnknown() || priorRefRes.IsNull() || priorRefRes.IsUnknown() ||
		priorVolSize.IsNull() || priorVolSize.IsUnknown() || priorRefRes.ValueInt64() != priorVolSize.ValueInt64() {
		return
	}
	p["refreservation"] = vs
}

// CommentsValue maps the description pool.dataset.get_instance reports to
// the comments attribute. TrueNAS 26.0 writes the create schema's default,
// the literal "INHERIT", as the description of every dataset and zvol
// created without comments; that is not a description anyone set, so it
// reads back as "" like an absent one. (B-147)
func CommentsValue(v string) string {
	if v == "INHERIT" {
		return ""
	}
	return v
}

// apiResponse matches the flat JSON structure returned by pool.dataset.get_instance.
// Fields are at the root level (no "properties" wrapper). Quota fields use *int64
// because TrueNAS returns JSON null when no limit is set.
type apiResponse struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	MountPoint string `json:"mountpoint"`
	Encrypted  bool   `json:"encrypted"`
	Locked     bool   `json:"locked"`
	Pool       string `json:"pool"`
	// EncryptionRoot is the dataset that owns the encryption key this dataset
	// uses: equal to Name when encryption is set locally here, an ancestor when
	// it is inherited, and null when the dataset is not encrypted. It is how
	// inherit_encryption is reconciled on read (#31/#32).
	EncryptionRoot *string `json:"encryption_root"`

	EncryptionAlgorithm struct {
		Value *string `json:"value"`
	} `json:"encryption_algorithm"`
	KeyFormat struct {
		Value *string `json:"value"`
	} `json:"key_format"`

	Compression struct {
		Parsed string `json:"parsed"` // lowercase: "lz4"
		Source string `json:"source"` // LOCAL, INHERITED, DEFAULT, RECEIVED
	} `json:"compression"`

	AClType struct {
		Parsed string `json:"parsed"` // lowercase: "posix", "nfsv4", "off"
		Source string `json:"source"` // LOCAL, INHERITED, DEFAULT, RECEIVED
	} `json:"acltype"`

	Quota struct {
		Parsed *int64 `json:"parsed"` // null when unlimited
	} `json:"quota"`

	RefQuota struct {
		Parsed *int64 `json:"parsed"`
	} `json:"refquota"`

	Reservation struct {
		Parsed *int64 `json:"parsed"`
	} `json:"reservation"`

	VolSize struct {
		Parsed int64 `json:"parsed"` // 0 for FILESYSTEM datasets
	} `json:"volsize"`

	// Source-aware ZFS tuning properties (coverage audit). Each carries "source" so an
	// inherited value can be told from a locally-set one; see zfsprops.go.
	ACLModeP    zfsSourced `json:"aclmode"`
	ATimeP      zfsSourced `json:"atime"`
	ExecP       zfsSourced `json:"exec"`
	ReadOnlyP   zfsSourced `json:"readonly"`
	SyncP       zfsSourced `json:"sync"`
	ChecksumP   zfsSourced `json:"checksum"`
	SnapdirP    zfsSourced `json:"snapdir"`
	DedupP      zfsSourced `json:"deduplication"`
	RecordSizeP zfsSourced `json:"recordsize"`
	CopiesP     zfsSourced `json:"copies"`
	SSBSP       zfsSourced `json:"special_small_block_size"`
	RefResP     zfsSourced `json:"refreservation"`
	XAttrP      zfsSourced `json:"xattr"` // read-only

	// Comments live under user_properties in TrueNAS 24+
	UserProperties struct {
		Comments struct {
			Value string `json:"value"`
		} `json:"comments"`
	} `json:"user_properties"`
}

// injectEncryptionSecrets adds the write-only encryption_passphrase /
// encryption_key from config into the create payload's encryption_options.
func injectEncryptionSecrets(payload map[string]any, cfg *DatasetModel) {
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

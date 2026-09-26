// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"
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

	// SpecialSmallBlockSize is source-aware (see sourcedString): it holds
	// its value in state only when it is set LOCAL on this dataset, and the
	// literal "INHERIT" when it is inherited, left at the ZFS default or
	// received. "INHERIT" is also what the user writes to state explicitly
	// that the property follows the parent. It is sent to
	// pool.dataset.create as is; an update never sends it, because changing
	// a LOCAL value to "INHERIT" is refused at plan time (see
	// keepLocalSpecialSmallBlockSize) and dropUnchangedInherit drops it
	// everywhere else. It is null only when get_instance does not report it
	// at all, i.e. for a dataset type that does not carry it.
	//
	// It is an integer in ZFS but a string here, holding a decimal integer
	// or "INHERIT"; apiPayload sends the number as a JSON integer.
	SpecialSmallBlockSize types.String `tfsdk:"special_small_block_size"`

	// Computed
	MountPoint types.String `tfsdk:"mountpoint"`
	Encrypted  types.Bool   `tfsdk:"encrypted"`
	Pool       types.String `tfsdk:"pool"`
}

// apiPayload converts the model to the create/update JSON payload.
func (m *DatasetModel) apiPayload() map[string]any {
	p := map[string]any{"name": m.Name.ValueString()}
	if !m.Type.IsNull() && !m.Type.IsUnknown() {
		p["type"] = strings.ToUpper(m.Type.ValueString())
	}
	if !m.Compression.IsNull() && !m.Compression.IsUnknown() {
		p["compression"] = strings.ToUpper(m.Compression.ValueString())
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
	// special_small_block_size is deliberately NOT guarded on != 0 the way
	// volsize is: 0 is a meaningful value here (it disables writing small
	// blocks to the special vdev), not a stand-in for "unset". The
	// equivalent protection is in responseToModel, which records "INHERIT"
	// rather than the effective value unless pool.dataset.get_instance
	// reports the property's source as LOCAL. An inherited or default value
	// therefore never reaches this payload as a number, so an apply cannot
	// silently convert an inherited property into a local one.
	putInteger(p, "special_small_block_size", m.SpecialSmallBlockSize)
	return p
}

// putInteger sets key for an integer-valued property held as a string (see
// SpecialSmallBlockSize): "INHERIT", in any case, is sent as the string
// "INHERIT", and anything else as a JSON integer. The schema validators only
// admit those two forms; a value that still fails to parse is passed through
// as a string so that TrueNAS rejects it with its own message instead of the
// property being dropped silently.
func putInteger(p map[string]any, key string, v types.String) {
	if v.IsNull() || v.IsUnknown() {
		return
	}
	s := v.ValueString()
	if isInherit(s) {
		p[key] = inherit
		return
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		p[key] = n
		return
	}
	p[key] = s
}

// inherit is the value pool.dataset.create/update take, for any of the
// source-aware properties, to mean "inherit from the parent" (zfs inherit),
// and the value responseToModel records for a property that is not LOCAL.
const inherit = "INHERIT"

// isInherit reports whether a configured value is "INHERIT" in any case.
func isInherit(s string) bool { return strings.EqualFold(s, inherit) }

// sourceAwareKeys maps each source-aware property's pool.dataset key to
// its value in a model. dropUnchangedInherit uses it.
var sourceAwareKeys = []struct {
	key string
	get func(*DatasetModel) types.String
}{
	{"special_small_block_size", func(m *DatasetModel) types.String { return m.SpecialSmallBlockSize }},
}

// dropUnchangedInherit removes from an update payload every source-aware
// property that is "INHERIT" in both the plan and the prior state. Every
// inherited property reads back as "INHERIT", and the attributes are
// Optional+Computed, so without this each update would resend "INHERIT"
// for every property the configuration does not set. That is harmless
// but not a no-op request, so the update carries "INHERIT" only when it
// changes something: a LOCAL property being reverted to inherited. For
// special_small_block_size that change is refused at plan time (see
// keepLocalSpecialSmallBlockSize), so it never reaches an update.
func dropUnchangedInherit(p map[string]any, plan, state *DatasetModel) {
	for _, a := range sourceAwareKeys {
		pv, sv := a.get(plan), a.get(state)
		if pv.IsNull() || pv.IsUnknown() || sv.IsNull() || sv.IsUnknown() {
			continue
		}
		if isInherit(pv.ValueString()) && isInherit(sv.ValueString()) {
			delete(p, a.key)
		}
	}
}

// updateAPIPayload converts the model to the pool.dataset.update JSON
// payload. It starts from apiPayload (the create payload) and strips keys
// that pool.dataset.update rejects as create-only: "name" (the dataset's
// id is passed as the update method's first positional arg, not a payload
// key) and "type" (changing a dataset's type after creation isn't
// supported; TrueNAS returns "[EINVAL] data.type: Extra inputs are not
// permitted" if it's included).
func (m *DatasetModel) updateAPIPayload() map[string]any {
	p := m.apiPayload()
	delete(p, "name")
	delete(p, "type")
	return p
}

// apiResponse matches the flat JSON structure returned by pool.dataset.get_instance.
// Fields are at the root level (no "properties" wrapper). Quota fields use *int64
// because TrueNAS returns JSON null when no limit is set.
type apiResponse struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	MountPoint string `json:"mountpoint"`
	Encrypted  bool   `json:"encrypted"`
	Pool       string `json:"pool"`

	Compression struct {
		Parsed string `json:"parsed"` // lowercase: "lz4"
	} `json:"compression"`

	AClType struct {
		Parsed string `json:"parsed"` // lowercase: "posix", "nfsv4", "off"
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

	// SpecialSmallBlockSize carries "source" as well as the value, because
	// the value alone cannot distinguish "set to 0 on this dataset" from
	// "inherited". source is LOCAL, INHERITED, DEFAULT or RECEIVED. The
	// whole sub-object is absent for dataset types that do not carry the
	// property, which leaves Source empty and propertyBytes unset; see
	// sourcedString for how that differs from "inherited".
	SpecialSmallBlockSize struct {
		Parsed propertyBytes `json:"parsed"`
		Source string        `json:"source"`
	} `json:"special_small_block_size"`

	// Comments live under user_properties in TrueNAS 24+
	UserProperties struct {
		Comments struct {
			Value string `json:"value"`
		} `json:"comments"`
	} `json:"user_properties"`
}

// sourcedString is the read rule a source-aware attribute follows, given
// whether the property was reported at all and, if it is set on this
// dataset, the value to record:
//
//   - not reported (absent): null, because the property does not apply to
//     this dataset and "INHERIT" would be meaningless;
//   - set LOCAL: the value, as produced by the caller;
//   - LOCAL but with a null value, which the API model allows but should
//     not happen: null, since there is no value to record and the property
//     is not inherited either;
//   - any other source (INHERITED, DEFAULT, RECEIVED), or a reported
//     property without a source: "INHERIT", kept in the configured casing
//     when the configuration wrote "inherit" in another one, so that the
//     read does not produce a diff.
func sourcedString(current types.String, absent, isLocal, localNull bool, localValue func() types.String) types.String {
	switch {
	case absent:
		return types.StringNull()
	case isLocal:
		return localValue()
	case localNull:
		return types.StringNull()
	}
	if !current.IsNull() && !current.IsUnknown() && isInherit(current.ValueString()) {
		return current
	}
	return types.StringValue(inherit)
}

// integerString records an integer property's LOCAL value, keeping the
// configured spelling when it denotes the same number (e.g. "016" for 16)
// so that the read produces no diff.
func integerString(current types.String, n int64) types.String {
	if !current.IsNull() && !current.IsUnknown() {
		if c, err := strconv.ParseInt(current.ValueString(), 10, 64); err == nil && c == n {
			return current
		}
	}
	return types.StringValue(strconv.FormatInt(n, 10))
}

// propertyBytes decodes the "parsed" field of a byte-valued dataset property
// from pool.dataset.get_instance. It exists because that field is not
// consistently typed: within a single response, some byte-valued properties
// parse as a JSON number and others as a JSON string. Probed live against
// TrueNAS SCALE 25.10 (pool.dataset.get_instance on a filesystem dataset,
// 2026-09-22):
//
//	"special_small_block_size": {"parsed": "0",     "rawvalue": "0",
//	                             "source": "INHERITED", "source_info": "Tank",
//	                             "value": "0"}
//	"recordsize":               {"parsed": 1048576, "rawvalue": "1048576",
//	                             "source": "LOCAL",     "source_info": null,
//	                             "value": "1M"}
//
// Decoding special_small_block_size straight into an int64 is what the first
// acceptance run against a live box failed on, so this accepts either form.
//
// A string carrying a ZFS size suffix ("16K") is accepted as well. The probed
// sample is zero, where a plain decimal string and a suffixed one are
// indistinguishable, and guessing wrong fails the read outright rather than
// degrading it - the same reason "value" is not used here, since that field
// is the human-readable form ("1M") rather than a byte count.
type propertyBytes struct {
	Set   bool
	Value int64
}

func (p *propertyBytes) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		if s == "" {
			return nil
		}
		v, err := parseZFSSize(s)
		if err != nil {
			return err
		}
		p.Set, p.Value = true, v
		return nil
	}
	var v int64
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	p.Set, p.Value = true, v
	return nil
}

// parseZFSSize parses a byte count written either as a plain decimal or with
// a binary ZFS size suffix, e.g. "16384" or "16K". Suffixes are binary
// multiples, as zfs(8) reports them.
func parseZFSSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty size value")
	}
	mult := int64(1)
	switch s[len(s)-1] {
	case 'K', 'k':
		mult = 1 << 10
	case 'M', 'm':
		mult = 1 << 20
	case 'G', 'g':
		mult = 1 << 30
	case 'T', 't':
		mult = 1 << 40
	case 'P', 'p':
		mult = 1 << 50
	}
	if mult != 1 {
		s = s[:len(s)-1]
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing size %q: %w", s, err)
	}
	return n * mult, nil
}

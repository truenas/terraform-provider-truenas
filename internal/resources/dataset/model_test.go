// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

// Package dataset contains unit tests for the truenas_dataset resource model.
package dataset

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestDatasetUpdateAPIPayloadOmitsCreateOnlyKeys is a regression test for a
// live acceptance failure: pool.dataset.update rejected the update payload
// with "[EINVAL] data.type: Extra inputs are not permitted" because the
// payload (built from the same apiPayload used for create) still included
// "type". The dataset id is passed as pool.dataset.update's first
// positional argument, not as a "name" payload key, so "name" must be
// stripped too.
func TestDatasetUpdateAPIPayloadOmitsCreateOnlyKeys(t *testing.T) {
	m := &DatasetModel{
		Name:        types.StringValue("tank/mydata"),
		Type:        types.StringValue("filesystem"),
		Compression: types.StringValue("lz4"),
		Comments:    types.StringValue("updated"),
	}

	payload := m.updateAPIPayload(nil)

	if _, ok := payload["type"]; ok {
		t.Errorf("update payload must not include \"type\", got: %v", payload)
	}
	if _, ok := payload["name"]; ok {
		t.Errorf("update payload must not include \"name\", got: %v", payload)
	}

	// Sanity: other writable fields still make it through.
	if payload["compression"] != "LZ4" {
		t.Errorf("expected compression=LZ4 to survive, got %v", payload["compression"])
	}
	if payload["comments"] != "updated" {
		t.Errorf("expected comments=updated to survive, got %v", payload["comments"])
	}
}

// TestDatasetUpdateAPIPayloadOmitsUnchangedQuota is a regression test for #61:
// a comment-only update resent quota/refquota/reservation at their unchanged
// read-back value (0 = unlimited), which flipped the TrueNAS property source
// from DEFAULT to LOCAL. With prior state equal to the plan for those fields,
// they must be omitted from the update payload, while a genuinely changed field
// (here comments) still goes through.
func TestDatasetUpdateAPIPayloadOmitsUnchangedQuota(t *testing.T) {
	prior := &DatasetModel{
		Name:        types.StringValue("tank/mydata"),
		Type:        types.StringValue("filesystem"),
		Comments:    types.StringValue("before"),
		Quota:       types.Int64Value(0),
		RefQuota:    types.Int64Value(0),
		Reservation: types.Int64Value(0),
	}
	plan := &DatasetModel{
		Name:        types.StringValue("tank/mydata"),
		Type:        types.StringValue("filesystem"),
		Comments:    types.StringValue("after"), // the only real change
		Quota:       types.Int64Value(0),        // carried forward by UseStateForUnknown
		RefQuota:    types.Int64Value(0),
		Reservation: types.Int64Value(0),
	}

	payload := plan.updateAPIPayload(prior)

	for _, k := range []string{"quota", "refquota", "reservation"} {
		if _, ok := payload[k]; ok {
			t.Errorf("unchanged %q must be omitted from update payload, got: %v", k, payload)
		}
	}
	if payload["comments"] != "after" {
		t.Errorf("changed comments must survive, got %v", payload["comments"])
	}
}

// TestDatasetUpdateAPIPayloadKeepsQuotaChanges verifies the strip does not
// swallow real changes, including a reset to 0 that clears a prior limit and a
// newly-set limit.
func TestDatasetUpdateAPIPayloadKeepsQuotaChanges(t *testing.T) {
	prior := &DatasetModel{
		Name:        types.StringValue("tank/mydata"),
		Type:        types.StringValue("filesystem"),
		Quota:       types.Int64Value(5368709120), // 5Gi, to be cleared
		RefQuota:    types.Int64Value(0),          // unset, to be set
		Reservation: types.Int64Value(0),          // unchanged
	}
	plan := &DatasetModel{
		Name:        types.StringValue("tank/mydata"),
		Type:        types.StringValue("filesystem"),
		Quota:       types.Int64Value(0),          // reset -> must send 0
		RefQuota:    types.Int64Value(1073741824), // 1Gi -> must send
		Reservation: types.Int64Value(0),          // unchanged -> omit
	}

	payload := plan.updateAPIPayload(prior)

	if payload["quota"] != int64(0) {
		t.Errorf("quota reset to 0 must be sent, got %v (%T)", payload["quota"], payload["quota"])
	}
	if payload["refquota"] != int64(1073741824) {
		t.Errorf("new refquota must be sent, got %v", payload["refquota"])
	}
	if _, ok := payload["reservation"]; ok {
		t.Errorf("unchanged reservation must be omitted, got: %v", payload)
	}
}

// TestDatasetUpdateAPIPayloadOmitsUnchangedComments is a regression test for
// #62: an absent description reads back as a known "" and was resent on an
// unrelated update, writing org.freenas:description as a LOCAL empty string.
// An unchanged comments must be omitted; a real change (set or clear) is kept.
func TestDatasetUpdateAPIPayloadOmitsUnchangedComments(t *testing.T) {
	prior := &DatasetModel{
		Name:     types.StringValue("tank/mydata"),
		Type:     types.StringValue("filesystem"),
		Comments: types.StringValue(""), // absent description reads back as ""
	}
	plan := &DatasetModel{
		Name:        types.StringValue("tank/mydata"),
		Type:        types.StringValue("filesystem"),
		Compression: types.StringValue("zstd"), // the only real change
		Comments:    types.StringValue(""),     // carried forward by UseStateForUnknown
	}

	payload := plan.updateAPIPayload(prior)

	if _, ok := payload["comments"]; ok {
		t.Errorf("unchanged comments must be omitted from update payload, got: %v", payload)
	}
	if payload["compression"] != "ZSTD" {
		t.Errorf("changed compression must survive, got %v", payload["compression"])
	}
}

// TestDatasetUpdateAPIPayloadKeepsCommentsChange verifies a real description
// change still goes through, including clearing it back to "".
func TestDatasetUpdateAPIPayloadKeepsCommentsChange(t *testing.T) {
	// Set a description.
	set := (&DatasetModel{
		Name: types.StringValue("tank/mydata"), Type: types.StringValue("filesystem"),
		Comments: types.StringValue("hello"),
	}).updateAPIPayload(&DatasetModel{
		Name: types.StringValue("tank/mydata"), Type: types.StringValue("filesystem"),
		Comments: types.StringValue(""),
	})
	if set["comments"] != "hello" {
		t.Errorf("new description must be sent, got %v", set["comments"])
	}

	// Clear a description back to "".
	clear := (&DatasetModel{
		Name: types.StringValue("tank/mydata"), Type: types.StringValue("filesystem"),
		Comments: types.StringValue(""),
	}).updateAPIPayload(&DatasetModel{
		Name: types.StringValue("tank/mydata"), Type: types.StringValue("filesystem"),
		Comments: types.StringValue("hello"),
	})
	if v, ok := clear["comments"]; !ok || v != "" {
		t.Errorf("clearing a description must send comments=\"\", got %v (present=%v)", v, ok)
	}
}

// TestDatasetCreateAPIPayloadStillIncludesType verifies the create payload
// (apiPayload) is unaffected by the update-only stripping in
// updateAPIPayload - type and name must still be present for pool.dataset.create.
func TestDatasetCreateAPIPayloadStillIncludesType(t *testing.T) {
	m := &DatasetModel{
		Name: types.StringValue("tank/mydata"),
		Type: types.StringValue("filesystem"),
	}

	payload := m.apiPayload()

	if payload["name"] != "tank/mydata" {
		t.Errorf("expected name=tank/mydata in create payload, got %v", payload["name"])
	}
	if payload["type"] != "FILESYSTEM" {
		t.Errorf("expected type=FILESYSTEM in create payload, got %v", payload["type"])
	}
}

// TestDatasetUpdateAPIPayloadOmitsVolsizeForFilesystem is a regression test
// for a live acceptance failure: "truenas API error (code 22): 'volsize'".
// After a FILESYSTEM dataset is created and read back, VolSize is a known
// (but zero) value in state - responseToModel sets it from
// api.VolSize.Parsed, which is 0 for FILESYSTEM datasets. Because VolSize
// is Computed with UseStateForUnknown, that known-zero value flows into
// every later plan, so a plain null/unknown guard on apiPayload wasn't
// enough to keep "volsize" out of pool.dataset.update payloads for
// FILESYSTEM datasets. Only a real, known, non-zero size (as set for
// type=VOLUME) should be sent.
func TestDatasetUpdateAPIPayloadOmitsVolsizeForFilesystem(t *testing.T) {
	m := &DatasetModel{
		Name:        types.StringValue("tank/mydata"),
		Type:        types.StringValue("filesystem"),
		Compression: types.StringValue("lz4"),
		VolSize:     types.Int64Value(0), // as read back for FILESYSTEM datasets
	}

	payload := m.updateAPIPayload(nil)

	if _, ok := payload["volsize"]; ok {
		t.Errorf("update payload for FILESYSTEM dataset must not include \"volsize\", got: %v", payload)
	}
}

// TestDatasetAPIPayloadIncludesVolsizeForVolume verifies that a real,
// non-zero volsize (as used by type=VOLUME datasets) still makes it into
// the payload - the fix must not suppress legitimate volsize values.
func TestDatasetAPIPayloadIncludesVolsizeForVolume(t *testing.T) {
	m := &DatasetModel{
		Name:    types.StringValue("tank/myzvol"),
		Type:    types.StringValue("volume"),
		VolSize: types.Int64Value(1073741824),
	}

	payload := m.apiPayload()

	if payload["volsize"] != int64(1073741824) {
		t.Errorf("expected volsize=1073741824 for VOLUME dataset, got %v", payload["volsize"])
	}
}

// createAPIPayload fills the create-only defaults: inherit_encryption = false
// when encryption is set (B-042), aclmode = DISCARD for a POSIX/OFF acltype
// (B-148). A configured value is never overridden.
func TestDatasetCreateAPIPayloadDefaults(t *testing.T) {
	m := &DatasetModel{Name: types.StringValue("tank/x"), Encryption: types.BoolValue(false), AClType: types.StringValue("posix")}
	p := m.createAPIPayload()
	if p["inherit_encryption"] != false || p["aclmode"] != "DISCARD" {
		t.Errorf("payload = %v, want inherit_encryption=false aclmode=DISCARD", p)
	}
	m = &DatasetModel{Name: types.StringValue("tank/x"), AClType: types.StringValue("OFF"), ACLMode: types.StringValue("RESTRICTED"),
		Encryption: types.BoolUnknown(), InheritEncryption: types.BoolValue(true)}
	p = m.createAPIPayload()
	if p["aclmode"] != "RESTRICTED" || p["inherit_encryption"] != true {
		t.Errorf("configured values overridden: %v", p)
	}
	m = &DatasetModel{Name: types.StringValue("tank/x"), AClType: types.StringValue("nfsv4")}
	p = m.createAPIPayload()
	if _, ok := p["aclmode"]; ok {
		t.Errorf("NFSV4 payload carries aclmode: %v", p)
	}
	if _, ok := p["inherit_encryption"]; ok {
		t.Errorf("payload without encryption carries inherit_encryption: %v", p)
	}
}

// StripUnchanged keeps only changed keys, and keeps a configured
// refreservation when volsize changes. (B-038)
func TestStripUnchanged(t *testing.T) {
	p := map[string]any{"volsize": int64(2), "refreservation": int64(5), "sync": "ALWAYS", "comments": ""}
	StripUnchanged(p, map[string]any{"volsize": int64(1), "refreservation": int64(5), "sync": "STANDARD", "comments": ""})
	if len(p) != 3 || p["refreservation"] != int64(5) || p["sync"] != "ALWAYS" || p["volsize"] != int64(2) {
		t.Errorf("payload = %v", p)
	}
	p = map[string]any{"volsize": int64(1), "refreservation": int64(5)}
	StripUnchanged(p, map[string]any{"volsize": int64(1), "refreservation": int64(5)})
	if len(p) != 0 {
		t.Errorf("unchanged payload = %v, want empty", p)
	}
}

// 26.0 writes the literal "INHERIT" description; it reads back as "". (B-147)
func TestCommentsValue(t *testing.T) {
	for in, want := range map[string]string{"INHERIT": "", "": "", "hello": "hello", "inherit": "inherit"} {
		if got := CommentsValue(in); got != want {
			t.Errorf("CommentsValue(%q) = %q, want %q", in, got, want)
		}
	}
}

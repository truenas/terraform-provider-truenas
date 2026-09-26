// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

// Package dataset contains unit tests for the truenas_dataset resource model.
package dataset

import (
	"encoding/json"
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

	payload := m.updateAPIPayload()

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

	payload := m.updateAPIPayload()

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

// TestDatasetPayloadIncludesZeroSpecialSmallBlockSize pins the difference
// between special_small_block_size and volsize. For volsize a zero is an
// artefact of reading back a FILESYSTEM dataset and must be suppressed; for
// special_small_block_size zero is a real, meaningful setting (it disables
// routing small blocks to the pool's special vdev), so a configured 0 has
// to reach the payload. Guarding this one on != 0 as well would make
// "special_small_block_size = 0" silently unenforceable.
func TestDatasetPayloadIncludesZeroSpecialSmallBlockSize(t *testing.T) {
	m := &DatasetModel{
		Name:                  types.StringValue("tank/mydata"),
		SpecialSmallBlockSize: types.StringValue("0"),
	}

	payload := m.apiPayload()

	v, ok := payload["special_small_block_size"]
	if !ok {
		t.Fatalf("configured special_small_block_size=0 must reach the payload, got: %v", payload)
	}
	if v != int64(0) {
		t.Errorf("expected special_small_block_size=0, got %v", v)
	}
}

// TestDatasetPayloadOmitsUnsetSpecialSmallBlockSize covers the ordinary
// case: a dataset whose configuration says nothing about the property must
// not send the key at all, so TrueNAS leaves it inherited.
func TestDatasetPayloadOmitsUnsetSpecialSmallBlockSize(t *testing.T) {
	m := &DatasetModel{
		Name:                  types.StringValue("tank/mydata"),
		SpecialSmallBlockSize: types.StringNull(),
	}

	if _, ok := m.apiPayload()["special_small_block_size"]; ok {
		t.Errorf("null special_small_block_size must be omitted from the payload")
	}
}

// TestDatasetResponseToModelKeepsLocalSpecialSmallBlockSize verifies that a
// property genuinely set on this dataset is read into state.
func TestDatasetResponseToModelKeepsLocalSpecialSmallBlockSize(t *testing.T) {
	api := &apiResponse{Name: "tank/mydata", Type: "FILESYSTEM"}
	api.SpecialSmallBlockSize.Parsed = propertyBytes{Set: true, Value: 16384}
	api.SpecialSmallBlockSize.Source = "LOCAL"

	m := &DatasetModel{}
	(&DatasetResource{}).responseToModel(api, m)

	if m.SpecialSmallBlockSize.IsNull() {
		t.Fatal("a LOCAL special_small_block_size must be recorded in state")
	}
	if got := m.SpecialSmallBlockSize.ValueString(); got != "16384" {
		t.Errorf("expected \"16384\", got %q", got)
	}
	if got := m.updateAPIPayload()["special_small_block_size"]; got != int64(16384) {
		t.Errorf("expected special_small_block_size=16384 sent as an integer, got %#v", got)
	}
}

// TestDatasetResponseToModelInheritsSpecialSmallBlockSize is the
// regression this attribute's design exists for. pool.dataset.get_instance
// reports the *effective* value for an inherited property, so reading the
// value alone would put a number in state for every dataset in the pool.
// Because the attribute is Optional+Computed, that number would then be
// written back by the next pool.dataset.update and convert an inherited
// property into a local one behind the user's back - the same shape of bug
// as the volsize regression above, except that a zero check cannot catch
// it, because 0 is a legitimate value here. "source" is the discriminator:
// anything other than LOCAL reads back as "INHERIT", and sending that back
// keeps the property inherited.
func TestDatasetResponseToModelInheritsSpecialSmallBlockSize(t *testing.T) {
	for _, source := range []string{"INHERITED", "DEFAULT", "RECEIVED"} {
		t.Run(source, func(t *testing.T) {
			api := &apiResponse{Name: "tank/mydata", Type: "FILESYSTEM"}
			api.SpecialSmallBlockSize.Parsed = propertyBytes{Set: true, Value: 16384}
			api.SpecialSmallBlockSize.Source = source

			m := &DatasetModel{}
			(&DatasetResource{}).responseToModel(api, m)

			if got := m.SpecialSmallBlockSize; got.IsNull() || got.ValueString() != "INHERIT" {
				t.Fatalf("source=%s must read back as INHERIT, got %v", source, got)
			}
			if got := m.updateAPIPayload()["special_small_block_size"]; got != "INHERIT" {
				t.Errorf("source=%s must be sent as INHERIT, never as the effective value, got %#v", source, got)
			}
		})
	}
}

// TestDatasetResponseToModelHandlesAbsentSpecialSmallBlockSize covers a
// get_instance response that omits the property entirely, as for a dataset
// type that does not carry it. That is not "inherited": state stays null
// and nothing is sent.
func TestDatasetResponseToModelHandlesAbsentSpecialSmallBlockSize(t *testing.T) {
	api := &apiResponse{Name: "tank/myzvol", Type: "VOLUME"}

	m := &DatasetModel{}
	(&DatasetResource{}).responseToModel(api, m)

	if !m.SpecialSmallBlockSize.IsNull() {
		t.Errorf("an absent special_small_block_size must be null, got %v", m.SpecialSmallBlockSize)
	}
	if _, ok := m.updateAPIPayload()["special_small_block_size"]; ok {
		t.Error("an absent special_small_block_size must not be sent")
	}
}

// TestDatasetIntegerPropertiesPayload covers the integer-valued properties
// held as strings: a number is sent as a JSON integer (0 included, see
// TestDatasetPayloadIncludesZeroSpecialSmallBlockSize), "INHERIT" in any
// case as the string "INHERIT", and null not at all.
func TestDatasetIntegerPropertiesPayload(t *testing.T) {
	for _, tc := range []struct {
		attr string
		put  func(*DatasetModel, types.String)
	}{
		{"special_small_block_size", func(m *DatasetModel, v types.String) { m.SpecialSmallBlockSize = v }},
	} {
		for _, c := range []struct {
			in   types.String
			want any
		}{
			{types.StringValue("2"), int64(2)},
			{types.StringValue("0"), int64(0)},
			{types.StringValue("INHERIT"), "INHERIT"},
			{types.StringValue("inherit"), "INHERIT"},
			{types.StringNull(), nil},
			{types.StringUnknown(), nil},
		} {
			t.Run(tc.attr+"/"+c.in.String(), func(t *testing.T) {
				m := &DatasetModel{Name: types.StringValue("tank/mydata")}
				tc.put(m, c.in)
				got, ok := m.apiPayload()[tc.attr]
				if c.want == nil {
					if ok {
						t.Fatalf("%s=%v must be omitted, got %#v", tc.attr, c.in, got)
					}
					return
				}
				if got != c.want {
					t.Errorf("%s=%v: sent %#v, want %#v", tc.attr, c.in, got, c.want)
				}
			})
		}
	}
}

// TestDatasetInheritKeepsConfiguredCase: the configuration may write
// "inherit" in any case, and the read, which reports "INHERIT", must keep
// that spelling or every plan would show "inherit" -> "INHERIT". A value
// that drifted from LOCAL to inherited reads back as "INHERIT".
func TestDatasetInheritKeepsConfiguredCase(t *testing.T) {
	for _, c := range []struct {
		name    string
		current types.String
		want    types.String
	}{
		{"configured inherit", types.StringValue("inherit"), types.StringValue("inherit")},
		{"configured Inherit", types.StringValue("Inherit"), types.StringValue("Inherit")},
		{"drifted to inherited", types.StringValue("16384"), types.StringValue("INHERIT")},
		{"import", types.StringNull(), types.StringValue("INHERIT")},
	} {
		t.Run(c.name, func(t *testing.T) {
			api := &apiResponse{Name: "tank/mydata", Type: "FILESYSTEM"}
			api.SpecialSmallBlockSize.Parsed = propertyBytes{Set: true, Value: 0}
			api.SpecialSmallBlockSize.Source = "INHERITED"

			m := &DatasetModel{SpecialSmallBlockSize: c.current}
			if diags := (&DatasetResource{}).responseToModel(api, m); diags.HasError() {
				t.Fatalf("unexpected error: %v", diags)
			}
			if !m.SpecialSmallBlockSize.Equal(c.want) {
				t.Errorf("got %v, want %v", m.SpecialSmallBlockSize, c.want)
			}
		})
	}
}

// TestDatasetUpdateSendsInheritOnlyOnChange: an inherited property reads
// back as "INHERIT" and plans as it (Optional+Computed), so the update drops
// a property that is "INHERIT" on both sides, and sends it when it reverts a
// LOCAL value.
func TestDatasetUpdateSendsInheritOnlyOnChange(t *testing.T) {
	for _, c := range []struct {
		name        string
		plan, state types.String
		want        any
	}{
		{"inherited, unchanged", types.StringValue("inherit"), types.StringValue("INHERIT"), nil},
		{"reverted to inherited", types.StringValue("INHERIT"), types.StringValue("16384"), "INHERIT"},
		{"set locally", types.StringValue("16384"), types.StringValue("INHERIT"), int64(16384)},
		{"unchanged number", types.StringValue("16384"), types.StringValue("16384"), int64(16384)},
		{"absent", types.StringNull(), types.StringNull(), nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			state := &DatasetModel{Name: types.StringValue("tank/mydata"), SpecialSmallBlockSize: c.state}
			plan := &DatasetModel{Name: types.StringValue("tank/mydata"), SpecialSmallBlockSize: c.plan}
			p := plan.updateAPIPayload()
			dropUnchangedInherit(p, plan, state)

			got, ok := p["special_small_block_size"]
			if c.want == nil {
				if ok {
					t.Fatalf("must not be sent, got %#v", got)
				}
				return
			}
			if got != c.want {
				t.Errorf("sent %#v, want %#v", got, c.want)
			}
		})
	}
}

// TestPropertyBytesAcceptsBothWireShapes is the regression test for the
// failure that the first live acceptance run hit: every dataset test failed
// at json.Unmarshal because special_small_block_size's "parsed" arrives as a
// JSON string ("0"), while the sibling byte-valued property recordsize
// arrives as a JSON number (1048576) in the very same response. Both forms
// have to decode.
func TestPropertyBytesAcceptsBothWireShapes(t *testing.T) {
	cases := []struct {
		name    string
		json    string
		wantSet bool
		want    int64
	}{
		{"string zero, as probed on 25.10", `{"parsed": "0"}`, true, 0},
		{"string decimal", `{"parsed": "16384"}`, true, 16384},
		{"string with binary suffix", `{"parsed": "16K"}`, true, 16384},
		{"number, as recordsize returns", `{"parsed": 1048576}`, true, 1048576},
		{"null", `{"parsed": null}`, false, 0},
		{"key absent entirely", `{}`, false, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got struct {
				Parsed propertyBytes `json:"parsed"`
			}
			if err := json.Unmarshal([]byte(tc.json), &got); err != nil {
				t.Fatalf("unmarshalling %s: %v", tc.json, err)
			}
			if got.Parsed.Set != tc.wantSet {
				t.Fatalf("Set = %v, want %v", got.Parsed.Set, tc.wantSet)
			}
			if got.Parsed.Value != tc.want {
				t.Errorf("Value = %d, want %d", got.Parsed.Value, tc.want)
			}
		})
	}
}

// TestPropertyBytesRejectsNonNumericString makes sure a value that is not a
// byte count surfaces as a decode error rather than a silent zero, which
// would read back as "not set" and hide a real API change.
func TestPropertyBytesRejectsNonNumericString(t *testing.T) {
	var got struct {
		Parsed propertyBytes `json:"parsed"`
	}
	if err := json.Unmarshal([]byte(`{"parsed": "INHERIT"}`), &got); err == nil {
		t.Fatal("expected an error for a non-numeric parsed value, got none")
	}
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package zvol

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestZvolSchema(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["volsize"]
	if !ok {
		t.Fatal("schema missing 'volsize' attribute")
	}

	i64, ok := attr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("volsize attribute is %T, want schema.Int64Attribute", attr)
	}
	if !i64.IsRequired() {
		t.Error("volsize attribute should be Required")
	}
}

func TestZvolAPIPayload(t *testing.T) {
	m := &ZvolModel{
		ID:           types.StringValue("tank/myvol"),
		Name:         types.StringValue("tank/myvol"),
		VolSize:      types.Int64Value(1073741824),
		VolBlockSize: types.Int64Null(),
		Compression:  types.StringValue("lz4"),
		Sync:         types.StringValue("standard"),
		Dedup:        types.StringValue("off"),
		Sparse:       types.BoolNull(),
		Comments:     types.StringNull(),
		Pool:         types.StringNull(),
		Encrypted:    types.BoolNull(),
	}

	p := m.apiPayload()

	if v, ok := p["type"]; !ok || v != "VOLUME" {
		t.Errorf("payload type = %v, want VOLUME", p["type"])
	}
	if v, ok := p["volsize"]; !ok || v != int64(1073741824) {
		t.Errorf("payload volsize = %v, want 1073741824", p["volsize"])
	}
	if _, ok := p["sparse"]; ok {
		t.Error("payload must not include sparse when it is null")
	}
	if _, ok := p["volblocksize"]; ok {
		t.Error("payload must not include volblocksize when null")
	}
	if v, ok := p["compression"]; !ok || v != "LZ4" {
		t.Errorf("payload compression = %v, want LZ4", p["compression"])
	}
	if _, ok := p["dedup"]; ok {
		t.Error("payload must use 'deduplication' key, not 'dedup'")
	}
	if v, ok := p["deduplication"]; !ok || v != "OFF" {
		t.Errorf("payload deduplication = %v, want OFF", p["deduplication"])
	}
}

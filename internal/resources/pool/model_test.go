// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

// Package pool contains unit tests for the truenas_pool resource model.
package pool

import (
	"encoding/json"
	"testing"
)

// TestAutotrimParsedUnmarshalJSON is a regression test for a live
// acceptance failure: pool.query / pool.get_instance return autotrim as a
// ZFS property object whose "parsed" field is the string "on"/"off", not a
// JSON bool. A naive `bool` struct field failed with "json: cannot
// unmarshal string into Go struct field .autotrim.parsed of type bool".
func TestAutotrimParsedUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		want    bool
		wantErr bool
	}{
		{name: "string on", json: `"on"`, want: true},
		{name: "string off", json: `"off"`, want: false},
		{name: "string ON uppercase", json: `"ON"`, want: true},
		{name: "string OFF uppercase", json: `"OFF"`, want: false},
		{name: "bool true", json: `true`, want: true},
		{name: "bool false", json: `false`, want: false},
		{name: "unrecognized string", json: `"maybe"`, wantErr: true},
		{name: "unsupported type", json: `1`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got autotrimParsed
			err := json.Unmarshal([]byte(tt.json), &got)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for input %s, got none", tt.json)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for input %s: %v", tt.json, err)
			}
			if bool(got) != tt.want {
				t.Errorf("input %s: expected %v, got %v", tt.json, tt.want, bool(got))
			}
		})
	}
}

// TestPoolAPIAutotrimPropertyObjectDecode verifies that a full poolAPI
// response, shaped like the live pool.query payload (autotrim.parsed as a
// string), decodes without error and maps to the correct bool.
func TestPoolAPIAutotrimPropertyObjectDecode(t *testing.T) {
	raw := []byte(`{
		"id": 1,
		"name": "tank",
		"guid": "123",
		"status": "ONLINE",
		"healthy": true,
		"path": "/mnt/tank",
		"size": 100,
		"free": 50,
		"allocated": 50,
		"autotrim": {"parsed": "on", "rawvalue": "on", "value": "on", "source": "DEFAULT"},
		"topology": {"data": [], "log": [], "cache": [], "spare": []}
	}`)

	var api poolAPI
	if err := json.Unmarshal(raw, &api); err != nil {
		t.Fatalf("failed to decode pool API response: %v", err)
	}
	if !bool(api.AutoTrim.Parsed) {
		t.Errorf("expected AutoTrim.Parsed=true, got false")
	}
}

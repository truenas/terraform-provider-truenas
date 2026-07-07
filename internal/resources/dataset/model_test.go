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

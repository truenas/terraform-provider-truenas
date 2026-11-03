// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func TestListMetadata(t *testing.T) {
	l := &DatasetListResource{}
	req := resource.MetadataRequest{ProviderTypeName: "truenas"}
	resp := &resource.MetadataResponse{}
	l.Metadata(context.Background(), req, resp)
	if resp.TypeName != "truenas_dataset" {
		t.Errorf("TypeName = %q, want %q", resp.TypeName, "truenas_dataset")
	}
}

func TestDatasetIdentitySchema(t *testing.T) {
	r := &DatasetResource{}
	resp := &resource.IdentitySchemaResponse{}
	r.IdentitySchema(context.Background(), resource.IdentitySchemaRequest{}, resp)
	// The identity is the ZFS guid (F8: a rename keeps it), schema v1.
	if _, ok := resp.IdentitySchema.Attributes["guid"]; !ok {
		t.Error("identity schema missing 'guid' attribute")
	}
	if resp.IdentitySchema.Version != IdentityVersion || IdentityVersion != 1 {
		t.Errorf("identity schema version = %d, want 1", resp.IdentitySchema.Version)
	}
}

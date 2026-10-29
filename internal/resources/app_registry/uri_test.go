// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package app_registry

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/truenas/terraform-provider-truenas/internal/normalize"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestSchema_URISemanticEquality guards #47 and B-122: TrueNAS appends "/" to
// an http(s) registry URI. The URL type keeps the configured spelling after
// apply, and KeepStateIfEquivalent plans no change after import (state holds
// the server's spelling).
func TestSchema_URISemanticEquality(t *testing.T) {
	a := resourceSchema().Attributes["uri"].(schema.StringAttribute)
	if _, ok := a.CustomType.(normalize.URLType); !ok {
		t.Errorf("uri CustomType = %T, want normalize.URLType", a.CustomType)
	}
	if len(a.PlanModifiers) != 2 {
		t.Errorf("uri plan modifiers = %d, want UseStateForUnknown + KeepStateIfEquivalent", len(a.PlanModifiers))
	}
	eq, d := normalize.NewURLValue("http://127.0.0.1:5000").StringSemanticEquals(context.Background(), normalize.NewURLValue("http://127.0.0.1:5000/"))
	if d.HasError() || !eq {
		t.Error("trailing-slash URI spellings are not equal")
	}
}

// TestResponseToModel_RedactedUsername guards B-032: a READONLY_ADMIN session
// gets username as "********". The resource keeps the prior value (null after
// import) and the data source reports null, instead of storing the mask.
func TestResponseToModel_RedactedUsername(t *testing.T) {
	api := &appRegistryAPI{Name: "r", URI: "https://ghcr.io/", Username: "********"}

	m := &AppRegistryModel{Username: types.StringValue("alice")}
	responseToModel(api, m)
	if got := m.Username.ValueString(); got != "alice" {
		t.Errorf("resource username = %q, want the prior alice", got)
	}
	m = &AppRegistryModel{}
	responseToModel(api, m)
	if !m.Username.IsNull() {
		t.Errorf("imported username = %v, want null", m.Username)
	}
	dm := &AppRegistryDataSourceModel{}
	responseToDataSourceModel(api, dm)
	if !dm.Username.IsNull() {
		t.Errorf("data source username = %v, want null", dm.Username)
	}

	api.Username = "bob"
	responseToModel(api, m)
	responseToDataSourceModel(api, dm)
	if m.Username.ValueString() != "bob" || dm.Username.ValueString() != "bob" {
		t.Errorf("unredacted username not read: %v %v", m.Username, dm.Username)
	}
}

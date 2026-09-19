// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acme_dns_authenticator

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

// TestAcmeDnsAuthenticatorDataSourceModel_MatchesSchema verifies that every
// tfsdk tag on AcmeDnsAuthenticatorDataSourceModel has a corresponding
// attribute in the datasource schema, and vice versa. terraform-plugin-framework
// requires an exact field/attribute match: any mismatch causes every
// datasource Read to fail with "Struct defines fields not found in object: ...".
func TestAcmeDnsAuthenticatorDataSourceModel_MatchesSchema(t *testing.T) {
	d := &AcmeDnsAuthenticatorDataSource{}
	var resp datasource.SchemaResponse
	d.Schema(context.Background(), datasource.SchemaRequest{}, &resp)

	schemaAttrs := make([]string, 0, len(resp.Schema.Attributes))
	for name := range resp.Schema.Attributes {
		schemaAttrs = append(schemaAttrs, name)
	}
	sort.Strings(schemaAttrs)

	modelType := reflect.TypeOf(AcmeDnsAuthenticatorDataSourceModel{})
	modelFields := make([]string, 0, modelType.NumField())
	for i := 0; i < modelType.NumField(); i++ {
		tag := modelType.Field(i).Tag.Get("tfsdk")
		if tag == "" {
			t.Fatalf("field %q has no tfsdk tag", modelType.Field(i).Name)
		}
		modelFields = append(modelFields, tag)
	}
	sort.Strings(modelFields)

	if !reflect.DeepEqual(schemaAttrs, modelFields) {
		t.Fatalf("AcmeDnsAuthenticatorDataSourceModel tfsdk tags %v do not match datasource schema attributes %v", modelFields, schemaAttrs)
	}
}

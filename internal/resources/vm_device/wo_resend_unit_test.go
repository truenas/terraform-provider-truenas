// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm_device

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestValidateNoSecretAttributes: a password in attributes is a plan-time
// error; other documents and unknown values pass. (B-159)
func TestValidateNoSecretAttributes(t *testing.T) {
	cases := []struct {
		v       types.String
		wantErr bool
	}{
		{types.StringValue(`{"dtype":"DISPLAY","password":"x"}`), true},
		{types.StringValue(`{"dtype":"DISPLAY","password":null}`), true},
		{types.StringValue(`{"dtype":"DISPLAY","port":5900}`), false},
		{types.StringValue(`not json`), false},
		{types.StringUnknown(), false},
		{types.StringNull(), false},
	}
	for _, c := range cases {
		resp := &validator.StringResponse{}
		validateNoSecretAttributes{}.ValidateString(context.Background(),
			validator.StringRequest{Path: path.Root("attributes"), ConfigValue: c.v}, resp)
		if resp.Diagnostics.HasError() != c.wantErr {
			t.Errorf("%v: error = %v, want %v", c.v, resp.Diagnostics.HasError(), c.wantErr)
		}
	}
}

// TestSecretsNeverRead: import (apiAttributesJSON) drops the password and the
// data source reports it as null. (B-159)
func TestSecretsNeverRead(t *testing.T) {
	api := &deviceAPI{Attributes: map[string]any{"dtype": "DISPLAY", "password": "s3cret", "port": float64(5900)}}
	got, d := apiAttributesJSON(api)
	if d.HasError() {
		t.Fatal(d)
	}
	if got != `{"dtype":"DISPLAY","port":5900}` {
		t.Errorf("import attributes = %s", got)
	}

	api = &deviceAPI{Attributes: map[string]any{"dtype": "DISPLAY", "password": "s3cret"}}
	var m VMDeviceDataSourceModel
	if d := responseToDataSourceModel(api, &m); d.HasError() {
		t.Fatal(d)
	}
	var ds map[string]any
	if err := json.Unmarshal([]byte(m.Attributes.ValueString()), &ds); err != nil {
		t.Fatal(err)
	}
	if v, ok := ds["password"]; !ok || v != nil {
		t.Errorf("data source password = %v (present %v), want null", v, ok)
	}
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package ipmi_lan

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestDropUnversionedPassword: the BMC password goes out on update only when
// password_wo_version changed. (B-141)
func TestDropUnversionedPassword(t *testing.T) {
	cases := []struct {
		name        string
		plan, prior types.Int64
		wantSent    bool
	}{
		{"unchanged", types.Int64Value(1), types.Int64Value(1), false},
		{"both unset", types.Int64Null(), types.Int64Null(), false},
		{"bumped", types.Int64Value(2), types.Int64Value(1), true},
		{"added", types.Int64Value(1), types.Int64Null(), true},
	}
	for _, c := range cases {
		m := IPMILanModel{
			Channel:           types.Int64Value(1),
			DHCP:              types.BoolValue(true),
			Password:          types.StringValue("Secret-123"),
			PasswordWOVersion: c.plan,
		}
		m.dropUnversionedPassword(IPMILanModel{PasswordWOVersion: c.prior})
		p, err := m.updatePayload()
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if _, sent := p["password"]; sent != c.wantSent {
			t.Errorf("%s: password sent = %v, want %v", c.name, sent, c.wantSent)
		}
	}
}

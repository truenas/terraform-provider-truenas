// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package user

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestPasswordResendNeeded: the write-only password goes out on update only
// when password_wo_version changed or smb is being turned on. (B-141)
func TestPasswordResendNeeded(t *testing.T) {
	m := func(ver types.Int64, smb bool) UserModel {
		return UserModel{PasswordWOVersion: ver, SMB: types.BoolValue(smb)}
	}
	v1, v2, null := types.Int64Value(1), types.Int64Value(2), types.Int64Null()
	cases := []struct {
		name        string
		plan, prior UserModel
		want        bool
	}{
		{"unchanged version", m(v1, false), m(v1, false), false},
		{"both unset", m(null, true), m(null, true), false},
		{"bumped", m(v2, false), m(v1, false), true},
		{"version added", m(v1, false), m(null, false), true},
		{"version removed", m(null, false), m(v1, false), true},
		{"smb turned on", m(v1, true), m(v1, false), true},
		{"smb turned off", m(v1, false), m(v1, true), false},
	}
	for _, c := range cases {
		if got := passwordResendNeeded(c.plan, c.prior); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

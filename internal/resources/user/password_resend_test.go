// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package user_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
	"golang.org/x/crypto/md4" //nolint:staticcheck // NT hash of the test password, to compare with smbhash
)

// userPasswordState is the part of user.query that changes when a password is
// (re-)sent: unixhash gets a new salt every time, smbhash is the NT hash of the
// password and last_password_change is the time of the last send.
type userPasswordState struct {
	UnixHash           string `json:"unixhash"`
	SMBHash            string `json:"smbhash"`
	LastPasswordChange any    `json:"last_password_change"`
}

func fetchUserPasswordState(username string) (userPasswordState, error) {
	raw, err := acctest.Client().Call(context.Background(), "user.query", [][]any{{"username", "=", username}})
	if err != nil {
		return userPasswordState{}, err
	}
	var rows []userPasswordState
	if err := json.Unmarshal(raw, &rows); err != nil {
		return userPasswordState{}, err
	}
	if len(rows) != 1 {
		return userPasswordState{}, fmt.Errorf("user %s: %d rows", username, len(rows))
	}
	return rows[0], nil
}

// ntHash is the NT hash TrueNAS stores in smbhash: MD4 over the UTF-16LE password.
func ntHash(password string) string {
	h := md4.New()
	for _, u := range utf16.Encode([]rune(password)) {
		h.Write([]byte{byte(u), byte(u >> 8)})
	}
	return strings.ToUpper(hex.EncodeToString(h.Sum(nil)))
}

// checkUserPassword records the account's password state in *got and checks
// that the stored password is want. When same is non-nil, unixhash and
// last_password_change must equal it (no re-send); when differ is non-nil,
// both must differ from it (a re-send happened).
func checkUserPassword(username, want string, got, same, differ *userPasswordState) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		cur, err := fetchUserPasswordState(username)
		if err != nil {
			return err
		}
		if cur.SMBHash != ntHash(want) {
			return fmt.Errorf("stored password is not %q (smbhash %s)", want, cur.SMBHash)
		}
		if same != nil {
			if cur.UnixHash != same.UnixHash || fmt.Sprint(cur.LastPasswordChange) != fmt.Sprint(same.LastPasswordChange) {
				return fmt.Errorf("password was re-sent: unixhash/last_password_change changed (%v -> %v)",
					same.LastPasswordChange, cur.LastPasswordChange)
			}
		}
		if differ != nil {
			if cur.UnixHash == differ.UnixHash {
				return fmt.Errorf("password was not re-sent: unixhash unchanged")
			}
			if fmt.Sprint(cur.LastPasswordChange) == fmt.Sprint(differ.LastPasswordChange) {
				return fmt.Errorf("password was re-sent but last_password_change did not move (%v)", cur.LastPasswordChange)
			}
		}
		*got = cur
		return nil
	}
}

// TestAccUser_passwordSentOnlyOnVersionChange: an update that does not change
// password_wo_version leaves the password alone (no re-hash, last_password_change
// kept, a changed password value is NOT applied); bumping the version sends it;
// turning smb on sends it too, because TrueNAS requires the password for that. (B-141)
func TestAccUser_passwordSentOnlyOnVersionChange(t *testing.T) {
	username := acctest.RandName("tf-acc-wo")
	var s1, s2, s3, s4 userPasswordState

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_11_0)},
		CheckDestroy:             testAccCheckUserDestroyed(username),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccUserResendConfig(username, "One", "Tf-Wo-Passw0rd-A!", 1, false),
				Check:  checkUserPassword(username, "Tf-Wo-Passw0rd-A!", &s1, nil, nil),
			},
			// Unrelated change plus a new password value, version unchanged:
			// nothing is re-sent, the old password stays.
			{
				Config: acctest.ProviderConfig() + testAccUserResendConfig(username, "Two", "Tf-Wo-Passw0rd-B!", 1, false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_user.test", "full_name", "Two"),
					checkUserPassword(username, "Tf-Wo-Passw0rd-A!", &s2, &s1, nil),
				),
			},
			// Version bump: the new password is sent.
			{
				Config: acctest.ProviderConfig() + testAccUserResendConfig(username, "Two", "Tf-Wo-Passw0rd-B!", 2, false),
				Check:  checkUserPassword(username, "Tf-Wo-Passw0rd-B!", &s3, nil, &s2),
			},
			// smb false -> true: the API requires the password, so it is sent
			// even though the version did not change.
			{
				Config: acctest.ProviderConfig() + testAccUserResendConfig(username, "Two", "Tf-Wo-Passw0rd-C!", 2, true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_user.test", "smb", "true"),
					checkUserPassword(username, "Tf-Wo-Passw0rd-C!", &s4, nil, &s3),
				),
			},
		},
	})
}

func testAccUserResendConfig(username, fullName, password string, version int, smb bool) string {
	return fmt.Sprintf(`
resource "truenas_user" "test" {
  username            = %q
  full_name           = %q
  password            = %q
  password_wo_version = %d
  password_disabled   = false
  home                = "/var/empty"
  shell               = "/usr/sbin/nologin"
  smb                 = %t
  group_create        = true
}
`, username, fullName, password, version, smb)
}

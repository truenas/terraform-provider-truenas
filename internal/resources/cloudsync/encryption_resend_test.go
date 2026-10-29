// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cloudsync_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// checkCloudSyncCrypt checks the encryption password/salt TrueNAS has stored
// for the task (cloudsync.get_instance returns them to an admin session).
func checkCloudSyncCrypt(wantPW, wantSalt string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		id, _ := strconv.ParseInt(s.RootModule().Resources["truenas_cloudsync_task.test"].Primary.ID, 10, 64)
		raw, err := acctest.Client().Call(context.Background(), "cloudsync.get_instance", id)
		if err != nil {
			return err
		}
		var got struct {
			Password string `json:"encryption_password"`
			Salt     string `json:"encryption_salt"`
		}
		if err := json.Unmarshal(raw, &got); err != nil {
			return err
		}
		if got.Password != wantPW || got.Salt != wantSalt {
			return fmt.Errorf("server encryption_password/salt = %q/%q, want %q/%q", got.Password, got.Salt, wantPW, wantSalt)
		}
		return nil
	}
}

// TestAccCloudSync_encryptionSentOnlyOnVersionChange: an unrelated update with
// changed encryption_password/salt and the same encryption_wo_version leaves
// the stored values alone; bumping the version sends them; turning encryption
// on sends them without a bump, because the server requires a password for
// it. (B-141)
func TestAccCloudSync_encryptionSentOnlyOnVersionChange(t *testing.T) {
	name := acctest.RandName("tf-acc-cswo")
	body := func(enc bool, pw, salt string, version int, symlinks bool) string {
		return fmt.Sprintf(`
  encryption            = %t
  encryption_password   = %q
  encryption_salt       = %q
  encryption_wo_version = %d
  follow_symlinks       = %t
`, enc, pw, salt, version, symlinks)
	}
	cfg := func(b string) string { return acctest.ProviderConfig() + cloudSyncS3Config(name, b) }
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_11_0)},
		Steps: []resource.TestStep{
			{Config: cfg(body(true, "tfwopw1", "tfwosalt1", 1, false)), Check: checkCloudSyncCrypt("tfwopw1", "tfwosalt1")},
			{
				Config: cfg(body(true, "tfwopw2", "tfwosalt2", 1, true)),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_cloudsync_task.test", "follow_symlinks", "true"),
					checkCloudSyncCrypt("tfwopw1", "tfwosalt1"),
				),
			},
			{Config: cfg(body(true, "tfwopw2", "tfwosalt2", 2, true)), Check: checkCloudSyncCrypt("tfwopw2", "tfwosalt2")},
			{Config: cfg(body(false, "tfwopw2", "tfwosalt2", 2, true)), Check: resource.TestCheckResourceAttr("truenas_cloudsync_task.test", "encryption", "false")},
			{Config: cfg(body(true, "tfwopw3", "tfwosalt3", 2, true)), Check: checkCloudSyncCrypt("tfwopw3", "tfwosalt3")},
		},
	})
}

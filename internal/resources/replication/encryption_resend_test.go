// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package replication_test

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

func checkReplicationKey(want string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		id, _ := strconv.ParseInt(s.RootModule().Resources["truenas_replication_task.test"].Primary.ID, 10, 64)
		raw, err := acctest.Client().Call(context.Background(), "replication.get_instance", id)
		if err != nil {
			return err
		}
		var got struct {
			EncryptionKey *string `json:"encryption_key"`
		}
		if err := json.Unmarshal(raw, &got); err != nil {
			return err
		}
		if got.EncryptionKey == nil || *got.EncryptionKey != want {
			return fmt.Errorf("server encryption_key = %v, want %q", got.EncryptionKey, want)
		}
		return nil
	}
}

// TestAccReplication_encryptionKeySentOnlyOnVersionChange: an unrelated update
// with a changed encryption_key and the same encryption_key_wo_version leaves
// the stored key alone; bumping the version sends it; turning own-key
// encryption back on sends it without a bump, because the server requires a
// key for it. (B-141)
func TestAccReplication_encryptionKeySentOnlyOnVersionChange(t *testing.T) {
	name := acctest.RandName("tf-acc-replwo")
	keyFile := "/root/" + name + ".key"
	enc := func(key string, version int) string {
		return fmt.Sprintf(`
  encryption                = true
  encryption_inherit        = false
  encryption_key            = %q
  encryption_key_wo_version = %d
  encryption_key_format     = "HEX"
  encryption_key_location   = %q
`, key, version, keyFile)
	}
	body := func(encBlock string, hold bool) string {
		return fmt.Sprintf(`
  auto             = false
  retention_policy = "SOURCE"
  also_include_naming_schema = ["tfwo-%%Y%%m%%d-%%H%%M"]
  hold_pending_snapshots = %t
%s`, hold, encBlock)
	}
	k1 := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	k2 := "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
	k3 := "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
	cfg := func(b string) string { return acctest.ProviderConfig() + replConfig(name, b) }
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_11_0)},
		CheckDestroy:             testAccCheckReplicationDestroyed(name),
		Steps: []resource.TestStep{
			{Config: cfg(body(enc(k1, 1), false)), Check: checkReplicationKey(k1)},
			{
				Config: cfg(body(enc(k2, 1), true)),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_replication_task.test", "hold_pending_snapshots", "true"),
					checkReplicationKey(k1),
				),
			},
			{Config: cfg(body(enc(k2, 2), true)), Check: checkReplicationKey(k2)},
			{Config: cfg(body(fmt.Sprintf("  encryption = false\n  encryption_key = %q\n  encryption_key_wo_version = 2\n", k2), true)), Check: resource.TestCheckResourceAttr("truenas_replication_task.test", "encryption", "false")},
			{Config: cfg(body(enc(k3, 2), true)), Check: checkReplicationKey(k3)},
		},
	})
}

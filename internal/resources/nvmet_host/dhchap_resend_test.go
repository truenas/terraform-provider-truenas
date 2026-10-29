// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_host_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccNVMetHost_keySentOnlyOnVersionChange: an unrelated update with a
// changed dhchap_key and the same dhchap_key_wo_version leaves the stored key
// alone; adding the controller key sends it without a version; bumping the
// version sends the new key. (B-141)
func TestAccNVMetHost_keySentOnlyOnVersionChange(t *testing.T) {
	nqn := "nqn.2014-08.org.example:" + acctest.RandName("tfacchostwo")
	body := func(key string, ver int, hash, ctrl string) string {
		return fmt.Sprintf("  dhchap_key = %q\n  dhchap_key_wo_version = %d\n  dhchap_hash = %q\n%s", key, ver, hash, ctrl)
	}
	ctrl := fmt.Sprintf("  dhchap_ctrl_key = %q", dhCtrl1)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: hostCfg(nqn, body(dhKey1, 1, "SHA-256", "")), Check: checkServerHostField(t, "dhchap_key", dhKey1)},
			{
				Config: hostCfg(nqn, body(dhKey2, 1, "SHA-384", "")),
				Check: resource.ComposeTestCheckFunc(
					checkServerHostField(t, "dhchap_hash", "SHA-384"),
					checkServerHostField(t, "dhchap_key", dhKey1),
				),
			},
			{
				Config: hostCfg(nqn, body(dhKey2, 1, "SHA-384", ctrl)),
				Check: resource.ComposeTestCheckFunc(
					checkServerHostField(t, "dhchap_ctrl_key", dhCtrl1),
					checkServerHostField(t, "dhchap_key", dhKey1),
				),
			},
			{
				Config: hostCfg(nqn, body(dhKey2, 2, "SHA-384", ctrl)),
				Check: resource.ComposeTestCheckFunc(
					checkServerHostField(t, "dhchap_key", dhKey2),
					checkServerHostField(t, "dhchap_ctrl_key", dhCtrl1),
				),
			},
			// Removing the controller key still clears it.
			{Config: hostCfg(nqn, body(dhKey2, 2, "SHA-384", "")), Check: checkServerHostField(t, "dhchap_ctrl_key", nil)},
		},
	})
}

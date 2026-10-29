// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package kerberos_keytab_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func checkKeytabFile(want string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		id, _ := strconv.ParseInt(s.RootModule().Resources["truenas_kerberos_keytab.test"].Primary.ID, 10, 64)
		raw, err := acctest.Client().Call(context.Background(), "kerberos.keytab.get_instance", id)
		if err != nil {
			return err
		}
		var got struct {
			File string `json:"file"`
		}
		if err := json.Unmarshal(raw, &got); err != nil {
			return err
		}
		if got.File != want {
			return fmt.Errorf("server keytab file is not the expected one (%d bytes, want %d)", len(got.File), len(want))
		}
		return nil
	}
}

// TestAccKerberosKeytab_fileSentOnlyOnVersionChange: a rename with a different
// file_wo and the same file_wo_version leaves the stored keytab alone; bumping
// the version sends the new one. (B-141) Needs a second keytab in
// TRUENAS_DS_KEYTAB2_B64 (any valid keytab other than TRUENAS_DS_KEYTAB_B64's).
func TestAccKerberosKeytab_fileSentOnlyOnVersionChange(t *testing.T) {
	a := testAccKeytabPreCheck(t)
	b := os.Getenv("TRUENAS_DS_KEYTAB2_B64")
	if b == "" || b == a {
		t.Skip("set TRUENAS_DS_KEYTAB2_B64 to a second, different keytab")
	}
	name := acctest.RandName("tf-acc-keytabwo")
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_11_0)},
		CheckDestroy:             testAccCheckKerberosKeytabDestroyed,
		Steps: []resource.TestStep{
			{Config: acctest.ProviderConfig() + testAccKerberosKeytabWOConfig(name, a, 1), Check: checkKeytabFile(a)},
			{
				Config: acctest.ProviderConfig() + testAccKerberosKeytabWOConfig(name+"-r", b, 1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_kerberos_keytab.test", "name", name+"-r"),
					checkKeytabFile(a),
				),
			},
			{Config: acctest.ProviderConfig() + testAccKerberosKeytabWOConfig(name+"-r", b, 2), Check: checkKeytabFile(b)},
		},
	})
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package kerberos_keytab_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccKerberosKeytab_importFileWO imports a keytab managed through the
// write-only file_wo. Import cannot see the configuration, and it used to read
// the keytab secret into `file` in state (B-172); it must leave file out, and
// the re-applied configuration only records file_wo_version in place.
func TestAccKerberosKeytab_importFileWO(t *testing.T) {
	fileB64 := testAccKeytabPreCheck(t)
	name := acctest.RandName("tffix-keytab-wo")
	addr := "truenas_kerberos_keytab.test"
	cfg := acctest.ProviderConfig() + testAccKerberosKeytabWOConfig(name, fileB64, 1)

	steps := []resource.TestStep{
		{Config: cfg},
		{
			ResourceName: addr,
			ImportState:  true,
			ImportStateCheck: func(states []*terraform.InstanceState) error {
				if len(states) != 1 {
					return fmt.Errorf("imported %d instances, want 1", len(states))
				}
				if v, ok := states[0].Attributes["file"]; ok && v != "" {
					return fmt.Errorf("import stored the keytab secret in file (%d chars)", len(v))
				}
				return nil
			},
		},
	}
	steps = append(steps, acctest.ImportReapplyNoop(addr, cfg, "file_wo_version")...)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckKerberosKeytabDestroyed,
		Steps:                    steps,
	})
}

// TestAccKerberosKeytab_importFile imports a keytab managed through file:
// import leaves file null, and the next apply records it in place (never a
// replacement), after which the plan is empty.
func TestAccKerberosKeytab_importFile(t *testing.T) {
	fileB64 := testAccKeytabPreCheck(t)
	name := acctest.RandName("tffix-keytab-file")
	cfg := acctest.ProviderConfig() + testAccKerberosKeytabConfig(name, fileB64)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckKerberosKeytabDestroyed,
		Steps:                    acctest.ImportReapplyNoop("truenas_kerberos_keytab.test", cfg, "file"),
	})
}

// readonlyProvider is the only provider block of a test run with a
// READONLY_ADMIN API key (TRUENAS_RO_API_KEY / TRUENAS_RO_USERNAME). It is not
// an alias next to the full-admin provider: with both blocks in one config the
// same test passed and failed on identical code (live, .70), i.e. the
// read-only data source was sometimes served the full-admin client.
func readonlyProvider() string {
	return fmt.Sprintf(`
provider "truenas" {
  endpoint = %q
  insecure = true
  api_key  = %q
  username = %q
}
`, acctest.Endpoint(), os.Getenv("TRUENAS_RO_API_KEY"), os.Getenv("TRUENAS_RO_USERNAME"))
}

// TestAccKerberosKeytabDataSource_readonly reads a keytab through a
// READONLY_ADMIN key (e.g. a CI plan job). The API returns file="********" to
// that session; the data source must report null, not the mask as if it were
// keytab data (B-028). The keytab is created and deleted through the
// full-admin API client. The resource side (a read-only refresh keeps the
// state value instead of planning a write of the mask) is covered by
// TestResponseToModel_FileSecretHandling.
func TestAccKerberosKeytabDataSource_readonly(t *testing.T) {
	fileB64 := testAccKeytabPreCheck(t)
	if os.Getenv("TRUENAS_RO_API_KEY") == "" || os.Getenv("TRUENAS_RO_USERNAME") == "" {
		t.Skip("Set TRUENAS_RO_API_KEY and TRUENAS_RO_USERNAME (a READONLY_ADMIN user's API key) to run")
	}
	name := acctest.RandName("tffix-keytab-ro")
	raw, err := acctest.Client().Call(context.Background(), "kerberos.keytab.create",
		map[string]any{"name": name, "file": fileB64})
	if err != nil {
		t.Fatalf("create keytab: %v", err)
	}
	var created struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(raw, &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	t.Cleanup(func() {
		if _, err := acctest.Client().Call(context.Background(), "kerberos.keytab.delete", created.ID); err != nil {
			t.Errorf("delete keytab %d: %v", created.ID, err)
		}
	})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: readonlyProvider() + fmt.Sprintf(`
data "truenas_kerberos_keytab" "ro" {
  name = %q
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_kerberos_keytab.ro", "id", fmt.Sprint(created.ID)),
					resource.TestCheckNoResourceAttr("data.truenas_kerberos_keytab.ro", "file"),
				),
			},
		},
	})
}

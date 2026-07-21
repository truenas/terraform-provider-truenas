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
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// testAccKeytabPreCheck runs the standard acctest.PreCheck and additionally
// requires TRUENAS_DS_KEYTAB_B64 (base64 of a real Kerberos keytab, e.g.
// generated on the Samba AD DC test box via `samba-tool domain exportkeytab
// /tmp/tfacc.keytab --principal=Administrator@TFTEST.LAN` then `base64 -w0
// /tmp/tfacc.keytab`). Skipping (rather than failing) when it is unset lets
// plain Tier-1 acceptance sweeps stay green without a domain controller
// available.
func testAccKeytabPreCheck(t *testing.T) string {
	t.Helper()
	acctest.PreCheck(t)
	b64 := os.Getenv("TRUENAS_DS_KEYTAB_B64")
	if b64 == "" {
		t.Skip("Set TRUENAS_DS_KEYTAB_B64 (base64 of a real Kerberos keytab) to run TestAccKerberosKeytab_basic; " +
			"see TESTING.md's directory-services test environment section for how to generate one against the " +
			"tftest-dc AD DC.")
	}
	return b64
}

// TestAccKerberosKeytab_basic creates a Kerberos keytab entry from a real
// keytab exported from a live Samba AD domain controller, checks its
// attributes (including that "file" round-trips intact into state — see
// model.go's kerberosKeytabAPI doc comment for the probe that established
// this), renames it in place, imports it by numeric id, and verifies
// destruction via a live kerberos.keytab.query.
func TestAccKerberosKeytab_basic(t *testing.T) {
	fileB64 := testAccKeytabPreCheck(t)

	name := acctest.RandName("tf-acc-keytab")
	renamed := name + "-renamed"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckKerberosKeytabDestroyed(),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccKerberosKeytabConfig(name, fileB64),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_kerberos_keytab.test", "id"),
					resource.TestCheckResourceAttr("truenas_kerberos_keytab.test", "name", name),
					resource.TestCheckResourceAttr("truenas_kerberos_keytab.test", "file", fileB64),
					testAccStashKeytabID("truenas_kerberos_keytab.test"),
				),
			},
			// Rename in place — must not require replacement.
			{
				Config: acctest.ProviderConfig() + testAccKerberosKeytabConfig(renamed, fileB64),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_kerberos_keytab.test", "name", renamed),
					resource.TestCheckResourceAttr("truenas_kerberos_keytab.test", "file", fileB64),
				),
			},
			// Import by numeric id. "file" is a normal Sensitive attribute
			// (not WriteOnly — the probe showed kerberos.keytab.query
			// returns it intact), so it is verified like any other
			// attribute rather than ignored.
			{
				ResourceName:      "truenas_kerberos_keytab.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccKerberosKeytabConfig(name, fileB64 string) string {
	return fmt.Sprintf(`
resource "truenas_kerberos_keytab" "test" {
  name = %q
  file = %q
}
`, name, fileB64)
}

// stashedKeytabID carries the numeric id captured before the resource is
// renamed/destroyed forward to CheckDestroy, since the resource's own name
// changes mid-test and TrueNAS assigns no other stable external handle
// (mirrors the pattern used by truenas_api_key's acceptance test).
var stashedKeytabID string

func testAccStashKeytabID(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %s not found in state", resourceName)
		}
		id, ok := rs.Primary.Attributes["id"]
		if !ok || id == "" {
			return fmt.Errorf("resource %s has no id attribute", resourceName)
		}
		stashedKeytabID = id
		return nil
	}
}

func testAccCheckKerberosKeytabDestroyed() resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if stashedKeytabID == "" {
			return fmt.Errorf("no keytab id was captured during the test run")
		}
		id, err := strconv.ParseInt(stashedKeytabID, 10, 64)
		if err != nil {
			return fmt.Errorf("parsing keytab id %q: %v", stashedKeytabID, err)
		}
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "kerberos.keytab.query", [][]any{{"id", "=", id}})
		if err != nil {
			return fmt.Errorf("error checking Kerberos keytab id=%d: %v", id, err)
		}
		var results []struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing kerberos.keytab.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("Kerberos keytab id=%s still exists", stashedKeytabID)
		}
		return nil
	}
}

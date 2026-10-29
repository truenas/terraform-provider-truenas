// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package smb_config_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccSMBConfigDataSource_basic reads the current TrueNAS SMB
// configuration through the truenas_smb_config datasource only. It never
// writes: SMB is a system-critical singleton service (existing shares and
// domain membership may depend on it), and the box's real SMB configuration
// must not be overwritten by an acceptance test run.
//
// Requires:
//   - TF_ACC=1
//   - TRUENAS_API_KEY (or TRUENAS_USERNAME+TRUENAS_PASSWORD) set for
//     acctest.PreCheck
//   - A running TrueNAS instance
func TestAccSMBConfigDataSource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure a TrueNAS instance is available")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
data "truenas_smb_config" "test" {}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_smb_config.test", "id", "smb_config"),
					resource.TestCheckResourceAttrSet("data.truenas_smb_config.test", "netbiosname"),
					resource.TestCheckResourceAttrSet("data.truenas_smb_config.test", "server_sid"),
				),
			},
		},
	})
}

// smbConfigOriginal captures every smb.config field
// TestAccSMBConfig_setAndRestore writes. The test configuration sets each of
// them to the box's own value (only description changes), and the cleanup
// writes all of them back, so the test can never leave a setting such as
// netbiosname behind. (An earlier version hard-coded netbiosname =
// "TFTESTNAS", workgroup, masks and so on, and restored only description.)
type smbConfigOriginal struct {
	Description      string   `json:"description"`
	AaplExtensions   bool     `json:"aapl_extensions"`
	AdminGroup       *string  `json:"admin_group"`
	BindIP           []string `json:"bindip"`
	Debug            bool     `json:"debug"`
	Dirmask          string   `json:"dirmask"`
	Encryption       string   `json:"encryption"`
	Filemask         string   `json:"filemask"`
	Guest            string   `json:"guest"`
	Localmaster      bool     `json:"localmaster"`
	Multichannel     bool     `json:"multichannel"`
	NetbiosAlias     []string `json:"netbiosalias"`
	NetbiosName      string   `json:"netbiosname"`
	NTLMv1Auth       bool     `json:"ntlmv1_auth"`
	SMBOptions       string   `json:"smb_options"`
	Syslog           bool     `json:"syslog"`
	UnixCharset      string   `json:"unixcharset"`
	Workgroup        string   `json:"workgroup"`
	MinimumProtocol  *string  `json:"minimum_protocol"`
	StatefulFailover *bool    `json:"stateful_failover"`
}

// readSMBConfigOriginal reads the box's current SMB configuration via
// acctest.RestoreCall (smb.update is job:false, probed live), so the test
// can restore it exactly afterward.
func readSMBConfigOriginal(t *testing.T) smbConfigOriginal {
	t.Helper()
	raw, err := acctest.RestoreCall(context.Background(), "smb.config")
	if err != nil {
		t.Fatalf("error reading current smb config: %v", err)
	}
	var orig smbConfigOriginal
	if err := json.Unmarshal(raw, &orig); err != nil {
		t.Fatalf("error parsing smb.config response: %v", err)
	}
	return orig
}

// restoreSMBConfig writes every field the test sets back to its original
// value via smb.update, through acctest.RestoreCall so a t.Cleanup-time
// stale connection is retried and reconnected. It runs from t.Cleanup, so it
// restores the box even if the Terraform steps fail partway through.
func restoreSMBConfig(t *testing.T, orig smbConfigOriginal) {
	t.Helper()
	p := map[string]any{
		"description": orig.Description, "aapl_extensions": orig.AaplExtensions, "admin_group": orig.AdminGroup,
		"bindip": nonNil(orig.BindIP), "debug": orig.Debug, "dirmask": orig.Dirmask, "encryption": orig.Encryption,
		"filemask": orig.Filemask, "guest": orig.Guest, "localmaster": orig.Localmaster,
		"multichannel": orig.Multichannel, "netbiosalias": nonNil(orig.NetbiosAlias), "netbiosname": orig.NetbiosName,
		"ntlmv1_auth": orig.NTLMv1Auth, "smb_options": orig.SMBOptions, "syslog": orig.Syslog,
		"unixcharset": orig.UnixCharset, "workgroup": orig.Workgroup,
	}
	if orig.MinimumProtocol != nil {
		p["minimum_protocol"] = *orig.MinimumProtocol
	}
	if orig.StatefulFailover != nil {
		p["stateful_failover"] = *orig.StatefulFailover
	}
	if _, err := acctest.RestoreCall(context.Background(), "smb.update", p); err != nil {
		t.Fatalf("error restoring smb config: %v", err)
	}
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func hclList(s []string) string {
	b, _ := json.Marshal(nonNil(s))
	return string(b)
}

// TestAccSMBConfig_setAndRestore drives the singleton truenas_smb_config
// resource's "description" field (an inert, cosmetic label) through a test
// value and back to the value read from the box before the test ran, then
// imports it. Every other attribute is written with the box's own current
// value, so the full attribute surface round-trips without changing SMB
// behavior. It requires TF_ACC=1 and TRUENAS_DISRUPTIVE=1
// (acctest.DisruptiveCheck), since it mutates the box's live SMB
// configuration; a t.Cleanup-registered API restore of every written field is
// the safety net if the Terraform steps fail.
func TestAccSMBConfig_setAndRestore(t *testing.T) {
	acctest.DisruptiveCheck(t)

	orig := readSMBConfigOriginal(t)
	t.Cleanup(func() { restoreSMBConfig(t, orig) })

	testValue := acctest.RandName("tf-acc-description")
	// minimum_protocol, search_protocols, and stateful_failover exist only on
	// TrueNAS 26.0+; include them only when the target server has them.
	v26 := acctest.ServerVersionAtLeast(t, 26, 0)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccSMBConfigConfig(testValue, orig, v26),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_smb_config.test", "id", "smb_config"),
					resource.TestCheckResourceAttr("truenas_smb_config.test", "description", testValue),
					resource.TestCheckResourceAttr("truenas_smb_config.test", "netbiosname", orig.NetbiosName),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccSMBConfigConfig(orig.Description, orig, v26),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_smb_config.test", "description", orig.Description),
				),
			},
			{
				ResourceName:      "truenas_smb_config.test",
				ImportState:       true,
				ImportStateId:     "smb_config",
				ImportStateVerify: true,
			},
		},
	})
}

func testAccSMBConfigConfig(description string, o smbConfigOriginal, v26 bool) string {
	v26Fields := ""
	if v26 && o.MinimumProtocol != nil && o.StatefulFailover != nil {
		v26Fields = fmt.Sprintf(`
  minimum_protocol  = %q
  stateful_failover = %t`, *o.MinimumProtocol, *o.StatefulFailover)
	}
	adminGroup := ""
	if o.AdminGroup != nil {
		adminGroup = *o.AdminGroup
	}
	return fmt.Sprintf(`
resource "truenas_smb_config" "test" {
  description = %q

  aapl_extensions   = %t
  admin_group       = %q
  bindip            = %s
  debug             = %t
  dirmask           = %q
  encryption        = %q
  filemask          = %q
  guest             = %q
  localmaster       = %t
  multichannel      = %t
  netbiosalias      = %s
  netbiosname       = %q
  ntlmv1_auth       = %t
  smb_options       = %q
  syslog            = %t
  unixcharset       = %q
  workgroup         = %q%s
}
`, description, o.AaplExtensions, adminGroup, hclList(o.BindIP), o.Debug, o.Dirmask, o.Encryption, o.Filemask,
		o.Guest, o.Localmaster, o.Multichannel, hclList(o.NetbiosAlias), o.NetbiosName, o.NTLMv1Auth, o.SMBOptions,
		o.Syslog, o.UnixCharset, o.Workgroup, v26Fields)
}

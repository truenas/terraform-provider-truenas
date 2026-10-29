// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package system_advanced_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// syslogRestore snapshots syslogservers and returns a cleanup that puts it
// back, so a box's real remote syslog configuration survives the test.
func syslogRestore(t *testing.T) func() {
	t.Helper()
	raw, err := acctest.RestoreCall(context.Background(), "system.advanced.config")
	if err != nil {
		t.Fatalf("read system.advanced.config: %v", err)
	}
	var orig struct {
		Syslogservers json.RawMessage `json:"syslogservers"`
	}
	if err := json.Unmarshal(raw, &orig); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return func() {
		var v any
		_ = json.Unmarshal(orig.Syslogservers, &v)
		if _, err := acctest.RestoreCall(context.Background(), "system.advanced.update", map[string]any{"syslogservers": v}); err != nil {
			t.Errorf("restore syslogservers: %v", err)
		}
	}
}

// syslogTLSCertificateID returns the id of any certificate on the box, for
// the mutual-TLS syslogservers entry.
func syslogTLSCertificateID(t *testing.T) int64 {
	t.Helper()
	raw, err := acctest.RestoreCall(context.Background(), "certificate.query",
		[]any{[]any{"cert_type", "=", "CERTIFICATE"}}, map[string]any{"select": []string{"id"}})
	if err != nil {
		t.Fatalf("certificate.query: %v", err)
	}
	var certs []struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(raw, &certs); err != nil || len(certs) == 0 {
		t.Fatalf("no certificate on the box (err=%v)", err)
	}
	return certs[0].ID
}

// TestAccSystemAdvanced_syslogservers configures remote syslog servers in
// the API's object shape (B-012: the attribute used to be list(string),
// which the API rejects), changes them in place, imports, and clears them.
func TestAccSystemAdvanced_syslogservers(t *testing.T) {
	acctest.DisruptiveCheck(t)
	t.Cleanup(syslogRestore(t))
	certID := syslogTLSCertificateID(t)

	const r = "truenas_system_advanced.test"
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_system_advanced" "test" {
  syslogservers = [
    { host = "192.0.2.10" },
    { host = "192.0.2.11:6514", transport = "TLS", tls_certificate = %d },
  ]
}
`, certID),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(r, "syslogservers.#", "2"),
					resource.TestCheckResourceAttr(r, "syslogservers.0.host", "192.0.2.10"),
					resource.TestCheckResourceAttr(r, "syslogservers.0.transport", "UDP"),
					resource.TestCheckNoResourceAttr(r, "syslogservers.0.tls_certificate"),
					resource.TestCheckResourceAttr(r, "syslogservers.1.host", "192.0.2.11:6514"),
					resource.TestCheckResourceAttr(r, "syslogservers.1.transport", "TLS"),
					resource.TestCheckResourceAttr(r, "syslogservers.1.tls_certificate", fmt.Sprint(certID)),
				),
			},
			{
				Config: acctest.ProviderConfig() + `
resource "truenas_system_advanced" "test" {
  syslogservers = [
    { host = "syslog.example.com", transport = "TCP" },
  ]
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(r, "syslogservers.#", "1"),
					resource.TestCheckResourceAttr(r, "syslogservers.0.host", "syslog.example.com"),
					resource.TestCheckResourceAttr(r, "syslogservers.0.transport", "TCP"),
				),
			},
			{
				ResourceName:            r,
				ImportState:             true,
				ImportStateId:           "system_advanced",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"sed_passwd"},
			},
			{
				Config: acctest.ProviderConfig() + `
resource "truenas_system_advanced" "test" {
  syslogservers = []
}
`,
				Check: resource.TestCheckResourceAttr(r, "syslogservers.#", "0"),
			},
		},
	})
}

// TestAccSystemAdvanced_syslogserversConfiguredOutOfBand: a box that already
// has a remote syslog server (set in the UI) must still be readable by the
// data source and by the resource (B-012: decoding the object array into
// []string failed every read).
func TestAccSystemAdvanced_syslogserversConfiguredOutOfBand(t *testing.T) {
	acctest.DisruptiveCheck(t)
	t.Cleanup(syslogRestore(t))

	if _, err := acctest.RestoreCall(context.Background(), "system.advanced.update", map[string]any{
		"syslogservers": []map[string]any{{"host": "192.0.2.10", "transport": "UDP"}},
	}); err != nil {
		t.Fatalf("seed syslogservers: %v", err)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
data "truenas_system_advanced" "test" {}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_system_advanced.test", "syslogservers.#", "1"),
					resource.TestCheckResourceAttr("data.truenas_system_advanced.test", "syslogservers.0.host", "192.0.2.10"),
					resource.TestCheckResourceAttr("data.truenas_system_advanced.test", "syslogservers.0.transport", "UDP"),
					resource.TestCheckNoResourceAttr("data.truenas_system_advanced.test", "syslogservers.0.tls_certificate"),
				),
			},
			{
				Config: acctest.ProviderConfig() + `
resource "truenas_system_advanced" "test" {}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_system_advanced.test", "syslogservers.#", "1"),
					resource.TestCheckResourceAttr("truenas_system_advanced.test", "syslogservers.0.host", "192.0.2.10"),
				),
			},
			{
				ResourceName:            "truenas_system_advanced.test",
				ImportState:             true,
				ImportStateId:           "system_advanced",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"sed_passwd"},
			},
		},
	})
}

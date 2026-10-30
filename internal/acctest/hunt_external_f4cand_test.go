// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest_test

// F4 candidates from the static scan: resources whose schema still carries
// Default: on Optional+Computed attributes. Create a customized object through
// the API, import it with Required-only HCL, require an empty plan. A diff
// means import would rewrite the user's settings (the nfs/smb share bug).

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func TestHuntExternalCustom_reportingExporter(t *testing.T) {
	acctest.PreCheck(t)
	name := acctest.RandName("tfhuntcexp")
	id := apiCreate(t, "reporting.exporters.create", map[string]any{
		"name": name, "enabled": true,
		"attributes": map[string]any{
			"exporter_type": "GRAPHITE", "destination_ip": "192.0.2.50", "destination_port": 2004,
			"prefix": "custom-prefix", "namespace": "customns", "update_every": 7,
			"buffer_on_failures": 33, "send_names_instead_of_ids": false, "matching_charts": "cpu.*",
		},
	}, "")
	t.Cleanup(func() { apiDelete("reporting.exporters.delete", mustInt(t, id)) })
	huntImportExternal(t, "truenas_reporting_exporter.e", fmt.Sprintf(`
resource "truenas_reporting_exporter" "e" {
  name    = %q
  enabled = true
  attributes = {
    destination_ip   = "192.0.2.50"
    destination_port = 2004
    namespace        = "customns"
  }
}
`, name), id)
}

func TestHuntExternalCustom_keychainSSHConnection(t *testing.T) {
	acctest.PreCheck(t)
	c := acctest.Client()
	ctx := context.Background()
	raw, err := c.Call(ctx, "keychaincredential.generate_ssh_key_pair")
	if err != nil {
		t.Fatal(err)
	}
	var kp struct {
		Private string `json:"private_key"`
		Public  string `json:"public_key"`
	}
	if err := json.Unmarshal(raw, &kp); err != nil {
		t.Fatal(err)
	}
	name := acctest.RandName("tfhuntckc")
	kpID := apiCreate(t, "keychaincredential.create", map[string]any{
		"name": name + "-key", "type": "SSH_KEY_PAIR",
		"attributes": map[string]any{"private_key": kp.Private, "public_key": kp.Public},
	}, "")
	t.Cleanup(func() { apiDelete("keychaincredential.delete", mustInt(t, kpID)) })
	raw, err = c.Call(ctx, "keychaincredential.remote_ssh_host_key_scan", map[string]any{"host": "127.0.0.1", "port": 22, "connect_timeout": 10})
	if err != nil {
		t.Fatal(err)
	}
	var hostKey string
	if err := json.Unmarshal(raw, &hostKey); err != nil {
		t.Fatal(err)
	}
	// non-default username and connect_timeout: the Defaults the scan flagged
	id := apiCreate(t, "keychaincredential.create", map[string]any{
		"name": name, "type": "SSH_CREDENTIALS",
		"attributes": map[string]any{
			"host": "127.0.0.1", "port": 22, "username": "tfhuntcustom", "private_key": mustInt(t, kpID),
			"remote_host_key": hostKey, "connect_timeout": 15,
		},
	}, "")
	t.Cleanup(func() { apiDelete("keychaincredential.delete", mustInt(t, id)) })
	huntImportExternal(t, "truenas_keychain_ssh_connection.k", fmt.Sprintf(`
resource "truenas_keychain_ssh_connection" "k" {
  name            = %q
  host            = "127.0.0.1"
  private_key_id  = %s
  remote_host_key = %q
}
`, name, kpID, hostKey), id)
}

func TestHuntExternalCustom_webshare(t *testing.T) {
	acctest.PreCheck(t)
	if !acctest.ServerVersionAtLeast(t, 26, 0) {
		t.Skip("webshare needs TrueNAS 26.0+")
	}
	ds := acctest.TestPool() + "/" + acctest.RandName("tfhuntcws")
	apiCreate(t, "pool.dataset.create", map[string]any{"name": ds}, "")
	t.Cleanup(func() { apiDelete("pool.dataset.delete", ds) })
	name := acctest.RandName("tfhuntcws")
	// non-default: disabled and home-base — the Defaults the scan flagged
	id := apiCreate(t, "sharing.webshare.create", map[string]any{
		"name": name, "path": "/mnt/" + ds, "enabled": false, "is_home_base": true,
	}, "")
	t.Cleanup(func() { apiDelete("sharing.webshare.delete", mustInt(t, id)) })
	huntImportExternal(t, "truenas_webshare.w", fmt.Sprintf(`
resource "truenas_webshare" "w" {
  name = %q
  path = %q
}
`, name, "/mnt/"+ds), id)
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest_test

// Second batch of F4-class candidates from the provider-wide guard
// (TestNoDefaultOnOptionalComputed): Defaults on server-owned values.
//   iscsi_auth.peeruser        Default ""      -> imported mutual-CHAP auth lost its peer user (confirmed 2026-10-30, .68)
//   replication_task.transport Default "LOCAL" -> imported SSH task planned back to LOCAL, and transport changes
//                                                 force replacement, so the import would destroy and recreate it.

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func TestHuntExternalCustom_iscsiAuthMutualCHAP(t *testing.T) {
	acctest.PreCheck(t)
	id := apiCreate(t, "iscsi.auth.create", map[string]any{
		"tag": 9931, "user": "tfhuntchap", "secret": "tfhuntsecret12",
		"peeruser": "tfhuntpeer", "peersecret": "tfhuntpeersec12",
	}, "")
	t.Cleanup(func() { apiDelete("iscsi.auth.delete", mustInt(t, id)) })
	huntImportExternal(t, "truenas_iscsi_auth.a", `
resource "truenas_iscsi_auth" "a" {
  tag    = 9931
  user   = "tfhuntchap"
  secret = "tfhuntsecret12"
}
`, id)
}

// replicationFixture creates two datasets and returns a replication.create
// payload for a PUSH task between them with the given transport fields merged.
func replicationFixture(t *testing.T, extra map[string]any) (name, src, dst string, payload map[string]any) {
	t.Helper()
	src = acctest.TestPool() + "/" + acctest.RandName("tfhuntrepsrc")
	dst = acctest.TestPool() + "/" + acctest.RandName("tfhuntrepdst")
	apiCreate(t, "pool.dataset.create", map[string]any{"name": src}, "")
	t.Cleanup(func() { apiDelete("pool.dataset.delete", src, map[string]any{"recursive": true}) })
	apiCreate(t, "pool.dataset.create", map[string]any{"name": dst}, "")
	t.Cleanup(func() { apiDelete("pool.dataset.delete", dst, map[string]any{"recursive": true}) })
	name = acctest.RandName("tfhuntrep")
	payload = map[string]any{
		"name": name, "direction": "PUSH", "transport": "LOCAL",
		"source_datasets": []string{src}, "target_dataset": dst,
		"also_include_naming_schema": []string{"auto-%Y-%m-%d_%H-%M"},
		"recursive":                  false, "auto": false, "retention_policy": "NONE", "enabled": false,
	}
	for k, v := range extra {
		payload[k] = v
	}
	return
}

func TestHuntExternalCustom_replicationLocal(t *testing.T) {
	acctest.PreCheck(t)
	name, src, dst, payload := replicationFixture(t, nil)
	id := apiCreate(t, "replication.create", payload, "")
	t.Cleanup(func() { apiDelete("replication.delete", mustInt(t, id)) })
	huntImportExternal(t, "truenas_replication_task.r", fmt.Sprintf(`
resource "truenas_replication_task" "r" {
  name                       = %q
  direction                  = "PUSH"
  source_datasets            = [%q]
  target_dataset             = %q
  also_include_naming_schema = ["auto-%%Y-%%m-%%d_%%H-%%M"]
  retention_policy           = "NONE"
  recursive                  = false
  auto                       = false
  enabled                    = false
}
`, name, src, dst), id)
}

// An SSH replication task made in the UI: key pair + loopback SSH credential
// through the API, then imported with HCL that does not mention transport.
func TestHuntExternalCustom_replicationSSH(t *testing.T) {
	acctest.PreCheck(t)
	ctx := context.Background()
	rawKP, err := acctest.Client().Call(ctx, "keychaincredential.generate_ssh_key_pair")
	if err != nil {
		t.Fatalf("generate_ssh_key_pair: %v", err)
	}
	var kp map[string]any
	if err := json.Unmarshal(rawKP, &kp); err != nil {
		t.Fatal(err)
	}
	kpID := apiCreate(t, "keychaincredential.create", map[string]any{
		"name": acctest.RandName("tfhuntkp"), "type": "SSH_KEY_PAIR", "attributes": kp,
	}, "")
	t.Cleanup(func() { apiDelete("keychaincredential.delete", mustInt(t, kpID)) })
	rawHK, err := acctest.Client().Call(ctx, "keychaincredential.remote_ssh_host_key_scan", map[string]any{"host": "127.0.0.1", "port": 22})
	if err != nil {
		t.Fatalf("remote_ssh_host_key_scan: %v", err)
	}
	var hostKey string
	if err := json.Unmarshal(rawHK, &hostKey); err != nil {
		t.Fatal(err)
	}
	credID := apiCreate(t, "keychaincredential.create", map[string]any{
		"name": acctest.RandName("tfhuntssh"), "type": "SSH_CREDENTIALS",
		"attributes": map[string]any{
			"host": "127.0.0.1", "port": 22, "username": "root",
			"private_key": mustInt(t, kpID), "remote_host_key": hostKey, "connect_timeout": 10,
		},
	}, "")
	t.Cleanup(func() { apiDelete("keychaincredential.delete", mustInt(t, credID)) })

	name, src, dst, payload := replicationFixture(t, map[string]any{
		"transport": "SSH", "ssh_credentials": mustInt(t, credID),
	})
	id := apiCreate(t, "replication.create", payload, "")
	t.Cleanup(func() { apiDelete("replication.delete", mustInt(t, id)) })
	huntImportExternal(t, "truenas_replication_task.r", fmt.Sprintf(`
resource "truenas_replication_task" "r" {
  name                       = %q
  direction                  = "PUSH"
  ssh_credentials            = %s
  source_datasets            = [%q]
  target_dataset             = %q
  also_include_naming_schema = ["auto-%%Y-%%m-%%d_%%H-%%M"]
  retention_policy           = "NONE"
  recursive                  = false
  auto                       = false
  enabled                    = false
}
`, name, credID, src, dst), id)
}

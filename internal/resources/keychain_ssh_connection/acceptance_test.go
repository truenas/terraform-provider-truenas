// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package keychain_ssh_connection_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"golang.org/x/crypto/ssh"

	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// genEd25519OpenSSHKeyPair generates an ed25519 key pair and returns the
// OpenSSH-format PEM private key (via golang.org/x/crypto/ssh's
// MarshalPrivateKey — already a transitive module dependency, per go.mod,
// so this adds no new dependency) plus the exact authorized_keys-format
// public key line. Mirrors the identically-named, identically-behaved
// helper in the sibling keychain_ssh_keypair and replication packages'
// acceptance tests — kept package-local rather than shared/exported since
// this is just one more place that needs it.
func genEd25519OpenSSHKeyPair(t *testing.T, comment string) (privateKeyPEM, publicKeyLine string) {
	t.Helper()

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating ed25519 key: %v", err)
	}
	block, err := ssh.MarshalPrivateKey(priv, comment)
	if err != nil {
		t.Fatalf("marshaling OpenSSH private key: %v", err)
	}
	privateKeyPEM = string(pem.EncodeToMemory(block))

	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("deriving ssh public key: %v", err)
	}
	publicKeyLine = string(ssh.MarshalAuthorizedKey(sshPub))

	return privateKeyPEM, publicKeyLine
}

// scanRemoteHostKey calls keychaincredential.remote_ssh_host_key_scan
// against the acceptance-test TrueNAS box's own SSH server — a genuine
// loopback to the box itself, per the brief — returning the multi-line
// host key blob "remote_host_key" expects verbatim (probed live: the
// result is one line per host key algorithm the server offers, e.g.
// ssh-rsa/ecdsa-sha2-nistp256/ssh-ed25519, newline-separated).
func scanRemoteHostKey(t *testing.T, host string) string {
	t.Helper()
	raw, err := acctest.Client().Call(context.Background(), "keychaincredential.remote_ssh_host_key_scan", map[string]any{
		"host": host,
		"port": 22,
	})
	if err != nil {
		t.Fatalf("remote_ssh_host_key_scan against %s failed: %v", host, err)
	}
	var hostKey string
	if err := json.Unmarshal(raw, &hostKey); err != nil {
		t.Fatalf("parsing remote_ssh_host_key_scan response: %v", err)
	}
	return hostKey
}

// TestAccKeychainSSHConnection_loopback exercises the full contract of
// truenas_keychain_ssh_connection against a loopback connection to the
// acceptance-test TrueNAS box itself: a fixture truenas_keychain_ssh_keypair
// (an ed25519 key generated in-test, so its public half is known ahead of
// apply time — unlike generate=true, whose key only exists after the API
// call), a throwaway truenas_user fixture (HCL-managed, own home dataset,
// destroyed along with everything else at the end of the test — see
// replication's TestAccReplication_RemoteSSH, whose fixture-user pattern
// this copies) with the generated public key authorized directly via its
// sshpubkey attribute, and the target host's key discovered via a live
// remote_ssh_host_key_scan. This never touches truenas_admin or any other
// pre-existing box account. Full CRUD: create, in-place update (rename +
// connect_timeout, neither of which carries RequiresReplace —
// keychaincredential.update always resends the complete attributes
// object, see model.go), import, and CheckDestroy via a live
// keychaincredential.query (plus a user.query check that the fixture user
// itself is gone).
func TestAccKeychainSSHConnection_loopback(t *testing.T) {
	acctest.PreCheck(t)

	host := acctest.EndpointHost()
	username := acctest.RandName("tf-acc-sshconn-user")
	homeDS := acctest.RandName("tf-acc-sshconn-home")
	keypairName := acctest.RandName("tf-acc-sshconn-keypair")
	privateKeyPEM, publicKeyLine := genEd25519OpenSSHKeyPair(t, keypairName)
	// ssh.MarshalAuthorizedKey appends a trailing newline, but TrueNAS's
	// user.create/user.update strip it before persisting sshpubkey, so the
	// server's post-create read-back never carries it. Terraform's plan
	// step otherwise uses the config value (including the newline) as the
	// planned value for this Optional+Computed attribute, which then
	// mismatches the applied state and trips "provider produced
	// inconsistent result after apply" (probed live, same trap documented
	// in replication's TestAccReplicationSSHConfig). Trimming here,
	// test-side, keeps the config's value identical to what the box
	// stores.
	publicKeyLine = strings.TrimSpace(publicKeyLine)
	hostKey := scanRemoteHostKey(t, host)

	name := acctest.RandName("tf-acc-sshconn")
	renamed := name + "-renamed"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckKeychainSSHConnectionDestroyed(username),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccKeychainSSHConnectionConfig(
					username, homeDS, publicKeyLine, keypairName, privateKeyPEM, name, host, hostKey, 10),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_keychain_ssh_connection.test", "id"),
					resource.TestCheckResourceAttr("truenas_keychain_ssh_connection.test", "name", name),
					resource.TestCheckResourceAttr("truenas_keychain_ssh_connection.test", "host", host),
					resource.TestCheckResourceAttr("truenas_keychain_ssh_connection.test", "port", "22"),
					resource.TestCheckResourceAttr("truenas_keychain_ssh_connection.test", "username", username),
					resource.TestCheckResourceAttr("truenas_keychain_ssh_connection.test", "remote_host_key", hostKey),
					resource.TestCheckResourceAttr("truenas_keychain_ssh_connection.test", "connect_timeout", "10"),
					resource.TestCheckResourceAttrPair(
						"truenas_keychain_ssh_connection.test", "private_key_id",
						"truenas_keychain_ssh_keypair.test", "id"),
				),
			},
			// In-place update: rename + change connect_timeout. No schema
			// attribute here carries RequiresReplace (see schema.go), since
			// keychaincredential.update always resends the complete
			// "attributes" object.
			{
				Config: acctest.ProviderConfig() + testAccKeychainSSHConnectionConfig(
					username, homeDS, publicKeyLine, keypairName, privateKeyPEM, renamed, host, hostKey, 20),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_keychain_ssh_connection.test", "name", renamed),
					resource.TestCheckResourceAttr("truenas_keychain_ssh_connection.test", "connect_timeout", "20"),
				),
			},
			{
				ResourceName:      "truenas_keychain_ssh_connection.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccKeychainSSHConnectionConfig(username, homeDS, publicKeyLine, keypairName, privateKeyPEM, name, host, hostKey string, connectTimeout int) string {
	pool := acctest.TestPool()
	return fmt.Sprintf(`
# user.create requires the home directory of any user with an sshpubkey to
# be a writable path inside a data pool (a stub like /var/empty is
# rejected), so the throwaway SSH user below gets its own dedicated,
# Terraform-managed dataset as a home — destroyed along with the user
# fixture, never touching any pre-existing pool path or account (in
# particular, truenas_admin is never referenced by this test).
resource "truenas_dataset" "sshuser_home" {
  name = %[2]q
}

resource "truenas_user" "sshuser" {
  username          = %[1]q
  full_name         = "TF Acceptance SSH Connection User"
  password_disabled = true
  home              = "/mnt/${truenas_dataset.sshuser_home.name}"
  shell             = "/usr/bin/bash"
  group_create      = true
  sshpubkey         = %[3]q
}

resource "truenas_keychain_ssh_keypair" "test" {
  name        = %[4]q
  private_key = %[5]q
}

resource "truenas_keychain_ssh_connection" "test" {
  name            = %[6]q
  host            = %[7]q
  port            = 22
  username        = truenas_user.sshuser.username
  private_key_id  = truenas_keychain_ssh_keypair.test.id
  remote_host_key = %[8]q
  connect_timeout = %[9]d
}
`, username, pool+"/"+homeDS, publicKeyLine, keypairName, privateKeyPEM, name, host, hostKey, connectTimeout)
}

// testAccCheckKeychainSSHConnectionDestroyed queries keychaincredential by
// the ids captured in the pre-destroy state for both fixture resources
// (the connection and its referenced keypair), matching the established
// pattern used elsewhere in this provider (e.g. truenas_kerberos_keytab's
// acceptance test), plus a live user.query confirming the throwaway
// fixture user itself was torn down.
func testAccCheckKeychainSSHConnectionDestroyed(username string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()

		for _, addr := range []string{"truenas_keychain_ssh_connection.test", "truenas_keychain_ssh_keypair.test"} {
			rs, ok := s.RootModule().Resources[addr]
			if !ok {
				return fmt.Errorf("resource %s not found in pre-destroy state", addr)
			}
			idStr, ok := rs.Primary.Attributes["id"]
			if !ok {
				return fmt.Errorf("%s has no id attribute in pre-destroy state", addr)
			}
			id, err := strconv.ParseInt(idStr, 10, 64)
			if err != nil {
				return fmt.Errorf("parsing id %q for %s: %v", idStr, addr, err)
			}

			raw, err := c.Call(context.Background(), "keychaincredential.query", [][]any{{"id", "=", id}})
			if err != nil {
				return fmt.Errorf("error checking keychain credential id=%d (%s): %v", id, addr, err)
			}
			var results []struct {
				ID int64 `json:"id"`
			}
			if err := json.Unmarshal(raw, &results); err != nil {
				return fmt.Errorf("error parsing keychaincredential.query response: %v", err)
			}
			if len(results) > 0 {
				return fmt.Errorf("keychain credential id=%d (%s) still exists", id, addr)
			}
		}

		raw, err := c.Call(context.Background(), "user.query", [][]any{{"username", "=", username}})
		if err != nil {
			return fmt.Errorf("error checking fixture user %q: %v", username, err)
		}
		var users []struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(raw, &users); err != nil {
			return fmt.Errorf("error parsing user.query response: %v", err)
		}
		if len(users) > 0 {
			return fmt.Errorf("fixture user %q still exists", username)
		}

		return nil
	}
}

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

// testAccConnectionUsername is the local TrueNAS user this test
// temporarily authorizes a throwaway SSH key for, to make the loopback
// connection fixture a genuine, working credential rather than just an
// API-accepted shape (keychaincredential.create itself never validates
// SSH connectivity — probed live, see schema.go's doc comment — but
// authorizing the key anyway proves the credential this resource stores
// is real).
const testAccConnectionUsername = "truenas_admin"

// genEd25519OpenSSHKeyPair generates an ed25519 key pair and returns the
// OpenSSH-format PEM private key (via golang.org/x/crypto/ssh's
// MarshalPrivateKey — already a transitive module dependency, per go.mod,
// so this adds no new dependency) plus the exact authorized_keys-format
// public key line. Mirrors the identically-named, identically-behaved
// helper in the sibling keychain_ssh_keypair package's acceptance test —
// kept package-local rather than shared/exported since this is the only
// other place that needs it.
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

// authorizeTestKey appends pubKeyLine to testAccConnectionUsername's
// "sshpubkey" (an authorized_keys-style blob, one key per line — probed
// live via user.query/user.update, see model.go's doc comments elsewhere
// in this provider) via a live user.query + user.update, and returns a
// restore func that puts the exact original value back (nil if the user
// had none set). The caller registers the restore func via t.Cleanup, so
// the box is restored even if the Terraform steps themselves fail partway
// through.
func authorizeTestKey(t *testing.T, pubKeyLine string) func() {
	t.Helper()
	c := acctest.Client()

	raw, err := c.Call(context.Background(), "user.query", [][]any{{"username", "=", testAccConnectionUsername}})
	if err != nil {
		t.Fatalf("querying user %q: %v", testAccConnectionUsername, err)
	}
	var users []struct {
		ID        int64   `json:"id"`
		SSHPubKey *string `json:"sshpubkey"`
	}
	if err := json.Unmarshal(raw, &users); err != nil {
		t.Fatalf("parsing user.query response: %v", err)
	}
	if len(users) == 0 {
		t.Fatalf("user %q not found on the acceptance-test box", testAccConnectionUsername)
	}
	userID := users[0].ID
	original := users[0].SSHPubKey

	updated := ""
	if original != nil {
		updated = strings.TrimRight(*original, "\n") + "\n"
	}
	updated += strings.TrimRight(pubKeyLine, "\n") + "\n"

	if _, err := c.Call(context.Background(), "user.update", userID, map[string]any{"sshpubkey": updated}); err != nil {
		t.Fatalf("authorizing test key for %q: %v", testAccConnectionUsername, err)
	}

	return func() {
		var restoreVal any
		if original != nil {
			restoreVal = *original
		}
		if _, err := c.Call(context.Background(), "user.update", userID, map[string]any{"sshpubkey": restoreVal}); err != nil {
			t.Errorf("restoring original sshpubkey for %q: %v", testAccConnectionUsername, err)
		}
	}
}

// TestAccKeychainSSHConnection_loopback exercises the full contract of
// truenas_keychain_ssh_connection against a loopback connection to the
// acceptance-test TrueNAS box itself: a fixture truenas_keychain_ssh_keypair
// (an ed25519 key generated in-test, so its public half is known ahead of
// apply time — unlike generate=true, whose key only exists after the API
// call), the fixture's public key authorized for testAccConnectionUsername
// via a live user.update (deauthorized in t.Cleanup), and the target
// host's key discovered via a live remote_ssh_host_key_scan. Full CRUD:
// create, in-place update (rename + connect_timeout, neither of which
// carries RequiresReplace — keychaincredential.update always resends the
// complete attributes object, see model.go), import, and CheckDestroy via
// a live keychaincredential.query.
func TestAccKeychainSSHConnection_loopback(t *testing.T) {
	acctest.PreCheck(t)

	host := acctest.EndpointHost()
	keypairName := acctest.RandName("tf-acc-sshconn-keypair")
	privateKeyPEM, publicKeyLine := genEd25519OpenSSHKeyPair(t, keypairName)
	hostKey := scanRemoteHostKey(t, host)

	restore := authorizeTestKey(t, publicKeyLine)
	t.Cleanup(restore)

	name := acctest.RandName("tf-acc-sshconn")
	renamed := name + "-renamed"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckKeychainSSHConnectionDestroyed,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccKeychainSSHConnectionConfig(
					keypairName, privateKeyPEM, name, host, hostKey, testAccConnectionUsername, 10),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_keychain_ssh_connection.test", "id"),
					resource.TestCheckResourceAttr("truenas_keychain_ssh_connection.test", "name", name),
					resource.TestCheckResourceAttr("truenas_keychain_ssh_connection.test", "host", host),
					resource.TestCheckResourceAttr("truenas_keychain_ssh_connection.test", "port", "22"),
					resource.TestCheckResourceAttr("truenas_keychain_ssh_connection.test", "username", testAccConnectionUsername),
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
					keypairName, privateKeyPEM, renamed, host, hostKey, testAccConnectionUsername, 20),
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

func testAccKeychainSSHConnectionConfig(keypairName, privateKeyPEM, name, host, hostKey, username string, connectTimeout int) string {
	return fmt.Sprintf(`
resource "truenas_keychain_ssh_keypair" "test" {
  name        = %q
  private_key = %q
}

resource "truenas_keychain_ssh_connection" "test" {
  name            = %q
  host            = %q
  port            = 22
  username        = %q
  private_key_id  = truenas_keychain_ssh_keypair.test.id
  remote_host_key = %q
  connect_timeout = %d
}
`, keypairName, privateKeyPEM, name, host, username, hostKey, connectTimeout)
}

// testAccCheckKeychainSSHConnectionDestroyed queries keychaincredential by
// the ids captured in the pre-destroy state for BOTH fixture resources
// (the connection and its referenced keypair), matching the established
// pattern used elsewhere in this provider (e.g. truenas_kerberos_keytab's
// acceptance test).
func testAccCheckKeychainSSHConnectionDestroyed(s *terraform.State) error {
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
	return nil
}

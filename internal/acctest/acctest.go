// Copyright iXsystems, Inc. 2026
// SPDX-License-Identifier: MPL-2.0

package acctest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
	"github.com/truenas/terraform-provider-truenas/internal/provider"
)

var ProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"truenas": providerserver.NewProtocol6WithError(provider.New("test")()),
}

var (
	clientOnce sync.Once
	testClient *client.Client
)

// Endpoint returns the TrueNAS WebSocket endpoint to use for acceptance
// tests, from the TRUENAS_ENDPOINT environment variable. When unset it
// returns a syntactically valid RFC 6761 .invalid placeholder: test config
// strings are built before PreCheck gets a chance to skip, so this must not
// fail — PreCheck enforces the real requirement before anything dials out.
func Endpoint() string {
	if v := os.Getenv("TRUENAS_ENDPOINT"); v != "" {
		// Same legacy-path rewrite the provider applies: the client speaks
		// JSON-RPC 2.0, so a configured /websocket endpoint must land on
		// /api/current (acctest.Client() dials this URL directly).
		if base, ok := strings.CutSuffix(v, "/websocket"); ok {
			return base + "/api/current"
		}
		return v
	}
	return "wss://truenas.invalid/api/current"
}

// EndpointHost returns the bare host (IP or name) portion of Endpoint(),
// for tests that must bind or reference an address on the target box
// (e.g. iSCSI portal listen IPs, NVMe-oF port addresses).
func EndpointHost() string {
	u, err := url.Parse(Endpoint())
	if err != nil || u.Host == "" {
		panic("acctest: cannot parse host from TRUENAS_ENDPOINT")
	}
	return u.Hostname()
}

// TestPool returns the ZFS pool to use for acceptance tests, from
// TRUENAS_TEST_POOL, defaulting to "tank".
func TestPool() string {
	if v := os.Getenv("TRUENAS_TEST_POOL"); v != "" {
		return v
	}
	return "tank"
}

// RandName returns prefix + "-" + an 8 character lowercase hex suffix,
// suitable for generating unique resource names in acceptance tests.
// It panics only if the system RNG fails.
func RandName(prefix string) string {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		panic("acctest: rand.Read failed: " + err.Error())
	}
	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(buf))
}

// RandNQN returns a unique NVMe host NQN in the uuid form TrueNAS validates:
// the uuid segment must be a full RFC-4122 UUID, so a short random suffix is
// rejected ("uuid is incorrect length").
func RandNQN() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		panic("acctest: rand.Read failed: " + err.Error())
	}
	buf[6] = (buf[6] & 0x0f) | 0x40 // version 4
	buf[8] = (buf[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("nqn.2014-08.org.nvmexpress:uuid:%x-%x-%x-%x-%x",
		buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:16])
}

// Client returns a singleton *client.Client connected to the acceptance test TrueNAS.
func Client() *client.Client {
	clientOnce.Do(func() {
		tlsCfg, _ := client.BuildTLSConfig(true, "")
		c := client.New(Endpoint(), tlsCfg)
		var authFn func(ctx context.Context) error
		if apiKey := os.Getenv("TRUENAS_API_KEY"); apiKey != "" {
			// Upgrades to SCRAM on 26.0+ when TRUENAS_USERNAME names the
			// key owner; plain login otherwise.
			username := os.Getenv("TRUENAS_USERNAME")
			authFn = func(ctx context.Context) error { return client.AuthAPIKeyAuto(ctx, c, username, apiKey) }
		} else {
			username := os.Getenv("TRUENAS_USERNAME")
			password := os.Getenv("TRUENAS_PASSWORD")
			authFn = func(ctx context.Context) error { return client.AuthPassword(ctx, c, username, password) }
		}
		if err := c.Connect(context.Background(), authFn); err != nil {
			panic("acctest: cannot connect to TrueNAS: " + err.Error())
		}
		testClient = c
	})
	return testClient
}

// RestoreCall invokes an idempotent singleton-config read or write (e.g. a
// t.Cleanup-registered restore, or the paired "read the original value"
// call that precedes it) through the shared acctest.Client() connection,
// with the same transient-failure retry and reconnect behavior as
// client.CallRead.
//
// The long-lived acctest.Client() connection sits idle for the whole
// duration of a Tier-2 set-and-restore test's Terraform steps (which run
// over the provider's own, separate connection), so by the time t.Cleanup
// fires it has occasionally gone stale and a plain Call returns "not
// connected" — a real, observed flake (see task-2-report.md and this
// task's report), not a resource defect: the Terraform steps themselves,
// including the step that already applies the restored value, still
// succeeded. Both directions (read-original and restore-write) are
// idempotent, so retrying either on a transient failure is safe — same
// justification CallRead already relies on for reads.
func RestoreCall(ctx context.Context, method string, params ...any) (json.RawMessage, error) {
	return Client().CallRead(ctx, method, params...)
}

// ServerVersionAtLeast reports whether the acceptance-test server is at or
// above major.minor, for gating attributes that exist only on newer TrueNAS
// releases (e.g. nvmet_host description, 26.0+). Unlike most helpers it
// dials the box, so it enforces the PreCheck env requirements itself:
// tests call it while building config strings, before resource.Test runs
// PreCheck. Fatals if the version cannot be determined.
func ServerVersionAtLeast(t *testing.T, major, minor int) bool {
	t.Helper()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Set TF_ACC=1 to run acceptance tests")
	}
	if os.Getenv("TRUENAS_ENDPOINT") == "" {
		t.Fatal("Set TRUENAS_ENDPOINT (e.g. wss://truenas.example.com/api/current) for acceptance tests")
	}
	ok, err := Client().VersionAtLeast(context.Background(), major, minor)
	if err != nil {
		t.Fatalf("acctest: cannot determine server version: %v", err)
	}
	return ok
}

// PreCheck verifies required env vars are set before running acceptance tests.
func PreCheck(t *testing.T) {
	t.Helper()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Set TF_ACC=1 to run acceptance tests")
	}
	if os.Getenv("TRUENAS_ENDPOINT") == "" {
		t.Fatal("Set TRUENAS_ENDPOINT (e.g. wss://truenas.example.com/api/current) for acceptance tests")
	}
	if os.Getenv("TRUENAS_API_KEY") == "" && (os.Getenv("TRUENAS_USERNAME") == "" || os.Getenv("TRUENAS_PASSWORD") == "") {
		t.Fatal("Set TRUENAS_API_KEY or TRUENAS_USERNAME+TRUENAS_PASSWORD for acceptance tests")
	}
}

// DisruptiveCheck gates singleton set-and-restore tests that mutate
// box-wide, non-idempotent state (e.g. global config singletons). It runs
// PreCheck and then skips unless TRUENAS_DISRUPTIVE=1 is set, since these
// tests can leave the box in a different state than they found it.
func DisruptiveCheck(t *testing.T) {
	t.Helper()
	PreCheck(t)
	if os.Getenv("TRUENAS_DISRUPTIVE") != "1" {
		t.Skip("Set TRUENAS_DISRUPTIVE=1 to run disruptive acceptance tests")
	}
}

// AppsCheck gates tests that exercise app resources, which pull container
// images from the network and can be slow or bandwidth-heavy. It runs
// PreCheck and then skips unless TRUENAS_APPS=1 is set.
func AppsCheck(t *testing.T) {
	t.Helper()
	PreCheck(t)
	if os.Getenv("TRUENAS_APPS") != "1" {
		t.Skip("Set TRUENAS_APPS=1 to run app acceptance tests")
	}
}

// DSCheck gates tests that join/leave an Active Directory domain via the
// truenas_directoryservices resource. It runs PreCheck and then skips
// unless TRUENAS_DS=1 is set, since domain-join tests mutate real
// directory-service state and can take minutes to run.
//
// When TRUENAS_DS=1 it additionally enforces a disposable-box guard: the
// env var TRUENAS_DS_ALLOWED_ENDPOINT must be set and must equal
// Endpoint() exactly. Domain-join acceptance tests must run ONLY against a
// disposable test VM, never a shared or production TrueNAS box — an
// unintended AD join (or a failed join left half-configured) is much more
// disruptive than the other acceptance tests in this suite, so this check
// fails loudly (t.Fatal, not t.Skip) rather than silently doing the wrong
// thing if the guard env var is missing or mismatched.
func DSCheck(t *testing.T) {
	t.Helper()
	PreCheck(t)
	if os.Getenv("TRUENAS_DS") != "1" {
		t.Skip("Set TRUENAS_DS=1 to run directory services (Active Directory join) acceptance tests")
	}
	allowed := os.Getenv("TRUENAS_DS_ALLOWED_ENDPOINT")
	if allowed == "" {
		t.Fatal("TRUENAS_DS=1 requires TRUENAS_DS_ALLOWED_ENDPOINT to be set to the disposable test VM's " +
			"endpoint, as a guard against accidentally domain-joining a shared or production TrueNAS box")
	}
	if allowed != Endpoint() {
		t.Fatalf("TRUENAS_DS_ALLOWED_ENDPOINT (%q) does not match TRUENAS_ENDPOINT (%q): refusing to run "+
			"directory services acceptance tests against a box that isn't the disposable test VM", allowed, Endpoint())
	}
}

// HACheck gates tests that exercise Enterprise HA failover behavior via the
// truenas_failover_config resource/datasource. It runs PreCheck and then
// skips unless TRUENAS_HA=1 is set, since these tests mutate the box's
// live failover configuration (timeout, and — outside any committed test —
// the genuinely disruptive "disabled"/"master" fields) and are only
// meaningful against a disposable HA pair.
//
// When TRUENAS_HA=1 it additionally enforces a disposable-box guard,
// mirroring DSCheck's: TRUENAS_HA_ALLOWED_ENDPOINT must be set and must
// equal Endpoint() exactly, else t.Fatal — HA tests must never run against
// a box that isn't explicitly designated disposable, since an unintended
// failover trigger is far more disruptive than the other acceptance tests
// in this suite.
//
// Finally, once both env guards pass, it does a live failover.licensed
// probe: a box that is reachable and correctly guarded but not actually
// licensed for Enterprise HA (e.g. the cross-release 26.0 box used only
// for shape probing in this provider's own test matrix) has nothing
// meaningful for an HA-gated test to exercise. That is a clean t.Skip, not
// a t.Fatal — unlike the endpoint guard above, an unlicensed box is not a
// safety problem, just an environment that doesn't support the feature.
func HACheck(t *testing.T) {
	t.Helper()
	PreCheck(t)
	if os.Getenv("TRUENAS_HA") != "1" {
		t.Skip("Set TRUENAS_HA=1 to run Enterprise HA / failover acceptance tests")
	}
	allowed := os.Getenv("TRUENAS_HA_ALLOWED_ENDPOINT")
	if allowed == "" {
		t.Fatal("TRUENAS_HA=1 requires TRUENAS_HA_ALLOWED_ENDPOINT to be set to the disposable HA test box's " +
			"endpoint, as a guard against accidentally exercising real failover behavior against a shared or " +
			"production TrueNAS box")
	}
	if allowed != Endpoint() {
		t.Fatalf("TRUENAS_HA_ALLOWED_ENDPOINT (%q) does not match TRUENAS_ENDPOINT (%q): refusing to run "+
			"Enterprise HA acceptance tests against a box that isn't the disposable HA test box", allowed, Endpoint())
	}

	raw, err := Client().CallRead(context.Background(), "failover.licensed")
	if err != nil {
		t.Fatalf("acctest: cannot determine failover.licensed: %v", err)
	}
	var licensed bool
	if err := json.Unmarshal(raw, &licensed); err != nil {
		t.Fatalf("acctest: cannot parse failover.licensed response: %v", err)
	}
	if !licensed {
		t.Skip("failover.licensed=false on this box: not licensed for Enterprise HA, skipping")
	}
}

// ProviderConfig returns HCL for the provider block used in acceptance tests.
func ProviderConfig() string {
	return fmt.Sprintf(`
provider "truenas" {
  endpoint = %q
  insecure = true
}
`, Endpoint())
}

// Ensure resource package is importable
var _ resource.TestCheckFunc = nil

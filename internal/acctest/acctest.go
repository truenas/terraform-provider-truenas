package acctest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
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
		return v
	}
	return "wss://truenas.invalid/websocket"
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
			authFn = func(ctx context.Context) error { return client.AuthAPIKey(ctx, c, apiKey) }
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

// ServerVersionAtLeast reports whether the acceptance-test server is at or
// above major.minor, for gating attributes that exist only on newer SCALE
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
		t.Fatal("Set TRUENAS_ENDPOINT (e.g. wss://truenas.example.com/websocket) for acceptance tests")
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
		t.Fatal("Set TRUENAS_ENDPOINT (e.g. wss://truenas.example.com/websocket) for acceptance tests")
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

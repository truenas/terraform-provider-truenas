package acctest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
	"github.com/truenas/terraform-provider-truenas/internal/provider"
)

// TestTrueNASEndpoint is the default TrueNAS WebSocket endpoint used for
// acceptance tests when TRUENAS_ENDPOINT is not set. Prefer Endpoint() over
// referencing this constant directly.
const TestTrueNASEndpoint = "wss://192.168.1.68/websocket"

var ProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"truenas": providerserver.NewProtocol6WithError(provider.New("test")()),
}

var (
	clientOnce sync.Once
	testClient *client.Client
)

// Endpoint returns the TrueNAS WebSocket endpoint to use for acceptance
// tests, from TRUENAS_ENDPOINT, defaulting to TestTrueNASEndpoint.
func Endpoint() string {
	if v := os.Getenv("TRUENAS_ENDPOINT"); v != "" {
		return v
	}
	return TestTrueNASEndpoint
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

// PreCheck verifies required env vars are set before running acceptance tests.
func PreCheck(t *testing.T) {
	t.Helper()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Set TF_ACC=1 to run acceptance tests")
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

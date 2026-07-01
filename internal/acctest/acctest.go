package acctest

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
	"github.com/truenas/terraform-provider-truenas/internal/provider"
)

const TestTrueNASEndpoint = "wss://192.168.1.68/websocket"

var ProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"truenas": providerserver.NewProtocol6WithError(provider.New("test")()),
}

var (
	clientOnce sync.Once
	testClient *client.Client
)

// Client returns a singleton *client.Client connected to the acceptance test TrueNAS.
func Client() *client.Client {
	clientOnce.Do(func() {
		apiKey := os.Getenv("TRUENAS_API_KEY")
		tlsCfg, _ := client.BuildTLSConfig(true, "")
		c := client.New(TestTrueNASEndpoint, tlsCfg)
		authFn := func(ctx context.Context) error { return client.AuthAPIKey(ctx, c, apiKey) }
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

// ProviderConfig returns HCL for the provider block used in acceptance tests.
func ProviderConfig() string {
	return `
provider "truenas" {
  endpoint = "wss://192.168.1.68/websocket"
  insecure = true
}
`
}

// Ensure resource package is importable
var _ resource.TestCheckFunc = nil

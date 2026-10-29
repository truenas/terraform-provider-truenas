// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package docker_config_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// dockerNetConfig is the part of docker.config the spelling tests write.
// RegistryMirrors is TrueNAS 26.0+'s shape; Secure/Insecure is 25.10's.
type dockerNetConfig struct {
	Pool             *string          `json:"pool"`
	AddressPools     []map[string]any `json:"address_pools"`
	CIDRv6           string           `json:"cidr_v6"`
	RegistryMirrors  []map[string]any `json:"registry_mirrors,omitempty"`
	SecureMirrors    []string         `json:"secure_registry_mirrors,omitempty"`
	InsecureMirrors  []string         `json:"insecure_registry_mirrors,omitempty"`
	hasUnifiedMirror bool
}

func readDockerNetConfig(t *testing.T) dockerNetConfig {
	t.Helper()
	raw, err := acctest.RestoreCall(context.Background(), "docker.config")
	if err != nil {
		t.Fatalf("read docker.config: %v", err)
	}
	var c dockerNetConfig
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("parse docker.config: %v", err)
	}
	var keys map[string]json.RawMessage
	_ = json.Unmarshal(raw, &keys)
	_, c.hasUnifiedMirror = keys["registry_mirrors"]
	return c
}

// spellingTestSetup skips unless the test may write the Docker singleton and
// Docker is configured, and registers a restore of the address pools, IPv6
// CIDR and registry mirrors. The restore is sent only when something changed,
// since docker.update restarts Docker.
func spellingTestSetup(t *testing.T) dockerNetConfig {
	t.Helper()
	acctest.DisruptiveCheck(t)
	orig := readDockerNetConfig(t)
	if orig.Pool == nil || *orig.Pool == "" {
		t.Skip("Docker is unconfigured on the target box")
	}
	t.Cleanup(func() {
		now := readDockerNetConfig(t)
		if reflect.DeepEqual(now, orig) {
			return
		}
		payload := map[string]any{"address_pools": orig.AddressPools, "cidr_v6": orig.CIDRv6}
		if orig.hasUnifiedMirror {
			payload["registry_mirrors"] = orig.RegistryMirrors
			if orig.RegistryMirrors == nil {
				payload["registry_mirrors"] = []any{}
			}
		} else {
			payload["secure_registry_mirrors"] = append([]string{}, orig.SecureMirrors...)
			payload["insecure_registry_mirrors"] = append([]string{}, orig.InsecureMirrors...)
		}
		if _, err := acctest.Client().CallJob(context.Background(), "docker.update", payload); err != nil {
			t.Errorf("restore docker config: %v", err)
			return
		}
		if after := readDockerNetConfig(t); !reflect.DeepEqual(after, orig) {
			t.Errorf("docker config not restored: got %+v, want %+v", after, orig)
		}
	})
	return orig
}

// withHostBits respells a network with a host bit set (172.16.0.0/12 ->
// 172.16.0.1/12): the same pool, a different string.
func withHostBits(t *testing.T, base string) string {
	t.Helper()
	p, err := netip.ParsePrefix(base)
	if err != nil {
		t.Fatalf("parse %q: %v", base, err)
	}
	return fmt.Sprintf("%s/%d", p.Masked().Addr().Next(), p.Bits())
}

// TestAccDockerConfig_poolHostBits guards B-078: TrueNAS 28.0 stores an
// address pool base as its network address, so a base with host bits (the
// resource's own documented example, 172.17.0.0/12) failed the apply. The
// box's own pools are respelled, so the server value does not change.
func TestAccDockerConfig_poolHostBits(t *testing.T) {
	orig := spellingTestSetup(t)
	if len(orig.AddressPools) == 0 {
		t.Skip("no address pools configured")
	}
	var pools []string
	for _, p := range orig.AddressPools {
		pools = append(pools, fmt.Sprintf("    { base = %q, size = %v },", withHostBits(t, p["base"].(string)), p["size"]))
	}
	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_docker_config" "test" {
  address_pools = [
%s
  ]
}
`, strings.Join(pools, "\n"))
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps:                    append([]resource.TestStep{{Config: cfg}}, acctest.ImportReapplyNoop("truenas_docker_config.test", cfg)...),
	})
}

// TestAccDockerConfig_cidrV6Spelling guards B-079: a non-canonical IPv6
// spelling of cidr_v6 (upper case, zeros written out) failed the apply. The
// box's own cidr_v6 is respelled.
func TestAccDockerConfig_cidrV6Spelling(t *testing.T) {
	orig := spellingTestSetup(t)
	p, err := netip.ParsePrefix(orig.CIDRv6)
	if err != nil {
		t.Skipf("cidr_v6 %q is not a prefix: %v", orig.CIDRv6, err)
	}
	spelled := fmt.Sprintf("%s/%d", strings.ToUpper(p.Addr().StringExpanded()), p.Bits())
	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_docker_config" "test" {
  cidr_v6 = %q
}
`, spelled)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: append([]resource.TestStep{
			{Config: cfg, Check: resource.TestCheckResourceAttr("truenas_docker_config.test", "cidr_v6", spelled)},
		}, acctest.ImportReapplyNoop("truenas_docker_config.test", cfg)...),
	})
}

// TestAccDockerConfig_mirrorURL guards B-080: TrueNAS stores a registry
// mirror URL with a trailing slash, so a URL without one failed the apply.
// This changes the box's mirrors (Docker restarts); the cleanup restores them.
func TestAccDockerConfig_mirrorURL(t *testing.T) {
	spellingTestSetup(t)
	cfg := acctest.ProviderConfig() + `
resource "truenas_docker_config" "test" {
  registry_mirrors = [{ url = "https://mirror.gcr.io", insecure = false }]
}
`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: append([]resource.TestStep{
			{Config: cfg, Check: resource.TestCheckResourceAttr("truenas_docker_config.test", "registry_mirrors.0.url", "https://mirror.gcr.io")},
		}, acctest.ImportReapplyNoop("truenas_docker_config.test", cfg)...),
	})
}

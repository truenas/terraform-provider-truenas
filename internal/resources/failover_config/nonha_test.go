// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package failover_config_test

import (
	"context"
	"encoding/json"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccFailoverConfig_nonHA guards B-128: on a non-HA box
// (failover.licensed = false) adopting the singleton with no fields, or with
// timeout equal to the current value, must not fail (failover.update rejected
// even an empty payload there), and a real change must fail with a clear
// "not licensed for HA" error rather than one about the never-sent
// "disabled" field.
//
// It writes nothing on a non-HA system (every step either sends no update or
// fails before one), so it runs without TRUENAS_DISRUPTIVE.
func TestAccFailoverConfig_nonHA(t *testing.T) {
	acctest.PreCheck(t)
	if licensedForHA(t) {
		t.Skip("this test needs a system that is not licensed for HA")
	}
	base := acctest.ProviderConfig()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: base + `
resource "truenas_failover_config" "test" {}
`},
			{Config: base + `
resource "truenas_failover_config" "test" {
  timeout = 0
}
`},
			{
				Config: base + `
resource "truenas_failover_config" "test" {
  timeout = 7
}
`,
				ExpectError: regexp.MustCompile(`not\s+licensed\s+for\s+TrueNAS\s+Enterprise\s+HA`),
			},
		},
	})
}

func licensedForHA(t *testing.T) bool {
	t.Helper()
	raw, err := acctest.RestoreCall(context.Background(), "failover.licensed")
	if err != nil {
		t.Fatal(err)
	}
	var licensed bool
	if err := json.Unmarshal(raw, &licensed); err != nil {
		t.Fatal(err)
	}
	return licensed
}

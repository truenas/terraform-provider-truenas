// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package app_test

import (
	"context"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// Second apply on a stopped app: change values while running=false.
func TestHuntApp_stoppedValuesChange(t *testing.T) {
	name := "tfhuntappst"
	v1 := `jsonencode({ flame = { password = "x" }, network = { web_port = { port_number = 30997 } } })`
	v2 := `jsonencode({ flame = { password = "y" }, network = { web_port = { port_number = 30997 } } })`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.AppsCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: acctest.ProviderConfig() + flameAppConfig(name, `  running = false`, v1)},
			{Config: acctest.ProviderConfig() + flameAppConfig(name, `  running = false`, v2),
				Check: resource.TestCheckResourceAttr("truenas_app.test", "running", "false")},
			{Config: acctest.ProviderConfig() + flameAppConfig(name, `  running = true`, v1),
				Check: resource.TestCheckResourceAttr("truenas_app.test", "running", "true")},
			appCleanupStep(t, name, ""),
		},
	})
}

// Pattern 3 probe: what a READONLY_ADMIN key sees from app.config / vm.device.
func TestHuntApp_readonlyProbe(t *testing.T) {
	if os.Getenv("TFHUNT_RO") == "" {
		t.Skip("TFHUNT_RO unset")
	}
	c := acctest.Client()
	for _, call := range [][]any{
		{"app.config", "flame"},
		{"vm.device.query", [][]any{{"attributes.dtype", "=", "DISPLAY"}}},
		{"app.registry.query"},
	} {
		raw, err := c.Call(context.Background(), call[0].(string), call[1:]...)
		s := string(raw)
		if len(s) > 400 {
			s = s[:400]
		}
		t.Logf("%v -> err=%v %s", call[0], err, s)
	}
}

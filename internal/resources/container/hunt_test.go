// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package container_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// Running container: change cpuset while running.
func TestHuntContainer_runningUpdate(t *testing.T) {
	name := "tfhuntct4"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: acctest.ProviderConfig() + containerConfig(name, latest, `  running = true`)},
			{Config: acctest.ProviderConfig() + containerConfig(name, latest, `
  running = true
  cpuset  = "0-1"
  time    = "UTC"
`)},
		},
	})
}

// idmap ISOLATED with slice = null; import then replan.
func TestHuntContainer_idmapIsolated(t *testing.T) {
	name := "tfhuntct3"
	var id string
	cfg := acctest.ProviderConfig() + containerConfig(name, latest, `  idmap = jsonencode({ type = "ISOLATED", slice = null })`)
	steps := []resource.TestStep{{Config: cfg, Check: captureID("truenas_container.test", &id)}}
	steps = append(steps, importReplanSteps(cfg, acctest.ProviderConfig()+forgetCt, "truenas_container.test", &id)...)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps:                    steps,
	})
}

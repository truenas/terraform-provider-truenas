// Copyright iXsystems, Inc. 2026
// SPDX-License-Identifier: MPL-2.0

// Package load holds the Terraform end-to-end load tests and their config
// generator. See docs/superpowers/specs/2026-08-12-load-testing-design.md.
package load

import (
	"fmt"
	"strings"
)

// GenerateConfig returns HCL declaring n truenas_dataset resources (and, when
// snapshots is true, one truenas_snapshot each) under pool, each named
// "<pool>/<prefix>-<i>". The caller passes a unique prefix per config so that
// concurrent or repeated applies never collide on dataset names. The provider
// block reads endpoint/auth from TRUENAS_* env vars, so no secrets are written
// into the config.
func GenerateConfig(pool, prefix string, n int, snapshots bool) string {
	var b strings.Builder
	b.WriteString(`terraform {
  required_providers {
    truenas = {
      source = "truenas/truenas"
    }
  }
}

provider "truenas" {
  insecure = true
}

`)
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, `resource "truenas_dataset" "d%d" {
  name = "%s/%s-%d"
}

`, i, pool, prefix, i)
		if snapshots {
			fmt.Fprintf(&b, `resource "truenas_snapshot" "s%d" {
  dataset = truenas_dataset.d%d.name
  name    = "load"
}

`, i, i)
		}
	}
	return b.String()
}

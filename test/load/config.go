// Copyright TrueNAS 2026
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

// GenerateMixedConfig returns HCL declaring n instances each of eight resource
// types, all tf-load-prefixed, under pool. The variant value is woven into an
// updatable attribute (comments/comment/full_name/description) so applying a
// second config with a different variant is an in-place update, not a replace.
// Shares reference their dataset's mountpoint; snapshots reference the dataset
// name — so Terraform orders creation correctly.
func GenerateMixedConfig(pool string, n, variant int) string {
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
		fmt.Fprintf(&b, `resource "truenas_dataset" "ds%d" {
  name = "%s/tf-load-ds-%d"
  comments = "load v%d"
}

resource "truenas_zvol" "zv%d" {
  name = "%s/tf-load-zv-%d"
  volsize = 134217728
}

resource "truenas_snapshot" "sn%d" {
  dataset = truenas_dataset.ds%d.name
  name = "load"
}

resource "truenas_smb_share" "smb%d" {
  path = truenas_dataset.ds%d.mountpoint
  name = "tf-load-smb-%d"
  comment = "load v%d"
  enabled = true
}

resource "truenas_nfs_share" "nfs%d" {
  path = truenas_dataset.ds%d.mountpoint
  comment = "tf-load v%d"
  enabled = true
}

resource "truenas_user" "usr%d" {
  username  = "tf-load-usr-%d"
  full_name = "load v%d"
  password = "Tf-Load-Passw0rd!"
  shell = "/usr/sbin/nologin"
  home = "/var/empty"
  smb = false
  group_create = true
}

resource "truenas_group" "grp%d" {
  name = "tf-load-grp-%d"
  smb = false
}

resource "truenas_cronjob" "cron%d" {
  command = "/bin/true"
  user = "root"
  description = "tf-load-cron-%d v%d"
  enabled = false
  schedule = {
    minute = "0"
    hour = "0"
    dom = "*"
    month = "*"
    dow = "*"
  }
}

`,
			i, pool, i, variant, // dataset
			i, pool, i, // zvol
			i, i, // snapshot
			i, i, i, variant, // smb
			i, i, variant, // nfs
			i, i, variant, // user
			i, i, // group
			i, i, variant, // cronjob
		)
	}
	return b.String()
}

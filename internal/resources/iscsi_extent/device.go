// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_extent

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/truenas/terraform-provider-truenas/internal/client"
)

// deviceWaitTimeout bounds how long create/update keep retrying while the
// backing zvol's device node does not exist yet.
const (
	deviceWaitTimeout  = 60 * time.Second
	deviceWaitInterval = 2 * time.Second
)

// isDeviceMissing reports whether err is TrueNAS's validation error for a
// DISK extent whose /dev/zvol node does not exist ("Device '/dev/zvol/...'
// for volume '...' does not exist", the same text on 25.10, 26.0 and 28.0).
func isDeviceMissing(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Device '/dev/zvol/") && strings.Contains(msg, "does not exist")
}

// callWaitingForDevice calls method and, while TrueNAS rejects it because
// the zvol device does not exist yet, retries until deviceWaitTimeout.
//
// Issue #40: when disk is a literal string rather than a reference to the
// truenas_zvol, Terraform creates the zvol and the extent at the same time,
// and the extent's create runs before udev has made /dev/zvol/<name>. The
// zvol resource's own wait does not help because nothing orders the two.
// The rejected call creates nothing, so retrying it is safe.
func callWaitingForDevice(ctx context.Context, c *client.Client, method string, params ...any) (json.RawMessage, error) {
	deadline := time.Now().Add(deviceWaitTimeout)
	for {
		raw, err := c.Call(ctx, method, params...)
		if !isDeviceMissing(err) || time.Now().After(deadline) {
			return raw, err
		}
		select {
		case <-ctx.Done():
			return nil, err
		case <-time.After(deviceWaitInterval):
		}
	}
}

// zvolName returns the ZFS name behind a "zvol/<pool>/<name>" disk path, the
// way the server maps it (zvol_path_to_name: "+" in the device path is a
// space in the dataset name). ok is false for anything that is not a zvol
// path, and for snapshots, which have no readonly property.
func zvolName(disk string) (string, bool) {
	if !strings.HasPrefix(disk, "zvol/") {
		return "", false
	}
	name := strings.ReplaceAll(strings.TrimPrefix(disk, "zvol/"), "+", " ")
	if name == "" || strings.Contains(name, "@") {
		return "", false
	}
	return name, true
}

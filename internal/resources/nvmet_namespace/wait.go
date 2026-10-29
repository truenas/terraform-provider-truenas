// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_namespace

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
// ZVOL namespace whose /dev/zvol node does not exist yet ("ZVOL device_path
// must be a block device: zvol/...", the same text on 25.10, 26.0 and 28.0).
func isDeviceMissing(err error) bool {
	return err != nil && strings.Contains(err.Error(), "ZVOL device_path must be a block device")
}

// callWaitingForDevice calls method and, while TrueNAS rejects it because
// the zvol device does not exist yet, retries until deviceWaitTimeout.
//
// Same gap as issue #40 for iscsi_extent: when device_path is a literal
// string rather than a reference to the truenas_zvol, Terraform creates the
// zvol and the namespace at the same time and the namespace call can run
// before /dev/zvol/<name> exists. The rejected call changes nothing, so
// retrying it is safe.
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

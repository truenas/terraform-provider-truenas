// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
)

// ServerVersion returns the TrueNAS release as reported by
// system.version_short (e.g. "25.10.3.1" or "26.0.0"), cached after the
// first successful call.
func (c *Client) ServerVersion(ctx context.Context) (string, error) {
	c.versionMu.Lock()
	defer c.versionMu.Unlock()
	if c.version != "" {
		return c.version, nil
	}

	raw, err := c.CallRead(ctx, "system.version_short")
	if err != nil {
		return "", err
	}
	var version string
	if err := json.Unmarshal(raw, &version); err != nil {
		return "", err
	}
	c.version = version
	return version, nil
}

// VersionAtLeast reports whether the server release is at or above
// major.minor. It is used to gate fields that exist only on newer TrueNAS
// releases so the provider can fail with a clear message instead of the
// API's generic "Extra inputs are not permitted".
func (c *Client) VersionAtLeast(ctx context.Context, major, minor int) (bool, error) {
	v, err := c.ServerVersion(ctx)
	if err != nil {
		return false, err
	}
	return versionAtLeast(v, major, minor), nil
}

// VersionAtLeastString is the pure comparator VersionAtLeast wraps: given an
// already-probed dotted release string (e.g. from ServerVersion), it reports
// whether the string's major.minor prefix is at or above the given floor,
// with no client or network access. Exported so resources that gate their
// entire Create/Read/Update behind a version floor (e.g. lxc_config, absent
// below TrueNAS 26.0) can build their own diagnostics as pure,
// client-independent functions of a probed version string, independently
// unit-testable without a live TrueNAS connection.
func VersionAtLeastString(version string, major, minor int) bool {
	return versionAtLeast(version, major, minor)
}

// versionAtLeast compares a dotted release string's leading major.minor
// against the given floor. Unparseable strings compare as 0.0 (below any
// real floor), so a malformed version fails closed on "new enough" checks.
func versionAtLeast(version string, major, minor int) bool {
	parts := strings.Split(version, ".")
	var vMajor, vMinor int
	if len(parts) > 0 {
		vMajor, _ = strconv.Atoi(parts[0])
	}
	if len(parts) > 1 {
		vMinor, _ = strconv.Atoi(parts[1])
	}
	if vMajor != major {
		return vMajor > major
	}
	return vMinor >= minor
}

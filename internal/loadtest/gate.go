// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package loadtest

import (
	"os"
	"testing"

	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// loadGateDecision is the pure core of LoadCheck. Given the three env values
// it reports whether to skip and, if not skipped, a fatal message (empty when
// the gate passes).
func loadGateDecision(load, allowed, endpoint string) (skip bool, fatal string) {
	if load != "1" {
		return true, ""
	}
	if allowed == "" {
		return false, "TRUENAS_LOAD=1 requires TRUENAS_LOAD_ALLOWED_ENDPOINT set to the disposable " +
			"load-test box's endpoint, as a guard against load-testing a shared or production TrueNAS box"
	}
	if allowed != endpoint {
		return false, "TRUENAS_LOAD_ALLOWED_ENDPOINT does not match TRUENAS_ENDPOINT: refusing to load-test " +
			"a box that isn't the designated disposable load-test box"
	}
	return false, ""
}

// LoadCheck gates the live load tests. It runs acctest.PreCheck, skips unless
// TRUENAS_LOAD=1, then requires TRUENAS_LOAD_ALLOWED_ENDPOINT to equal
// acctest.Endpoint() exactly.
func LoadCheck(t *testing.T) {
	t.Helper()
	acctest.PreCheck(t)
	skip, fatal := loadGateDecision(
		os.Getenv("TRUENAS_LOAD"),
		os.Getenv("TRUENAS_LOAD_ALLOWED_ENDPOINT"),
		acctest.Endpoint(),
	)
	if skip {
		t.Skip("Set TRUENAS_LOAD=1 (and TRUENAS_LOAD_ALLOWED_ENDPOINT) to run load tests")
	}
	if fatal != "" {
		t.Fatal(fatal)
	}
}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest_test

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// logPlanDiff keeps the hunt probes' short spelling of acctest.LogPlanDiff.
type logPlanDiff struct{ t *testing.T }

func (c logPlanDiff) CheckPlan(ctx context.Context, req plancheck.CheckPlanRequest, resp *plancheck.CheckPlanResponse) {
	acctest.LogPlanDiff(c.t).CheckPlan(ctx, req, resp)
}

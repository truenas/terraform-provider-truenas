// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest_test

// immutableDefaultTrapRatchet lists resource attributes that carry both a
// schema Default and an unconditional RequiresReplace(). Key is
// "<type>.<attr>", value is the reason. Seeded empty: no such attribute exists.
// Do not add entries to silence the audit; fix the field with a conditional
// RequiresReplaceIf instead.
var immutableDefaultTrapRatchet = map[string]string{}

// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package normalize

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseTimeOfDay parses "H:MM", "HH:MM", "H:M" or the same with ":SS" and
// returns seconds since midnight. Hours are 0-23, minutes and seconds 0-59.
func ParseTimeOfDay(s string) (int, error) {
	parts := strings.Split(s, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, fmt.Errorf("%q is not a time of day (expected HH:MM)", s)
	}
	limits := []int{23, 59, 59}
	total := 0
	for i, p := range parts {
		if len(p) < 1 || len(p) > 2 || strings.Trim(p, "0123456789") != "" {
			return 0, fmt.Errorf("%q is not a time of day (expected HH:MM)", s)
		}
		n, _ := strconv.Atoi(p)
		if n > limits[i] {
			return 0, fmt.Errorf("%q is not a time of day: %d is out of range", s, n)
		}
		total = total*60 + n
	}
	if len(parts) == 2 {
		total *= 60
	}
	return total, nil
}

// EqualTimeOfDay reports whether two "HH:MM" strings are the same time
// ("8:00" == "08:00", "8:5" == "08:05", "08:00" == "08:00:00").
func EqualTimeOfDay(a, b string) bool {
	if a == b {
		return true
	}
	x, err1 := ParseTimeOfDay(a)
	y, err2 := ParseTimeOfDay(b)
	return err1 == nil && err2 == nil && x == y
}

type timeOfDayKind struct{}

func (timeOfDayKind) name() string                 { return "TimeOfDay" }
func (timeOfDayKind) equal(prior, got string) bool { return EqualTimeOfDay(prior, got) }
func (timeOfDayKind) validate(s string) error {
	_, err := ParseTimeOfDay(s)
	return err
}

// EqualRFC3339 reports whether two RFC3339 timestamps are the same instant to
// the second, whatever their offset or fractional seconds
// ("2035-01-01T02:00:00+02:00" == "2035-01-01T00:00:00Z"). The provider reads
// timestamps back at one-second precision, so sub-second parts are dropped.
func EqualRFC3339(a, b string) bool {
	if a == b {
		return true
	}
	x, err1 := time.Parse(time.RFC3339Nano, a)
	y, err2 := time.Parse(time.RFC3339Nano, b)
	return err1 == nil && err2 == nil && x.Truncate(time.Second).Equal(y.Truncate(time.Second))
}

type rfc3339Kind struct{}

func (rfc3339Kind) name() string                 { return "RFC3339" }
func (rfc3339Kind) equal(prior, got string) bool { return EqualRFC3339(prior, got) }
func (rfc3339Kind) validate(s string) error {
	if _, err := time.Parse(time.RFC3339Nano, s); err != nil {
		return fmt.Errorf("%q is not an RFC3339 timestamp (for example 2035-01-01T00:00:00Z): %v", s, err)
	}
	return nil
}

// ParseOctalMode parses a Unix permission mode written in octal, with or
// without a leading "0" or "0o" ("755", "0755", "0o755", "1777").
func ParseOctalMode(s string) (uint32, error) {
	t := strings.TrimPrefix(strings.TrimPrefix(s, "0o"), "0O")
	if t == "" || len(t) > 5 || strings.Trim(t, "01234567") != "" {
		return 0, fmt.Errorf("%q is not an octal file mode (for example 0755)", s)
	}
	n, err := strconv.ParseUint(t, 8, 32)
	if err != nil || n > 0o7777 {
		return 0, fmt.Errorf("%q is not an octal file mode (at most 07777)", s)
	}
	return uint32(n), nil
}

// EqualOctalMode reports whether two octal mode strings are the same mode
// ("755" == "0755").
func EqualOctalMode(a, b string) bool {
	if a == b {
		return true
	}
	x, err1 := ParseOctalMode(a)
	y, err2 := ParseOctalMode(b)
	return err1 == nil && err2 == nil && x == y
}

type octalModeKind struct{}

func (octalModeKind) name() string                 { return "OctalMode" }
func (octalModeKind) equal(prior, got string) bool { return EqualOctalMode(prior, got) }
func (octalModeKind) validate(s string) error {
	_, err := ParseOctalMode(s)
	return err
}

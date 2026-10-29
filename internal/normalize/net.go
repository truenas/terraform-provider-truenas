// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package normalize

import (
	"errors"
	"fmt"
	"math/bits"
	"net/netip"
	"strconv"
	"strings"
)

// ParsePrefix parses an IP address or network in any spelling TrueNAS
// accepts: a bare address ("10.0.0.1", "2001:db8::1", with any case or zero
// padding), an address with a prefix length ("10.0.0.0/24"), or an IPv4
// address with a dotted netmask ("10.0.0.0/255.255.255.0"). A bare address
// becomes a full-length prefix (/32 or /128). Host bits are kept; call
// Masked() on the result for the network address.
func ParsePrefix(s string) (netip.Prefix, error) {
	addrPart, maskPart, hasMask := strings.Cut(s, "/")
	addr, err := netip.ParseAddr(addrPart)
	if err != nil {
		return netip.Prefix{}, err
	}
	if !hasMask {
		return netip.PrefixFrom(addr, addr.BitLen()), nil
	}
	if n, err := strconv.Atoi(maskPart); err == nil && maskPart != "" && maskPart[0] != '+' && maskPart[0] != '-' {
		if n > addr.BitLen() {
			return netip.Prefix{}, fmt.Errorf("prefix length %d is too long for %s", n, addrPart)
		}
		return netip.PrefixFrom(addr, n), nil
	}
	mask, err := netip.ParseAddr(maskPart)
	if err != nil || !addr.Is4() || !mask.Is4() {
		return netip.Prefix{}, fmt.Errorf("%q is not a prefix length or IPv4 netmask", maskPart)
	}
	m4 := mask.As4()
	m := uint32(m4[0])<<24 | uint32(m4[1])<<16 | uint32(m4[2])<<8 | uint32(m4[3])
	n := bits.LeadingZeros32(^m)
	if n < 32 && m<<n != 0 {
		return netip.Prefix{}, fmt.Errorf("netmask %s is not contiguous", maskPart)
	}
	return netip.PrefixFrom(addr, n), nil
}

// EqualIP reports whether two strings are the same IP address (IPv6
// compressed/exploded/case differences ignored). A string that does not parse
// as an address is equal only to itself.
func EqualIP(a, b string) bool {
	if a == b {
		return true
	}
	x, err1 := netip.ParseAddr(a)
	y, err2 := netip.ParseAddr(b)
	return err1 == nil && err2 == nil && x == y
}

// EqualCIDR reports whether two strings are the same address and prefix
// length. A bare address equals its /32 (/128) form, a dotted netmask equals
// its prefix length, and IPv6 spelling is ignored. Host bits matter:
// "10.0.0.5/24" is not "10.0.0.0/24" (use EqualNetwork for that).
func EqualCIDR(a, b string) bool {
	if a == b {
		return true
	}
	x, err1 := ParsePrefix(a)
	y, err2 := ParsePrefix(b)
	return err1 == nil && err2 == nil && x == y
}

// EqualNetwork is EqualCIDR with host bits ignored: "172.17.0.0/12" equals
// "172.16.0.0/12" because both name the same network.
func EqualNetwork(a, b string) bool {
	if a == b {
		return true
	}
	x, err1 := ParsePrefix(a)
	y, err2 := ParsePrefix(b)
	return err1 == nil && err2 == nil && x.Masked() == y.Masked()
}

type ipKind struct{}

func (ipKind) name() string                 { return "IPAddress" }
func (ipKind) equal(prior, got string) bool { return EqualIP(prior, got) }
func (ipKind) validate(s string) error {
	if _, err := netip.ParseAddr(s); err != nil {
		return fmt.Errorf("%q is not an IP address", s)
	}
	return nil
}

type cidrKind struct{}

func (cidrKind) name() string                 { return "CIDR" }
func (cidrKind) equal(prior, got string) bool { return EqualCIDR(prior, got) }
func (cidrKind) validate(s string) error      { return validatePrefix(s) }

type networkKind struct{}

func (networkKind) name() string                 { return "Network" }
func (networkKind) equal(prior, got string) bool { return EqualNetwork(prior, got) }
func (networkKind) validate(s string) error      { return validatePrefix(s) }

func validatePrefix(s string) error {
	if _, err := ParsePrefix(s); err != nil {
		return errors.New(`"` + s + `" is not an IP address or network (expected forms: 10.0.0.1, 10.0.0.0/24, 10.0.0.0/255.255.255.0, 2001:db8::/64)`)
	}
	return nil
}

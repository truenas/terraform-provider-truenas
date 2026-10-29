// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package normalize

import (
	"bytes"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"net"
	"net/netip"
	"net/url"
	"strings"
)

// EqualMAC reports whether two strings are the same MAC address, whatever the
// case or separator ("00:A0:98:12:34:56" == "00-a0-98-12-34-56"). Strings that
// do not parse as a MAC are equal only to themselves.
func EqualMAC(a, b string) bool {
	if a == b {
		return true
	}
	x, err1 := net.ParseMAC(a)
	y, err2 := net.ParseMAC(b)
	return err1 == nil && err2 == nil && bytes.Equal(x, y)
}

type caseInsensitiveKind struct{}

func (caseInsensitiveKind) name() string                 { return "CaseInsensitive" }
func (caseInsensitiveKind) equal(prior, got string) bool { return strings.EqualFold(prior, got) }
func (caseInsensitiveKind) validate(string) error        { return nil }

// EqualEmail reports whether two e-mail addresses are the same with the
// domain part compared case-insensitively (pydantic EmailStr lower-cases the
// domain; the local part is kept as given and compared verbatim). Strings
// without an "@" are equal only to themselves.
func EqualEmail(a, b string) bool {
	if a == b {
		return true
	}
	ia, ib := strings.LastIndexByte(a, '@'), strings.LastIndexByte(b, '@')
	if ia < 0 || ib < 0 {
		return false
	}
	return a[:ia] == b[:ib] && strings.EqualFold(a[ia+1:], b[ib+1:])
}

type emailKind struct{}

func (emailKind) name() string                 { return "Email" }
func (emailKind) equal(prior, got string) bool { return EqualEmail(prior, got) }
func (emailKind) validate(string) error        { return nil }

// EqualTrimmed reports whether two strings are equal after CRLF is turned
// into LF and leading/trailing whitespace is removed (the server strips the
// trailing newline from SSH public keys and adds one to PEM keys).
func EqualTrimmed(a, b string) bool {
	return trimText(a) == trimText(b)
}

func trimText(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
}

type trimmedKind struct{}

func (trimmedKind) name() string                 { return "Trimmed" }
func (trimmedKind) equal(prior, got string) bool { return EqualTrimmed(prior, got) }
func (trimmedKind) validate(string) error        { return nil }

// EqualURL reports whether two URLs are the same after the normalization
// pydantic HttpUrl applies on the server: scheme and host are compared
// case-insensitively, the default port (:80 for http/ws, :443 for https/wss)
// is dropped, and a trailing "/" on the path is ignored ("https://h" ==
// "https://h/", "https://h/x/" == "https://h/x"). Query, fragment, user info
// and path case are compared verbatim. Strings that are not absolute URLs
// (no scheme or host) are compared after trimming trailing "/" only.
func EqualURL(a, b string) bool {
	if a == b {
		return true
	}
	ca, oka := canonicalURL(a)
	cb, okb := canonicalURL(b)
	if !oka || !okb {
		return !oka && !okb && strings.TrimRight(a, "/") == strings.TrimRight(b, "/")
	}
	return ca == cb
}

func canonicalURL(s string) (string, bool) {
	u, err := url.Parse(s)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", false
	}
	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	switch {
	case port == "80" && (scheme == "http" || scheme == "ws"),
		port == "443" && (scheme == "https" || scheme == "wss"):
		port = ""
	}
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if port != "" {
		host += ":" + port
	}
	user := ""
	if u.User != nil {
		user = u.User.String() + "@"
	}
	q := ""
	if u.ForceQuery || u.RawQuery != "" {
		q = "?" + u.RawQuery
	}
	f := ""
	if u.Fragment != "" {
		f = "#" + u.EscapedFragment()
	}
	return scheme + "://" + user + host + strings.TrimRight(u.EscapedPath(), "/") + q + f, true
}

type urlKind struct{}

func (urlKind) name() string                 { return "URL" }
func (urlKind) equal(prior, got string) bool { return EqualURL(prior, got) }
func (urlKind) validate(s string) error {
	_, err := url.Parse(s)
	return err
}

// EqualSAN reports whether two certificate subjectAltName entries are the
// same. TrueNAS stores "10.0.0.1" and "IP:10.0.0.1" as "IP Address:10.0.0.1",
// and a bare name as "DNS:name". Prefixes are matched case-insensitively; an
// entry without a prefix is an IP when it parses as one and a DNS name
// otherwise. IPs compare as addresses, DNS names case-insensitively, e-mail
// with EqualEmail, anything else verbatim.
func EqualSAN(a, b string) bool {
	if a == b {
		return true
	}
	ta, va := splitSAN(a)
	tb, vb := splitSAN(b)
	if ta != tb {
		return false
	}
	switch ta {
	case "ip":
		return EqualIP(va, vb)
	case "dns":
		return strings.EqualFold(va, vb)
	case "email":
		return EqualEmail(va, vb)
	}
	return va == vb
}

func splitSAN(s string) (string, string) {
	s = strings.TrimSpace(s)
	lower := strings.ToLower(s)
	for _, p := range []struct{ prefix, typ string }{
		{"ip address:", "ip"},
		{"ip:", "ip"},
		{"dns:", "dns"},
		{"email:", "email"},
		{"uri:", "uri"},
	} {
		if strings.HasPrefix(lower, p.prefix) {
			return p.typ, strings.TrimSpace(s[len(p.prefix):])
		}
	}
	if _, err := netip.ParseAddr(s); err == nil {
		return "ip", s
	}
	return "dns", s
}

type sanKind struct{}

func (sanKind) name() string                 { return "SAN" }
func (sanKind) equal(prior, got string) bool { return EqualSAN(prior, got) }
func (sanKind) validate(string) error        { return nil }

// EqualPrivateKey reports whether two PEM private keys hold the same key,
// whatever the encoding: PKCS#1 ("RSA PRIVATE KEY"), SEC1 ("EC PRIVATE KEY",
// optionally after an "EC PARAMETERS" block) or PKCS#8 ("PRIVATE KEY"). The
// server re-encodes imported keys as PKCS#8. Keys that do not parse
// (passphrase-encrypted keys included) fall back to EqualTrimmed.
func EqualPrivateKey(a, b string) bool {
	if EqualTrimmed(a, b) {
		return true
	}
	ka, ok1 := parsePrivateKey(a)
	kb, ok2 := parsePrivateKey(b)
	if !ok1 || !ok2 {
		return false
	}
	eq, ok := ka.(interface{ Equal(crypto.PrivateKey) bool })
	return ok && eq.Equal(kb)
}

func parsePrivateKey(s string) (crypto.PrivateKey, bool) {
	rest := []byte(s)
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			return nil, false
		}
		if _, encrypted := block.Headers["Proc-Type"]; encrypted {
			return nil, false
		}
		switch block.Type {
		case "PRIVATE KEY":
			k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
			return k, err == nil
		case "RSA PRIVATE KEY":
			k, err := x509.ParsePKCS1PrivateKey(block.Bytes)
			return k, err == nil
		case "EC PRIVATE KEY":
			k, err := x509.ParseECPrivateKey(block.Bytes)
			return k, err == nil
		}
	}
}

type privateKeyKind struct{}

func (privateKeyKind) name() string                 { return "PrivateKey" }
func (privateKeyKind) equal(prior, got string) bool { return EqualPrivateKey(prior, got) }
func (privateKeyKind) validate(string) error        { return nil }

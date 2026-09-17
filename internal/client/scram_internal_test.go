// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
)

// TestScramConversation_ClientFirstWireForm pins the exact client-first
// message bytes: GS2 header, username:key_id identity, base64 nonce.
func TestScramConversation_ClientFirstWireForm(t *testing.T) {
	nonce := []byte("0123456789abcdef0123456789abcdef") // 32 bytes
	sc := &scramConversation{}
	got := sc.clientFirstMessage("truenas_admin", 2, nonce)
	want := "n,,n=truenas_admin:2,r=" + base64.StdEncoding.EncodeToString(nonce)
	if got != want {
		t.Fatalf("client-first = %q, want %q", got, want)
	}
	if sc.clientFirstBare != strings.TrimPrefix(want, "n,,") {
		t.Fatalf("client-first-bare = %q", sc.clientFirstBare)
	}
}

// TestScramConversation_BadServerSignatureRejected verifies the client
// rejects a signature it cannot reproduce (mutual auth). Pure input test:
// a syntactically valid v= that does not match the conversation.
func TestScramConversation_BadServerSignatureRejected(t *testing.T) {
	nonce := []byte("0123456789abcdef0123456789abcdef")
	combined := base64.StdEncoding.EncodeToString(append(append([]byte{}, nonce...), []byte("SERVERNONCE-32-BYTES-0123456789a")...))
	sc := &scramConversation{}
	sc.clientFirstMessage("u", 1, nonce)
	serverFirst := fmt.Sprintf("r=%s,s=%s,i=500000",
		combined, base64.StdEncoding.EncodeToString([]byte("0123456789abcdef")))
	if err := sc.processServerFirst(serverFirst, "secret"); err != nil {
		t.Fatal(err)
	}
	sc.clientFinalMessage()
	bogus := "v=" + base64.StdEncoding.EncodeToString([]byte("not the signature"))
	if err := sc.verifyServerFinal(bogus); err == nil {
		t.Fatal("client accepted a bogus server signature")
	}
	if err := sc.verifyServerFinal("garbage"); err == nil {
		t.Fatal("client accepted a malformed server-final message")
	}
}

// TestScramConversation_NonceMismatchRejected verifies the client refuses a
// server nonce that does not extend the client nonce.
func TestScramConversation_NonceMismatchRejected(t *testing.T) {
	sc := &scramConversation{}
	sc.clientFirstMessage("u", 1, []byte("0123456789abcdef0123456789abcdef"))
	serverFirst := fmt.Sprintf("r=%s,s=%s,i=50000",
		base64.StdEncoding.EncodeToString([]byte("DIFFERENT-NONCE-BYTES-0123456789-abcdef")),
		base64.StdEncoding.EncodeToString([]byte("0123456789abcdef")))
	if err := sc.processServerFirst(serverFirst, "secret"); err == nil {
		t.Fatal("accepted server nonce that does not extend client nonce")
	}
}

// TestScramConversation_BadServerFirstRejected verifies the client
// enforces the truenas_scram wire limits on the server-first message.
func TestScramConversation_BadServerFirstRejected(t *testing.T) {
	nonce := []byte("0123456789abcdef0123456789abcdef")
	combined := base64.StdEncoding.EncodeToString(append(append([]byte{}, nonce...), []byte("SERVERNONCE")...))
	salt16 := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef"))

	cases := []struct {
		name        string
		serverFirst string
	}{
		{"iterations below floor", fmt.Sprintf("r=%s,s=%s,i=49999", combined, salt16)},
		{"iterations above ceiling", fmt.Sprintf("r=%s,s=%s,i=5000001", combined, salt16)},
		{"salt wrong size", fmt.Sprintf("r=%s,s=%s,i=500000", combined, base64.StdEncoding.EncodeToString([]byte("short")))},
		{"salt not base64", fmt.Sprintf("r=%s,s=!!!,i=500000", combined)},
		{"server nonce not base64", fmt.Sprintf("r=!!!,s=%s,i=500000", salt16)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sc := &scramConversation{}
			sc.clientFirstMessage("u", 1, nonce)
			if err := sc.processServerFirst(tc.serverFirst, "secret"); err == nil {
				t.Fatalf("accepted bad server-first %q", tc.serverFirst)
			}
		})
	}
}

func TestSplitAPIKey(t *testing.T) {
	id, secret, err := splitAPIKey("2-8P4RSecretPart")
	if err != nil || id != 2 || secret != "8P4RSecretPart" {
		t.Fatalf("splitAPIKey = (%d, %q, %v)", id, secret, err)
	}
	if _, _, err := splitAPIKey("no-dash-prefix"); err == nil {
		t.Fatal("accepted key with non-numeric id")
	}
	if _, _, err := splitAPIKey("nodash"); err == nil {
		t.Fatal("accepted key without separator")
	}
}

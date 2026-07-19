package client

import (
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
)

// testScramServer implements the server half of SCRAM-SHA-512 the way the
// TrueNAS middleware does, so the client conversation can be verified
// end-to-end without a box.
type testScramServer struct {
	secret     string
	salt       []byte
	iterations int
	nonceExt   string

	serverFirst string
}

func (s *testScramServer) handleClientFirst(clientFirst string) (string, error) {
	bare, ok := strings.CutPrefix(clientFirst, "n,,")
	if !ok {
		return "", fmt.Errorf("missing gs2 header: %q", clientFirst)
	}
	var clientNonce string
	for _, part := range strings.Split(bare, ",") {
		if v, ok := strings.CutPrefix(part, "r="); ok {
			clientNonce = v
		}
	}
	if clientNonce == "" {
		return "", fmt.Errorf("no client nonce in %q", clientFirst)
	}
	// Binary nonce extension, as the TrueNAS server does it: decode the
	// client nonce, append the server bytes, re-encode the combined value.
	raw, err := base64.StdEncoding.DecodeString(clientNonce)
	if err != nil {
		return "", fmt.Errorf("client nonce not base64: %w", err)
	}
	combined := base64.StdEncoding.EncodeToString(append(raw, []byte(s.nonceExt)...))
	s.serverFirst = fmt.Sprintf("r=%s,s=%s,i=%d",
		combined, base64.StdEncoding.EncodeToString(s.salt), s.iterations)
	return s.serverFirst, nil
}

// handleClientFinal verifies the client proof and returns the server-final
// message, mirroring RFC 5802 server-side verification.
func (s *testScramServer) handleClientFinal(clientFirstBare, clientFinal string) (string, error) {
	idx := strings.LastIndex(clientFinal, ",p=")
	if idx < 0 {
		return "", fmt.Errorf("no proof in %q", clientFinal)
	}
	withoutProof := clientFinal[:idx]
	proof, err := base64.StdEncoding.DecodeString(clientFinal[idx+3:])
	if err != nil {
		return "", err
	}

	salted, err := pbkdf2.Key(sha512.New, s.secret, s.salt, s.iterations, sha512.Size)
	if err != nil {
		return "", err
	}
	clientKey := hmacSHA512(salted, []byte("Client Key"))
	storedKey := sha512.Sum512(clientKey)
	authMessage := clientFirstBare + "," + s.serverFirst + "," + withoutProof
	clientSignature := hmacSHA512(storedKey[:], []byte(authMessage))

	// Recover ClientKey from proof and check its hash against StoredKey.
	recovered := make([]byte, len(proof))
	for i := range proof {
		recovered[i] = proof[i] ^ clientSignature[i]
	}
	recoveredHash := sha512.Sum512(recovered)
	if !hmac.Equal(recoveredHash[:], storedKey[:]) {
		return "", fmt.Errorf("client proof verification failed")
	}

	serverKey := hmacSHA512(salted, []byte("Server Key"))
	serverSignature := hmacSHA512(serverKey, []byte(authMessage))
	return "v=" + base64.StdEncoding.EncodeToString(serverSignature), nil
}

// TestScramConversation_FullExchange drives the client conversation against
// the RFC-faithful test server: the proof must verify server-side and the
// server signature must verify client-side.
func TestScramConversation_FullExchange(t *testing.T) {
	srv := &testScramServer{
		secret:     "uz8DhKHFhRIUQIvjzabPYtpy5wf1DJ3ZBLlDgNVhRAFT7Y6pJGUlm0n3apwxWEU4",
		salt:       []byte("0123456789abcdef"),
		iterations: 5000, // fast for tests; TrueNAS uses 500000
		nonceExt:   "SERVERNONCE",
	}

	nonce := []byte("0123456789abcdef0123456789abcdef") // 32 bytes
	sc := &scramConversation{}
	clientFirst := sc.clientFirstMessage("truenas_admin", 2, nonce)
	want := "n,,n=truenas_admin:2,r=" + base64.StdEncoding.EncodeToString(nonce)
	if clientFirst != want {
		t.Fatalf("client-first = %q, want %q", clientFirst, want)
	}

	serverFirst, err := srv.handleClientFirst(clientFirst)
	if err != nil {
		t.Fatal(err)
	}
	if err := sc.processServerFirst(serverFirst, srv.secret); err != nil {
		t.Fatal(err)
	}

	serverFinal, err := srv.handleClientFinal(sc.clientFirstBare, sc.clientFinalMessage())
	if err != nil {
		t.Fatalf("server rejected client proof: %v", err)
	}
	if err := sc.verifyServerFinal(serverFinal); err != nil {
		t.Fatalf("client rejected server signature: %v", err)
	}
}

// TestScramConversation_WrongSecretRejected verifies a proof computed from
// the wrong secret fails server-side verification.
func TestScramConversation_WrongSecretRejected(t *testing.T) {
	srv := &testScramServer{
		secret:     "the-real-secret",
		salt:       []byte("0123456789abcdef"),
		iterations: 5000,
		nonceExt:   "SERVERNONCE",
	}

	sc := &scramConversation{}
	serverFirst, err := srv.handleClientFirst(sc.clientFirstMessage("truenas_admin", 2, []byte("0123456789abcdef0123456789abcdef")))
	if err != nil {
		t.Fatal(err)
	}
	if err := sc.processServerFirst(serverFirst, "the-wrong-secret"); err != nil {
		t.Fatal(err)
	}
	if _, err := srv.handleClientFinal(sc.clientFirstBare, sc.clientFinalMessage()); err == nil {
		t.Fatal("server accepted a proof computed from the wrong secret")
	}
}

// TestScramConversation_BadServerSignatureRejected verifies the client
// rejects a server that cannot produce the right signature (mutual auth).
func TestScramConversation_BadServerSignatureRejected(t *testing.T) {
	srv := &testScramServer{
		secret:     "secret",
		salt:       []byte("0123456789abcdef"),
		iterations: 5000,
		nonceExt:   "SERVERNONCE",
	}
	sc := &scramConversation{}
	serverFirst, _ := srv.handleClientFirst(sc.clientFirstMessage("u", 1, []byte("0123456789abcdef0123456789abcdef")))
	if err := sc.processServerFirst(serverFirst, "secret"); err != nil {
		t.Fatal(err)
	}
	sc.clientFinalMessage()
	bogus := "v=" + base64.StdEncoding.EncodeToString([]byte("not the signature"))
	if err := sc.verifyServerFinal(bogus); err == nil {
		t.Fatal("client accepted a bogus server signature")
	}
}

// TestScramConversation_NonceMismatchRejected verifies the client refuses a
// server nonce that does not extend the client nonce.
func TestScramConversation_NonceMismatchRejected(t *testing.T) {
	sc := &scramConversation{}
	sc.clientFirstMessage("u", 1, []byte("0123456789abcdef0123456789abcdef"))
	serverFirst := fmt.Sprintf("r=%s,s=%s,i=5000",
		base64.StdEncoding.EncodeToString([]byte("DIFFERENT-NONCE-BYTES-0123456789-abcdef")),
		base64.StdEncoding.EncodeToString([]byte("0123456789abcdef")))
	if err := sc.processServerFirst(serverFirst, "secret"); err == nil {
		t.Fatal("accepted server nonce that does not extend client nonce")
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

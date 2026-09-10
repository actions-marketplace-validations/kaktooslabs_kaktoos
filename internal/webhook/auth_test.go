package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthenticator_NilConfig(t *testing.T) {
	a := NewAuthenticator(nil)
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	if !a.Verify(r, nil) {
		t.Fatal("expected nil auth config to always pass")
	}
}

func TestAuthenticator_BearerToken(t *testing.T) {
	a := NewAuthenticator(&AuthConfig{Type: "bearer_token", Token: "s3cret"})

	ok := httptest.NewRequest(http.MethodPost, "/", nil)
	ok.Header.Set("Authorization", "Bearer s3cret")
	if !a.Verify(ok, nil) {
		t.Fatal("expected valid bearer token to pass")
	}

	bad := httptest.NewRequest(http.MethodPost, "/", nil)
	bad.Header.Set("Authorization", "Bearer wrong")
	if a.Verify(bad, nil) {
		t.Fatal("expected invalid bearer token to fail")
	}

	missing := httptest.NewRequest(http.MethodPost, "/", nil)
	if a.Verify(missing, nil) {
		t.Fatal("expected missing Authorization header to fail")
	}
}

func TestAuthenticator_Basic(t *testing.T) {
	a := NewAuthenticator(&AuthConfig{Type: "basic", Username: "alice", Password: "pw"})

	ok := httptest.NewRequest(http.MethodPost, "/", nil)
	ok.SetBasicAuth("alice", "pw")
	if !a.Verify(ok, nil) {
		t.Fatal("expected valid basic auth to pass")
	}

	bad := httptest.NewRequest(http.MethodPost, "/", nil)
	bad.SetBasicAuth("alice", "wrong")
	if a.Verify(bad, nil) {
		t.Fatal("expected wrong password to fail")
	}
}

func TestAuthenticator_HMACSHA256(t *testing.T) {
	secret := "whsec"
	body := []byte(`{"hello":"world"}`)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	sig := hex.EncodeToString(mac.Sum(nil))

	a := NewAuthenticator(&AuthConfig{Type: "hmac_sha256", Secret: secret, Header: "X-Hub-Signature-256"})

	ok := httptest.NewRequest(http.MethodPost, "/", nil)
	ok.Header.Set("X-Hub-Signature-256", "sha256="+sig)
	if !a.Verify(ok, body) {
		t.Fatal("expected valid HMAC signature to pass")
	}

	tampered := httptest.NewRequest(http.MethodPost, "/", nil)
	tampered.Header.Set("X-Hub-Signature-256", "sha256="+sig)
	if a.Verify(tampered, []byte(`{"hello":"tampered"}`)) {
		t.Fatal("expected signature mismatch on tampered body to fail")
	}
}

func TestAuthenticator_UnknownType(t *testing.T) {
	a := NewAuthenticator(&AuthConfig{Type: "unknown"})
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	if a.Verify(r, nil) {
		t.Fatal("expected unknown auth type to fail closed")
	}
}

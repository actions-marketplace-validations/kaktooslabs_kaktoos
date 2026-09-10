package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"
)

// Authenticator verifies inbound webhook requests against a route's AuthConfig.
type Authenticator struct{ cfg *AuthConfig }

// NewAuthenticator builds an Authenticator for cfg. cfg may be nil.
func NewAuthenticator(cfg *AuthConfig) *Authenticator {
	return &Authenticator{cfg: cfg}
}

// Verify returns true if the request passes authentication.
// body is the raw request body (required for HMAC). Returns true unconditionally
// when cfg is nil (no auth configured for the route).
func (a *Authenticator) Verify(r *http.Request, body []byte) bool {
	if a.cfg == nil {
		return true
	}

	switch a.cfg.Type {
	case "bearer_token":
		header := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(header, prefix) {
			return false
		}
		token := strings.TrimPrefix(header, prefix)
		return constantTimeEqual(token, a.cfg.Token)

	case "hmac_sha256":
		sig := r.Header.Get(a.cfg.Header)
		sig = strings.TrimPrefix(sig, "sha256=")
		mac := hmac.New(sha256.New, []byte(a.cfg.Secret))
		mac.Write(body)
		expected := hex.EncodeToString(mac.Sum(nil))
		return constantTimeEqual(sig, expected)

	case "basic":
		username, password, ok := r.BasicAuth()
		if !ok {
			return false
		}
		return constantTimeEqual(username, a.cfg.Username) && constantTimeEqual(password, a.cfg.Password)

	default:
		return false
	}
}

func constantTimeEqual(a, b string) bool {
	if len(a) != len(b) {
		// Still compare to avoid a length-based timing signal; result is discarded.
		subtle.ConstantTimeCompare([]byte(a), []byte(a))
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

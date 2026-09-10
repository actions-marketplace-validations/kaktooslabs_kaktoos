package webhook

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "webhook.yml")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadConfig_Valid(t *testing.T) {
	p := writeTemp(t, `
server:
  port: 9090
  host: 127.0.0.1
routes:
  - path: /hooks/order
    method: post
    workflow:
      openapi: openapi.yml
      environment: env.yml
      scenario: scenario.yml
    auth:
      type: bearer_token
      token: secret
`)
	cfg, err := LoadConfig(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Port != 9090 || cfg.Server.Host != "127.0.0.1" {
		t.Fatalf("server config not parsed: %+v", cfg.Server)
	}
	if cfg.Routes[0].Method != "POST" {
		t.Fatalf("method not uppercased: %q", cfg.Routes[0].Method)
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	p := writeTemp(t, `
routes:
  - path: /hooks/order
    method: post
    workflow:
      openapi: openapi.yml
      environment: env.yml
      scenario: scenario.yml
`)
	cfg, err := LoadConfig(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Port != 8080 || cfg.Server.Host != "0.0.0.0" {
		t.Fatalf("defaults not applied: %+v", cfg.Server)
	}
}

func TestLoadConfig_InvalidPort(t *testing.T) {
	p := writeTemp(t, `
server:
  port: 70000
routes:
  - path: /hooks/order
    method: post
    workflow: {openapi: a, environment: b, scenario: c}
`)
	if _, err := LoadConfig(p); err == nil {
		t.Fatal("expected error for out-of-range port")
	}
}

func TestLoadConfig_NoRoutes(t *testing.T) {
	p := writeTemp(t, `server: {port: 8080}`)
	if _, err := LoadConfig(p); err == nil {
		t.Fatal("expected error for no routes")
	}
}

func TestLoadConfig_MissingWorkflowField(t *testing.T) {
	p := writeTemp(t, `
routes:
  - path: /hooks/order
    method: post
    workflow:
      openapi: a.yml
      environment: b.yml
`)
	if _, err := LoadConfig(p); err == nil {
		t.Fatal("expected error for missing scenario path")
	}
}

func TestLoadConfig_InvalidAuthType(t *testing.T) {
	p := writeTemp(t, `
routes:
  - path: /hooks/order
    method: post
    workflow: {openapi: a, environment: b, scenario: c}
    auth: {type: totally_bogus}
`)
	if _, err := LoadConfig(p); err == nil {
		t.Fatal("expected error for invalid auth type")
	}
}

func TestLoadConfig_HMACDefaultHeader(t *testing.T) {
	p := writeTemp(t, `
routes:
  - path: /hooks/order
    method: post
    workflow: {openapi: a, environment: b, scenario: c}
    auth: {type: hmac_sha256, secret: shh}
`)
	cfg, err := LoadConfig(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Routes[0].Auth.Header != "X-Hub-Signature-256" {
		t.Fatalf("expected default header, got %q", cfg.Routes[0].Auth.Header)
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	if _, err := LoadConfig("/nonexistent/path/webhook.yml"); err == nil {
		t.Fatal("expected error for missing file")
	}
}

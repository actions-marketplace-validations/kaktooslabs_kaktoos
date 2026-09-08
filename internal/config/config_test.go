package config

import (
	"errors"
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	"pgregory.net/rapid"
)

func TestLoad_Success(t *testing.T) {
	t.Run("FullConfigLoadsSuccessfully", func(t *testing.T) {
		content := `
base_url: https://api.example.com/v1
headers:
  Accept: application/json
  X-Client-ID: "123"
variables:
  default_currency: USD
`
		f := writeTempFile(t, content)
		env, err := Load(f)
		if err != nil {
			t.Fatalf("expected success, got error: %v", err)
		}
		if env.BaseURL != "https://api.example.com/v1" {
			t.Errorf("BaseURL: got %q want %q", env.BaseURL, "https://api.example.com/v1")
		}
		if len(env.Headers) != 2 || env.Headers["Accept"] != "application/json" {
			t.Errorf("Headers mismatch: %v", env.Headers)
		}
		if len(env.Variables) != 1 || env.Variables["default_currency"] != "USD" {
			t.Errorf("Variables mismatch: %v", env.Variables)
		}
	})

	t.Run("MinimalConfigLoadsSuccessfully", func(t *testing.T) {
		f := writeTempFile(t, "base_url: http://localhost:8080")
		env, err := Load(f)
		if err != nil {
			t.Fatalf("expected success, got error: %v", err)
		}
		if env.BaseURL != "http://localhost:8080" {
			t.Errorf("BaseURL: got %q want %q", env.BaseURL, "http://localhost:8080")
		}
		if len(env.Headers) != 0 {
			t.Errorf("expected empty Headers, got %v", env.Headers)
		}
		if len(env.Variables) != 0 {
			t.Errorf("expected empty Variables, got %v", env.Variables)
		}
	})
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load("non_existent_file_path_for_test.yml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	var cfgErr *ConfigError
	if !errors.As(err, &cfgErr) {
		t.Errorf("expected *ConfigError, got %T", err)
	}
	if !strings.Contains(err.Error(), "file not found") {
		t.Errorf("expected error to contain 'file not found', got: %v", err)
	}
	if !strings.Contains(err.Error(), "non_existent_file_path_for_test.yml") {
		t.Errorf("expected error to contain the file path, got: %v", err)
	}
}

func TestLoad_Failures(t *testing.T) {
	cases := []struct {
		name       string
		content    string
		wantSubstr string
	}{
		{
			name:       "MissingBaseURL",
			content:    "headers: {}\nvariables: {}",
			wantSubstr: "base_url is required",
		},
		{
			name:       "InvalidYAML",
			content:    "base_url: [unclosed bracket",
			wantSubstr: "cannot be parsed",
		},
		{
			name:       "BadURLScheme",
			content:    "base_url: ftp://private.server",
			wantSubstr: "not a valid absolute http/https URL",
		},
		{
			name:       "EmptyHost",
			content:    "base_url: http://",
			wantSubstr: "has an empty host",
		},
		{
			name:       "MailtoScheme",
			content:    "base_url: mailto:test@example.com",
			wantSubstr: "not a valid absolute http/https URL",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := writeTempFile(t, tc.content)
			env, err := Load(f)
			if err == nil {
				t.Fatalf("expected error, got nil; loaded env: %+v", env)
			}
			var cfgErr *ConfigError
			if !errors.As(err, &cfgErr) {
				t.Errorf("expected *ConfigError, got %T", err)
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Errorf("expected error containing %q, got: %v", tc.wantSubstr, err)
			}
		})
	}
}

// Feature: kaktoos-cli, Property 4: Environment round-trip
// Validates: Requirements 2.2
func TestPropEnvironmentRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a valid http/https base URL
		scheme := rapid.SampledFrom([]string{"http", "https"}).Draw(t, "scheme")
		host := rapid.StringMatching(`[a-z]{3,10}\.[a-z]{2,4}`).Draw(t, "host")
		baseURL := scheme + "://" + host

		type envYAML struct {
			BaseURL   string            `yaml:"base_url"`
			Headers   map[string]string `yaml:"headers"`
			Variables map[string]string `yaml:"variables"`
		}

		headers := rapid.MapOf(
			rapid.StringMatching(`[A-Za-z][A-Za-z0-9-]{1,19}`),
			rapid.StringMatching(`[A-Za-z0-9 ]{1,30}`),
		).Draw(t, "headers")

		variables := rapid.MapOf(
			rapid.StringMatching(`[a-z][a-z0-9_]{1,14}`),
			rapid.StringMatching(`[A-Za-z0-9]{1,20}`),
		).Draw(t, "variables")

		src := envYAML{BaseURL: baseURL, Headers: headers, Variables: variables}
		data, err := yaml.Marshal(src)
		if err != nil {
			t.Skip()
		}

		f, err := os.CreateTemp("", "env-*.yml")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(f.Name())
		f.Write(data)
		f.Close()

		env, err := Load(f.Name())
		if err != nil {
			t.Fatalf("Load failed for base_url %q: %v", baseURL, err)
		}
		if env.BaseURL != baseURL {
			t.Errorf("BaseURL mismatch: got %q want %q", env.BaseURL, baseURL)
		}

		// Assert Headers round-trip
		for k, v := range headers {
			if env.Headers[k] != v {
				t.Errorf("Headers[%q]: got %q want %q", k, env.Headers[k], v)
			}
		}
		// Assert Variables round-trip
		for k, v := range variables {
			if env.Variables[k] != v {
				t.Errorf("Variables[%q]: got %q want %q", k, env.Variables[k], v)
			}
		}
	})
}

// Feature: kaktoos-cli, Property 5: base_url validation
// Validates: Requirements 2.7
func TestPropBaseURLValidation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		scheme := rapid.SampledFrom([]string{"ftp", "mailto", "ws", "file", "ssh"}).Draw(t, "scheme")
		host := rapid.StringMatching(`[a-z]{3,10}\.[a-z]{2,4}`).Draw(t, "host")
		rawURL := scheme + "://" + host

		f, err := os.CreateTemp("", "env-*.yml")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(f.Name())
		f.WriteString("base_url: " + rawURL)
		f.Close()

		_, err = Load(f.Name())
		if err == nil {
			t.Errorf("expected error for non-http/https URL %q, got nil", rawURL)
		}
	})
}

// writeTempFile writes content to a temp file and returns its path.
// The file is removed when the test completes.
func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "kaktoos-config-*.yml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	t.Cleanup(func() { os.Remove(f.Name()) })
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	f.Close()
	return f.Name()
}

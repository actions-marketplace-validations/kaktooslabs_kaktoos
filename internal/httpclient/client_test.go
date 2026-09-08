package httpclient

import (
	"io"
	"strings"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	c := NewClient()
	if c.httpClient.Timeout != 30*time.Second {
		t.Errorf("timeout = %v, want 30s", c.httpClient.Timeout)
	}
	if c.httpClient.CheckRedirect == nil {
		t.Fatal("CheckRedirect must be set to enforce the redirect limit")
	}
}

func TestBuildRequestSuccess(t *testing.T) {
	req, err := BuildRequest(RequestSpec{
		Method:       "get",
		BaseURL:      "https://api.example.com",
		PathTemplate: "/users/profile",
		BaseHeaders:  map[string]string{"X-Env": "base", "X-Keep": "yes"},
		Headers:      map[string]string{"X-Env": "step"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Method != "GET" {
		t.Errorf("method = %s, want GET", req.Method)
	}
	if got := req.URL.String(); got != "https://api.example.com/users/profile" {
		t.Errorf("url = %s", got)
	}
	if got := req.Header.Get("X-Env"); got != "step" {
		t.Errorf("step header must override base header, got %q", got)
	}
	if got := req.Header.Get("X-Keep"); got != "yes" {
		t.Errorf("base header lost, got %q", got)
	}
}

func TestBuildRequestPathParamsAndQuery(t *testing.T) {
	req, err := BuildRequest(RequestSpec{
		Method:       "POST",
		BaseURL:      "https://api.example.com/v1",
		PathTemplate: "/users/{userId}/items",
		PathParams:   map[string]string{"userId": "123"},
		Query:        map[string]string{"limit": "10"},
		Body:         []byte(`{"a":1}`),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := req.URL.String(); got != "https://api.example.com/v1/users/123/items?limit=10" {
		t.Errorf("url = %s", got)
	}
	body, _ := io.ReadAll(req.Body)
	if string(body) != `{"a":1}` {
		t.Errorf("body = %s", body)
	}
}

func TestBuildRequestUnresolvedPathParam(t *testing.T) {
	_, err := BuildRequest(RequestSpec{
		Method:       "GET",
		BaseURL:      "https://api.example.com",
		PathTemplate: "/users/{userId}",
	})
	if err == nil || !strings.Contains(err.Error(), "unresolved path parameter") {
		t.Fatalf("want unresolved path parameter error, got %v", err)
	}
}

func TestBuildRequestInvalidScheme(t *testing.T) {
	_, err := BuildRequest(RequestSpec{
		Method:       "GET",
		BaseURL:      "ftp://api.example.com",
		PathTemplate: "/",
	})
	if err == nil || !strings.Contains(err.Error(), "invalid URL scheme") {
		t.Fatalf("want invalid URL scheme error, got %v", err)
	}
}

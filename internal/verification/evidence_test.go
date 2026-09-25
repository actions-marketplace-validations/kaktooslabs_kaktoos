package verification

import "strings"

import "testing"

func TestNewEvidenceRedactsAndTruncates(t *testing.T) {
	longBody := strings.Repeat("x", bodyBudget+500)
	ev := NewEvidence("POST", "http://x/orders", 200,
		map[string]string{"Authorization": "Bearer t"}, longBody,
		map[string]string{"Set-Cookie": "s=1"}, longBody)

	if ev.RequestHeaders["Authorization"] != "***" {
		t.Fatalf("request auth header not redacted: %+v", ev.RequestHeaders)
	}
	if ev.ResponseHeaders["Set-Cookie"] != "***" {
		t.Fatalf("response cookie not redacted: %+v", ev.ResponseHeaders)
	}
	if len(ev.RequestBody) != bodyBudget || len(ev.ResponseBody) != bodyBudget {
		t.Fatalf("bodies not truncated to budget: req=%d resp=%d", len(ev.RequestBody), len(ev.ResponseBody))
	}
	if ev.Method != "POST" || ev.URL != "http://x/orders" || ev.StatusCode != 200 {
		t.Fatalf("basic fields not preserved: %+v", ev)
	}
}

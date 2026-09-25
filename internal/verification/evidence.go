package verification

// bodyBudget mirrors the truncation budget already used for trace attempts
// (internal/engine/runner.go's truncate(..., 2048)).
const bodyBudget = 2048

// Evidence is the redacted request/response pair behind a failed step —
// enough to act on the failure without re-running it.
type Evidence struct {
	Method          string            `json:"method,omitempty"`
	URL             string            `json:"url,omitempty"`
	StatusCode      int               `json:"status_code,omitempty"`
	RequestHeaders  map[string]string `json:"request_headers,omitempty"`
	RequestBody     string            `json:"request_body,omitempty"`
	ResponseHeaders map[string]string `json:"response_headers,omitempty"`
	ResponseBody    string            `json:"response_body,omitempty"`
}

// NewEvidence builds redacted Evidence from raw traced values. Headers are
// redacted unconditionally — evidence has no --trace-sensitive-style opt-out.
func NewEvidence(method, url string, statusCode int, reqHeaders map[string]string, reqBody string, respHeaders map[string]string, respBody string) *Evidence {
	return &Evidence{
		Method:          method,
		URL:             url,
		StatusCode:      statusCode,
		RequestHeaders:  RedactHeaders(reqHeaders),
		RequestBody:     truncate(reqBody, bodyBudget),
		ResponseHeaders: RedactHeaders(respHeaders),
		ResponseBody:    truncate(respBody, bodyBudget),
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

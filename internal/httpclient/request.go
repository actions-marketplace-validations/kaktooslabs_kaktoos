package httpclient

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// RequestSpec describes one HTTP request to build.
// PathParams fill {placeholders} in PathTemplate; Headers are merged over
// BaseHeaders (request-level wins).
type RequestSpec struct {
	Method       string
	BaseURL      string
	PathTemplate string
	PathParams   map[string]string
	Query        map[string]string
	BaseHeaders  map[string]string
	Headers      map[string]string
	Body         []byte
}

// BuildRequest constructs an *http.Request from spec: joins base URL and path,
// substitutes path parameters, appends query parameters, and merges headers.
func BuildRequest(spec RequestSpec) (*http.Request, error) {
	base, err := url.Parse(spec.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base_url %q: %w", spec.BaseURL, err)
	}
	if base.Scheme != "http" && base.Scheme != "https" {
		return nil, fmt.Errorf("invalid URL scheme %q in base_url %q", base.Scheme, spec.BaseURL)
	}
	if base.Host == "" {
		return nil, fmt.Errorf("invalid base_url %q: empty host", spec.BaseURL)
	}

	path := spec.PathTemplate
	for name, value := range spec.PathParams {
		path = strings.ReplaceAll(path, "{"+name+"}", url.PathEscape(value))
	}
	if left := strings.Index(path, "{"); left != -1 {
		return nil, fmt.Errorf("unresolved path parameter in %q", path)
	}

	base.Path = strings.TrimSuffix(base.Path, "/") + "/" + strings.TrimPrefix(path, "/")

	if len(spec.Query) > 0 {
		q := base.Query()
		for k, v := range spec.Query {
			q.Set(k, v)
		}
		base.RawQuery = q.Encode()
	}

	var body *bytes.Reader
	if spec.Body != nil {
		body = bytes.NewReader(spec.Body)
	} else {
		body = bytes.NewReader(nil)
	}

	req, err := http.NewRequest(strings.ToUpper(spec.Method), base.String(), body)
	if err != nil {
		return nil, err
	}
	for k, v := range spec.BaseHeaders {
		req.Header.Set(k, v)
	}
	for k, v := range spec.Headers {
		req.Header.Set(k, v)
	}
	return req, nil
}

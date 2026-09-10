package httpclient

import (
	"errors"
	"net/http"
	"time"
)

// maxRedirects is the redirect limit enforced on every request.
const maxRedirects = 10

// Client holds the configured HTTP client used throughout the application.
type Client struct {
	httpClient *http.Client
}

// NewClient creates a Client with a 30-second timeout and a 10-redirect limit.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= maxRedirects {
					return errors.New("stopped after 10 redirects")
				}
				return nil
			},
		},
	}
}

// Do executes the request using the configured client.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.httpClient.Do(req)
}

// HTTPClient returns the configured standard library client.
func (c *Client) HTTPClient() *http.Client { return c.httpClient }

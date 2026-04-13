package utils

import (
	"net/http"
	"time"
)

// NewHTTPClient returns an *http.Client configured with the given timeout.
// The client does NOT follow redirects so we capture the first response status
// faithfully (e.g. 301/302 is reported as-is).
func NewHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: &http.Transport{
			// Sensible defaults for a microservice:
			MaxIdleConns:        100,
			IdleConnTimeout:     30 * time.Second,
			TLSHandshakeTimeout: 5 * time.Second,
			DisableCompression:  false,
		},
	}
}

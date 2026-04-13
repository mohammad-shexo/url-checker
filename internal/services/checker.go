package services

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/mohammad-shexo/url-checker/internal/models"
)

// CheckerError represents a structured error produced by the checker service.
type CheckerError struct {
	Kind    string // "dns", "timeout", "invalid_url", "network", "unknown"
	Message string
}

func (e *CheckerError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Kind, e.Message)
}

// cacheEntry stores a cached result together with its expiry timestamp.
type cacheEntry struct {
	result    models.URLResult
	expiresAt time.Time
}

// Checker is the service responsible for checking a list of URLs.
// It can be constructed with a custom *http.Client for testability.
type Checker struct {
	client   *http.Client
	cacheTTL time.Duration
	mu       sync.RWMutex
	cache    map[string]cacheEntry
}

// NewChecker creates a Checker with the provided HTTP client and an optional
// in-memory cache TTL. Pass 0 to disable caching.
func NewChecker(client *http.Client, cacheTTL time.Duration) *Checker {
	return &Checker{
		client:   client,
		cacheTTL: cacheTTL,
		cache:    make(map[string]cacheEntry),
	}
}

// CheckURLs checks every URL in parallel and returns an ordered slice of results.
func (c *Checker) CheckURLs(urls []string) []models.URLResult {
	results := make([]models.URLResult, len(urls))
	var wg sync.WaitGroup

	for i, rawURL := range urls {
		wg.Add(1)
		go func(idx int, target string) {
			defer wg.Done()
			results[idx] = c.checkOne(target)
		}(i, rawURL)
	}

	wg.Wait()
	return results
}

// checkOne checks a single URL, using the in-memory cache when available.
func (c *Checker) checkOne(rawURL string) models.URLResult {
	// --- cache read ---
	if c.cacheTTL > 0 {
		c.mu.RLock()
		entry, found := c.cache[rawURL]
		c.mu.RUnlock()
		if found && time.Now().Before(entry.expiresAt) {
			return entry.result
		}
	}

	result := c.fetch(rawURL)

	// --- cache write ---
	if c.cacheTTL > 0 {
		c.mu.Lock()
		c.cache[rawURL] = cacheEntry{
			result:    result,
			expiresAt: time.Now().Add(c.cacheTTL),
		}
		c.mu.Unlock()
	}

	return result
}

// fetch performs the actual HTTP HEAD (falling back to GET) and records timing.
func (c *Checker) fetch(rawURL string) models.URLResult {
	result := models.URLResult{URL: rawURL}

	// --- validate URL ---
	if err := validateURL(rawURL); err != nil {
		result.Error = err.Error()
		return result
	}

	start := time.Now()
	resp, err := c.client.Head(rawURL) //nolint:noctx
	elapsed := time.Since(start).Milliseconds()
	result.DurationMS = elapsed

	if err != nil {
		result.Error = classifyError(err)
		return result
	}
	defer resp.Body.Close()

	result.Status = resp.StatusCode
	return result
}

// validateURL returns a CheckerError when rawURL is syntactically invalid or
// lacks a recognised scheme.
func validateURL(rawURL string) error {
	if strings.TrimSpace(rawURL) == "" {
		return &CheckerError{Kind: "invalid_url", Message: "URL must not be empty"}
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return &CheckerError{Kind: "invalid_url", Message: err.Error()}
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return &CheckerError{
			Kind:    "invalid_url",
			Message: fmt.Sprintf("unsupported scheme %q (must be http or https)", parsed.Scheme),
		}
	}
	if parsed.Host == "" {
		return &CheckerError{Kind: "invalid_url", Message: "URL has no host"}
	}
	return nil
}

// classifyError converts a net/http error into a human-readable, typed message.
func classifyError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	// Timeout check
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return (&CheckerError{Kind: "timeout", Message: "request timed out"}).Error()
	}
	// DNS check
	if strings.Contains(msg, "no such host") || strings.Contains(msg, "lookup") {
		return (&CheckerError{Kind: "dns", Message: "DNS resolution failed"}).Error()
	}
	// Connection refused
	if strings.Contains(msg, "connection refused") {
		return (&CheckerError{Kind: "network", Message: "connection refused"}).Error()
	}
	return (&CheckerError{Kind: "unknown", Message: msg}).Error()
}

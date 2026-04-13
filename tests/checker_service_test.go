package tests

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mohammad-shexo/url-checker/internal/services"
)

// ─── helpers ────────────────────────────────────────────────────────────────

func newStatusServer(t *testing.T, code int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(code)
	}))
}

func assertEqual(t *testing.T, label string, got, want interface{}) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %v, want %v", label, got, want)
	}
}

func assertNotEmpty(t *testing.T, label, s string) {
	t.Helper()
	if strings.TrimSpace(s) == "" {
		t.Errorf("%s: expected non-empty string, got empty", label)
	}
}

func assertContains(t *testing.T, label, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Errorf("%s: expected %q to contain %q", label, s, sub)
	}
}

func assertLen(t *testing.T, label string, got, want int) {
	t.Helper()
	if got != want {
		t.Fatalf("%s: length got %d, want %d", label, got, want)
	}
}

// ─── Test 1: Valid URL → 200 ─────────────────────────────────────────────────

func TestChecker_ValidURL_Returns200(t *testing.T) {
	srv := newStatusServer(t, http.StatusOK)
	defer srv.Close()

	checker := services.NewChecker(srv.Client(), 0)
	results := checker.CheckURLs([]string{srv.URL})

	assertLen(t, "results", len(results), 1)
	assertEqual(t, "status", results[0].Status, http.StatusOK)
	assertEqual(t, "error", results[0].Error, "")
}

// ─── Test 2: Valid URL → 404 ─────────────────────────────────────────────────

func TestChecker_ValidURL_Returns404(t *testing.T) {
	srv := newStatusServer(t, http.StatusNotFound)
	defer srv.Close()

	results := services.NewChecker(srv.Client(), 0).CheckURLs([]string{srv.URL})

	assertLen(t, "results", len(results), 1)
	assertEqual(t, "status", results[0].Status, http.StatusNotFound)
	assertEqual(t, "error", results[0].Error, "")
}

// ─── Test 3: Valid URL → 500 ─────────────────────────────────────────────────

func TestChecker_ValidURL_Returns500(t *testing.T) {
	srv := newStatusServer(t, http.StatusInternalServerError)
	defer srv.Close()

	results := services.NewChecker(srv.Client(), 0).CheckURLs([]string{srv.URL})

	assertLen(t, "results", len(results), 1)
	assertEqual(t, "status", results[0].Status, http.StatusInternalServerError)
}

// ─── Test 4: Invalid URL — bad scheme ────────────────────────────────────────

func TestChecker_InvalidURL_BadScheme(t *testing.T) {
	results := services.NewChecker(http.DefaultClient, 0).CheckURLs([]string{"ftp://example.com"})

	assertLen(t, "results", len(results), 1)
	assertEqual(t, "status", results[0].Status, 0)
	assertContains(t, "error", results[0].Error, "invalid_url")
}

// ─── Test 5: Empty URL string ─────────────────────────────────────────────────

func TestChecker_EmptyURL(t *testing.T) {
	results := services.NewChecker(http.DefaultClient, 0).CheckURLs([]string{""})

	assertLen(t, "results", len(results), 1)
	assertEqual(t, "status", results[0].Status, 0)
	assertContains(t, "error", results[0].Error, "invalid_url")
}

// ─── Test 6: Timeout behaviour ────────────────────────────────────────────────

func TestChecker_Timeout(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer slow.Close()

	timedOut := &http.Client{Timeout: 50 * time.Millisecond}
	results := services.NewChecker(timedOut, 0).CheckURLs([]string{slow.URL})

	assertLen(t, "results", len(results), 1)
	assertEqual(t, "status", results[0].Status, 0)
	assertContains(t, "error", results[0].Error, "timeout")
}

// ─── Test 7: Unreachable host ─────────────────────────────────────────────────

func TestChecker_UnreachableHost(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not bind ephemeral port: %v", err)
	}
	addr := l.Addr().String()
	l.Close()

	client := &http.Client{Timeout: 2 * time.Second}
	results := services.NewChecker(client, 0).CheckURLs([]string{"http://" + addr})

	assertLen(t, "results", len(results), 1)
	assertEqual(t, "status", results[0].Status, 0)
	assertNotEmpty(t, "error", results[0].Error)
}

// ─── Test 8: Parallel execution ───────────────────────────────────────────────

func TestChecker_ParallelExecution(t *testing.T) {
	var callCount int
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		mu.Unlock()
		time.Sleep(30 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	urls := make([]string, 10)
	for i := range urls {
		urls[i] = srv.URL
	}

	start := time.Now()
	results := services.NewChecker(srv.Client(), 0).CheckURLs(urls)
	elapsed := time.Since(start)

	assertLen(t, "results", len(results), 10)
	if elapsed >= 200*time.Millisecond {
		t.Errorf("parallel: elapsed %v >= 200ms — goroutines may not be running concurrently", elapsed)
	}
	if callCount != 10 {
		t.Errorf("call count: got %d, want 10", callCount)
	}
}

// ─── Test 9: In-memory cache ──────────────────────────────────────────────────

func TestChecker_Caching(t *testing.T) {
	var callCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cached := services.NewChecker(srv.Client(), 30*time.Second)
	cached.CheckURLs([]string{srv.URL})
	cached.CheckURLs([]string{srv.URL})

	if callCount != 1 {
		t.Errorf("caching: server called %d times, want 1", callCount)
	}
}

// ─── Test 10: Result URL = input URL ─────────────────────────────────────────

func TestChecker_ResultURLMatchesInput(t *testing.T) {
	target := "not-a-url"
	results := services.NewChecker(http.DefaultClient, 0).CheckURLs([]string{target})

	assertLen(t, "results", len(results), 1)
	assertEqual(t, "url field", results[0].URL, target)
}

// ─── Test 11: Result order preserved ─────────────────────────────────────────

func TestChecker_OrderPreserved(t *testing.T) {
	srv1 := newStatusServer(t, 200)
	defer srv1.Close()
	srv2 := newStatusServer(t, 404)
	defer srv2.Close()

	checker := services.NewChecker(srv1.Client(), 0)
	urls := []string{srv1.URL, srv2.URL}
	results := checker.CheckURLs(urls)

	assertLen(t, "results", len(results), 2)
	assertEqual(t, "url[0]", results[0].URL, urls[0])
	assertEqual(t, "url[1]", results[1].URL, urls[1])
}

// ─── Test 12: DNS failure ─────────────────────────────────────────────────────

func TestChecker_DNSFailure(t *testing.T) {
	client := &http.Client{Timeout: 3 * time.Second}
	results := services.NewChecker(client, 0).CheckURLs(
		[]string{"https://this-domain-absolutely-does-not-exist-xyz-abc-123.invalid"},
	)

	assertLen(t, "results", len(results), 1)
	assertEqual(t, "status", results[0].Status, 0)
	errLower := strings.ToLower(results[0].Error)
	if !strings.Contains(errLower, "dns") &&
		!strings.Contains(errLower, "lookup") &&
		!strings.Contains(errLower, "unknown") {
		t.Errorf("expected DNS-like error, got: %s", results[0].Error)
	}
}

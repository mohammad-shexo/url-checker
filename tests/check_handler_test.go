package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mohammad-shexo/url-checker/internal/handlers"
	"github.com/mohammad-shexo/url-checker/internal/models"
	"github.com/mohammad-shexo/url-checker/internal/services"
)

// newHandler wires a CheckHandler backed by the given http.Client (no cache).
func newHandler(client *http.Client) *handlers.CheckHandler {
	return handlers.NewCheckHandler(services.NewChecker(client, 0))
}

// postCheck sends a POST /check with the given body and returns the recorder.
func postCheck(t *testing.T, h http.Handler, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/check", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

// ─── Handler Test 1: Valid request → 200 + results ───────────────────────────

func TestHandler_ValidRequest_Returns200(t *testing.T) {
	srv := newStatusServer(t, http.StatusOK)
	defer srv.Close()

	rr := postCheck(t, newHandler(srv.Client()), models.CheckRequest{URLs: []string{srv.URL}})

	assertEqual(t, "http status", rr.Code, http.StatusOK)

	var resp models.CheckResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	assertLen(t, "results", len(resp.Results), 1)
	assertEqual(t, "url status", resp.Results[0].Status, http.StatusOK)
}

// ─── Handler Test 2: GET method → 405 ────────────────────────────────────────

func TestHandler_GetMethodRejected(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/check", nil)
	rr := httptest.NewRecorder()
	newHandler(http.DefaultClient).ServeHTTP(rr, req)

	assertEqual(t, "http status", rr.Code, http.StatusMethodNotAllowed)

	var errResp models.ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	assertNotEmpty(t, "message", errResp.Message)
}

// ─── Handler Test 3: Empty URLs array → 400 ──────────────────────────────────

func TestHandler_EmptyURLsArray_Rejected(t *testing.T) {
	rr := postCheck(t, newHandler(http.DefaultClient), models.CheckRequest{URLs: []string{}})

	assertEqual(t, "http status", rr.Code, http.StatusBadRequest)

	var errResp models.ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	assertContains(t, "message", errResp.Message, "empty")
}

// ─── Handler Test 4: Malformed JSON → 400 ────────────────────────────────────

func TestHandler_MalformedJSON_Rejected(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/check", bytes.NewBufferString(`{bad json`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	newHandler(http.DefaultClient).ServeHTTP(rr, req)

	assertEqual(t, "http status", rr.Code, http.StatusBadRequest)
}

// ─── Handler Test 5: >50 URLs → 400 ──────────────────────────────────────────

func TestHandler_TooManyURLs_Rejected(t *testing.T) {
	urls := make([]string, 51)
	for i := range urls {
		urls[i] = "https://example.com"
	}
	rr := postCheck(t, newHandler(http.DefaultClient), models.CheckRequest{URLs: urls})

	assertEqual(t, "http status", rr.Code, http.StatusBadRequest)

	var errResp models.ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	assertContains(t, "message", errResp.Message, "50")
}

// ─── Handler Test 6: Response Content-Type is application/json ───────────────

func TestHandler_ResponseContentType(t *testing.T) {
	srv := newStatusServer(t, http.StatusOK)
	defer srv.Close()

	rr := postCheck(t, newHandler(srv.Client()), models.CheckRequest{URLs: []string{srv.URL}})

	assertEqual(t, "Content-Type", rr.Header().Get("Content-Type"), "application/json")
}

// ─── Handler Test 7: Multiple URLs → multiple results ────────────────────────

func TestHandler_MultipleURLs_ReturnsMultipleResults(t *testing.T) {
	srv := newStatusServer(t, http.StatusOK)
	defer srv.Close()

	rr := postCheck(t, newHandler(srv.Client()), models.CheckRequest{
		URLs: []string{srv.URL, srv.URL, srv.URL},
	})

	assertEqual(t, "http status", rr.Code, http.StatusOK)

	var resp models.CheckResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	assertLen(t, "results", len(resp.Results), 3)
}

// ─── Handler Test 8: Invalid URL in body → error in result, not HTTP error ───

func TestHandler_InvalidURLInBody_ErrorInResult(t *testing.T) {
	rr := postCheck(t, newHandler(http.DefaultClient), models.CheckRequest{
		URLs: []string{"not-a-valid-url"},
	})

	assertEqual(t, "http status", rr.Code, http.StatusOK) // request is valid

	var resp models.CheckResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	assertLen(t, "results", len(resp.Results), 1)
	assertNotEmpty(t, "error field", resp.Results[0].Error)
	assertEqual(t, "status field", resp.Results[0].Status, 0)
}

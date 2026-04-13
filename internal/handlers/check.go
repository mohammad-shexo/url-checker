package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/mohammad-shexo/url-checker/internal/models"
	"github.com/mohammad-shexo/url-checker/internal/services"
)

// CheckHandler handles POST /check requests.
type CheckHandler struct {
	checker *services.Checker
}

// NewCheckHandler constructs a CheckHandler with the provided Checker service.
func NewCheckHandler(checker *services.Checker) *CheckHandler {
	return &CheckHandler{checker: checker}
}

// ServeHTTP implements http.Handler.
func (h *CheckHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "only POST is accepted")
		return
	}

	var req models.CheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	defer r.Body.Close()

	if len(req.URLs) == 0 {
		writeError(w, http.StatusBadRequest, "urls array must not be empty")
		return
	}

	const maxURLs = 50
	if len(req.URLs) > maxURLs {
		writeError(w, http.StatusBadRequest, "too many URLs: maximum is 50 per request")
		return
	}

	results := h.checker.CheckURLs(req.URLs)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(models.CheckResponse{Results: results}) //nolint:errcheck
}

// writeError writes a JSON error response with the given status code.
func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(models.ErrorResponse{Message: message}) //nolint:errcheck
}

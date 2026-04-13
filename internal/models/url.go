package models

// CheckRequest is the incoming payload for POST /check.
type CheckRequest struct {
	URLs []string `json:"urls"`
}

// URLResult holds the check outcome for a single URL.
type URLResult struct {
	URL        string `json:"url"`
	Status     int    `json:"status"`
	DurationMS int64  `json:"duration_ms"`
	Error      string `json:"error"`
}

// CheckResponse is the full response body returned to the caller.
type CheckResponse struct {
	Results []URLResult `json:"results"`
}

// ErrorResponse is returned when the request itself is invalid.
type ErrorResponse struct {
	Message string `json:"message"`
}

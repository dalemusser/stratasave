package settingsbrowser

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/dalemusser/stratasave/internal/app/system/viewdata"
	"github.com/dalemusser/waffle/pantry/templates"
)

// PlaygroundVM is the view model for the playground page.
type PlaygroundVM struct {
	viewdata.BaseVM
	APIEndpoint string
	APIKey      string
}

// DocsVM is the view model for the documentation page.
type DocsVM struct {
	viewdata.BaseVM
	BaseURL string
}

// PlaygroundRequest is the request body for playground execute.
type PlaygroundRequest struct {
	Operation string          `json:"operation"` // "save" or "load"
	Body      json.RawMessage `json:"body"`
}

// PlaygroundResponse is the response from playground execute.
type PlaygroundResponse struct {
	Status     int               `json:"status"`
	StatusText string            `json:"status_text"`
	DurationMs int64             `json:"duration_ms"`
	Headers    map[string]string `json:"headers"`
	Body       json.RawMessage   `json:"body"`
	Error      string            `json:"error,omitempty"`
}

// ServePlayground renders the playground page.
func (h *Handler) ServePlayground(w http.ResponseWriter, r *http.Request) {
	data := PlaygroundVM{
		BaseVM:      viewdata.NewBaseVM(r, h.db, "Settings API Playground", "/console/api/settings"),
		APIEndpoint: "/api/settings",
		APIKey:      h.apiKey,
	}
	templates.Render(w, r, "settingsbrowser/playground", data)
}

// ServeDocs renders the documentation page.
func (h *Handler) ServeDocs(w http.ResponseWriter, r *http.Request) {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	baseURL := scheme + "://" + r.Host

	data := DocsVM{
		BaseVM:  viewdata.NewBaseVM(r, h.db, "Settings API Documentation", "/console/api/settings"),
		BaseURL: baseURL,
	}
	templates.Render(w, r, "settingsbrowser/docs", data)
}

// HandlePlaygroundExecute proxies requests to the real API and returns results.
func (h *Handler) HandlePlaygroundExecute(w http.ResponseWriter, r *http.Request) {
	var req PlaygroundRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writePlaygroundError(w, "Invalid request: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate operation
	var targetPath string
	switch req.Operation {
	case "save":
		targetPath = "/api/settings/save"
	case "load":
		targetPath = "/api/settings/load"
	default:
		writePlaygroundError(w, "Invalid operation: must be 'save' or 'load'", http.StatusBadRequest)
		return
	}

	// Get the API key from the handler
	apiKey := h.apiKey
	if apiKey == "" {
		writePlaygroundError(w, "API key not configured", http.StatusInternalServerError)
		return
	}

	// Build internal HTTP request
	// Use the same host to make an internal request
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	targetURL := scheme + "://" + r.Host + targetPath

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	proxyReq, err := http.NewRequestWithContext(ctx, "POST", targetURL, bytes.NewReader(req.Body))
	if err != nil {
		writePlaygroundError(w, "Failed to create request: "+err.Error(), http.StatusInternalServerError)
		return
	}

	proxyReq.Header.Set("Content-Type", "application/json")
	proxyReq.Header.Set("Authorization", "Bearer "+apiKey)

	// Execute request and measure timing
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	startTime := time.Now()
	resp, err := client.Do(proxyReq)
	elapsed := time.Since(startTime)

	if err != nil {
		writePlaygroundError(w, "Request failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		writePlaygroundError(w, "Failed to read response: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Build headers map (simplified - just key headers)
	headers := make(map[string]string)
	for _, key := range []string{"Content-Type", "X-Request-Id"} {
		if v := resp.Header.Get(key); v != "" {
			headers[key] = v
		}
	}

	// Return result
	result := PlaygroundResponse{
		Status:     resp.StatusCode,
		StatusText: http.StatusText(resp.StatusCode),
		DurationMs: elapsed.Milliseconds(),
		Headers:    headers,
		Body:       bodyBytes,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		h.logger.Error("failed to encode playground response")
	}
}

func writePlaygroundError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	resp := PlaygroundResponse{
		Status:     code,
		StatusText: http.StatusText(code),
		Error:      msg,
	}
	json.NewEncoder(w).Encode(resp)
}

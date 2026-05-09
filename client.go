package apibind

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// errorBody is an internal type for decoding HTTP error response bodies.
type errorBody struct {
	Message string `json:"message"`
}

// Client is an API client that sends typed HTTP requests.
type Client struct {
	// BaseURL is the scheme+host of the API server (e.g. "http://localhost:8080").
	// If empty, the path built by Endpoint.Path is used as-is (same-origin).
	// Set to empty string for Go WebAssembly running on the same origin as the API.
	BaseURL string
	// HTTP is the underlying HTTP client. Defaults to http.DefaultClient when nil.
	HTTP *http.Client
}

// NewClient returns a new API client with the given base URL.
//
// Set baseURL to the scheme and host of the API server (e.g. "http://localhost:8080").
// For Go WebAssembly running on the same origin as the API, pass an empty string.
//
//	// For tests or a remote server:
//	client := apibind.NewClient("http://localhost:8080")
//
//	// For Wasm (same-origin):
//	client := apibind.NewClient("")
func NewClient(baseURL string) *Client {
	return &Client{BaseURL: baseURL, HTTP: http.DefaultClient}
}

// Call sends an HTTP request to the given endpoint and returns a typed response.
//
// GET and DELETE requests are sent without a body; remaining fields are sent as
// URL query parameters.
// POST, PUT, and PATCH requests encode req as JSON (excluding path parameter
// fields) and send it as the request body.
// HTTP errors (4xx/5xx) are returned as *APIError.
// Use errors.Is(err, apibind.ErrBadRequest) to check the error type.
func Call[Req, Resp any](c *Client, ep Endpoint[Req, Resp], req Req) (Resp, error) {
	var zero Resp

	// Build URL
	path := ep.Path.Build(req)
	url := path
	if c != nil && c.BaseURL != "" && len(path) > 0 && path[0] == '/' {
		url = c.BaseURL + path
	}
	// For GET and DELETE, append non-path fields as query parameters.
	if ep.Method == MethodGet || ep.Method == MethodDelete {
		if qs := ep.Path.BuildQueryString(req); qs != "" {
			url += "?" + qs
		}
	}

	// Build the request
	var httpReq *http.Request
	var err error
	if ep.Method == http.MethodPost || ep.Method == http.MethodPut || ep.Method == http.MethodPatch {
		data, encErr := ep.Path.BuildBody(req)
		if encErr != nil {
			return zero, fmt.Errorf("failed to encode request: %w", encErr)
		}
		httpReq, err = http.NewRequest(string(ep.Method), url, bytes.NewReader(data))
		if err != nil {
			return zero, err
		}
		httpReq.Header.Set("Content-Type", "application/json")
	} else {
		httpReq, err = http.NewRequest(string(ep.Method), url, nil)
		if err != nil {
			return zero, err
		}
	}

	// Choose HTTP client
	hc := http.DefaultClient
	if c != nil && c.HTTP != nil {
		hc = c.HTTP
	}

	resp, err := hc.Do(httpReq)
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return zero, nil
	}

	if resp.StatusCode >= 400 {
		var body errorBody
		json.NewDecoder(resp.Body).Decode(&body) //nolint:errcheck
		return zero, &APIError{StatusCode: resp.StatusCode, Message: body.Message}
	}

	var result Resp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return zero, fmt.Errorf("failed to decode response: %w", err)
	}
	return result, nil
}

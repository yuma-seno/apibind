package apibind

import (
	"encoding/json"
	"errors"
	"net/http"
)

// Handler returns an http.HandlerFunc that handles requests for this endpoint.
//
// For POST, PUT, and PATCH requests, the request body is decoded as JSON into Req,
// unless Req implements [RequestDecoder] (which takes precedence).
// If decoding fails, Handler responds with 400 Bad Request.
// For other methods (GET, DELETE, etc.), req is the zero value of Req.
//
// Path parameters defined with P() are automatically extracted from the URL
// using r.PathValue and written into the corresponding Req fields.
//
// If fn returns an *APIError, Handler responds with its StatusCode and a JSON
// error body. Any other error results in a 500 Internal Server Error.
//
// If Resp implements [ResponseEncoder], its WriteResponse method is called
// to produce the response. Otherwise, if Resp is [HTML], it is served as
// text/html. Otherwise, the response is JSON-encoded.
//
// The returned http.HandlerFunc is compatible with net/http and any framework
// that uses the standard net/http handler interface (chi, gorilla/mux, etc.).
//
// Example:
//
//	mux.HandleFunc(shared.GetUserAPI.RoutePattern(), shared.GetUserAPI.Handler(
//	    func(r *http.Request, req shared.GetUserRequest) (shared.User, error) {
//	        // req.ID is already populated from the URL path
//	        return fetchUser(req.ID)
//	    },
//	))
func (ep Endpoint[Req, Resp]) Handler(fn func(r *http.Request, req Req) (Resp, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req Req
		if ep.Method == http.MethodPost || ep.Method == http.MethodPut || ep.Method == http.MethodPatch {
			if decoder, ok := any(&req).(RequestDecoder); ok {
				if err := decoder.DecodeRequest(r); err != nil {
					writeJSONError(w, &APIError{StatusCode: http.StatusBadRequest, Message: "invalid request"})
					return
				}
			} else {
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					writeJSONError(w, &APIError{StatusCode: http.StatusBadRequest, Message: "invalid request body"})
					return
				}
			}
		}
		ep.Path.SetPathParams(&req, r.PathValue)
		if ep.Method == MethodGet || ep.Method == MethodDelete {
			ep.Path.SetQueryParams(&req, r.URL.Query().Get)
		}

		resp, err := fn(r, req)
		if err != nil {
			var apiErr *APIError
			if errors.As(err, &apiErr) {
				writeJSONError(w, apiErr)
			} else {
				writeJSONError(w, &APIError{StatusCode: http.StatusInternalServerError, Message: "internal server error"})
			}
			return
		}

		// Custom response encoding (takes highest precedence).
		// WriteResponse is responsible for writing the full response
		// (including any error state); no additional write is made after it returns.
		if encoder, ok := any(&resp).(ResponseEncoder); ok {
			encoder.WriteResponse(w)
			return
		} else if encoder, ok := any(resp).(ResponseEncoder); ok {
			encoder.WriteResponse(w)
			return
		}

		// HTML type support
		if html, ok := any(resp).(HTML); ok {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(html)) //nolint:errcheck
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}
}

// writeJSONError writes an APIError as a JSON response.
// If the error message starts with "<", it is treated as HTML.
func writeJSONError(w http.ResponseWriter, e *APIError) {
	if len(e.Message) > 0 && e.Message[0] == '<' {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(e.StatusCode)
		w.Write([]byte(e.Message)) //nolint:errcheck
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(e.StatusCode)
	json.NewEncoder(w).Encode(errorBody{Message: e.Message}) //nolint:errcheck
}

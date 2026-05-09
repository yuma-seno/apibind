// Package apibind provides a type-safe HTTP client for Go WebAssembly applications.
//
// # Overview
//
// apibind enables type-safe API calls between a Go WebAssembly frontend and a Go backend.
// When both sides are written in Go, you can share request/response types and
// guarantee the schema at compile time.
//
// # Usage
//
//  1. Define API endpoints (typically in a shared package):
//
//     var CreateUserAPI = apibind.Endpoint[CreateUserRequest, CreateUserResponse]{
//     Method: apibind.MethodPost,
//     Path:   apibind.NewPath[CreateUserRequest]().S("/api/users"),
//     }
//
//  2. Create a client and call the endpoint (in the frontend):
//
//     client := apibind.NewClient("") // empty string = same origin for Wasm
//     resp, err := apibind.Call(client, CreateUserAPI, CreateUserRequest{Name: "Alice"})
//
//  3. Check HTTP errors:
//
//     if errors.Is(err, apibind.ErrBadRequest) { ... }
//
// # Path Parameters
//
// Use P() when building a PathDef to define named path parameters.
// Call() substitutes the parameter value (extracted by the provided function) into the URL.
// RoutePattern() uses the names to generate the pattern string for server-side routing.
//
//	type GetUserRequest struct {
//	    ID string
//	}
//	var GetUserAPI = apibind.Endpoint[GetUserRequest, User]{
//	    Method: apibind.MethodGet,
//	    Path: apibind.NewPath[GetUserRequest]().
//	        S("/api/users/").
//	        P("id", func(r GetUserRequest) string { return r.ID }),
//	}
package apibind

// Method represents an HTTP method.
// Use the predefined constants (MethodGet, MethodPost, etc.) for type safety and IDE completion.
type Method string

// HTTP method constants.
const (
	MethodGet    Method = "GET"
	MethodPost   Method = "POST"
	MethodPut    Method = "PUT"
	MethodDelete Method = "DELETE"
	MethodPatch  Method = "PATCH"
)

// Endpoint is a typed API endpoint definition that associates a request type Req
// with a response type Resp.
//
// The type parameters make the input/output schema visible at the call site,
// serving as a shared API contract between frontend and backend.
//
// Example:
//
//	// Endpoint without path parameters:
//	var CreateUserAPI = Endpoint[CreateUserRequest, CreateUserResponse]{
//	    Method: MethodPost,
//	    Path:   NewPath[CreateUserRequest]().S("/api/users"),
//	}
//
//	// Endpoint with a path parameter:
//	var GetUserAPI = Endpoint[GetUserRequest, User]{
//	    Method: MethodGet,
//	    Path: NewPath[GetUserRequest]().
//	        S("/api/users/").
//	        P("id", func(r GetUserRequest) string { return r.ID }),
//	}
type Endpoint[Req, Resp any] struct {
	// Method is the HTTP method. Use the predefined constants (MethodGet, MethodPost, etc.).
	Method Method
	// Path defines the URL path for this endpoint.
	// Use NewPath to construct a PathDef, then chain S() for static segments and P() for
	// named path parameters.
	//
	// For cross-origin requests or tests, start the first static segment with
	// a full URL (e.g. "http://localhost:8080/api/users"); BaseURL is not prepended then.
	Path PathDef[Req]
}

// RoutePattern returns the route pattern string suitable for use as the first argument
// to http.ServeMux.HandleFunc (e.g. "GET /api/users/{id}").
func (ep Endpoint[Req, Resp]) RoutePattern() string {
	return string(ep.Method) + " " + ep.Path.Pattern()
}

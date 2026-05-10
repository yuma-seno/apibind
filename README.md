# apibind

A lightweight, type-safe HTTP client and server library for Go WebAssembly applications.

When you write both the frontend and backend in Go, you can share request/response types between them. `apibind` enforces those types at the call site so schema mismatches are caught at compile time.

## Installation

```sh
go get github.com/yuma-seno/apibind
```

## Quick Start

### 1. Define your API endpoints (in a shared package)

```go
// shared/api.go
package shared

import apibind "github.com/yuma-seno/apibind"

type CreateUserRequest struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

type CreateUserResponse struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
}

var CreateUserAPI = apibind.Endpoint[CreateUserRequest, CreateUserResponse]{
    Method: apibind.MethodPost,
    Path:   apibind.NewPath[CreateUserRequest]().S("/api/users"),
}
```

### 2. Implement the backend

```go
// cmd/backend/main.go
mux.HandleFunc(shared.CreateUserAPI.RoutePattern(), shared.CreateUserAPI.Handler(
    func(r *http.Request, req shared.CreateUserRequest) (shared.CreateUserResponse, error) {
        // req is already decoded from the JSON body
        return shared.CreateUserResponse{ID: 1, Name: req.Name}, nil
    },
))
```

### 3. Call the API from the frontend (Go Wasm)

```go
// cmd/frontend/main.go
//go:build js && wasm

var client = apibind.NewClient("") // empty = same origin

resp, err := apibind.Call(client, shared.CreateUserAPI, shared.CreateUserRequest{
    Name:  "Alice",
    Email: "alice@example.com",
})
if err != nil {
    if errors.Is(err, apibind.ErrBadRequest) {
        // handle 400
    }
    return
}
fmt.Println(resp.ID, resp.Name)
```

## Example Project

A complete, runnable example of apibind in a Go + WebAssembly full-stack application:

**[go-web-fullstack-template](https://github.com/yuma-seno/go-web-fullstack-template)** — Text analysis app with shared API types, go-app SPA frontend, and gomponents SSR preview.

## How Parameters Are Sent

`apibind` automatically routes request fields to the correct part of the HTTP request based on the method and how the path is defined:

| | `GET` / `DELETE` | `POST` / `PUT` |
|---|---|---|
| **Path params** (`P()`) | Substituted into the URL path | Substituted into the URL path |
| **All other fields** | URL query string (`?key=value`) | JSON request body |

## Path Parameters

Use `P()` to define named path parameters. A pointer accessor provides compile-time type safety and is used for both reading (in `Call`) and writing (in `Handler`).

```go
// shared/api.go
type UpdateUserRequest struct {
    ID   string
    Name string `json:"name"`
}

var UpdateUserAPI = apibind.Endpoint[UpdateUserRequest, User]{
    Method: apibind.MethodPut,
    Path: apibind.NewPath[UpdateUserRequest]().
        S("/api/users/").
        P("id", func(r *UpdateUserRequest) *string { return &r.ID }),
}
```

```go
// Frontend
resp, err := apibind.Call(client, shared.UpdateUserAPI, shared.UpdateUserRequest{
    ID:   "123",
    Name: "Alice",
})
// Sends: PUT /api/users/123
// Body:  {"name":"Alice"}  — ID is excluded from the body automatically
```

```go
// Backend
mux.HandleFunc(shared.UpdateUserAPI.RoutePattern(), shared.UpdateUserAPI.Handler(
    func(r *http.Request, req shared.UpdateUserRequest) (shared.User, error) {
        // req.ID   ← populated from the URL path automatically
        // req.Name ← decoded from the JSON body automatically
        return updateUser(req.ID, req.Name)
    },
))
```

## Query Parameters

For `GET` and `DELETE` requests, all non-path fields are automatically sent as URL query parameters.

Use pointer types (`*int`, `*string`, etc.) for **optional** parameters — they are omitted from the URL when `nil`. Non-pointer types are **always** included.

```go
// shared/api.go
type ListUsersRequest struct {
    Page  *int    `json:"page"`   // optional: omitted when nil
    Limit *int    `json:"limit"`  // optional
    Name  *string `json:"name"`   // optional
}

var ListUsersAPI = apibind.Endpoint[ListUsersRequest, []User]{
    Method: apibind.MethodGet,
    Path:   apibind.NewPath[ListUsersRequest]().S("/api/users"),
}
```

```go
// Frontend
page, limit := 1, 20
resp, err := apibind.Call(client, shared.ListUsersAPI, shared.ListUsersRequest{
    Page:  &page,
    Limit: &limit,
    // Name: nil → omitted
})
// Sends: GET /api/users?page=1&limit=20
```

```go
// Backend
mux.HandleFunc(shared.ListUsersAPI.RoutePattern(), shared.ListUsersAPI.Handler(
    func(r *http.Request, req shared.ListUsersRequest) ([]shared.User, error) {
        // req.Page  ← set from ?page=1 automatically (nil if absent)
        // req.Limit ← set from ?limit=20 automatically
        // req.Name  ← nil (not in query string)
        return listUsers(req)
    },
))
```

The json tag name is used as the query parameter key; the field name is used when no tag is present.

For `POST` and `PUT` body fields, optional parameters follow standard Go JSON behavior:
use a pointer type with `json:",omitempty"` to omit the field when `nil`, or a plain pointer to send `null`.

```go
type CreateUserRequest struct {
    Name  string  `json:"name"`            // required
    Email *string `json:"email,omitempty"` // optional: omitted from body when nil
    Bio   *string `json:"bio"`             // optional: sent as null when nil
}
```

## API Reference

### `Endpoint[Req, Resp]`

Defines a typed API endpoint. The type parameters `Req` and `Resp` represent the request and response schemas respectively.

```go
type Endpoint[Req, Resp any] struct {
    Method Method
    Path   PathDef[Req]
}
```

#### Methods

| Method | Description |
|---|---|
| `RoutePattern() string` | Returns `"METHOD /path/{param}"` for use with `http.ServeMux.HandleFunc` |
| `Handler(fn) http.HandlerFunc` | Returns a handler that decodes the request and encodes the response |

### `PathDef[Req]` — `NewPath[Req]()`

Defines the URL path. Chain `S()` for static segments and `P()` for named path parameters.

```go
// No path parameters
apibind.NewPath[CreateUserRequest]().S("/api/users")

// With path parameter
apibind.NewPath[GetUserRequest]().
    S("/api/users/").
    P("id", func(r *GetUserRequest) *string { return &r.ID })
```

`P(name, field)` takes a pointer accessor `func(*Req) *string` that:
- Returns a pointer to a **direct, non-embedded** field of `Req`
- Provides compile-time safety — referencing a non-existent field is a build error
- Is used by `Call()` to read the value (for URL construction) and by `Handler()` to write the value (from `r.PathValue`)

### `Method` constants

```go
apibind.MethodGet
apibind.MethodPost
apibind.MethodPut
apibind.MethodDelete
apibind.MethodPatch
```

### `Client` — `NewClient`

Holds the base URL and the underlying HTTP client.

```go
client := apibind.NewClient("")                        // same-origin (Wasm)
client := apibind.NewClient("http://localhost:8080")   // remote or test server
```

### `Call[Req, Resp]`

Sends the HTTP request defined by the endpoint and returns the typed response.

```go
resp, err := apibind.Call(client, MyAPI, req)
```

- `GET` / `DELETE`: path params in URL path, all other fields as query parameters
- `POST` / `PUT`: path params in URL path, all other fields as JSON body (path param fields are excluded from the body automatically)
- HTTP 4xx/5xx: returned as `*APIError`

### Error handling

```go
var apiErr *apibind.APIError
if errors.As(err, &apiErr) {
    fmt.Println(apiErr.StatusCode, apiErr.Message)
}

// Sentinel errors for common status codes
errors.Is(err, apibind.ErrBadRequest)   // 400
errors.Is(err, apibind.ErrNotFound)     // 404
errors.Is(err, apibind.ErrServerError)  // 500
```

To return an HTTP error from a `Handler` callback:

```go
return User{}, &apibind.APIError{StatusCode: http.StatusNotFound, Message: "user not found"}
```

## Design

`apibind` is intentionally minimal:

- **`Endpoint[Req, Resp]`** — the API contract. Put this in a shared package imported by both frontend and backend.
- **`Call()`** — the client-side function. Only the frontend uses this.
- **`ep.Handler(fn)`** — the server-side method. Wraps a typed function into `http.HandlerFunc`.
- The backend is unconstrained. Use any `net/http`-compatible framework you like.

This is the Go equivalent of what [tRPC](https://trpc.io/) does for TypeScript — type-safe API calls without code generation.

## Requirements

- Go 1.22 or later (`r.PathValue` requires Go 1.22; generics require Go 1.18+)
- Works with standard `net/http`, including Go's WebAssembly runtime

## License

See [LICENSE](./LICENSE).



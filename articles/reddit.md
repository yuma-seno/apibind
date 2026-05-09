# Reddit 投稿文（r/golang 向け）

---

**Title:**
I built a type-safe HTTP client/server library for Go Wasm full-stack apps — no codegen, no external deps

---

**Body:**

I've been building a full-stack Go app where the frontend is compiled to WebAssembly. Since both sides are Go, I can share request/response types — but I wanted the **compiler** to enforce those types at the call site, not just at runtime.

Existing options (gRPC/connect-go, huma, etc.) either require protobuf/codegen, focus only on the server side, or pull in significant dependencies. So I built **apibind**: a ~400-line library that makes the compiler your API contract checker.

**GitHub:** https://github.com/yuma-seno/apibind

---

The idea is to define an endpoint once in a shared package, then use it on both sides:

```go
// shared/api.go — imported by both frontend and backend
var UpdateUserAPI = apibind.Endpoint[UpdateUserRequest, User]{
    Method: apibind.MethodPut,
    Path: apibind.NewPath[UpdateUserRequest]().
        S("/api/users/").
        P("id", func(r *UpdateUserRequest) *string { return &r.ID }),
}
```

**Backend:**

```go
mux.HandleFunc(shared.UpdateUserAPI.RoutePattern(), shared.UpdateUserAPI.Handler(
    func(r *http.Request, req shared.UpdateUserRequest) (shared.User, error) {
        // req.ID   ← populated from URL path automatically
        // req.Name ← decoded from JSON body automatically
        return db.UpdateUser(req.ID, req.Name)
    },
))
```

**Frontend (Go Wasm):**

```go
resp, err := apibind.Call(ctx, client, shared.UpdateUserAPI, shared.UpdateUserRequest{
    ID:   "123",
    Name: "Alice",
})
// → PUT /api/users/123, body: {"name":"Alice"}
```

If the types don't match — wrong field name, wrong type — you get a compile error, not a 400 at midnight.

---

**Key features:**

- Path parameters defined with a pointer accessor (`func(*Req) *string`) — compile-time safe, referencing a non-existent field is a build error
- `context.Context` support in `Call()` — cancellation and timeouts work out of the box
- GET/DELETE: non-path fields sent as query parameters; pointer types (`*int`, `*string`) omitted when nil
- POST/PUT/PATCH: non-path fields go to the JSON body; path param fields excluded from the body automatically
- Zero external dependencies — pure `net/http`
- Works with Go's `http.ServeMux` (Go 1.22+) pattern `"PUT /api/users/{id}"`

---

The closest analogy is tRPC for TypeScript — one definition, type-safe on both client and server, no codegen.

Would love feedback on two things:
1. The path parameter API (`P(name, func(*Req) *string)`) — ergonomic, or too verbose?
2. Is there a use case you'd want this to cover that it currently doesn't?

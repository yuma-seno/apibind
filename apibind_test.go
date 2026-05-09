package apibind_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apibind "github.com/yuma-seno/apibind"
)

// ---- shared types ----

type userReq struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type userResp struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type getReq struct {
	ID   string `json:"id"`
	Page int    `json:"page"`
}

type patchReq struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ---- endpoint definitions ----

var createAPI = apibind.Endpoint[userReq, userResp]{
	Method: apibind.MethodPost,
	Path:   apibind.NewPath[userReq]().S("/api/users"),
}

var getAPI = apibind.Endpoint[getReq, userResp]{
	Method: apibind.MethodGet,
	Path: apibind.NewPath[getReq]().
		S("/api/users/").
		P("id", func(r *getReq) *string { return &r.ID }),
}

var deleteAPI = apibind.Endpoint[getReq, userResp]{
	Method: apibind.MethodDelete,
	Path: apibind.NewPath[getReq]().
		S("/api/users/").
		P("id", func(r *getReq) *string { return &r.ID }),
}

var putAPI = apibind.Endpoint[userReq, userResp]{
	Method: apibind.MethodPut,
	Path:   apibind.NewPath[userReq]().S("/api/users/1"),
}

var patchAPI = apibind.Endpoint[patchReq, userResp]{
	Method: apibind.MethodPatch,
	Path: apibind.NewPath[patchReq]().
		S("/api/users/").
		P("id", func(r *patchReq) *string { return &r.ID }),
}

// ---- Call tests ----

func TestCall_POST(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", ct)
		}
		var req userReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if req.Name != "Alice" {
			t.Errorf("expected name Alice, got %s", req.Name)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(userResp{ID: 1, Name: req.Name}) //nolint:errcheck
	}))
	defer srv.Close()

	ep := apibind.Endpoint[userReq, userResp]{
		Method: apibind.MethodPost,
		Path:   apibind.NewPath[userReq]().S("/api/users"),
	}
	client := apibind.NewClient(srv.URL)
	resp, err := apibind.Call(context.Background(), client, ep, userReq{Name: "Alice", Email: "alice@example.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != 1 || resp.Name != "Alice" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestCall_GET_QueryParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		// path param is in the URL path, page should be in query string
		if r.URL.Query().Get("page") != "2" {
			t.Errorf("expected page=2, got %s", r.URL.Query().Get("page"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(userResp{ID: 42, Name: "Bob"}) //nolint:errcheck
	}))
	defer srv.Close()

	ep := apibind.Endpoint[getReq, userResp]{
		Method: apibind.MethodGet,
		Path: apibind.NewPath[getReq]().
			S("/api/users/").
			P("id", func(r *getReq) *string { return &r.ID }),
	}
	client := apibind.NewClient(srv.URL)
	resp, err := apibind.Call(context.Background(), client, ep, getReq{ID: "42", Page: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != 42 {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestCall_PUT_Body(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		var req userReq
		json.NewDecoder(r.Body).Decode(&req) //nolint:errcheck
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(userResp{ID: 1, Name: req.Name}) //nolint:errcheck
	}))
	defer srv.Close()

	ep := apibind.Endpoint[userReq, userResp]{
		Method: apibind.MethodPut,
		Path:   apibind.NewPath[userReq]().S("/api/users/1"),
	}
	client := apibind.NewClient(srv.URL)
	resp, err := apibind.Call(context.Background(), client, ep, userReq{Name: "Carol"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Name != "Carol" {
		t.Errorf("unexpected name: %s", resp.Name)
	}
}

func TestCall_PATCH_Body(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("expected PATCH, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", ct)
		}
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		// "id" is a path param and must NOT appear in the body
		if _, ok := body["id"]; ok {
			t.Errorf("path param 'id' must not appear in the request body")
		}
		if _, ok := body["name"]; !ok {
			t.Errorf("'name' must appear in the request body")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(userResp{ID: 5, Name: "Dave"}) //nolint:errcheck
	}))
	defer srv.Close()

	ep := apibind.Endpoint[patchReq, userResp]{
		Method: apibind.MethodPatch,
		Path: apibind.NewPath[patchReq]().
			S("/api/users/").
			P("id", func(r *patchReq) *string { return &r.ID }),
	}
	client := apibind.NewClient(srv.URL)
	resp, err := apibind.Call(context.Background(), client, ep, patchReq{ID: "5", Name: "Dave"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != 5 {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestCall_DELETE_NoBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.Body != nil {
			// Body should be nil or empty for DELETE
			buf := make([]byte, 1)
			n, _ := io.ReadFull(r.Body, buf)
			if n > 0 {
				t.Errorf("expected no body for DELETE")
			}
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	ep := apibind.Endpoint[getReq, userResp]{
		Method: apibind.MethodDelete,
		Path: apibind.NewPath[getReq]().
			S("/api/users/").
			P("id", func(r *getReq) *string { return &r.ID }),
	}
	client := apibind.NewClient(srv.URL)
	_, err := apibind.Call(context.Background(), client, ep, getReq{ID: "99"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCall_PathParam_InURL(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(userResp{ID: 7, Name: "Eve"}) //nolint:errcheck
	}))
	defer srv.Close()

	ep := apibind.Endpoint[getReq, userResp]{
		Method: apibind.MethodGet,
		Path: apibind.NewPath[getReq]().
			S("/api/users/").
			P("id", func(r *getReq) *string { return &r.ID }),
	}
	client := apibind.NewClient(srv.URL)
	_, err := apibind.Call(context.Background(), client, ep, getReq{ID: "7"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/api/users/7" {
		t.Errorf("expected path /api/users/7, got %s", gotPath)
	}
}

func TestCall_4xx_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"message": "not found"}) //nolint:errcheck
	}))
	defer srv.Close()

	ep := apibind.Endpoint[getReq, userResp]{
		Method: apibind.MethodGet,
		Path:   apibind.NewPath[getReq]().S("/api/users/1"),
	}
	client := apibind.NewClient(srv.URL)
	_, err := apibind.Call(context.Background(), client, ep, getReq{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, apibind.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestCall_5xx_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"message": "server error"}) //nolint:errcheck
	}))
	defer srv.Close()

	ep := apibind.Endpoint[getReq, userResp]{
		Method: apibind.MethodGet,
		Path:   apibind.NewPath[getReq]().S("/api/users/1"),
	}
	client := apibind.NewClient(srv.URL)
	_, err := apibind.Call(context.Background(), client, ep, getReq{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, apibind.ErrServerError) {
		t.Errorf("expected ErrServerError, got %v", err)
	}
}

func TestCall_Context_Cancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// never respond — wait for client to cancel
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	ep := apibind.Endpoint[getReq, userResp]{
		Method: apibind.MethodGet,
		Path:   apibind.NewPath[getReq]().S("/api/users/1"),
	}
	client := apibind.NewClient(srv.URL)
	_, err := apibind.Call(ctx, client, ep, getReq{})
	if err == nil {
		t.Fatal("expected error due to cancelled context, got nil")
	}
}

// ---- Handler tests ----

func TestHandler_POST(t *testing.T) {
	ep := apibind.Endpoint[userReq, userResp]{
		Method: apibind.MethodPost,
		Path:   apibind.NewPath[userReq]().S("/api/users"),
	}
	handler := ep.Handler(func(r *http.Request, req userReq) (userResp, error) {
		return userResp{ID: 1, Name: req.Name}, nil
	})

	body := `{"name":"Frank","email":"frank@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp userResp
	json.NewDecoder(w.Body).Decode(&resp) //nolint:errcheck
	if resp.Name != "Frank" {
		t.Errorf("unexpected name: %s", resp.Name)
	}
}

func TestHandler_PATCH(t *testing.T) {
	ep := apibind.Endpoint[patchReq, userResp]{
		Method: apibind.MethodPatch,
		Path: apibind.NewPath[patchReq]().
			S("/api/users/").
			P("id", func(r *patchReq) *string { return &r.ID }),
	}
	handler := ep.Handler(func(r *http.Request, req patchReq) (userResp, error) {
		return userResp{ID: 9, Name: req.Name}, nil
	})

	body := `{"name":"Grace"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/users/9", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: body=%s", w.Code, w.Body.String())
	}
	var resp userResp
	json.NewDecoder(w.Body).Decode(&resp) //nolint:errcheck
	if resp.Name != "Grace" {
		t.Errorf("unexpected name: %s", resp.Name)
	}
}

func TestHandler_GET_PathParam(t *testing.T) {
	ep := apibind.Endpoint[getReq, userResp]{
		Method: apibind.MethodGet,
		Path: apibind.NewPath[getReq]().
			S("/api/users/").
			P("id", func(r *getReq) *string { return &r.ID }),
	}
	handler := ep.Handler(func(r *http.Request, req getReq) (userResp, error) {
		if req.ID != "42" {
			t.Errorf("expected ID 42, got %s", req.ID)
		}
		return userResp{ID: 42, Name: "Hank"}, nil
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users/42", nil)
	// Simulate path value extraction (Go 1.22+ mux sets this; use a wrapper for testing)
	req.SetPathValue("id", "42")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandler_GET_QueryParam(t *testing.T) {
	ep := apibind.Endpoint[getReq, userResp]{
		Method: apibind.MethodGet,
		Path:   apibind.NewPath[getReq]().S("/api/users"),
	}
	handler := ep.Handler(func(r *http.Request, req getReq) (userResp, error) {
		if req.Page != 3 {
			t.Errorf("expected page 3, got %d", req.Page)
		}
		return userResp{ID: 1, Name: "Ivy"}, nil
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users?page=3", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandler_InvalidBody_Returns400(t *testing.T) {
	ep := apibind.Endpoint[userReq, userResp]{
		Method: apibind.MethodPost,
		Path:   apibind.NewPath[userReq]().S("/api/users"),
	}
	handler := ep.Handler(func(r *http.Request, req userReq) (userResp, error) {
		return userResp{}, nil
	})

	req := httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandler_APIError(t *testing.T) {
	ep := apibind.Endpoint[userReq, userResp]{
		Method: apibind.MethodPost,
		Path:   apibind.NewPath[userReq]().S("/api/users"),
	}
	handler := ep.Handler(func(r *http.Request, req userReq) (userResp, error) {
		return userResp{}, apibind.ErrNotFound
	})

	req := httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandler_UnexpectedError_Returns500(t *testing.T) {
	ep := apibind.Endpoint[userReq, userResp]{
		Method: apibind.MethodPost,
		Path:   apibind.NewPath[userReq]().S("/api/users"),
	}
	handler := ep.Handler(func(r *http.Request, req userReq) (userResp, error) {
		return userResp{}, errors.New("something went wrong")
	})

	req := httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestRoutePattern(t *testing.T) {
	ep := apibind.Endpoint[getReq, userResp]{
		Method: apibind.MethodGet,
		Path: apibind.NewPath[getReq]().
			S("/api/users/").
			P("id", func(r *getReq) *string { return &r.ID }),
	}
	want := "GET /api/users/{id}"
	if got := ep.RoutePattern(); got != want {
		t.Errorf("RoutePattern() = %q, want %q", got, want)
	}
}

func TestAPIError_Is(t *testing.T) {
	err := &apibind.APIError{StatusCode: 400, Message: "bad input"}
	if !errors.Is(err, apibind.ErrBadRequest) {
		t.Error("expected errors.Is(err, ErrBadRequest) to be true")
	}
	if errors.Is(err, apibind.ErrNotFound) {
		t.Error("expected errors.Is(err, ErrNotFound) to be false")
	}
}

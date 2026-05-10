package apibind_test

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
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

// ---- Custom codec tests ----

// xmlReq implements RequestBody (client-side custom encoding) and RequestDecoder
// (server-side custom decoding) for XML-based endpoints.
type xmlReq struct {
	ID   string `xml:"id"`
	Name string `xml:"name"`
}

func (r xmlReq) RequestBody() (string, io.Reader, error) {
	data, err := xml.Marshal(r)
	if err != nil {
		return "", nil, err
	}
	return "application/xml", bytes.NewReader(data), nil
}

func (r *xmlReq) DecodeRequest(req *http.Request) error {
	data, err := io.ReadAll(req.Body)
	if err != nil {
		return err
	}
	return xml.Unmarshal(data, r)
}

func TestCall_RequestBody_CustomEncoding(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "application/xml" {
			t.Errorf("expected Content-Type application/xml, got %s", ct)
		}
		var req xmlReq
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if err := xml.Unmarshal(data, &req); err != nil {
			t.Fatalf("unmarshal xml request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"result": "hello " + req.Name})
	}))
	defer srv.Close()

	ep := apibind.Endpoint[xmlReq, map[string]string]{
		Method: apibind.MethodPost,
		Path:   apibind.NewPath[xmlReq]().S("/api/xml"),
	}
	client := apibind.NewClient(srv.URL)
	resp, err := apibind.Call(context.Background(), client, ep, xmlReq{Name: "Alice"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp["result"] != "hello Alice" {
		t.Errorf("expected 'hello Alice', got %q", resp["result"])
	}
}

// rawResp implements ResponseDecoder for raw byte responses.
type rawResp struct {
	Data []byte
}

func (r *rawResp) DecodeResponse(body io.ReadCloser) error {
	defer body.Close()
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	r.Data = data
	return nil
}

func TestCall_ResponseDecoder_RawBytes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write([]byte("raw binary data"))
	}))
	defer srv.Close()

	ep := apibind.Endpoint[getReq, rawResp]{
		Method: apibind.MethodGet,
		Path:   apibind.NewPath[getReq]().S("/api/raw"),
	}
	client := apibind.NewClient(srv.URL)
	resp, err := apibind.Call(context.Background(), client, ep, getReq{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(resp.Data) != "raw binary data" {
		t.Errorf("expected 'raw binary data', got %q", string(resp.Data))
	}
}

func TestCall_Stream_Download(t *testing.T) {
	payload := "streamed payload content"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(payload))
	}))
	defer srv.Close()

	ep := apibind.Endpoint[getReq, apibind.Stream]{
		Method: apibind.MethodGet,
		Path:   apibind.NewPath[getReq]().S("/api/stream"),
	}
	client := apibind.NewClient(srv.URL)
	stream, err := apibind.Call(context.Background(), client, ep, getReq{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer stream.Body.Close()
	data, _ := io.ReadAll(stream.Body)
	if string(data) != payload {
		t.Errorf("expected %q, got %q", payload, string(data))
	}
}

// multipartUploadReq implements RequestBody for multipart file upload.
type multipartUploadReq struct {
	Title    string
	Filename string
	Content  io.Reader
}

func (r multipartUploadReq) RequestBody() (string, io.Reader, error) {
	return apibind.NewMultipartBody(
		apibind.MultipartField{Name: "title", Value: r.Title},
		apibind.MultipartField{Name: "file", FileName: r.Filename, Reader: r.Content},
	)
}

func TestCall_Multipart_Upload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1024); err != nil {
			t.Fatalf("parse multipart: %v", err)
		}
		if title := r.FormValue("title"); title != "my file" {
			t.Errorf("expected title 'my file', got %q", title)
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("form file: %v", err)
		}
		defer file.Close()
		data, _ := io.ReadAll(file)
		if string(data) != "file content" {
			t.Errorf("expected 'file content', got %q", string(data))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer srv.Close()

	ep := apibind.Endpoint[multipartUploadReq, map[string]string]{
		Method: apibind.MethodPost,
		Path:   apibind.NewPath[multipartUploadReq]().S("/api/upload"),
	}
	client := apibind.NewClient(srv.URL)
	_, err := apibind.Call(context.Background(), client, ep, multipartUploadReq{
		Title:    "my file",
		Filename: "test.txt",
		Content:  strings.NewReader("file content"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewMultipartBody_InvalidFieldCombination(t *testing.T) {
	_, _, err := apibind.NewMultipartBody(
		apibind.MultipartField{Name: "file", FileName: "a.txt"},
	)
	if err == nil {
		t.Fatal("expected error when FileName is set but Reader is nil")
	}
}

// ---- Handler custom codec tests ----

// customReq implements RequestDecoder for server-side custom decoding.
type customReq struct {
	ID   string `json:"id"`
	Data string `json:"data"`
}

const customContentType = "application/vnd.custom+json"

func (r *customReq) DecodeRequest(req *http.Request) error {
	var payload struct {
		ID   string `json:"id"`
		Data string `json:"data"`
	}
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		return err
	}
	r.ID = "custom-" + payload.ID
	r.Data = payload.Data
	return nil
}

func TestHandler_RequestDecoder(t *testing.T) {
	ep := apibind.Endpoint[customReq, userResp]{
		Method: apibind.MethodPost,
		Path:   apibind.NewPath[customReq]().S("/api/custom"),
	}
	handler := ep.Handler(func(r *http.Request, req customReq) (userResp, error) {
		if req.ID != "custom-42" {
			t.Errorf("expected ID 'custom-42', got %q", req.ID)
		}
		return userResp{ID: 42, Name: req.Data}, nil
	})

	body := `{"id":"42","data":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/custom", strings.NewReader(body))
	req.Header.Set("Content-Type", customContentType)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp userResp
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Name != "hello" {
		t.Errorf("expected name 'hello', got %q", resp.Name)
	}
}

// countedResp implements ResponseEncoder for server-side custom response encoding.
type countedResp struct {
	Message string
}

func (r countedResp) WriteResponse(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "counted(%d): %s", len(r.Message), r.Message) //nolint:errcheck
}

func TestHandler_ResponseEncoder(t *testing.T) {
	ep := apibind.Endpoint[getReq, countedResp]{
		Method: apibind.MethodGet,
		Path:   apibind.NewPath[getReq]().S("/api/counted"),
	}
	handler := ep.Handler(func(r *http.Request, req getReq) (countedResp, error) {
		return countedResp{Message: "test"}, nil
	})

	req := httptest.NewRequest(http.MethodGet, "/api/counted", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/plain" {
		t.Errorf("expected Content-Type text/plain, got %s", ct)
	}
	if body := w.Body.String(); body != "counted(4): test" {
		t.Errorf("expected 'counted(4): test', got %q", body)
	}
}

func TestHandler_Stream_ServeFile(t *testing.T) {
	ep := apibind.Endpoint[getReq, apibind.Stream]{
		Method: apibind.MethodGet,
		Path:   apibind.NewPath[getReq]().S("/api/serve"),
	}
	handler := ep.Handler(func(r *http.Request, req getReq) (apibind.Stream, error) {
		return apibind.Stream{
			Body:        io.NopCloser(strings.NewReader("file content")),
			ContentType: "text/plain",
		}, nil
	})

	req := httptest.NewRequest(http.MethodGet, "/api/serve", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/plain" {
		t.Errorf("expected Content-Type text/plain, got %s", ct)
	}
	if body := w.Body.String(); body != "file content" {
		t.Errorf("expected 'file content', got %q", body)
	}
}

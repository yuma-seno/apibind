package apibind

import (
	"io"
	"net/http"
)

// RequestBody allows Req to provide a custom request body encoding
// for POST, PUT, and PATCH methods on the client side.
// When implemented, it takes precedence over the default JSON encoding.
type RequestBody interface {
	RequestBody() (contentType string, body io.Reader, err error)
}

// ResponseDecoder allows Resp to decode the HTTP response body
// on the client side. When implemented, it takes precedence over
// the default JSON decoding.
//
// The implementation takes ownership of body and must close it when done.
type ResponseDecoder interface {
	DecodeResponse(body io.ReadCloser) error
}

// RequestDecoder allows Req to decode itself from an HTTP request
// on the server side. When implemented, it takes precedence over
// the default JSON body decoding.
//
// Path and query parameters are still set automatically before this
// is called, so they are available in req when DecodeRequest runs.
type RequestDecoder interface {
	DecodeRequest(r *http.Request) error
}

// ResponseEncoder allows Resp to write itself as an HTTP response
// on the server side. When implemented, it takes precedence over
// the default JSON encoding and the HTML special case.
type ResponseEncoder interface {
	WriteResponse(w http.ResponseWriter) error
}

// Stream is a helper type for streaming binary data in HTTP responses.
// Use it as the response type in an Endpoint definition to handle
// file downloads or any binary streaming scenario.
//
// Client side (download):
//
//	var DownloadAPI = apibind.Endpoint[DownloadReq, apibind.Stream]{...}
//	stream, err := apibind.Call(ctx, client, DownloadAPI, req)
//	if err != nil { return }
//	defer stream.Body.Close()
//	io.Copy(outputFile, stream.Body)
//
// Server side (serve file):
//
//	ep.Handler(func(r *http.Request, req DownloadReq) (apibind.Stream, error) {
//	    file, err := os.Open(req.Path)
//	    if err != nil { return apibind.Stream{}, err }
//	    return apibind.Stream{Body: file, ContentType: "audio/wav"}, nil
//	})
type Stream struct {
	Body        io.ReadCloser
	ContentType string
}

// DecodeResponse implements ResponseDecoder for streaming downloads.
func (s *Stream) DecodeResponse(body io.ReadCloser) error {
	s.Body = body
	return nil
}

// WriteResponse implements ResponseEncoder for streaming file serving.
func (s *Stream) WriteResponse(w http.ResponseWriter) error {
	if s.ContentType != "" {
		w.Header().Set("Content-Type", s.ContentType)
	}
	_, err := io.Copy(w, s.Body)
	s.Body.Close()
	return err
}
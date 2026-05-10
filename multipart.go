package apibind

import (
	"fmt"
	"io"
	"mime/multipart"
)

// MultipartField represents a field in a multipart request body.
type MultipartField struct {
	Name     string    // form field name
	Value    string    // field value (for text fields)
	FileName string    // filename (set for file uploads; empty for text fields)
	Reader   io.Reader // file content (used when FileName is non-empty)
}

// NewMultipartBody builds a streaming multipart request body from the given fields.
// It returns the Content-Type header value (including boundary) and the body reader.
//
// The body is streamed via io.Pipe, so large files are not buffered in memory.
// Example usage in a RequestBody implementation:
//
//	func (r UploadRequest) RequestBody() (string, io.Reader, error) {
//	    return apibind.NewMultipartBody(
//	        apibind.MultipartField{Name: "title", Value: r.Title},
//	        apibind.MultipartField{Name: "file", FileName: r.Filename, Reader: r.File},
//	    )
//	}
func NewMultipartBody(fields ...MultipartField) (contentType string, body io.Reader, err error) {
	for _, f := range fields {
		if f.Name == "" {
			return "", nil, fmt.Errorf("multipart field name is required")
		}
		if f.FileName != "" && f.Reader == nil {
			return "", nil, fmt.Errorf("multipart file field %q: Reader is required when FileName is set", f.Name)
		}
		if f.FileName == "" && f.Reader != nil {
			return "", nil, fmt.Errorf("multipart field %q: FileName is required when Reader is set", f.Name)
		}
	}

	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)

	go func() {
		defer pw.Close()
		for _, f := range fields {
			if f.FileName != "" && f.Reader != nil {
				fw, ferr := mw.CreateFormFile(f.Name, f.FileName)
				if ferr != nil {
					pw.CloseWithError(ferr)
					return
				}
				if _, cerr := io.Copy(fw, f.Reader); cerr != nil {
					pw.CloseWithError(cerr)
					return
				}
			} else {
				if werr := mw.WriteField(f.Name, f.Value); werr != nil {
					pw.CloseWithError(werr)
					return
				}
			}
		}
		if cerr := mw.Close(); cerr != nil {
			pw.CloseWithError(cerr)
			return
		}
	}()

	return mw.FormDataContentType(), pr, nil
}

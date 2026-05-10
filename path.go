package apibind

import (
	"encoding/json"
	"reflect"
	"strings"
)

// PathDef defines a URL path with both a type-safe URL builder and a route pattern.
//
// Create a PathDef with NewPath and chain S() (static segment) and P() (path parameter)
// calls to compose the path definition.
//
// Example:
//
//	// Static path (no parameters):
//	apibind.NewPath[CreateUserRequest]().S("/api/users")
//
//	// Dynamic path (with a parameter):
//	apibind.NewPath[GetUserRequest]().
//	    S("/api/users/").
//	    P("id", func(r *GetUserRequest) *string { return &r.ID })
type PathDef[Req any] struct {
	segs []pathSeg[Req]
}

type pathSeg[Req any] struct {
	static string
	name   string
	field  func(*Req) *string // nil for static segments
}

// NewPath creates a PathDef builder for the given request type.
func NewPath[Req any]() PathDef[Req] {
	return PathDef[Req]{}
}

// S appends a static segment to the path (e.g. "/api/users/").
func (p PathDef[Req]) S(s string) PathDef[Req] {
	p.segs = append(p.segs, pathSeg[Req]{static: s})
	return p
}

// P appends a named path parameter segment.
// name appears as {name} in the route pattern returned by Endpoint.RoutePattern.
// field is a pointer accessor that returns a pointer to the request field holding the
// parameter value. It is used to read the value (in Call) and write it (in Handler).
//
// The accessor must return a non-nil pointer to a direct (non-embedded) field of
// the Req struct. Nested or embedded struct fields are not supported.
//
//	P("id", func(r *GetUserRequest) *string { return &r.ID })
func (p PathDef[Req]) P(name string, field func(*Req) *string) PathDef[Req] {
	p.segs = append(p.segs, pathSeg[Req]{name: name, field: field})
	return p
}

// Build constructs the concrete URL path for the given request.
// It is called internally by Call() and is not usually invoked directly.
func (p PathDef[Req]) Build(req Req) string {
	switch len(p.segs) {
	case 0:
		return ""
	case 1:
		if p.segs[0].field == nil {
			return p.segs[0].static
		}
		return *p.segs[0].field(&req)
	}
	var sb strings.Builder
	for _, seg := range p.segs {
		if seg.field != nil {
			sb.WriteString(*seg.field(&req))
		} else {
			sb.WriteString(seg.static)
		}
	}
	return sb.String()
}

// Pattern returns the URL route pattern with placeholders (e.g. "/api/users/{id}").
// It is used by Endpoint.RoutePattern and is not usually invoked directly.
func (p PathDef[Req]) Pattern() string {
	switch len(p.segs) {
	case 0:
		return ""
	case 1:
		if p.segs[0].field == nil {
			return p.segs[0].static
		}
		return "{" + p.segs[0].name + "}"
	}
	var sb strings.Builder
	for _, seg := range p.segs {
		if seg.field != nil {
			sb.WriteString("{" + seg.name + "}")
		} else {
			sb.WriteString(seg.static)
		}
	}
	return sb.String()
}

// SetPathParams writes path parameter values into req by calling valueFn for each
// parameter name. Use r.PathValue as valueFn when called from an http.Handler.
//
//	ep.Path.SetPathParams(&req, r.PathValue)
//
// req must not be nil.
func (p PathDef[Req]) SetPathParams(req *Req, valueFn func(string) string) {
	if req == nil {
		return
	}
	for _, seg := range p.segs {
		if seg.field != nil {
			*seg.field(req) = valueFn(seg.name)
		}
	}
}

// BuildBody returns the JSON-encoded request body with path parameter fields excluded.
// If there are no path parameters, it is equivalent to json.Marshal(req).
//
// The exclusion is performed by marshaling the full struct first (preserving all
// json tags, omitempty, and custom MarshalJSON implementations), then removing
// the path parameter keys from the resulting JSON object.
func (p PathDef[Req]) BuildBody(req Req) ([]byte, error) {
	if !p.hasPathParams() {
		return json.Marshal(req)
	}
	// Marshal the full struct to preserve all json semantics.
	fullJSON, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	// Decode into a raw key→value map.
	var bodyMap map[string]json.RawMessage
	if err := json.Unmarshal(fullJSON, &bodyMap); err != nil {
		return nil, err
	}
	// Remove path parameter fields from the map by matching field addresses.
	pathAddrs := p.pathParamAddrs(&req)
	rv := reflect.ValueOf(&req).Elem()
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		fv := rv.Field(i)
		if !fv.CanAddr() || !pathAddrs[fv.Addr().Pointer()] {
			continue
		}
		key := rt.Field(i).Name
		if tag := rt.Field(i).Tag.Get("json"); tag != "" {
			if parts := strings.SplitN(tag, ",", 2); parts[0] != "" && parts[0] != "-" {
				key = parts[0]
			}
		}
		delete(bodyMap, key)
	}
	return json.Marshal(bodyMap)
}

func (p PathDef[Req]) hasPathParams() bool {
	for _, seg := range p.segs {
		if seg.field != nil {
			return true
		}
	}
	return false
}

// pathParamAddrs returns the memory addresses of path parameter fields for the given request.
// This is used by BuildBody and BuildQueryString to identify and exclude path param fields.
func (p PathDef[Req]) pathParamAddrs(req *Req) map[uintptr]bool {
	addrs := make(map[uintptr]bool, len(p.segs))
	for _, seg := range p.segs {
		if seg.field != nil {
			ptr := seg.field(req)
			if ptr != nil {
				addrs[reflect.ValueOf(ptr).Pointer()] = true
			}
		}
	}
	return addrs
}

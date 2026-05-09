package apibind

import (
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"
)

// BuildQueryString builds a URL-encoded query string from the non-path fields of req.
//
// Pointer fields (e.g. *int, *string) are omitted when nil (optional parameters).
// Non-pointer fields are always included (required parameters).
// Fields tagged with json:"-" are skipped.
// The json struct tag is used as the query parameter key; the field name is used otherwise.
//
// This is called automatically by Call() for GET and DELETE requests.
func (p PathDef[Req]) BuildQueryString(req Req) string {
	pathAddrs := p.pathParamAddrs(&req)
	rv := reflect.ValueOf(&req).Elem()
	rt := rv.Type()

	var params []string
	for i := 0; i < rt.NumField(); i++ {
		fv := rv.Field(i)
		ft := rt.Field(i)

		// Skip path parameter fields.
		if fv.CanAddr() && pathAddrs[fv.Addr().Pointer()] {
			continue
		}

		// Determine the query key from the json tag or field name.
		key := ft.Name
		if tag := ft.Tag.Get("json"); tag != "" {
			parts := strings.SplitN(tag, ",", 2)
			if parts[0] == "-" {
				continue
			}
			if parts[0] != "" {
				key = parts[0]
			}
		}

		// Pointer fields: omit if nil (optional), dereference otherwise.
		if fv.Kind() == reflect.Ptr {
			if fv.IsNil() {
				continue
			}
			fv = fv.Elem()
		}

		params = append(params, url.QueryEscape(key)+"="+url.QueryEscape(queryValueToString(fv)))
	}
	return strings.Join(params, "&")
}

// SetQueryParams populates non-path query parameter fields of req from the URL query string.
// valueFn should be r.URL.Query().Get for use inside an http.Handler.
// Fields absent from the query string (empty valueFn result) are left unchanged.
//
// This is called automatically by Handler for GET and DELETE requests.
func (p PathDef[Req]) SetQueryParams(req *Req, valueFn func(string) string) {
	if req == nil {
		return
	}
	pathAddrs := p.pathParamAddrs(req)
	rv := reflect.ValueOf(req).Elem()
	rt := rv.Type()

	for i := 0; i < rt.NumField(); i++ {
		fv := rv.Field(i)
		ft := rt.Field(i)

		if !fv.CanSet() {
			continue
		}
		// Skip path parameter fields.
		if fv.CanAddr() && pathAddrs[fv.Addr().Pointer()] {
			continue
		}

		key := ft.Name
		if tag := ft.Tag.Get("json"); tag != "" {
			parts := strings.SplitN(tag, ",", 2)
			if parts[0] == "-" {
				continue
			}
			if parts[0] != "" {
				key = parts[0]
			}
		}

		value := valueFn(key)
		if value == "" {
			continue
		}

		setQueryValue(fv, value) //nolint:errcheck
	}
}

// queryValueToString converts a non-pointer reflect.Value to its string representation
// for use as a query parameter value. For non-primitive types, JSON encoding is used.
func queryValueToString(v reflect.Value) string {
	switch v.Kind() {
	case reflect.String:
		return v.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(v.Uint(), 10)
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(v.Float(), 'f', -1, 64)
	case reflect.Bool:
		return strconv.FormatBool(v.Bool())
	default:
		// For complex types (structs, slices, maps, etc.), use JSON encoding.
		b, err := json.Marshal(v.Interface())
		if err != nil {
			return fmt.Sprint(v.Interface())
		}
		return string(b)
	}
}

// setQueryValue sets a struct field from a query parameter string value, handling
// pointer fields and numeric/bool type conversions.
func setQueryValue(fv reflect.Value, s string) error {
	if fv.Kind() == reflect.Ptr {
		elem := reflect.New(fv.Type().Elem())
		if err := setQueryValueDirect(elem.Elem(), s); err != nil {
			return err
		}
		fv.Set(elem)
		return nil
	}
	return setQueryValueDirect(fv, s)
}

func setQueryValueDirect(fv reflect.Value, s string) error {
	switch fv.Kind() {
	case reflect.String:
		fv.SetString(s)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return err
		}
		fv.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return err
		}
		fv.SetUint(n)
	case reflect.Float32, reflect.Float64:
		n, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return err
		}
		fv.SetFloat(n)
	case reflect.Bool:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return err
		}
		fv.SetBool(b)
	default:
		// For complex types (structs, slices, maps, etc.), use JSON decoding.
		if fv.CanAddr() {
			return json.Unmarshal([]byte(s), fv.Addr().Interface())
		}
	}
	return nil
}

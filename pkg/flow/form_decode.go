package flow

import (
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"
)

// decodeForm maps url.Values into the struct pointed to by dst.
//
// Field mapping rules (in priority order):
//  1. `form:"name"` struct tag
//  2. lowercase struct field name
//
// Supported field types: string, bool, int, int8, int16, int32, int64,
// uint, uint8, uint16, uint32, uint64, float32, float64, and []string.
// Nested structs and unexported fields are skipped silently.
// Unknown form keys are silently ignored.
func decodeForm(vals url.Values, dst interface{}) error {
	if dst == nil {
		return fmt.Errorf("dst is nil")
	}
	rv := reflect.ValueOf(dst)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return fmt.Errorf("dst must be a non-nil pointer to a struct")
	}
	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return fmt.Errorf("dst must be a pointer to a struct, got pointer to %s", rv.Kind())
	}
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		sf := rt.Field(i)
		fv := rv.Field(i)
		if !sf.IsExported() || !fv.CanSet() {
			continue
		}
		key := sf.Tag.Get("form")
		if key == "" {
			key = strings.ToLower(sf.Name)
		}
		// skip explicitly ignored fields
		if key == "-" {
			continue
		}
		rawList, ok := vals[key]
		if !ok || len(rawList) == 0 {
			continue
		}
		raw := rawList[0]
		if err := setField(fv, sf.Type, raw, rawList); err != nil {
			return fmt.Errorf("field %q: %w", key, err)
		}
	}
	return nil
}

// setField assigns the string value raw (or the full rawList for slices)
// to the reflect.Value fv of the given type.
func setField(fv reflect.Value, ft reflect.Type, raw string, rawList []string) error {
	switch ft.Kind() {
	case reflect.String:
		fv.SetString(raw)
	case reflect.Bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return fmt.Errorf("cannot parse %q as bool", raw)
		}
		fv.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(raw, 10, ft.Bits())
		if err != nil {
			return fmt.Errorf("cannot parse %q as int", raw)
		}
		fv.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(raw, 10, ft.Bits())
		if err != nil {
			return fmt.Errorf("cannot parse %q as uint", raw)
		}
		fv.SetUint(n)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(raw, ft.Bits())
		if err != nil {
			return fmt.Errorf("cannot parse %q as float", raw)
		}
		fv.SetFloat(f)
	case reflect.Slice:
		if ft.Elem().Kind() != reflect.String {
			return fmt.Errorf("only []string slices are supported")
		}
		fv.Set(reflect.ValueOf(rawList))
	default:
		// silently skip unsupported types (nested structs, maps, etc.)
	}
	return nil
}

package form

import (
	"encoding"
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type encoder struct {
	e         *Encoder
	errs      EncodeErrors
	columns   []string
	values    url.Values
	goValues  map[string]interface{}
	namespace []byte
}

func (e *encoder) setError(namespace []byte, err error) {
	if e.errs == nil {
		e.errs = make(EncodeErrors)
	}

	e.errs[string(namespace)] = err
}

func (e *encoder) setVal(namespace []byte, v reflect.Value, vals ...string) {
	if e.goValues != nil {
		e.goValues[string(namespace)] = v.Interface()
	}

	arr, ok := e.values[string(namespace)]
	if ok {
		arr = append(arr, vals...)
	} else {
		if e.columns != nil {
			e.columns = append(e.columns, string(namespace))
		}
		arr = vals
	}

	e.values[string(namespace)] = arr
}

func (e *encoder) traverseStruct(v reflect.Value, namespace []byte, idx int) {
	typ := v.Type()
	l := len(namespace)
	first := l == 0

	// anonymous structs will still work for caching as the whole definition is stored
	// including tags
	s, ok := e.e.structCache.Get(typ)
	if !ok {
		s = e.e.structCache.parseStruct(e.e.mode, typ, e.e.tagName)
	}

	for _, f := range s.fields {
		namespace = namespace[:l]

		if f.isAnonymous && e.e.embedAnonymous {
			if f.hasExportedScalar {
				e.setFieldByType(v.Field(f.idx), namespace, idx, f)
			}

			continue
		}

		if first {
			namespace = append(namespace, f.name...)
		} else {
			namespace = append(namespace, e.e.namespacePrefix...)
			namespace = append(namespace, f.name...)
			namespace = append(namespace, e.e.namespaceSuffix...)
		}

		e.setFieldByType(v.Field(f.idx), namespace, idx, f)

		if f.sliceSeparator != 0 {
			ns := string(namespace)
			if len(e.values[ns]) > 0 {
				e.values[ns] = []string{strings.Join(e.values[ns], string(f.sliceSeparator))}
			}
		}
	}
}

func (e *encoder) setFieldByType(current reflect.Value, namespace []byte, idx int, f cachedField) {
	if idx > -1 && current.Kind() == reflect.Ptr {
		namespace = append(namespace, '[')
		namespace = strconv.AppendInt(namespace, int64(idx), 10)
		namespace = append(namespace, ']')
		idx = -2
	}

	if f.isOmitEmpty && !hasValue(current) {
		return
	}

	v, kind := ExtractType(current)

	if e.e.customTypeFuncs != nil {
		if cf, ok := e.e.customTypeFuncs[v.Type()]; ok {
			val, err := cf(v.Interface())
			if err != nil {
				e.setError(namespace, err)

				return
			}

			if idx > -1 {
				namespace = append(namespace, '[')
				namespace = strconv.AppendInt(namespace, int64(idx), 10)
				namespace = append(namespace, ']')
			}

			e.setVal(namespace, v, val)

			return
		}
	}

	if f.isExported && len(namespace) > 0 && !(kind == reflect.Ptr && v.IsNil()) {
		if tu, ok := v.Interface().(encoding.TextMarshaler); ok {
			val, err := tu.MarshalText()
			if err != nil {
				e.setError(namespace, err)

				return
			}

			if idx > -1 {
				namespace = append(namespace, '[')
				namespace = strconv.AppendInt(namespace, int64(idx), 10)
				namespace = append(namespace, ']')
			}

			e.setVal(namespace, v, string(val))
		}
	}

	switch kind {
	case reflect.Ptr, reflect.Interface, reflect.Invalid:
		return

	case reflect.String:
		e.setVal(namespace, v, v.String())

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		e.setVal(namespace, v, strconv.FormatUint(v.Uint(), 10))

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		e.setVal(namespace, v, strconv.FormatInt(v.Int(), 10))

	case reflect.Float32:
		e.setVal(namespace, v, strconv.FormatFloat(v.Float(), 'f', -1, 32))

	case reflect.Float64:
		e.setVal(namespace, v, strconv.FormatFloat(v.Float(), 'f', -1, 64))

	case reflect.Bool:
		e.setVal(namespace, v, strconv.FormatBool(v.Bool()))

	case reflect.Slice, reflect.Array:
		if idx == -1 {
			for i := 0; i < v.Len(); i++ {
				e.setFieldByType(v.Index(i), namespace, i, cachedField{})
			}

			return
		}

		if idx > -1 {
			namespace = append(namespace, '[')
			namespace = strconv.AppendInt(namespace, int64(idx), 10)
			namespace = append(namespace, ']')
		}

		namespace = append(namespace, '[')
		l := len(namespace)

		for i := 0; i < v.Len(); i++ {
			namespace = namespace[:l]
			namespace = strconv.AppendInt(namespace, int64(i), 10)
			namespace = append(namespace, ']')
			e.setFieldByType(v.Index(i), namespace, -2, cachedField{})
		}

	case reflect.Map:
		if idx > -1 {
			namespace = append(namespace, '[')
			namespace = strconv.AppendInt(namespace, int64(idx), 10)
			namespace = append(namespace, ']')
		}

		var (
			valid bool
			s     string
		)

		l := len(namespace)

		for _, key := range v.MapKeys() {
			namespace = namespace[:l]

			if s, valid = e.getMapKey(key, namespace); !valid {
				continue
			}

			namespace = append(namespace, '[')
			namespace = append(namespace, s...)
			namespace = append(namespace, ']')

			e.setFieldByType(v.MapIndex(key), namespace, -2, cachedField{})
		}

	case reflect.Struct:
		// if we get here then no custom time function declared so use RFC3339 by default
		if v.Type() == timeType {
			if idx > -1 {
				namespace = append(namespace, '[')
				namespace = strconv.AppendInt(namespace, int64(idx), 10)
				namespace = append(namespace, ']')
			}

			e.setVal(namespace, v, v.Interface().(time.Time).Format(time.RFC3339))

			return
		}

		if idx == -1 {
			e.traverseStruct(v, namespace, idx)

			return
		}

		if idx > -1 {
			namespace = append(namespace, '[')
			namespace = strconv.AppendInt(namespace, int64(idx), 10)
			namespace = append(namespace, ']')
		}

		e.traverseStruct(v, namespace, -2)
	}
}

func (e *encoder) getMapKey(key reflect.Value, namespace []byte) (string, bool) {
	v, kind := ExtractType(key)

	if e.e.customTypeFuncs != nil {
		if cf, ok := e.e.customTypeFuncs[v.Type()]; ok {
			val, err := cf(v.Interface())
			if err != nil {
				e.setError(namespace, err)

				return "", false
			}

			return val, true
		}
	}

	switch kind {
	case reflect.Interface, reflect.Ptr:
		return "", false

	case reflect.String:
		return v.String(), true

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(v.Uint(), 10), true

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10), true

	case reflect.Float32:
		return strconv.FormatFloat(v.Float(), 'f', -1, 32), true

	case reflect.Float64:
		return strconv.FormatFloat(v.Float(), 'f', -1, 64), true

	case reflect.Bool:
		return strconv.FormatBool(v.Bool()), true

	default:
		e.setError(namespace, fmt.Errorf("unsupported map key '%v' namespace '%s'", v.String(), namespace))

		return "", false
	}
}

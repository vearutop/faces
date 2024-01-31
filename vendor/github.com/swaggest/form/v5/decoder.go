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

const (
	errArraySize = "array size of '%d' is larger than the maximum currently set on the decoder of '%d', " +
		"see SetMaxArraySize(size uint)"
	errMissingStartBracket = "invalid formatting for key '%s' missing '[' bracket"
	errMissingEndBracket   = "invalid formatting for key '%s' missing ']' bracket"
)

type decoder struct {
	d         *Decoder
	errs      DecodeErrors
	dm        dataMap
	dmDone    bool
	values    url.Values
	goValues  map[string]interface{}
	maxKeyLen int
	namespace []byte
}

func (d *decoder) setError(namespace []byte, err error) {
	if d.errs == nil {
		d.errs = make(DecodeErrors)
	}

	d.errs[string(namespace)] = err
}

func (d *decoder) findAlias(ns string) *recursiveData {
	for i := 0; i < len(d.dm); i++ {
		if d.dm[i].alias == ns {
			return d.dm[i]
		}
	}

	return nil
}

func (d *decoder) parseMapData() error {
	// already parsed
	if d.dmDone {
		return nil
	}

	d.maxKeyLen = 0
	d.dm = d.dm[0:0]
	d.dmDone = true

	var (
		i             int
		idx           int
		l             int
		insideBracket bool
		rd            *recursiveData
		isNum         bool
	)

	for k := range d.values {
		if len(k) > d.maxKeyLen {
			d.maxKeyLen = len(k)
		}

		for i = 0; i < len(k); i++ {
			switch k[i] {
			case '[':
				idx = i
				insideBracket = true
				isNum = true
			case ']':
				if !insideBracket {
					return fmt.Errorf(errMissingStartBracket, k)
				}

				if rd = d.findAlias(k[:idx]); rd == nil {
					l = len(d.dm) + 1

					if l > cap(d.dm) {
						dm := make(dataMap, l)
						copy(dm, d.dm)

						rd = new(recursiveData)
						dm[len(d.dm)] = rd
						d.dm = dm
					} else {
						l = len(d.dm)
						d.dm = d.dm[:l+1]
						rd = d.dm[l]
						rd.sliceLen = 0
						rd.keys = rd.keys[0:0]
					}

					rd.alias = k[:idx]
				}

				// is map + key
				ke := key{
					ivalue:      -1,
					value:       k[idx+1 : i],
					searchValue: k[idx : i+1],
				}

				// is key is number, most likely array key, keep track of just in case an array/slice.
				if isNum {
					// no need to check for error, it will always pass
					// as we have done the checking to ensure
					// the value is a number ahead of time.
					var err error

					ke.ivalue, err = strconv.Atoi(ke.value)
					if err != nil {
						ke.ivalue = -1
					}

					if ke.ivalue > rd.sliceLen {
						rd.sliceLen = ke.ivalue
					}
				}

				rd.keys = append(rd.keys, ke)

				insideBracket = false
			default:
				// checking if not a number, 0-9 is 48-57 in byte, see for
				// yourself fmt.Println('0', '1', '2', '3', '4', '5', '6', '7', '8', '9')
				if insideBracket && (k[i] > 57 || k[i] < 48) {
					isNum = false
				}
			}
		}

		// if still inside bracket, that means no ending bracket was ever specified
		if insideBracket {
			return fmt.Errorf(errMissingEndBracket, k)
		}
	}

	return nil
}

func (d *decoder) traverseStruct(v reflect.Value, typ reflect.Type, namespace []byte) (set bool) {
	l := len(namespace)
	first := l == 0

	// anonymous structs will still work for caching as the whole definition is stored
	// including tags
	s, ok := d.d.structCache.Get(typ)
	if !ok {
		s = d.d.structCache.parseStruct(d.d.mode, typ, d.d.tagName)
	}

	for _, f := range s.fields {
		if !f.canSet {
			continue
		}

		namespace = namespace[:l]

		if f.isAnonymous && f.hasExportedScalar {
			if d.setFieldByType(v.Field(f.idx), false, namespace, 0) {
				set = true
			}
		}

		if first {
			namespace = append(namespace, f.name...)
		} else {
			namespace = append(namespace, d.d.namespacePrefix...)
			namespace = append(namespace, f.name...)
			namespace = append(namespace, d.d.namespaceSuffix...)
		}

		if f.sliceSeparator != 0 {
			if len(d.values[f.name]) > 0 {
				d.values[f.name] = strings.Split(d.values[f.name][0], string(f.sliceSeparator))
			}
		}

		if d.setFieldByType(v.Field(f.idx), false, namespace, 0) {
			if d.goValues != nil && f.name == string(namespace) {
				d.goValues[f.name] = v.Field(f.idx).Interface()
			}

			set = true
		}
	}

	return set
}

//nolint:maintidx // This function is indeed a bit large, but sequentially structured.
func (d *decoder) setFieldByType(current reflect.Value, isPtr bool, namespace []byte, idx int) bool {
	v, kind := ExtractType(current)
	arr, ok := d.values[string(namespace)]

	if d.d.customTypeFuncs != nil {
		if ok {
			if cf, ok := d.d.customTypeFuncs[v.Type()]; ok {
				val, err := cf(arr[idx])
				if err != nil {
					d.setError(namespace, err)

					return false
				}

				v.Set(reflect.ValueOf(val))

				return true
			}
		}
	}

	if v.Type() == timeType {
		if !ok || len(arr[idx]) == 0 {
			return false
		}

		if len(arr[idx]) == 0 {
			return false
		}

		t, err := time.Parse(time.RFC3339, arr[idx])
		if err != nil {
			d.setError(namespace, err)

			return false
		}

		v.Set(reflect.ValueOf(t))

		return true
	}

	if ok {
		if tu, ok := current.Addr().Interface().(encoding.TextUnmarshaler); ok {
			if err := tu.UnmarshalText([]byte(arr[idx])); err != nil {
				d.setError(namespace, err)

				return false
			}

			return true
		}
	}

	switch kind {
	case reflect.Interface:
		if !ok || idx == len(arr) {
			return false
		}

		v.Set(reflect.ValueOf(arr[idx]))

		return true

	case reflect.Ptr:
		newVal := reflect.New(v.Type().Elem())
		if set := d.setFieldByType(newVal.Elem(), true, namespace, idx); set {
			v.Set(newVal)

			return set
		}

	case reflect.String:
		if !ok || idx == len(arr) {
			return false
		}

		v.SetString(arr[idx])

		return true

	case reflect.Uint, reflect.Uint64:
		if !ok || idx == len(arr) || len(arr[idx]) == 0 {
			return false
		}

		u64, err := strconv.ParseUint(arr[idx], 10, 64)
		if err != nil {
			d.setError(namespace, fmt.Errorf("invalid unsigned integer value '%s' type '%v' namespace '%s'",
				arr[idx], v.Type(), string(namespace)))

			return false
		}

		v.SetUint(u64)

		return true

	case reflect.Uint8:
		if !ok || idx == len(arr) || len(arr[idx]) == 0 {
			return false
		}

		u64, err := strconv.ParseUint(arr[idx], 10, 8)
		if err != nil {
			d.setError(namespace, fmt.Errorf("invalid unsigned integer value '%s' type '%v' namespace '%s'",
				arr[idx], v.Type(), string(namespace)))

			return false
		}

		v.SetUint(u64)

		return true

	case reflect.Uint16:
		if !ok || idx == len(arr) || len(arr[idx]) == 0 {
			return false
		}

		u64, err := strconv.ParseUint(arr[idx], 10, 16)
		if err != nil {
			d.setError(namespace, fmt.Errorf("invalid unsigned integer value '%s' type '%v' namespace '%s'",
				arr[idx], v.Type(), string(namespace)))

			return false
		}

		v.SetUint(u64)

		return true

	case reflect.Uint32:
		if !ok || idx == len(arr) || len(arr[idx]) == 0 {
			return false
		}

		u64, err := strconv.ParseUint(arr[idx], 10, 32)
		if err != nil {
			d.setError(namespace, fmt.Errorf("invalid unsigned integer value '%s' type '%v' namespace '%s'",
				arr[idx], v.Type(), string(namespace)))

			return false
		}

		v.SetUint(u64)

		return true

	case reflect.Int, reflect.Int64:
		if !ok || idx == len(arr) || len(arr[idx]) == 0 {
			return false
		}

		i64, err := strconv.ParseInt(arr[idx], 10, 64)
		if err != nil {
			d.setError(namespace, fmt.Errorf("invalid integer value '%s' type '%v' namespace '%s'",
				arr[idx], v.Type(), string(namespace)))

			return false
		}

		v.SetInt(i64)

		return true

	case reflect.Int8:
		if !ok || idx == len(arr) || len(arr[idx]) == 0 {
			return false
		}

		i64, err := strconv.ParseInt(arr[idx], 10, 8)
		if err != nil {
			d.setError(namespace, fmt.Errorf("invalid integer value '%s' type '%v' namespace '%s'",
				arr[idx], v.Type(), string(namespace)))

			return false
		}

		v.SetInt(i64)

		return true

	case reflect.Int16:
		if !ok || idx == len(arr) || len(arr[idx]) == 0 {
			return false
		}

		i64, err := strconv.ParseInt(arr[idx], 10, 16)
		if err != nil {
			d.setError(namespace, fmt.Errorf("invalid integer value '%s' type '%v' namespace '%s'",
				arr[idx], v.Type(), string(namespace)))

			return false
		}

		v.SetInt(i64)

		return true

	case reflect.Int32:
		if !ok || idx == len(arr) || len(arr[idx]) == 0 {
			return false
		}

		i64, err := strconv.ParseInt(arr[idx], 10, 32)
		if err != nil {
			d.setError(namespace, fmt.Errorf("invalid integer value '%s' type '%v' namespace '%s'",
				arr[idx], v.Type(), string(namespace)))

			return false
		}

		v.SetInt(i64)

		return true

	case reflect.Float32:
		if !ok || idx == len(arr) || len(arr[idx]) == 0 {
			return false
		}

		f, err := strconv.ParseFloat(arr[idx], 32)
		if err != nil {
			d.setError(namespace, fmt.Errorf("invalid float value '%s' type '%v' namespace '%s'",
				arr[idx], v.Type(), string(namespace)))

			return false
		}

		v.SetFloat(f)

		return true

	case reflect.Float64:
		if !ok || idx == len(arr) || len(arr[idx]) == 0 {
			return false
		}

		f, err := strconv.ParseFloat(arr[idx], 64)
		if err != nil {
			d.setError(namespace, fmt.Errorf("invalid float value '%s' type '%v' namespace '%s'",
				arr[idx], v.Type(), string(namespace)))

			return false
		}

		v.SetFloat(f)

		return true

	case reflect.Bool:
		if !ok || idx == len(arr) || (isPtr && arr[idx] == "") {
			return false
		}

		b, err := parseBool(arr[idx])
		if err != nil {
			d.setError(namespace, fmt.Errorf("invalid boolean value '%s' type '%v' namespace '%s'",
				arr[idx], v.Type(), string(namespace)))

			return false
		}

		v.SetBool(b)

		return true

	case reflect.Slice:
		// check arr, current
		if err := d.parseMapData(); err != nil {
			d.setError(namespace, fmt.Errorf("failed to parse map data: %w", err))

			return false
		}
		// slice elements could be mixed eg. number and non-numbers Value[0]=[]string{"10"} and Value=[]string{"10","20"}

		set := false

		if ok && len(arr) > 0 {
			var varr reflect.Value

			var ol int

			l := len(arr)

			if v.IsNil() {
				varr = reflect.MakeSlice(v.Type(), len(arr), len(arr))
			} else {
				ol = v.Len()
				l += ol

				if v.Cap() <= l {
					varr = reflect.MakeSlice(v.Type(), l, l)
				} else {
					// preserve predefined capacity, possibly for reuse after decoding
					varr = reflect.MakeSlice(v.Type(), l, v.Cap())
				}
				reflect.Copy(varr, v)
			}

			for i := ol; i < l; i++ {
				newVal := reflect.New(v.Type().Elem()).Elem()

				if d.setFieldByType(newVal, false, namespace, i-ol) {
					set = true

					varr.Index(i).Set(newVal)
				}
			}

			v.Set(varr)
		}

		// maybe it's an numbered array i.e. Phone[0].Number
		if rd := d.findAlias(string(namespace)); rd != nil {
			var (
				varr reflect.Value
				kv   key
			)

			sl := rd.sliceLen + 1

			// checking below for defaultMaxArraySize, but if array exists and already
			// has sufficient capacity allocated then we do not check as the code
			// obviously allows a capacity greater than the defaultMaxArraySize.

			switch {
			case v.IsNil():
				if sl > d.d.maxArraySize {
					d.setError(namespace, fmt.Errorf(errArraySize, sl, d.d.maxArraySize))

					return false
				}

				varr = reflect.MakeSlice(v.Type(), sl, sl)
			case v.Len() < sl:
				if v.Cap() <= sl {
					if sl > d.d.maxArraySize {
						d.setError(namespace, fmt.Errorf(errArraySize, sl, d.d.maxArraySize))

						return false
					}

					varr = reflect.MakeSlice(v.Type(), sl, sl)
				} else {
					varr = reflect.MakeSlice(v.Type(), sl, v.Cap())
				}

				reflect.Copy(varr, v)
			default:
				varr = v
			}

			for i := 0; i < len(rd.keys); i++ {
				kv = rd.keys[i]
				newVal := reflect.New(varr.Type().Elem()).Elem()

				if kv.ivalue == -1 {
					d.setError(namespace, fmt.Errorf("invalid slice index '%s'", kv.value))

					continue
				}

				if d.setFieldByType(newVal, false, append(namespace, kv.searchValue...), 0) {
					set = true

					varr.Index(kv.ivalue).Set(newVal)
				}
			}

			if !set {
				return false
			}

			v.Set(varr)

			return true
		}

		return set

	case reflect.Array:
		if err := d.parseMapData(); err != nil {
			d.setError(namespace, fmt.Errorf("failed to parse map data: %w", err))

			return false
		}

		// array elements could be mixed eg. number and non-numbers Value[0]=[]string{"10"} and Value=[]string{"10","20"}
		set := false

		if ok && len(arr) > 0 {
			var varr reflect.Value

			l := len(arr)
			overCapacity := v.Len() < l

			if overCapacity {
				// more values than array capacity, ignore values over capacity as it's possible some would just want
				// to grab the first x number of elements; in the future strict mode logic should return an error
				fmt.Println("warning number of post form array values is larger than array capacity, ignoring overflow values")
			}

			varr = reflect.Indirect(reflect.New(reflect.ArrayOf(v.Len(), v.Type().Elem())))
			reflect.Copy(varr, v)

			if v.Len() < len(arr) {
				l = v.Len()
			}

			for i := 0; i < l; i++ {
				newVal := reflect.New(v.Type().Elem()).Elem()

				if d.setFieldByType(newVal, false, namespace, i) {
					set = true

					varr.Index(i).Set(newVal)
				}
			}
			v.Set(varr)
		}

		// maybe it's an numbered array i.e. Phone[0].Number
		if rd := d.findAlias(string(namespace)); rd != nil {
			var (
				varr reflect.Value
				kv   key
			)

			overCapacity := rd.sliceLen >= v.Len()
			if overCapacity {
				// more values than array capacity, ignore values over capacity as it's possible some would just want
				// to grab the first x number of elements; in the future strict mode logic should return an error
				fmt.Println("warning number of post form array values is larger than array capacity, ignoring overflow values")
			}

			varr = reflect.Indirect(reflect.New(reflect.ArrayOf(v.Len(), v.Type().Elem())))
			reflect.Copy(varr, v)

			for i := 0; i < len(rd.keys); i++ {
				kv = rd.keys[i]
				if kv.ivalue >= v.Len() {
					continue
				}

				newVal := reflect.New(varr.Type().Elem()).Elem()

				if kv.ivalue == -1 {
					d.setError(namespace, fmt.Errorf("invalid array index '%s'", kv.value))

					continue
				}

				if d.setFieldByType(newVal, false, append(namespace, kv.searchValue...), 0) {
					set = true

					varr.Index(kv.ivalue).Set(newVal)
				}
			}

			if !set {
				return false
			}

			v.Set(varr)

			return true
		}

		return set

	case reflect.Map:
		var rd *recursiveData

		if err := d.parseMapData(); err != nil {
			d.setError(namespace, fmt.Errorf("failed to parse map data: %w", err))

			return false
		}

		// no natural map support so skip directly to dm lookup
		if rd = d.findAlias(string(namespace)); rd == nil {
			return false
		}

		var (
			existing bool
			kv       key
			mp       reflect.Value
			mk       reflect.Value
		)

		typ := v.Type()

		if v.IsNil() {
			mp = reflect.MakeMap(typ)
		} else {
			existing = true
			mp = v
		}

		set := false

		for i := 0; i < len(rd.keys); i++ {
			newVal := reflect.New(typ.Elem()).Elem()
			mk = reflect.New(typ.Key()).Elem()
			kv = rd.keys[i]

			if err := d.getMapKey(kv.value, mk, namespace); err != nil {
				d.setError(namespace, err)

				continue
			}

			if d.setFieldByType(newVal, false, append(namespace, kv.searchValue...), 0) {
				set = true

				mp.SetMapIndex(mk, newVal)
			}
		}

		if !set || existing {
			return false
		}

		v.Set(mp)

		return true

	case reflect.Struct:
		if err := d.parseMapData(); err != nil {
			d.setError(namespace, fmt.Errorf("failed to parse map data: %w", err))

			return false
		}

		// we must be recursing infinitely...but that's ok we caught it on the very first overrun.
		if len(namespace) > d.maxKeyLen {
			return false
		}

		return d.traverseStruct(v, v.Type(), namespace)
	}

	return false
}

func (d *decoder) getMapKey(key string, current reflect.Value, namespace []byte) (err error) {
	v, kind := ExtractType(current)

	if d.d.customTypeFuncs != nil {
		if cf, ok := d.d.customTypeFuncs[v.Type()]; ok {
			val, er := cf(key)
			if er != nil {
				err = er

				return
			}

			v.Set(reflect.ValueOf(val))

			return
		}
	}

	switch kind {
	case reflect.Interface:
		// If interface would have been set on the struct before decoding,
		// say to a struct value we would not get here but kind would be struct.
		v.Set(reflect.ValueOf(key))

		return
	case reflect.Ptr:
		newVal := reflect.New(v.Type().Elem())
		if err = d.getMapKey(key, newVal.Elem(), namespace); err == nil {
			v.Set(newVal)
		}

	case reflect.String:
		v.SetString(key)

	case reflect.Uint, reflect.Uint64:
		u64, e := strconv.ParseUint(key, 10, 64)
		if e != nil {
			return fmt.Errorf("invalid unsigned integer value '%s' type '%v' namespace '%s'", key, v.Type(), string(namespace))
		}

		v.SetUint(u64)

	case reflect.Uint8:
		u64, e := strconv.ParseUint(key, 10, 8)
		if e != nil {
			return fmt.Errorf("invalid unsigned integer value '%s' type '%v' namespace '%s'", key, v.Type(), string(namespace))
		}

		v.SetUint(u64)

	case reflect.Uint16:
		u64, e := strconv.ParseUint(key, 10, 16)
		if e != nil {
			return fmt.Errorf("invalid unsigned integer value '%s' type '%v' namespace '%s'", key, v.Type(), string(namespace))
		}

		v.SetUint(u64)

	case reflect.Uint32:
		u64, e := strconv.ParseUint(key, 10, 32)
		if e != nil {
			return fmt.Errorf("invalid unsigned integer value '%s' type '%v' namespace '%s'", key, v.Type(), string(namespace))
		}

		v.SetUint(u64)

	case reflect.Int, reflect.Int64:
		i64, e := strconv.ParseInt(key, 10, 64)
		if e != nil {
			return fmt.Errorf("invalid integer value '%s' type '%v' namespace '%s'", key, v.Type(), string(namespace))
		}

		v.SetInt(i64)

	case reflect.Int8:
		i64, e := strconv.ParseInt(key, 10, 8)
		if e != nil {
			return fmt.Errorf("invalid integer value '%s' type '%v' namespace '%s'", key, v.Type(), string(namespace))
		}

		v.SetInt(i64)

	case reflect.Int16:
		i64, e := strconv.ParseInt(key, 10, 16)
		if e != nil {
			return fmt.Errorf("invalid integer value '%s' type '%v' namespace '%s'", key, v.Type(), string(namespace))
		}

		v.SetInt(i64)

	case reflect.Int32:
		i64, e := strconv.ParseInt(key, 10, 32)
		if e != nil {
			return fmt.Errorf("invalid integer value '%s' type '%v' namespace '%s'", key, v.Type(), string(namespace))
		}

		v.SetInt(i64)

	case reflect.Float32:
		f, e := strconv.ParseFloat(key, 32)
		if e != nil {
			return fmt.Errorf("invalid float value '%s' type '%v' namespace '%s'", key, v.Type(), string(namespace))
		}

		v.SetFloat(f)

	case reflect.Float64:
		f, e := strconv.ParseFloat(key, 64)
		if e != nil {
			return fmt.Errorf("invalid float value '%s' type '%v' namespace '%s'", key, v.Type(), string(namespace))
		}

		v.SetFloat(f)

	case reflect.Bool:
		b, e := parseBool(key)
		if e != nil {
			return fmt.Errorf("invalid boolean value '%s' type '%v' namespace '%s'", key, v.Type(), string(namespace))
		}

		v.SetBool(b)

	default:
		return fmt.Errorf("unsupported map key '%s' type '%v' namespace '%s'", key, v.Type(), string(namespace))
	}

	return nil
}

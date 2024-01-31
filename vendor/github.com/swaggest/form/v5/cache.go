package form

import (
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

var _ sort.Interface = cacheFields{}

type cacheFields []cachedField

func (s cacheFields) Len() int {
	return len(s)
}

func (s cacheFields) Less(i, j int) bool {
	return !s[i].isAnonymous
}

func (s cacheFields) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

type cachedField struct {
	idx               int
	name              string
	isAnonymous       bool
	isOmitEmpty       bool
	isExported        bool
	sliceSeparator    byte
	hasExportedScalar bool
	canSet            bool
}

type cachedStruct struct {
	hasExportedScalar bool
	fields            cacheFields
}

type structCacheMap struct {
	m     atomic.Value // map[reflect.Type]*cachedStruct
	lock  sync.Mutex
	tagFn TagNameFunc
}

// TagNameFunc allows for adding of a custom tag name parser.
type TagNameFunc func(field reflect.StructField) string

func newStructCacheMap() *structCacheMap {
	sc := new(structCacheMap)
	sc.m.Store(make(map[reflect.Type]*cachedStruct))

	return sc
}

func (s *structCacheMap) Get(key reflect.Type) (value *cachedStruct, ok bool) {
	value, ok = s.m.Load().(map[reflect.Type]*cachedStruct)[key]

	return
}

func (s *structCacheMap) Set(key reflect.Type, value *cachedStruct) {
	m := s.m.Load().(map[reflect.Type]*cachedStruct) //nolint:errcheck

	nm := make(map[reflect.Type]*cachedStruct, len(m)+1)

	for k, v := range m {
		nm[k] = v
	}

	nm[key] = value

	s.m.Store(nm)
}

func (s *structCacheMap) parseStruct(mode Mode, typ reflect.Type, tagName string) *cachedStruct {
	s.lock.Lock()
	defer s.lock.Unlock()

	return s.ps(mode, typ, tagName)
}

func (s *structCacheMap) ps(mode Mode, typ reflect.Type, tagName string) (cs *cachedStruct) {
	// could have been multiple trying to access, but once first is done this ensures struct
	// isn't parsed again.
	cs, ok := s.Get(typ)
	if ok {
		return cs
	}

	cs = &cachedStruct{}
	defer s.Set(typ, cs)

	if typ.Kind() == reflect.Ptr {
		s := s.ps(mode, typ.Elem(), tagName)
		*cs = *s

		return cs
	}

	if typ.Kind() == reflect.Interface {
		cs.hasExportedScalar = true

		return cs
	}

	if typ.Kind() != reflect.Struct {
		return cs
	}

	numFields := typ.NumField()

	var (
		fld            reflect.StructField
		name           string
		idx            int
		isOmitEmpty    bool
		sliceSeparator byte
	)

	hasExportedScalar := false

	for i := 0; i < numFields; i++ {
		isOmitEmpty = false
		sliceSeparator = 0
		fld = typ.Field(i)

		if fld.PkgPath != blank && !fld.Anonymous {
			continue
		}

		if s.tagFn != nil {
			name = s.tagFn(fld)
		} else {
			name = fld.Tag.Get(tagName)
		}

		if name == ignore {
			continue
		}

		if mode == ModeExplicit && len(name) == 0 && !fld.Anonymous {
			continue
		}

		// check for omitempty
		if idx = strings.LastIndexByte(name, ','); idx != -1 {
			isOmitEmpty = name[idx+1:] == "omitempty"
			name = name[:idx]
		}

		// add support for OAS Swagger 2.0 collectionFormat
		// https://github.com/OAI/OpenAPI-Specification/blob/master/schemas/v2.0/schema.json#L1528
		if cf := fld.Tag.Get("collectionFormat"); cf != "" {
			switch cf {
			case "csv":
				sliceSeparator = ','
			case "tsv":
				sliceSeparator = '\t'
			case "ssv":
				sliceSeparator = ' '
			case "pipes":
				sliceSeparator = '|'
			}
		}

		if len(name) == 0 {
			name = fld.Name
		}

		cf := cachedField{}
		cf.idx = i
		cf.name = name
		cf.isAnonymous = fld.Anonymous
		cf.isExported = fld.PkgPath == ""
		cf.isOmitEmpty = isOmitEmpty
		cf.sliceSeparator = sliceSeparator
		cf.canSet = true

		if fld.Type.Kind() == reflect.Interface && fld.Type.NumMethod() > 0 {
			cf.canSet = false
		}

		if cf.isAnonymous && !cf.isExported && fld.Type.Kind() == reflect.Ptr {
			cf.canSet = false
		}

		if cf.isAnonymous && !cf.hasExportedScalar {
			cs := s.ps(mode, fld.Type, tagName)
			if cs.hasExportedScalar {
				cf.hasExportedScalar = true
			}
		}

		if (len(name) > 0 && cf.isExported && !cf.isAnonymous) || cf.hasExportedScalar {
			hasExportedScalar = true
		}

		cs.fields = append(cs.fields, cf)
	}

	cs.hasExportedScalar = hasExportedScalar

	return cs
}

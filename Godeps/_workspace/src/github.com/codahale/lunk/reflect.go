package lunk

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

func flattenValue(prefix string, v reflect.Value, f func(k, v string)) {
	switch o := v.Interface().(type) {
	case time.Time:
		f(prefix, o.Format(time.RFC3339Nano))
		return
	case time.Duration:
		ms := float64(o.Nanoseconds()) / 1e6
		f(prefix, strconv.FormatFloat(ms, 'f', -1, 64))
		return
	case fmt.Stringer:
		f(prefix, o.String())
		return
	}

	switch v.Kind() {
	case reflect.Ptr:
		flattenValue(prefix, v.Elem(), f)
	case reflect.Bool:
		f(prefix, strconv.FormatBool(v.Bool()))
	case reflect.Float32, reflect.Float64:
		f(prefix, strconv.FormatFloat(v.Float(), 'f', -1, 64))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		f(prefix, strconv.FormatInt(v.Int(), 10))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		f(prefix, strconv.FormatUint(v.Uint(), 10))
	case reflect.String:
		f(prefix, v.String())
	case reflect.Struct:
		for i, name := range fieldNames(v) {
			flattenValue(nest(prefix, name), v.Field(i), f)
		}
	case reflect.Map:
		for _, key := range v.MapKeys() {
			// small bit of cuteness here: use flattenValue on the key first,
			// then on the value
			flattenValue("", key, func(_, k string) {
				flattenValue(nest(prefix, k), v.MapIndex(key), f)
			})
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			flattenValue(nest(prefix, strconv.Itoa(i)), v.Index(i), f)
		}
	default:
		f(prefix, fmt.Sprintf("%+v", v.Interface()))
	}
}

func fieldNames(v reflect.Value) map[int]string {
	t := v.Type()

	// check to see if a cached set exists
	cachedFieldNamesRW.RLock()
	m, ok := cachedFieldNames[t]
	cachedFieldNamesRW.RUnlock()

	if ok {
		return m
	}

	// otherwise, create it and return it
	cachedFieldNamesRW.Lock()
	m = make(map[int]string, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		fld := t.Field(i)
		if fld.PkgPath != "" {
			continue // ignore all unexpected fields
		}

		name := fld.Tag.Get("lunk")
		if name == "" {
			name = strings.ToLower(fld.Name)
		}
		m[i] = name
	}
	cachedFieldNames[t] = m
	cachedFieldNamesRW.Unlock()
	return m
}

var (
	cachedFieldNames   = make(map[reflect.Type]map[int]string, 20)
	cachedFieldNamesRW = new(sync.RWMutex)
)

func nest(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

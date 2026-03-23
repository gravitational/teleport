/*
Copyright 2025 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package structfill

import (
	"reflect"
	"time"

	"github.com/gravitational/trace"
)

// Fill sets non-default values for all fields in the value pointed to by ptr.
// It returns an error if it encounters a field it cannot set.
// NOTE: Use only it tests.
func Fill(ptr any) error {
	if ptr == nil {
		return trace.BadParameter("missing pointer")
	}
	root := reflect.ValueOf(ptr)
	if root.Kind() != reflect.Ptr || root.IsNil() {
		return trace.BadParameter("must pass a non-nil pointer")
	}
	return trace.Wrap(fill(root.Elem()))
}

func fill(v reflect.Value) error {
	if !v.CanSet() {
		return trace.BadParameter("cannot set value of type %v", v.Type())
	}

	switch v.Kind() {
	case reflect.Bool:
		v.SetBool(true)
		return nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if v.Type().PkgPath() == "time" && v.Type().Name() == "Duration" {
			v.SetInt(int64(time.Second))
		} else {
			v.SetInt(1)
		}
		return nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		v.SetUint(1)
		return nil

	case reflect.Float32, reflect.Float64:
		v.SetFloat(1.1)
		return nil

	case reflect.Complex64, reflect.Complex128:
		v.SetComplex(complex(1, 1))
		return nil

	case reflect.String:
		v.SetString("nonzero")
		return nil

	case reflect.Slice:
		s := reflect.MakeSlice(v.Type(), 1, 1)
		if err := fill(s.Index(0)); err != nil {
			return trace.Wrap(err)
		}
		v.Set(s)
		return nil

	case reflect.Array:
		for i := 0; i < v.Len(); i++ {
			if err := fill(v.Index(i)); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil

	case reflect.Map:
		kt, vt := v.Type().Key(), v.Type().Elem()
		m := reflect.MakeMapWithSize(v.Type(), 1)

		k := reflect.New(kt).Elem()
		if err := fill(k); err != nil {
			return trace.Wrap(err)
		}

		val := reflect.New(vt).Elem()
		if err := fill(val); err != nil {
			return trace.Wrap(err)
		}

		m.SetMapIndex(k, val)
		v.Set(m)
		return nil

	case reflect.Ptr:
		elem := reflect.New(v.Type().Elem())
		if err := fill(elem.Elem()); err != nil {
			return trace.Wrap(err)
		}
		v.Set(elem)
		return nil

	case reflect.Struct:
		if v.Type().PkgPath() == "time" && v.Type().Name() == "Time" {
			v.Set(reflect.ValueOf(time.Now()))
			return nil
		}
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			sf := t.Field(i)
			f := v.Field(i)

			if !f.CanSet() {
				return trace.BadParameter(
					"cannot set field %q of type %v (unexported or from another package)",
					sf.Name, sf.Type,
				)
			}
			if err := fill(f); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	default:
		// For unsupported kinds (func, chan, unsafe pointers, interfaces)
		return trace.BadParameter("unsupported kind %v", v.Kind())
	}
}

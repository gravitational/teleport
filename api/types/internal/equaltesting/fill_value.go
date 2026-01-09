// Copyright 2026 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.package equaltesting

package equaltesting

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"
)

type fillValueOptions struct {
	ignoreUnexported   bool
	skipProtoXXXFields bool
	timeVal            time.Time
}

type FillValueOpt func(*fillValueOptions)

func WithIgnoreUnexported(ignoreUnexported bool) FillValueOpt {
	return func(o *fillValueOptions) {
		o.ignoreUnexported = ignoreUnexported
	}
}

func WithSkipProtoXXXFields(skipProtoXXXFields bool) FillValueOpt {
	return func(o *fillValueOptions) {
		o.skipProtoXXXFields = skipProtoXXXFields
	}
}

func WithTimeVal(t time.Time) FillValueOpt {
	return func(o *fillValueOptions) {
		o.timeVal = t
	}
}

// FillValue fills the given value with a simple non-zero value.
func FillValue(i any, opts ...FillValueOpt) error {
	v := reflect.ValueOf(i)
	if v.Kind() != reflect.Pointer {
		return fmt.Errorf("expected a pointer, got %q", v.Kind())
	}

	opt := fillValueOptions{
		timeVal: time.Now(),
	}
	for _, o := range opts {
		o(&opt)
	}

	return fillValue(v.Elem(), opt)
}

func fillValue(v reflect.Value, opt fillValueOptions) error {
	if !v.CanSet() {
		if opt.ignoreUnexported {
			return nil
		}
		return errors.New("value is not addressable or is an unexported struct field")
	}

	// Handle pointers.
	if v.Kind() == reflect.Pointer {
		v.Set(reflect.New(v.Type().Elem()))
		return fillValue(v.Elem(), opt)
	}

	// Special handling for time.Time.
	if v.Type() == reflect.TypeOf(time.Time{}) {
		v.Set(reflect.ValueOf(opt.timeVal))
		return nil
	}

	switch kind := v.Kind(); kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(1)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		v.SetUint(1)
	case reflect.Float32, reflect.Float64:
		v.SetFloat(1.0)
	case reflect.Complex64, reflect.Complex128:
		v.SetComplex(1.0 + 1i)
	case reflect.Bool:
		v.SetBool(true)
	case reflect.String:
		v.SetString("SetA")

	case reflect.Struct:
		t := v.Type()
		for i := range v.NumField() {
			if opt.skipProtoXXXFields && strings.HasPrefix(t.Field(i).Name, "XXX_") {
				continue
			}
			if err := fillValue(v.Field(i), opt); err != nil {
				return fmt.Errorf("field %q: %w", v.Type().Field(i).Name, err)
			}
		}

	case reflect.Slice:
		slice := reflect.MakeSlice(v.Type(), 1, 1)
		if err := fillValue(slice.Index(0), opt); err != nil {
			return err
		}
		v.Set(slice)

	case reflect.Map:
		mapT := v.Type()
		mapV := reflect.MakeMap(mapT)

		key := reflect.New(mapT.Key()).Elem()
		if err := fillValue(key, opt); err != nil {
			return fmt.Errorf("map key: %w", err)
		}
		val := reflect.New(mapT.Elem()).Elem()
		if err := fillValue(val, opt); err != nil {
			return fmt.Errorf("map value: %w", err)
		}

		mapV.SetMapIndex(key, val)
		v.Set(mapV)
	default:
		return fmt.Errorf("unsupported kind %q", kind)
	}

	return nil
}

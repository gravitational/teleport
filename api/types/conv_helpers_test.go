// Copyright 2025 Gravitational, Inc.
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
// limitations under the License.

package types

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"reflect"
	"time"
)

type fillValueOptions struct {
	ignoreUnexported bool
	timeVal          time.Time

	rand                 *rand.Rand
	randomlySkipEveryNth int
}

type FillValueOpt func(*fillValueOptions)

func WithIgnoreUnexported(skip bool) FillValueOpt {
	return func(o *fillValueOptions) {
		o.ignoreUnexported = skip
	}
}

func WithTimeVal(t time.Time) FillValueOpt {
	return func(o *fillValueOptions) {
		o.timeVal = t
	}
}

func WithSkipFieldsRandomly(seed uint64, everyNthField int) FillValueOpt {
	return func(o *fillValueOptions) {
		if everyNthField <= 0 {
			panic("everyNthField has to be > 0")
		}
		o.rand = rand.New(rand.NewPCG(seed, seed))
		o.randomlySkipEveryNth = everyNthField
	}
}

// FillValue fills the given value with a simple non-zero value.
func FillValue(i any, opts ...FillValueOpt) error {
	v := reflect.ValueOf(i)
	if v.Kind() != reflect.Pointer {
		return fmt.Errorf("expected a pointer, got %q", v.Kind())
	}

	options := fillValueOptions{
		timeVal: time.Now(),
	}
	for _, o := range opts {
		o(&options)
	}

	return fillValue(v.Elem(), &options)
}

func fillValue(v reflect.Value, options *fillValueOptions) error {
	if !v.CanSet() {
		if options.ignoreUnexported {
			return nil
		}
		return errors.New("value is not addressable or is an unexported struct field")
	}

	if options.rand != nil {
		if options.rand.IntN(options.randomlySkipEveryNth) == 0 {
			return nil
		}
	}

	// Handle pointers.
	if v.Kind() == reflect.Pointer {
		v.Set(reflect.New(v.Type().Elem()))
		return fillValue(v.Elem(), options)
	}

	// Special handling for time.Time.
	if v.Type() == reflect.TypeOf(time.Time{}) {
		v.Set(reflect.ValueOf(options.timeVal))
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
		for i := range v.NumField() {
			if err := fillValue(v.Field(i), options); err != nil {
				return fmt.Errorf("field %q: %w", v.Type().Field(i).Name, err)
			}
		}

	case reflect.Slice:
		slice := reflect.MakeSlice(v.Type(), 1, 1)
		if err := fillValue(slice.Index(0), options); err != nil {
			return err
		}
		v.Set(slice)

	case reflect.Map:
		mapT := v.Type()
		mapV := reflect.MakeMap(mapT)

		key := reflect.New(mapT.Key()).Elem()
		if err := fillValue(key, options); err != nil {
			return fmt.Errorf("map key: %w", err)
		}
		val := reflect.New(mapT.Elem()).Elem()
		if err := fillValue(val, options); err != nil {
			return fmt.Errorf("map value: %w", err)
		}

		mapV.SetMapIndex(key, val)
		v.Set(mapV)
	default:
		return fmt.Errorf("unsupported kind %q", kind)
	}

	return nil
}

/*
Copyright 2015 Gravitational, Inc.

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
package configure

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/gravitational/configure/cstrings"
	"github.com/gravitational/trace"
	"gopkg.in/alecthomas/kingpin.v2"
)

// ParseCommandLine takes a pointer to a function and attempts
// to initialize it from environment variables.
func ParseCommandLine(v interface{}, args []string) error {
	app, err := NewCommandLineApp(v)
	if err != nil {
		return trace.Wrap(err)
	}
	if _, err := app.Parse(args); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// NewCommandLineApp generates a command line parsing tool based on the struct
// that was passed in as a parameter
func NewCommandLineApp(v interface{}) (*kingpin.Application, error) {
	s := reflect.ValueOf(v).Elem()
	app := kingpin.New("app", "Auto generated command line application")
	if err := setupApp(app, s); err != nil {
		return nil, trace.Wrap(err)
	}
	return app, nil
}

// CLISetter is an interface for setting any value from command line flag
type CLISetter interface {
	SetCLI(string) error
}

func setupApp(app *kingpin.Application, v reflect.Value) error {
	// for structs, walk every element and parse
	vType := v.Type()
	if v.Kind() != reflect.Struct {
		return nil
	}
	for i := 0; i < v.NumField(); i++ {
		structField := vType.Field(i)
		field := v.Field(i)
		if !field.CanSet() {
			continue
		}
		kind := field.Kind()
		if kind == reflect.Struct {
			if err := setupApp(app, field); err != nil {
				return trace.Wrap(err,
					fmt.Sprintf("failed parsing struct field %v",
						structField.Name))
			}
		}
		cliFlag := structField.Tag.Get("cli")
		if cliFlag == "" {
			continue
		}
		if !field.CanAddr() {
			continue
		}
		f := app.Flag(cliFlag, cliFlag)
		fieldPtr := field.Addr().Interface()
		if setter, ok := fieldPtr.(CLISetter); ok {
			f.SetValue(&cliValue{setter: setter})
			continue
		}
		if setter, ok := fieldPtr.(StringSetter); ok {
			f.SetValue(&cliValue{setter: &cliStringSetter{setter: setter}})
			continue
		}
		switch ptr := fieldPtr.(type) {
		case *map[string]string:
			f.SetValue(&cliMapValue{v: ptr})
			continue
		case *[]map[string]string:
			f.SetValue(&cliSliceMapValue{v: ptr})
			continue
		case *int:
			f.SetValue(&cliIntValue{v: ptr})
			continue
		case *int32:
			f.SetValue(&cliInt32Value{v: ptr})
			continue
		case *int64:
			f.SetValue(&cliInt64Value{v: ptr})
			continue
		case *string:
			f.SetValue(&cliStringValue{v: ptr})
		case *bool:
			f.SetValue(&cliBoolValue{v: ptr})
		default:
			return trace.Errorf("unsupported type: %T", ptr)
		}
	}
	return nil
}

type cliStringSetter struct {
	setter StringSetter
}

func (s *cliStringSetter) SetCLI(v string) error {
	return s.setter.Set(v)
}

type cliValue struct {
	setter CLISetter
}

func (c *cliValue) String() string {
	return ""
}

func (c *cliValue) Set(v string) error {
	return c.setter.SetCLI(v)
}

type cliMapValue struct {
	v *map[string]string
}

func (c *cliMapValue) IsCumulative() bool {
	return true
}

func (c *cliMapValue) String() string {
	return ""
}

func (c *cliMapValue) Set(v string) error {
	return setMap(c.v, v)
}

func setMap(kv *map[string]string, val string) error {
	if len(*kv) == 0 {
		*kv = make(map[string]string)
	}
	for _, i := range cstrings.SplitComma(val) {
		vals := strings.SplitN(i, ":", 2)
		if len(vals) != 2 {
			return trace.Errorf("extra options should be defined like KEY:VAL")
		}
		(*kv)[vals[0]] = vals[1]
	}
	return nil
}

type cliSliceMapValue struct {
	v *[]map[string]string
}

func (c *cliSliceMapValue) String() string {
	return ""
}

func (c *cliSliceMapValue) IsCumulative() bool {
	return true
}

func (c *cliSliceMapValue) Set(v string) error {
	if len(*c.v) == 0 {
		(*c.v) = make([]map[string]string, 0)
	}
	var kv map[string]string
	if err := setMap(&kv, v); err != nil {
		return trace.Wrap(err)
	}
	*c.v = append(*c.v, kv)
	return nil
}

type cliStringValue struct {
	v *string
}

func (c *cliStringValue) String() string {
	return *c.v
}

func (c *cliStringValue) Set(v string) error {
	*c.v = v
	return nil
}

type cliIntValue struct {
	v *int
}

func (c *cliIntValue) String() string {
	return fmt.Sprintf("%v", *c.v)
}

func (c *cliIntValue) Set(v string) error {
	intValue, err := strconv.ParseInt(v, 0, 0)
	if err != nil {
		return trace.Wrap(err)
	}
	*c.v = int(intValue)
	return nil
}

type cliInt64Value struct {
	v    *int64
	bits int
}

func (c *cliInt64Value) String() string {
	return fmt.Sprintf("%v", *c.v)
}

func (c *cliInt64Value) Set(v string) error {
	intValue, err := strconv.ParseInt(v, 0, 64)
	if err != nil {
		return trace.Wrap(err)
	}
	*c.v = intValue
	return nil
}

type cliInt32Value struct {
	v    *int32
	bits int
}

func (c *cliInt32Value) String() string {
	return fmt.Sprintf("%v", *c.v)
}

func (c *cliInt32Value) Set(v string) error {
	intValue, err := strconv.ParseInt(v, 0, 32)
	if err != nil {
		return trace.Wrap(err)
	}
	*c.v = int32(intValue)
	return nil
}

type cliBoolValue struct {
	v *bool
}

func (c *cliBoolValue) String() string {
	return fmt.Sprintf("%v", *c.v)
}

func (c *cliBoolValue) Set(v string) error {
	boolVal, err := strconv.ParseBool(v)
	if err != nil {
		return trace.Wrap(err)
	}
	*c.v = boolVal
	return nil
}

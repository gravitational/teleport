package service

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
)

func ParseEnv(v interface{}) error {
	env, err := parseEnvironment()
	if err != nil {
		return err
	}
	s := reflect.ValueOf(v).Elem()
	return setEnv(s, env)
}

type Setter interface {
	Set(string) error
}

func setEnv(v reflect.Value, env map[string]string) error {
	// for structs, walk every element and parse
	vType := v.Type()
	if v.Kind() == reflect.Struct {
		for i := 0; i < v.NumField(); i++ {
			structField := vType.Field(i)
			field := v.Field(i)
			if !field.CanSet() {
				continue
			}
			kind := field.Kind()
			if kind == reflect.Struct {
				if err := setEnv(field, env); err != nil {
					return trace.Wrap(err, fmt.Sprintf("failed parsing struct field %v", structField.Name))
				}
			}
			envKey := structField.Tag.Get("env")

			if envKey == "" {
				continue
			}
			val, ok := env[envKey]
			if !ok || val == "" { // assume defaults
				continue
			}
			if field.CanAddr() {
				if s, ok := field.Addr().Interface().(Setter); ok {
					if err := s.Set(val); err != nil {
						return trace.Wrap(err)
					}
					continue
				}
			}
			switch kind {
			case reflect.String:
				field.SetString(val)
			case reflect.Bool:
				boolVal, err := strconv.ParseBool(val)
				if err != nil {
					return trace.Wrap(err, fmt.Sprintf("failed parsing struct field %v, expected bool, got '%v'", structField.Name, val))
				}
				field.SetBool(boolVal)
			}
		}
		return nil
	}
	return nil
}

func parseEnvironment() (map[string]string, error) {
	values := os.Environ()
	env := make(map[string]string, len(values))
	for _, v := range values {
		vals := strings.SplitN(v, "=", 2)
		if len(vals) != 2 {
			return nil, trace.Errorf("failed to parse variable: '%v'", v)
		}
		env[vals[0]] = vals[1]
	}
	return env, nil
}

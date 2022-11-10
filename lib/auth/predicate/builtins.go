/*
Copyright 2022 Gravitational, Inc.

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

package predicate

import (
	"reflect"
	"regexp"
	"strings"

	"github.com/gravitational/trace"
)

func getIdentifier(obj any, selectors []string) (any, error) {
	switch selectors[0] {
	case "false":
		return false, nil
	case "true":
		return true, nil
	}

	for _, s := range selectors {
		if obj == nil || reflect.ValueOf(obj).IsNil() {
			return nil, trace.BadParameter("cannot take field of nil")
		}

		if m, ok := obj.(map[string]any); ok {
			obj = m[s]
			continue
		}

		val := reflect.ValueOf(obj)
		ty := reflect.TypeOf(obj)
		if ty.Kind() == reflect.Interface || ty.Kind() == reflect.Ptr {
			val = reflect.ValueOf(obj).Elem()
			ty = val.Type()
		}

		if ty.Kind() == reflect.Struct {
			for i := 0; i < ty.NumField(); i++ {
				tagValue := ty.Field(i).Tag.Get("json")
				parts := strings.Split(tagValue, ",")
				if parts[0] == s {
					obj = val.Field(i).Interface()
					break
				}
			}

			continue
		}

		return nil, trace.BadParameter("cannot take field of type: %T", obj)
	}

	return obj, nil
}

func getProperty(m any, k any) (any, error) {
	switch mT := m.(type) {
	case map[string]any:
		kS, ok := k.(string)
		if !ok {
			return nil, trace.BadParameter("unsupported key type: %T", k)
		}

		return mT[kS], nil
	default:
		return nil, trace.BadParameter("cannot take property of type: %T", m)
	}
}

func cloneSlice[T any](in []T) []T {
	out := make([]T, len(in))
	copy(out, in)
	return out
}

func cloneMap[K comparable, V any](in map[K]V) map[K]V {
	out := make(map[K]V, len(in))
	for k, v := range in {
		out[k] = v
	}

	return out
}

func builtinOpAnd(a, b bool) any {
	return a && b
}

func builtinOpOr(a, b bool) bool {
	return a || b
}

func builtinOpNot(a bool) bool {
	return !a
}

func builtinOpEquals(a, b any) bool {
	return reflect.DeepEqual(a, b)
}

func builtinOpLT(a, b any) (bool, error) {
	if reflect.TypeOf(a) != reflect.TypeOf(b) {
		return false, trace.BadParameter(`args to "<" of types %T and %T do not match`, a, b)
	}

	switch aT := a.(type) {
	case string:
		return aT < b.(string), nil
	case int:
		return aT < b.(int), nil
	case float32:
		return aT < b.(float32), nil
	default:
		return false, trace.BadParameter(`args to "<" must be either string, int or float32, got %T`, a)
	}
}

func builtinOpGT(a, b any) (bool, error) {
	return builtinOpLT(b, a)
}

func builtinOpLE(a, b any) (bool, error) {
	if reflect.TypeOf(a) != reflect.TypeOf(b) {
		return false, trace.BadParameter(`args to "<=" of types %T and %T do not match`, a, b)
	}

	switch aT := a.(type) {
	case string:
		return aT <= b.(string), nil
	case int:
		return aT <= b.(int), nil
	case float32:
		return aT <= b.(float32), nil
	default:
		return false, trace.BadParameter(`args to "<=" must be either string, int or float32, got %T`, a)
	}
}

func builtinOpGE(a, b any) (bool, error) {
	return builtinOpLE(b, a)
}

func builtinAdd(a, b any) (any, error) {
	if reflect.TypeOf(a) != reflect.TypeOf(b) {
		return nil, trace.BadParameter("cannot add types: %T and %T", a, b)
	}

	switch aT := a.(type) {
	case string:
		return aT + b.(string), nil
	case int:
		return aT + b.(int), nil
	case float32:
		return aT + b.(float32), nil
	default:
		return nil, trace.BadParameter("add unsupported for type type: %T, must be string, int or float", a)
	}
}

func builtinSub(a, b any) (any, error) {
	if reflect.TypeOf(a) != reflect.TypeOf(b) {
		return nil, trace.BadParameter("cannot sub types: %T and %T", a, b)
	}

	switch aT := a.(type) {
	case int:
		return aT - b.(int), nil
	case float32:
		return aT - b.(float32), nil
	default:
		return nil, trace.BadParameter("sub unsupported for type type: %T, must be int or float", a)
	}
}

func builtinMul(a, b any) (any, error) {
	if reflect.TypeOf(a) != reflect.TypeOf(b) {
		return nil, trace.BadParameter("cannot mul types: %T and %T", a, b)
	}

	switch aT := a.(type) {
	case int:
		return aT * b.(int), nil
	case float32:
		return aT * b.(float32), nil
	default:
		return nil, trace.BadParameter("mul unsupported for type type: %T, must be int or float", a)
	}
}

func builtinDiv(a, b any) (any, error) {
	if reflect.TypeOf(a) != reflect.TypeOf(b) {
		return nil, trace.BadParameter("cannot div types: %T and %T", a, b)
	}

	switch aT := a.(type) {
	case int:
		return aT / b.(int), nil
	case float32:
		return aT / b.(float32), nil
	default:
		return nil, trace.BadParameter("div unsupported for type type: %T, must be int or float", a)
	}
}

func builtinXor(a, b any) (any, error) {
	if reflect.TypeOf(a) != reflect.TypeOf(b) {
		return nil, trace.BadParameter("cannot xor types: %T and %T", a, b)
	}

	switch aT := a.(type) {
	case int:
		return aT ^ b.(int), nil
	case bool:
		return aT != b.(bool), nil
	default:
		return nil, trace.BadParameter("xor unsupported for type type: %T, must be int or bool", a)
	}
}

func builtinSplit(a, b string) (any, error) {
	return strings.Split(a, b), nil
}

func builtinUpper(a string) (any, error) {
	return strings.ToUpper(a), nil
}

func builtinLower(a string) (any, error) {
	return strings.ToLower(a), nil
}

func builtinContains(a any, b string) (any, error) {
	switch aT := a.(type) {
	case string:
		return strings.Contains(aT, b), nil
	case []string:
		for _, s := range aT {
			if s == b {
				return true, nil
			}
		}

		return false, nil
	default:
		return nil, trace.BadParameter("contains not valid for type: %T, must be string or []string", a)
	}
}

func builtinFirst(a []string) any {
	if len(a) == 0 {
		return ""
	}

	return a[0]
}

func builtinAppend(a []string, b string) (any, error) {
	return append(cloneSlice(a), b), nil
}

func builtinArray(elements ...any) (any, error) {
	arr := make([]string, len(elements))
	for i, e := range elements {
		s, ok := e.(string)
		if !ok {
			return nil, trace.BadParameter("cannot create array with element type %T, must be string", e)
		}

		arr[i] = s
	}

	return arr, nil
}

func builtinReplace(in any, match, with string) (any, error) {
	switch inT := in.(type) {
	case string:
		return strings.Replace(inT, match, with, -1), nil
	case []string:
		for i, s := range inT {
			if s == match {
				inT[i] = with
			}
		}

		return inT, nil
	default:
		return nil, trace.BadParameter("replace not valid for type: %T, must be string or []string", in)
	}
}

func builtinLen(a any) (any, error) {
	switch aT := a.(type) {
	case string:
		return len(aT), nil
	case []string:
		return len(aT), nil
	default:
		return nil, trace.BadParameter("len not valid for type: %T, must be string or []string", a)
	}
}

func builtinRegex(a string) (any, error) {
	return regexp.Compile(a)
}

func builtinMatches(to *regexp.Regexp, against string) any {
	return to.MatchString(against)
}

func builtinContainsRegex(arr []string, regex *regexp.Regexp) (any, error) {
	for _, s := range arr {
		if regex.MatchString(s) {
			return true, nil
		}
	}

	return false, nil
}

// type-generic for future extensibility
func builtinMapInsert(m, k, v any) (any, error) {
	mT, ok := m.(map[string]any)
	if !ok {
		return nil, trace.BadParameter("cannot insert into map of type: %T, must be map[string]any", m)
	}

	kS, ok := k.(string)
	if !ok {
		return nil, trace.BadParameter("cannot use string key of type: %T, must be string", k)
	}

	vS, ok := v.(string)
	if !ok {
		return nil, trace.BadParameter("cannot use string value of type: %T, must be string", k)
	}

	newMap := cloneMap(mT)
	newMap[kS] = vS
	return newMap, nil
}

// type-generic for future extensibility
func builtinMapRemove(m, k any) (any, error) {
	mT, ok := m.(map[string]any)
	if !ok {
		return nil, trace.BadParameter("cannot remove from map of type: %T, must be map[string]any", m)
	}

	kS, ok := k.(string)
	if !ok {
		return nil, trace.BadParameter("cannot remove key of type: %T, must be string", k)
	}

	newMap := cloneMap(mT)
	delete(newMap, kS)
	return newMap, nil
}

/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package utils

import (
	"reflect"
	"slices"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// Fields represents a generic string-keyed map.
type Fields map[string]interface{}

// GetString returns a string representation of a field.
func (f Fields) GetString(key string) string {
	val, found := f[key]
	if !found {
		return ""
	}
	str, _ := getString(val)
	return str
}

// GetStrings returns a slice-of-strings representation of a field.
func (f Fields) GetStrings(key string) []string {
	val, found := f[key]
	if !found {
		return nil
	}
	res, _ := getStrings(val)
	return res
}

func getString(value any) (string, bool) {
	str, ok := value.(string)
	return str, ok
}

func getStrings(val any) ([]string, bool) {
	strings, ok := val.([]string)
	if ok {
		return strings, true
	}
	slice, ok := val.([]any)
	if !ok {
		return nil, false
	}
	res := make([]string, 0, len(slice))
	for _, v := range slice {
		s, ok := getString(v)
		if !ok {
			return nil, false
		}
		res = append(res, s)
	}
	return res, true
}

// GetInt returns an int representation of a field.
func (f Fields) GetInt(key string) int {
	val, found := f[key]
	if !found {
		return 0
	}
	v, ok := val.(int)
	if !ok {
		f, ok := val.(float64)
		if ok {
			v = int(f)
		}
	}
	return v
}

// GetTime returns a time.Time representation of a field.
func (f Fields) GetTime(key string) time.Time {
	val, found := f[key]
	if !found {
		return time.Time{}
	}
	v, ok := val.(time.Time)
	if !ok {
		s := f.GetString(key)
		v, _ = time.Parse(time.RFC3339, s)
	}
	return v
}

// HasField returns true if the field exists.
func (f Fields) HasField(key string) bool {
	_, ok := f[key]
	return ok
}

// FieldsCondition is a boolean function on Fields.
type FieldsCondition func(Fields) bool

// ToFieldsCondition converts a WhereExpr into a FieldsCondition.
func ToFieldsCondition(expr *types.WhereExpr) (FieldsCondition, error) {
	if expr == nil {
		return nil, trace.BadParameter("expr is nil")
	}

	binOp := func(e types.WhereExpr2, op func(a, b bool) bool) (FieldsCondition, error) {
		left, err := ToFieldsCondition(e.L)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		right, err := ToFieldsCondition(e.R)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return func(f Fields) bool { return op(left(f), right(f)) }, nil
	}
	if expr, err := binOp(expr.And, func(a, b bool) bool { return a && b }); err == nil {
		return expr, nil
	}
	if expr, err := binOp(expr.Or, func(a, b bool) bool { return a || b }); err == nil {
		return expr, nil
	}
	if inner, err := ToFieldsCondition(expr.Not); err == nil {
		return func(f Fields) bool { return !inner(f) }, nil
	}

	if expr.Equals.L != nil && expr.Equals.R != nil {
		left, right := expr.Equals.L, expr.Equals.R
		switch {
		case left.Field != "" && right.Field != "":
			return func(f Fields) bool { return fieldsEqual(f[left.Field], f[right.Field]) }, nil
		case left.Literal != nil && right.Field != "":
			return func(f Fields) bool { return fieldsEqual(left.Literal, f[right.Field]) }, nil
		case left.Field != "" && right.Literal != nil:
			return func(f Fields) bool { return fieldsEqual(f[left.Field], right.Literal) }, nil
		}
	}
	if expr.Contains.L != nil && expr.Contains.R != nil {
		left, right := expr.Contains.L, expr.Contains.R
		switch {
		case left.Field != "" && right.Field != "":
			return func(f Fields) bool {
				value, ok := getString(f[right.Field])
				return ok && slices.Contains(f.GetStrings(left.Field), value)
			}, nil
		case left.Literal != nil && right.Field != "":
			if ss, ok := getStrings(left.Literal); ok {
				return func(f Fields) bool {
					value, ok := getString(f[right.Field])
					return ok && slices.Contains(ss, value)
				}, nil
			}
		case left.Field != "" && right.Literal != nil:
			if s, ok := getString(right.Literal); ok {
				return func(f Fields) bool { return slices.Contains(f.GetStrings(left.Field), s) }, nil
			}
		}
	}

	return nil, trace.BadParameter("failed to convert expression %q to FieldsCondition", expr)
}

// fieldsEqual reports whether a and b are equal, without panicking when
// their dynamic type is not comparable (e.g. a []any or map[string]any
// decoded from a JSON-encoded session event field).
func fieldsEqual(a, b any) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	aStrings, aIsStrings := getStrings(a)
	bStrings, bIsStrings := getStrings(b)
	if aIsStrings || bIsStrings {
		return aIsStrings && bIsStrings && slices.Equal(aStrings, bStrings)
	}

	aString, aIsString := getString(a)
	bString, bIsString := getString(b)
	if aIsString || bIsString {
		return aIsString && bIsString && aString == bString
	}

	return fieldsEqualFallback(a, b)
}

func fieldsEqualFallback(a, b any) bool {
	aType, bType := reflect.TypeOf(a), reflect.TypeOf(b)
	if aType != bType {
		return false
	}

	if aType.Comparable() {
		return a == b
	}

	switch aType.Kind() {
	case reflect.Slice:
		aSlice, bSlice := reflect.ValueOf(a), reflect.ValueOf(b)
		if aSlice.Len() != bSlice.Len() {
			return false
		}
		for i := 0; i < aSlice.Len(); i++ {
			if !fieldsEqual(aSlice.Index(i).Interface(), bSlice.Index(i).Interface()) {
				return false
			}
		}
		return true
	case reflect.Map:
		aMap, bMap := reflect.ValueOf(a), reflect.ValueOf(b)
		if aMap.Len() != bMap.Len() {
			return false
		}
		iter := aMap.MapRange()
		for iter.Next() {
			bValue := bMap.MapIndex(iter.Key())
			if !bValue.IsValid() || !fieldsEqual(iter.Value().Interface(), bValue.Interface()) {
				return false
			}
		}
		return true
	}

	return false
}

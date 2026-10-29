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
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	setutils "github.com/gravitational/teleport/lib/utils/set"
)

// Fields represents a generic string-keyed map.
type Fields map[string]any

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

	switch {
	case aType.Kind() == reflect.Slice:
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
	case aType.Kind() == reflect.Map:

		if aType.Kind() == reflect.Map {
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
	}

	return false
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

func (f Fields) Get(key string) (any, bool) {
	v, ok := f[key]
	return v, ok
}

func (f Fields) GetMapEntry(mapRef *types.WhereExpr2) (any, bool) {
	if mapRef == nil || mapRef.L == nil || mapRef.R == nil {
		return nil, false
	}
	field := mapRef.L.Field
	key, ok := getString(mapRef.R.Literal)
	if !ok {
		return nil, false
	}
	val, found := f.Get(field)
	if !found {
		return nil, false
	}
	m, ok := val.(map[string]any)
	if !ok {
		return nil, false
	}
	v, ok := m[key]
	if !ok {
		return nil, false
	}
	return v, true
}

func (f Fields) getWhereExprValue(expr *types.WhereExpr) (any, bool) {
	if expr == nil {
		return nil, false
	}
	if expr.MapRef != nil {
		return f.GetMapEntry(expr.MapRef)
	}
	if expr.Field != "" {
		return f.Get(expr.Field)
	}
	if expr.Literal != nil {
		return expr.Literal, true
	}
	return nil, false
}

// FieldsCondition is a boolean function on Fields.
type FieldsCondition func(Fields) bool

// ToFieldsConditionConfig is the configuration for ToFieldsCondition.
type ToFieldsConditionConfig struct {
	Expr *types.WhereExpr
	// CanView is an optional function that checks if the user is allowed to view the resource.
	CanView func(Fields) bool
}

// ToFieldsCondition converts a WhereExpr into a FieldsCondition.
func ToFieldsCondition(cfg ToFieldsConditionConfig) (FieldsCondition, error) {
	expr := cfg.Expr
	if cfg.Expr == nil {
		return nil, trace.BadParameter("expr is nil")
	}

	binOp := func(e types.WhereExpr2, op func(a, b bool) bool) (FieldsCondition, error) {
		left, err := ToFieldsCondition(ToFieldsConditionConfig{
			Expr:    e.L,
			CanView: cfg.CanView,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		right, err := ToFieldsCondition(
			ToFieldsConditionConfig{
				Expr:    e.R,
				CanView: cfg.CanView,
			},
		)
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
	if inner, err := ToFieldsCondition(
		ToFieldsConditionConfig{
			Expr:    expr.Not,
			CanView: cfg.CanView,
		},
	); err == nil {
		return func(f Fields) bool { return !inner(f) }, nil
	}

	if expr.Equals.L != nil && expr.Equals.R != nil {
		left, right := expr.Equals.L, expr.Equals.R
		switch {
		case left.MapRef != nil:
			return func(f Fields) bool {
				val, ok := f.GetMapEntry(left.MapRef)
				if !ok {
					return false
				}
				var other any
				if right.Field != "" {
					other = f[right.Field]
				} else if right.Literal != nil {
					other = right.Literal
				} else {
					return false
				}
				return fieldsEqual(val, other)
			}, nil
		case right.MapRef != nil:
			return func(f Fields) bool {
				val, ok := f.GetMapEntry(right.MapRef)
				if !ok {
					return false
				}
				var other any
				if left.Field != "" {
					other = f[left.Field]
				} else if left.Literal != nil {
					other = left.Literal
				} else {
					return false
				}
				return fieldsEqual(other, val)
			}, nil
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
		case left.MapRef != nil:
			return func(f Fields) bool {
				val, ok := f.GetMapEntry(left.MapRef)
				if !ok {
					return false
				}
				strs, ok := getStrings(val)
				if !ok {
					return false
				}
				var strsVals string
				if right.Field != "" {
					strsVals, ok = getString(f[right.Field])
					if !ok {
						return false
					}
				} else if right.Literal != nil {
					strsVals, ok = getString(right.Literal)
					if !ok {
						return false
					}
				}
				return slices.Contains(strs, strsVals)
			}, nil
		case right.MapRef != nil:
			return func(f Fields) bool {
				val, ok := f.GetMapEntry(right.MapRef)
				if !ok {
					return false
				}
				str, ok := getString(val)
				if !ok {
					return false
				}
				var strsVals []string
				if left.Field != "" {
					strsVals = f.GetStrings(left.Field)
				} else if left.Literal != nil {
					strsVals, ok = getStrings(left.Literal)
					if !ok {
						return false
					}
				}
				return slices.Contains(strsVals, str)
			}, nil
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

	if expr.ContainsAll.L != nil && expr.ContainsAll.R != nil {
		left, right := expr.ContainsAll.L, expr.ContainsAll.R
		switch {
		case left.MapRef != nil:
			return func(f Fields) bool {
				val, ok := f.GetMapEntry(left.MapRef)
				if !ok {
					return false
				}

				strs, ok := getStrings(val)
				if !ok {
					return false
				}
				var strsVals []string
				if right.Field != "" {
					strsVals = f.GetStrings(right.Field)
				} else if right.Literal != nil {
					strsVals, ok = getStrings(right.Literal)
					if !ok {
						return false
					}
				}
				return containsAll(strs, strsVals)
			}, nil
		case right.MapRef != nil:
			return func(f Fields) bool {
				val, ok := f.GetMapEntry(right.MapRef)
				if !ok {
					return false
				}
				strs, ok := getStrings(val)
				if !ok {
					return false
				}
				var strsVals []string
				if left.Field != "" {
					strsVals = f.GetStrings(left.Field)
				} else if left.Literal != nil {
					strsVals, ok = getStrings(left.Literal)
					if !ok {
						return false
					}
				}
				return containsAll(strsVals, strs)
			}, nil
		case left.Field != "" && right.Field != "":
			return func(f Fields) bool { return containsAll(f.GetStrings(left.Field), f.GetStrings(right.Field)) }, nil
		case left.Literal != nil && right.Field != "":
			if ss, ok := getStrings(left.Literal); ok {
				return func(f Fields) bool { return containsAll(ss, f.GetStrings(right.Field)) }, nil
			}
		case left.Field != "" && right.Literal != nil:
			if s, ok := getStrings(right.Literal); ok {
				return func(f Fields) bool { return containsAll(f.GetStrings(left.Field), s) }, nil
			}
		}
	}

	if expr.ContainsAny.L != nil && expr.ContainsAny.R != nil {
		left, right := expr.ContainsAny.L, expr.ContainsAny.R
		switch {
		case left.MapRef != nil:
			return func(f Fields) bool {
				val, ok := f.GetMapEntry(left.MapRef)
				if !ok {
					return false
				}
				strs, ok := getStrings(val)
				if !ok {
					return false
				}
				var strsVals []string
				if right.Field != "" {
					strsVals = f.GetStrings(right.Field)
				} else if right.Literal != nil {
					strsVals, ok = getStrings(right.Literal)
					if !ok {
						return false
					}
				}
				return containsAny(strs, strsVals)
			}, nil
		case right.MapRef != nil:
			return func(f Fields) bool {
				val, ok := f.GetMapEntry(right.MapRef)
				if !ok {
					return false
				}
				strs, ok := getStrings(val)
				if !ok {
					return false
				}
				var strsVals []string
				if left.Field != "" {
					strsVals = f.GetStrings(left.Field)
				} else if left.Literal != nil {
					strsVals, ok = getStrings(left.Literal)
					if !ok {
						return false
					}
				}
				return containsAny(strsVals, strs)
			}, nil
		case left.Field != "" && right.Field != "":
			return func(f Fields) bool { return containsAny(f.GetStrings(left.Field), f.GetStrings(right.Field)) }, nil
		case left.Literal != nil && right.Field != "":
			if ss, ok := getStrings(left.Literal); ok {
				return func(f Fields) bool { return containsAny(ss, f.GetStrings(right.Field)) }, nil
			}
		case left.Field != "" && right.Literal != nil:
			if s, ok := getStrings(right.Literal); ok {
				return func(f Fields) bool { return containsAny(f.GetStrings(left.Field), s) }, nil
			}
		}
	}

	binaryValueCondition := func(pair types.WhereExpr2, predicate func(a, b any) bool) (FieldsCondition, error) {
		if pair.L == nil || pair.R == nil {
			return nil, trace.BadParameter("missing predicate operand")
		}
		return func(f Fields) bool {
			left, ok := f.getWhereExprValue(pair.L)
			if !ok {
				return false
			}
			right, ok := f.getWhereExprValue(pair.R)
			return ok && predicate(left, right)
		}, nil
	}

	allEndWith := func(a, b any) bool {
		suffix, ok := getString(b)
		if !ok {
			return false
		}
		if value, ok := getString(a); ok {
			return strings.HasSuffix(value, suffix)
		}
		values, ok := getStrings(a)
		return ok && !slices.ContainsFunc(values, func(value string) bool {
			return !strings.HasSuffix(value, suffix)
		})
	}
	if expr.AllEndWith.L != nil || expr.AllEndWith.R != nil {
		return binaryValueCondition(expr.AllEndWith, allEndWith)
	}

	allEqual := func(a, b any) bool {
		expected, ok := getString(b)
		if !ok {
			return false
		}
		if value, ok := getString(a); ok {
			return value == expected
		}
		values, ok := getStrings(a)
		return ok && !slices.ContainsFunc(values, func(value string) bool {
			return value != expected
		})
	}
	if expr.AllEqual.L != nil || expr.AllEqual.R != nil {
		return binaryValueCondition(expr.AllEqual, allEqual)
	}

	if expr.HasPrefix.L != nil || expr.HasPrefix.R != nil {
		return binaryValueCondition(expr.HasPrefix, func(a, b any) bool {
			value, valueOK := getString(a)
			prefix, prefixOK := getString(b)
			return valueOK && prefixOK && strings.HasPrefix(value, prefix)
		})
	}

	if expr.IsSubset.L != nil {
		return func(f Fields) bool {
			left, ok := f.getWhereExprValue(expr.IsSubset.L)
			if !ok {
				return false
			}
			allowed, ok := getAllowedValues(f, expr.IsSubset.R)
			if !ok {
				return false
			}

			if value, ok := getString(left); ok {
				_, ok := allowed[value]
				return ok
			}

			values, ok := getStrings(left)
			if !ok {
				return false
			}
			for _, value := range values {
				if _, ok := allowed[value]; !ok {
					return false
				}
			}
			return true
		}, nil
	}

	if expr.CanView != nil {
		if cfg.CanView == nil {
			return nil, trace.BadParameter("canView expression provided but no canView function specified")
		}
		return func(f Fields) bool { return cfg.CanView(f) }, nil
	}

	return nil, trace.BadParameter("failed to convert expression %q to FieldsCondition", expr)
}

func getAllowedValues(f Fields, values []*types.WhereExpr) (map[string]struct{}, bool) {
	allowed := make(map[string]struct{}, len(values))
	for _, arg := range values {
		value, ok := f.getWhereExprValue(arg)
		if !ok {
			return nil, false
		}
		str, ok := getString(value)
		if !ok {
			return nil, false
		}
		allowed[str] = struct{}{}
	}
	return allowed, true
}

func containsAll(slice []string, items []string) bool {
	set := setutils.New(slice...)
	if len(items) == 0 {
		return false
	}
	for _, item := range items {
		if !set.Contains(item) {
			return false
		}
	}
	return true
}

func containsAny(slice []string, items []string) bool {
	set := setutils.New(slice...)
	if len(items) == 0 {
		return false
	}
	return slices.ContainsFunc(items, set.Contains)
}

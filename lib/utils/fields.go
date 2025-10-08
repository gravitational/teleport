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
	"slices"
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
	return val.(string)
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
		s, ok := v.(string)
		if ok {
			res = append(res, s)
		}
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

func (f Fields) Get(key string) (any, bool) {
	v, ok := f[key]
	return v, ok
}

func (f Fields) GetMapEntry(mapRef *types.WhereExpr2) (any, bool) {
	field := mapRef.L.Field
	key, ok := mapRef.R.Literal.(string)
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
			})
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
				strs, ok := val.(string)
				if !ok {
					return false
				}
				var strsVals string
				if right.Field != "" {
					strsVals = f.GetString(right.Field)
				} else if right.Literal != nil {
					strsVals, ok = right.Literal.(string)
					if !ok {
						return false
					}
				}
				return strs == strsVals
			}, nil
		case right.MapRef != nil:
			return func(f Fields) bool {
				val, ok := f.GetMapEntry(right.MapRef)
				if !ok {
					return false
				}
				str, ok := val.(string)
				if !ok {
					return false
				}
				var strsVals string
				if left.Field != "" {
					strsVals = f.GetString(left.Field)
				} else if left.Literal != nil {
					strsVals, ok = left.Literal.(string)
					if !ok {
						return false
					}
				}
				return str == strsVals
			}, nil
		case left.Field != "" && right.Field != "":
			return func(f Fields) bool { return f[left.Field] == f[right.Field] }, nil
		case left.Literal != nil && right.Field != "":
			return func(f Fields) bool { return left.Literal == f[right.Field] }, nil
		case left.Field != "" && right.Literal != nil:
			return func(f Fields) bool { return f[left.Field] == right.Literal }, nil
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
					strsVals = f.GetString(right.Field)
				} else if right.Literal != nil {
					strsVals, ok = right.Literal.(string)
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
				str, ok := val.(string)
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
			return func(f Fields) bool { return slices.Contains(f.GetStrings(left.Field), f.GetString(right.Field)) }, nil
		case left.Literal != nil && right.Field != "":
			if ss, ok := getStrings(left.Literal); ok {
				return func(f Fields) bool { return slices.Contains(ss, f.GetString(right.Field)) }, nil
			}
		case left.Field != "" && right.Literal != nil:
			if s, ok := right.Literal.(string); ok {
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

	if expr.CanView != nil {
		if cfg.CanView == nil {
			return nil, trace.BadParameter("canView expression provided but no canView function specified")
		}
		return func(f Fields) bool { return cfg.CanView(f) }, nil
	}

	return nil, trace.BadParameter("failed to convert expression %q to FieldsCondition", expr)
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
	for _, item := range items {
		if set.Contains(item) {
			return true
		}
	}
	return false
}

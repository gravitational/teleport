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
	strings, ok := val.([]string)
	if ok {
		return strings
	}
	slice, _ := val.([]any)
	res := make([]string, 0, len(slice))
	for _, v := range slice {
		s, ok := v.(string)
		if ok {
			res = append(res, s)
		}
	}
	return res
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
		case left.Field != "" && right.Field != "":
			return func(f Fields) bool { return slices.Contains(f.GetStrings(left.Field), f.GetString(right.Field)) }, nil
		case left.Literal != nil && right.Field != "":
			if ss, ok := left.Literal.([]string); ok {
				return func(f Fields) bool { return slices.Contains(ss, f.GetString(right.Field)) }, nil
			}
		case left.Field != "" && right.Literal != nil:
			if s, ok := right.Literal.(string); ok {
				return func(f Fields) bool { return slices.Contains(f.GetStrings(left.Field), s) }, nil
			}
		}
	}

	return nil, trace.BadParameter("failed to convert expression %q to FieldsCondition", expr)
}

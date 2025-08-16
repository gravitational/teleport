/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package testutils

import (
	"fmt"
	"reflect"
	"strings"
)

// ExhaustiveNonEmpty is a helper that uses reflection to check if a given value and its sub-elements are non-empty. Exhaustive
// non-emptiness is evaluated in the following ways:
//
// - Pointers/Interfaces are considered exhaustively non-empty if their underlying value is exhaustively non-empty.
// - Slices/Arrays are considered exhaustively non-empty if they have at least one exhaustively non-empty element.
// - Maps are considered exhaustively non-empty if they have at least one exhaustively non-empty value.
// - Structs are considered exhaustively non-empty if all their exported fields are non-empty.
// - All other types are considered exhaustively non-empty if reflect.Value.IsZero is false.
//
// The ignoreOpts parameter is a variadic list of strings that represent the fully qualified field names of struct fields that
// should be ignored when checking for non-emptiness. For example, to ignore the field Bar on type Foo pass in "Foo.Bar" as an
// ignore option. Note that embedded type fields have to be ignored by the parent type's name (i.e. `Outer.Field` rather than
// `Inner.Field`).
//
// The intended usecase of this helper is to ensure that new fields added to a struct are included in test cases that want to
// cover all fields. For example, a test of serialization/deserialization logic might assert that the sample struct is exhaustively
// non-empty in order to force new fields to be covered by the test.
func ExhaustiveNonEmpty(item any, ignoreOpts ...string) bool {
	value := reflect.ValueOf(item)

	ignore := make(map[string]struct{}, len(ignoreOpts))
	for _, opt := range ignoreOpts {
		ignore[opt] = struct{}{}
	}

	return exhaustiveNonEmpty(value, ignore)
}

func exhaustiveNonEmpty(value reflect.Value, ignore map[string]struct{}) bool {
	if !value.IsValid() {
		// indicates that reflect.ValueOf/Value.Elem was called on a nil pointer/interface
		return false
	}

	switch value.Kind() {
	case reflect.Pointer, reflect.Interface:
		// recursively check the underlying value
		return exhaustiveNonEmpty(value.Elem(), ignore)
	case reflect.Slice, reflect.Array:
		if value.Len() == 0 {
			return false
		}

		for i := range value.Len() {
			if exhaustiveNonEmpty(value.Index(i), ignore) {
				return true
			}
		}
		return false
	case reflect.Map:
		if value.Len() == 0 {
			return false
		}

		mr := value.MapRange()

		for mr.Next() {
			if exhaustiveNonEmpty(mr.Value(), ignore) {
				return true
			}
		}

		return false
	case reflect.Struct:
		var fieldsConsidered int
		for _, vf := range reflect.VisibleFields(value.Type()) {
			if vf.Anonymous {
				// skip the embedded type itself since this loop will
				// end up processing each of the embedded type's fields as
				// a member of this type's fields.
				continue
			}

			if !vf.IsExported() {
				// skip non-exported fields
				continue
			}

			fieldsConsidered++

			// skip fields if `<type>.<field>` is in the ignore list
			if _, ok := ignore[fmt.Sprintf("%s.%s", value.Type().Name(), vf.Name)]; ok {
				continue
			}

			if !exhaustiveNonEmpty(value.FieldByIndex(vf.Index), ignore) {
				return false
			}
		}

		if fieldsConsidered == 0 {
			// fallback to basic nonzeroness check for structs with no exported fields (necessary
			// in order to achieve expected behavior for types like time.Time).
			return !value.IsZero()
		}

		return true
	default:
		// fallback to basic nonzeroness check for all other types
		return !value.IsZero()
	}
}

// FindAllEmpty is a helper that uses reflection to find all empty sub-components of a given value. It functions similarly to the ExhaustiveNonEmpty
// check, but may return a non-empty list of paths in cases where ExhaustiveNonEmpty would return false since it records all empty members of
// collections even if the collection contains a non-empty member.
//
// The intended usecase for FindAllEmpty is to build helpful failure messages in tests that assert that a struct is non-empty.
//
// Note that this function panics if the top-level item passed in is nil.
func FindAllEmpty(item any, ignoreOpts ...string) []string {
	value := reflect.ValueOf(item)

	if !value.IsValid() {
		panic("FindAllEmpty called with nil top-level item")
	}

	// dereference pointers and interfaces so that the root find logic starts from
	// a concrete type (makes the returned paths more consistent/understandable).
	switch value.Kind() {
	case reflect.Ptr, reflect.Interface:
		if value.IsNil() {
			panic("FindAllEmpty called with nil top-level pointer/interface")
		}
		return FindAllEmpty(value.Elem().Interface(), ignoreOpts...)
	}

	ignore := make(map[string]struct{}, len(ignoreOpts))
	for _, opt := range ignoreOpts {
		ignore[opt] = struct{}{}
	}

	path := []string{value.Type().Name()}

	return findAllEmpty(value, ignore, path)
}

func findAllEmpty(value reflect.Value, ignore map[string]struct{}, path []string) []string {
	if !value.IsValid() {
		// indicates that reflect.ValueOf/Value.Elem was called on a nil pointer/interface
		return []string{strings.Join(path, ".")}
	}

	switch value.Kind() {
	case reflect.Pointer, reflect.Interface:
		// recursively check the underlying value
		return findAllEmpty(value.Elem(), ignore, path)
	case reflect.Slice, reflect.Array:
		if value.Len() == 0 {
			return []string{strings.Join(path, ".")}
		}

		var emptyPaths []string
		for i := range value.Len() {
			emptyPaths = append(emptyPaths, findAllEmpty(value.Index(i), ignore, append(path, fmt.Sprintf("%d", i)))...)
		}
		return emptyPaths
	case reflect.Map:
		if value.Len() == 0 {
			return []string{strings.Join(path, ".")}
		}

		mr := value.MapRange()

		var emptyPaths []string
		for mr.Next() {
			emptyPaths = append(emptyPaths, findAllEmpty(mr.Value(), ignore, append(path, fmt.Sprintf("%v", mr.Key().Interface())))...)
		}

		return emptyPaths
	case reflect.Struct:
		emptyPaths := make([]string, 0, value.NumField())
		var fieldsConsidered int
		for _, vf := range reflect.VisibleFields(value.Type()) {
			if vf.Anonymous {
				// skip the embedded type itself since this loop will
				// end up processing each of the embedded type's fields as
				// a member of this type's fields.
				continue
			}

			if !vf.IsExported() {
				// skip non-exported fields
				continue
			}

			fieldsConsidered++

			// skip fields if `<type>.<field>` is in the ignore list
			if _, ok := ignore[fmt.Sprintf("%s.%s", value.Type().Name(), vf.Name)]; ok {
				continue
			}

			emptyPaths = append(emptyPaths, findAllEmpty(value.FieldByIndex(vf.Index), ignore, append(path, vf.Name))...)
		}

		if fieldsConsidered == 0 {
			// fallback to basic nonzeroness check for structs with no exported fields (necessary
			// in order to achieve expected behavior for types like time.Time).
			if value.IsZero() {
				return []string{strings.Join(path, ".")}
			}
		}

		return emptyPaths
	default:
		// fallback to basic nonzeroness check for all other types
		if value.IsZero() {
			return []string{strings.Join(path, ".")}
		}
		return nil
	}
}

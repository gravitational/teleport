/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package expression

import (
	"strings"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/gravitational/teleport/lib/utils/typical"
)

// protoMessageVariables builds a map of `typical.Variable`s for the given proto
// message type. If zeroValError is true, accessing an "unset" string field will
// return an error instead of the zero value.
//
// We currently only support the following field types, and all other fields will
// be ignored:
//
//   - string
//   - int32
//   - int64
//   - uint32
//   - uint64
//   - bool
//   - map<string, string>
//
// Repeated fields are not yet supported.
//
// Note: we do not support self-referential messages or circular references, these
// fields will also be ignored.
func protoMessageVariables[TEnv proto.Message](zeroValError bool) map[string]typical.Variable {
	// addMessageFields adds each of a messages fields as variables. If it
	// encounters a sub-message, it recursively calls itself to add the sub
	// message's fields, prefixed its ancestors' names.
	var addMessageFields func(string, []protoreflect.FieldDescriptor, protoreflect.MessageDescriptor)

	// seen tracks which message types we've already processed, to prevent infinite
	// recursion in self-referential messages.
	seen := make(map[string]struct{})

	vars := make(map[string]typical.Variable)
	addMessageFields = func(prefix string, ancestors []protoreflect.FieldDescriptor, descriptor protoreflect.MessageDescriptor) {
		// Do not process this message type if we've already seen it.
		if _, ok := seen[string(descriptor.FullName())]; ok {
			return
		}
		seen[string(descriptor.FullName())] = struct{}{}

		fields := descriptor.Fields()
		for i := 0; i < fields.Len(); i++ {
			field := fields.Get(i)

			name := field.TextName()
			if prefix != "" {
				name = prefix + "." + name
			}

			// Read the field value from the given TEnv by reading each of the
			// ancestor messages.
			get := func(env TEnv) (protoreflect.Value, error) {
				msg := env.ProtoReflect()
				names := make([]string, 0)
				for _, ancestor := range ancestors {
					msg = msg.Get(ancestor).Message()
					names = append(names, ancestor.TextName())

					// Accessing a field on an unset sub-message is always an error.
					if !msg.IsValid() {
						return protoreflect.Value{}, trace.Errorf("%s is unset", strings.Join(names, "."))
					}
				}

				return msg.Get(field), nil
			}

			if field.IsMap() {
				// We currently only support map[string]string.
				if field.MapKey().Kind() != protoreflect.StringKind ||
					field.MapValue().Kind() != protoreflect.StringKind {
					continue
				}
				vars[name] = typical.DynamicMapFunction(func(env TEnv, key string) (string, error) {
					mapKey := protoreflect.ValueOf(key).MapKey()

					val, err := get(env)
					if err != nil {
						return "", err
					}
					mapVal := val.Map().Get(mapKey)

					if mapVal.IsValid() {
						return mapVal.String(), nil
					}
					return "", trace.Errorf("no value for key: %q", key)
				})
			}

			switch field.Kind() {
			case protoreflect.MessageKind:
				addMessageFields(name, append(ancestors, field), field.Message())
			case protoreflect.StringKind:
				vars[name] = typical.DynamicVariable(func(env TEnv) (string, error) {
					v, err := get(env)
					if err != nil {
						return "", err
					}
					s := v.String()
					if s == "" && zeroValError {
						return "", trace.Errorf("%s is unset", name)
					}
					return s, nil
				})
			case protoreflect.Int32Kind, protoreflect.Int64Kind:
				vars[name] = typical.DynamicVariable(func(env TEnv) (int, error) {
					if v, err := get(env); err == nil {
						return int(v.Int()), nil
					} else {
						return 0, err
					}
				})
			case protoreflect.Uint32Kind, protoreflect.Uint64Kind:
				vars[name] = typical.DynamicVariable(func(env TEnv) (uint64, error) {
					if v, err := get(env); err == nil {
						return v.Uint(), nil
					} else {
						return 0, err
					}
				})
			case protoreflect.BoolKind:
				vars[name] = typical.DynamicVariable(func(env TEnv) (bool, error) {
					if v, err := get(env); err == nil {
						return v.Bool(), nil
					} else {
						return false, err
					}
				})
			}
		}
	}

	var t TEnv
	addMessageFields(
		"",  /* prefix */
		nil, /* ancestors */
		t.ProtoReflect().Descriptor(),
	)
	return vars
}

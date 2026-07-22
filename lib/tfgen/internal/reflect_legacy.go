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

package internal

import (
	"reflect"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// LegacyProtoMessage is the interface satisfied by protobuf v1 and gogo-proto
// structs.
type LegacyProtoMessage interface{ ProtoMessage() }

// ReflectLegacy uses Go's reflect package to walk the given message and
// discover its attributes. This is suitable for legacy/gogo-proto resources.
func ReflectLegacy(message LegacyProtoMessage) (*Message, error) {
	msg := reflect.ValueOf(message)
	if msg.Kind() == reflect.Pointer {
		msg = msg.Elem()
	}
	return reflectMessageLegacy(msg)
}

func reflectMessageLegacy(value reflect.Value) (*Message, error) {
	msgType := value.Type()

	var msg Message
	for i := range msgType.NumField() {
		fieldType := msgType.Field(i)
		fieldName := fieldType.Name

		// Skip over protobuf internal fields.
		if strings.HasPrefix(fieldName, "XXX_") {
			continue
		}

		// Derive the field name from the JSON tag.
		jsonTag := fieldType.Tag.Get("json")
		if jsonTag != "" {
			fieldName, _, _ = strings.Cut(jsonTag, ",")
			if fieldName == "-" {
				continue
			}
		}

		fieldValue, err := reflectValueLegacy(value.Field(i))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if fieldValue == nil {
			continue
		}

		if fieldType.Anonymous {
			// Handle embedded struct fields.
			msg.Attributes = append(msg.Attributes, fieldValue.Message().Attributes...)
		} else {
			msg.Attributes = append(msg.Attributes, Attribute{
				Name:  fieldName,
				Value: fieldValue,
			})
		}
	}

	return &msg, nil
}

func reflectValueLegacy(value reflect.Value) (*Value, error) {
	if value.Kind() == reflect.Pointer {
		// Special case: if field is not populated at all, we simply don't emit
		// it as an attribute.
		if value.IsNil() {
			return nil, nil
		}

		// Dereference pointer.
		value = value.Elem()
	}

	// Handle custom types.
	switch v := value.Interface().(type) {
	case []byte:
		return &Value{
			Type:  TypeBytes,
			Value: v,
		}, nil
	case time.Time:
		return &Value{
			Type:  TypeTimestamp,
			Value: v,
		}, nil
	case types.Duration:
		return &Value{
			Type:  TypeDuration,
			Value: time.Duration(v),
		}, nil
	case types.BoolOption:
		return &Value{
			Type:  TypeBool,
			Value: v.Value,
		}, nil
	}

	switch value.Kind() {
	case reflect.Struct:
		message, err := reflectMessageLegacy(value)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &Value{
			Type:  TypeMessage,
			Value: message,
		}, nil
	case reflect.Map:
		return reflectMapLegacy(value)
	case reflect.Slice:
		return reflectSliceLegacy(value)
	case reflect.String:
		return &Value{
			Type:  TypeString,
			Value: value.String(),
		}, nil
	case reflect.Bool:
		return &Value{
			Type:  TypeBool,
			Value: value.Bool(),
		}, nil
	case reflect.Int32, reflect.Int64:
		return &Value{
			Type:  TypeInt,
			Value: int(value.Int()),
		}, nil
	case reflect.Uint32, reflect.Uint64:
		return &Value{
			Type:  TypeInt,
			Value: int(value.Uint()),
		}, nil
	case reflect.Float32, reflect.Float64:
		return &Value{
			Type:  TypeFloat,
			Value: value.Float(),
		}, nil
	}

	return nil, trace.NotImplemented("field type %s not supported", value.Kind())
}

func reflectSliceLegacy(value reflect.Value) (*Value, error) {
	elems := make([]*Value, value.Len())
	for i := range value.Len() {
		val, err := reflectValueLegacy(value.Index(i))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		elems[i] = val
	}

	return &Value{
		Type: TypeList,
		Value: &ListValue{
			Elems: elems,
		},
	}, nil
}

func reflectMapLegacy(value reflect.Value) (*Value, error) {
	elems := make(map[any]*Value)
	iter := value.MapRange()
	for iter.Next() {
		key, err := reflectValueLegacy(iter.Key())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		val, err := reflectValueLegacy(iter.Value())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		elems[key.Value] = val
	}

	return &Value{
		Type: TypeMap,
		Value: &MapValue{
			Elems: elems,
		},
	}, nil
}

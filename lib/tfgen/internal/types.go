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

import "time"

// Type is our internal representation of a field value data type.
type Type uint16

const (
	TypeUnknown Type = iota
	TypeMessage
	TypeList
	TypeMap
	TypeBool
	TypeString
	TypeInt
	TypeFloat
	TypeBytes
	TypeTimestamp
	TypeDuration
)

// String implements fmt.Stringer.
func (t Type) String() string {
	switch t {
	case TypeMessage:
		return "message"
	case TypeList:
		return "list"
	case TypeMap:
		return "map"
	case TypeBool:
		return "bool"
	case TypeString:
		return "string"
	case TypeInt:
		return "int"
	case TypeFloat:
		return "float"
	case TypeBytes:
		return "bytes"
	case TypeTimestamp:
		return "timestamp"
	case TypeDuration:
		return "duration"
	default:
		return "<unknown>"
	}
}

// Message represents a protobuf message/struct.
type Message struct {
	// Attributes of the message.
	Attributes []Attribute
}

// Attribute of a message.
type Attribute struct {
	// Name of the attribute.
	Name string

	// Value of the attribute.
	Value *Value
}

// Value is a tuple of data type and value.
type Value struct {
	// Type of value.
	Type Type

	// Value itself (check Type before casting).
	Value any
}

// ListValue represents a list/slice.
type ListValue struct {
	// Elems contains the list values.
	Elems []*Value
}

// MapValue represents a map.
type MapValue struct {
	// Elems contains the map values.
	Elems map[any]*Value
}

// List returns the value as a ListValue, or an empty ListValue if it isn't one.
func (v *Value) List() *ListValue {
	if v.Type != TypeList {
		return &ListValue{}
	}
	if l, ok := v.Value.(*ListValue); ok {
		return l
	}
	return &ListValue{}
}

// Map returns the value as a MapValue, or an empty MapValue if it isn't one.
func (v *Value) Map() *MapValue {
	if v.Type != TypeMap {
		return &MapValue{}
	}
	if m, ok := v.Value.(*MapValue); ok {
		return m
	}
	return &MapValue{}
}

// Message returns the value as a Message, or an empty Message if it isn't one.
func (v *Value) Message() *Message {
	if v.Type != TypeMessage {
		return &Message{}
	}
	if m, ok := v.Value.(*Message); ok {
		return m
	}
	return &Message{}
}

// Bool returns the value as a boolean, or false if it isn't one.
func (v *Value) Bool() bool {
	if v.Type != TypeBool {
		return false
	}
	b, _ := v.Value.(bool)
	return b
}

// Int returns the value as a integer, or 0 if it isn't one.
func (v *Value) Int() int {
	if v.Type != TypeInt {
		return 0
	}
	i, _ := v.Value.(int)
	return i
}

// Float returns the value as a float64, or 0 if it isn't one.
func (v *Value) Float() float64 {
	if v.Type != TypeFloat {
		return 0
	}
	f, _ := v.Value.(float64)
	return f
}

// String returns the value as a string, or "" if it isn't one.
func (v *Value) String() string {
	if v.Type != TypeString {
		return ""
	}
	s, _ := v.Value.(string)
	return s
}

// Bytes returns the value as a []byte, or nil if it isn't one.
func (v *Value) Bytes() []byte {
	if v.Type != TypeBytes {
		return nil
	}
	b, _ := v.Value.([]byte)
	return b
}

// Timestamp returns the value as a time.Time, or the zero value if it isn't one.
func (v *Value) Timestamp() time.Time {
	if v.Type != TypeTimestamp {
		return time.Time{}
	}
	t, _ := v.Value.(time.Time)
	return t
}

// Duration returns the value as a time.Duration, or the zero value if it isn't one.
func (v *Value) Duration() time.Duration {
	if v.Type != TypeDuration {
		return 0
	}
	d, _ := v.Value.(time.Duration)
	return d
}

// AttributeNamed returns a message attribute with the given name, or nil if no
// such attribute exists.
func (m *Message) AttributeNamed(name string) *Attribute {
	for _, a := range m.Attributes {
		if a.Name == name {
			return &a
		}
	}
	return nil
}

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
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ReflectModern uses the modern protoreflect package to walk the given message
// and discover its attributes. This is suitable for RFD 153-style resources.
func ReflectModern(message proto.Message) (*Message, error) {
	return reflectMessageModern(message.ProtoReflect())
}

func reflectMessageModern(
	value protoreflect.Message,
) (*Message, error) {
	desc := value.Descriptor()
	fields := desc.Fields()

	var msg Message
	for i := range fields.Len() {
		fieldDesc := fields.Get(i)
		fieldValue := value.Get(fieldDesc)

		value, err := reflectValueModern(fieldValue, fieldDesc)
		if err != nil {
			return nil, trace.Wrap(err, "reflecting field: %s", fieldDesc.Name())
		}

		msg.Attributes = append(msg.Attributes, Attribute{
			Name:  fieldDesc.TextName(),
			Value: value,
		})
	}

	return &msg, nil
}

func reflectValueModern(
	value protoreflect.Value,
	desc protoreflect.FieldDescriptor,
) (*Value, error) {
	if desc.IsList() {
		return reflectListModern(value.List(), desc)
	}
	if desc.IsMap() {
		return reflectMapModern(value.Map(), desc)
	}
	return reflectValueInnerModern(value, desc)
}

func reflectValueInnerModern(
	value protoreflect.Value,
	desc protoreflect.FieldDescriptor,
) (*Value, error) {
	switch desc.Kind() {
	case protoreflect.BoolKind:
		return &Value{
			Type:  TypeBool,
			Value: value.Bool(),
		}, nil
	case protoreflect.StringKind, protoreflect.EnumKind:
		return &Value{
			Type:  TypeString,
			Value: value.String(),
		}, nil
	case protoreflect.Int32Kind, protoreflect.Int64Kind,
		protoreflect.Sint32Kind, protoreflect.Sint64Kind,
		protoreflect.Sfixed32Kind, protoreflect.Sfixed64Kind:
		return &Value{
			Type:  TypeInt,
			Value: int(value.Int()),
		}, nil
	case protoreflect.Uint32Kind, protoreflect.Uint64Kind,
		protoreflect.Fixed32Kind, protoreflect.Fixed64Kind:
		return &Value{
			Type:  TypeInt,
			Value: int(value.Uint()),
		}, nil
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		return &Value{
			Type:  TypeFloat,
			Value: value.Float(),
		}, nil
	case protoreflect.BytesKind:
		return &Value{
			Type:  TypeBytes,
			Value: value.Bytes(),
		}, nil
	case protoreflect.MessageKind:
		switch desc.Message().FullName() {
		case googleProtobufTimestamp:
			return reflectTimestampModern(value.Message())
		case googleProtobufDuration:
			return reflectDurationModern(value.Message())
		}

		message, err := reflectMessageModern(value.Message())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &Value{
			Type:  TypeMessage,
			Value: message,
		}, nil
	}
	return nil, trace.NotImplemented("unsupported field kind: %s", desc.Kind())
}

func reflectListModern(
	value protoreflect.List,
	desc protoreflect.FieldDescriptor,
) (*Value, error) {
	elems := make([]*Value, value.Len())
	for i := range value.Len() {
		val, err := reflectValueInnerModern(value.Get(i), desc)
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

func reflectMapModern(
	value protoreflect.Map,
	desc protoreflect.FieldDescriptor,
) (*Value, error) {
	elems := make(map[any]*Value, value.Len())

	var err error
	value.Range(func(key protoreflect.MapKey, value protoreflect.Value) bool {
		var v *Value
		if v, err = reflectValueModern(value, desc.MapValue()); err != nil {
			return false
		}
		elems[key.Interface()] = v
		return true
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Value{
		Type: TypeMap,
		Value: &MapValue{
			Elems: elems,
		},
	}, nil
}

func reflectTimestampModern(value protoreflect.Message) (*Value, error) {
	ts, ok := value.Interface().(*timestamppb.Timestamp)
	if !ok {
		return nil, trace.BadParameter("expected timestamp, got: %T", value)
	}

	t := ts.AsTime()
	if !ts.IsValid() {
		t = time.Time{}
	}

	return &Value{
		Type:  TypeTimestamp,
		Value: t,
	}, nil
}

func reflectDurationModern(value protoreflect.Message) (*Value, error) {
	dur, ok := value.Interface().(*durationpb.Duration)
	if !ok {
		return nil, trace.BadParameter("expected duration, got: %T", value)
	}
	return &Value{
		Type:  TypeDuration,
		Value: dur.AsDuration(),
	}, nil
}

var (
	googleProtobufTimestamp = protoreflect.FullName("google.protobuf.Timestamp")
	googleProtobufDuration  = protoreflect.FullName("google.protobuf.Duration")
)

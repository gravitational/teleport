/*
Copyright 2021-2022 Gravitational, Inc.

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

package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gogo/protobuf/gogoproto"
	"github.com/gogo/protobuf/proto"
	gogodesc "github.com/gogo/protobuf/protoc-gen-gogo/descriptor"
	gogogen "github.com/gogo/protobuf/protoc-gen-gogo/generator"
)

// Forest is a forest of file trees. It's a wrapper for generator.Generator.
type Forest struct {
	*gogogen.Generator
	messageMap map[*gogodesc.DescriptorProto]*Message
}

// File is a wrapper for generator.FileDescriptor
type File struct {
	parent        *Forest
	desc          *gogodesc.FileDescriptorProto
	locations     map[string]Location
	messageMap    map[*gogodesc.DescriptorProto]*Message
	messageByName map[string]*Message

	Messages []*Message
}

// Message is a wrapper for generator.Descriptor.
type Message struct {
	index     int
	parent    *File
	parentMsg *Message
	desc      *gogodesc.DescriptorProto
	Fields    []*Field
	fieldMap  map[string]*Field
}

// Field is wrapper for descriptor.FieldDescriptorProto.
type Field struct {
	index  int
	parent *Message
	desc   *gogodesc.FieldDescriptorProto
}

// Location is a wrapper for descriptor.SourceCodeInfo_Location.
type Location struct {
	loc *gogodesc.SourceCodeInfo_Location
}

func (forest *Forest) addFile(fileDesc *gogodesc.FileDescriptorProto) *File {
	locs := fileDesc.GetSourceCodeInfo().GetLocation()
	msgs := fileDesc.GetMessageType()
	file := File{
		parent:        forest,
		desc:          fileDesc,
		locations:     make(map[string]Location, len(locs)),
		Messages:      make([]*Message, len(msgs)),
		messageMap:    make(map[*gogodesc.DescriptorProto]*Message, len(msgs)),
		messageByName: make(map[string]*Message, len(msgs)),
	}

	// Build locations map.
	for _, loc := range locs {
		path := loc.GetPath()
		pathStr := make([]string, len(path))
		for i, point := range path {
			pathStr[i] = strconv.Itoa(int(point))
		}
		file.locations[strings.Join(pathStr, ",")] = Location{loc: loc}
	}

	// Build messages list.
	for i, msgDesc := range msgs {
		msgDesc.GetName()
		flds := msgDesc.GetField()
		message := Message{
			index:    i,
			desc:     msgDesc,
			parent:   &file,
			Fields:   make([]*Field, 0, len(flds)),
			fieldMap: make(map[string]*Field, len(flds)),
		}
		for j, fld := range flds {
			field := Field{
				index:  j,
				parent: &message,
				desc:   fld,
			}
			message.Fields = append(message.Fields, &field)
			message.fieldMap[field.Name()] = &field
		}
		file.Messages[i] = &message
		file.messageMap[msgDesc] = &message
		file.messageByName[message.Name()] = &message
		forest.messageMap[msgDesc] = &message
	}

	return &file
}

func (file File) Forest() *Forest {
	return file.parent
}

func (file File) Name() string {
	return file.desc.GetName()
}

func (file File) Package() string {
	return file.desc.GetPackage()
}

func (message Message) Name() string {
	return message.desc.GetName()
}

func (message Message) File() *File {
	return message.parent
}

func (message Message) Forest() *Forest {
	return message.parent.Forest()
}

func (message Message) Path() string {
	if message.parentMsg != nil {
		return fmt.Sprintf("%s,4,%d", message.parentMsg.Path(), message.index)
	}
	return fmt.Sprintf("4,%d", message.index)
}

func (message Message) GetField(name string) (*Field, bool) {
	field, ok := message.fieldMap[name]
	return field, ok
}

func (message Message) LeadingComments() string {
	return message.File().locations[message.Path()].LeadingComments()
}

func (field Field) Name() string {
	return field.desc.GetName()
}

func (field Field) Message() *Message {
	return field.parent
}

func (field Field) File() *File {
	return field.parent.File()
}

func (field Field) Forest() *Forest {
	return field.parent.Forest()
}

func (field Field) Path() string {
	return fmt.Sprintf("%s,2,%d", field.parent.Path(), field.index)
}

func (field Field) TypeMessage() *Message {
	if !field.IsMessage() {
		return nil
	}
	forest := field.Forest()
	obj := forest.ObjectNamed(field.TypeName())
	desc, ok := obj.(*gogogen.Descriptor)
	if !ok {
		return nil
	}
	return forest.messageMap[desc.DescriptorProto]
}

func (field Field) TypeName() string {
	return field.desc.GetTypeName()
}

func (field Field) CastType() string {
	return gogoproto.GetCastType(field.desc)
}

func (field Field) CustomType() string {
	return gogoproto.GetCustomType(field.desc)
}

func (field Field) JSONName() string {
	if res := gogoproto.GetJsonTag(field.desc); res != nil {
		return strings.Split(*res, ",")[0]
	}
	if field.desc.JsonName != nil {
		return *field.desc.JsonName
	}
	return ""
}

func (field Field) IsTime() bool {
	isStdTime := gogoproto.IsStdTime(field.desc)
	isGoogleTime := strings.HasSuffix(field.TypeName(), "google.protobuf.Timestamp")
	isCastToTime := field.CastType() == "time.Time"

	return isStdTime || isGoogleTime || isCastToTime
}

// IsDuration returns true if field stores a duration value (protobuf or cast to a standard library type)
func (field Field) IsDuration() bool {
	isStdDuration := gogoproto.IsStdDuration(field.desc)
	isGoogleDuration := strings.HasSuffix(field.TypeName(), "google.protobuf.Duration")
	isCastToCustomDuration := field.CastType() == "Duration"
	isCastToDuration := field.CastType() == "time.Duration"

	return isStdDuration || isGoogleDuration || isCastToDuration || isCastToCustomDuration
}

// IsMap returns true if field stores a map.
func (field Field) IsMap() bool {
	return field.Forest().IsMap(field.desc)
}

// IsMessage returns true if field is a message.
func (field Field) IsMessage() bool {
	return field.desc.IsMessage()
}

func (field Field) IsString() bool {
	return field.desc.IsString()
}

func (field Field) IsBool() bool {
	return field.desc.IsBool() || field.TypeName() == ".types.BoolValue"
}

func (field Field) IsInt64() bool {
	return field.desc.GetType() == gogodesc.FieldDescriptorProto_TYPE_INT64
}

func (field Field) IsUint64() bool {
	return field.desc.GetType() == gogodesc.FieldDescriptorProto_TYPE_UINT64
}

func (field Field) IsInt32() bool {
	return field.desc.GetType() == gogodesc.FieldDescriptorProto_TYPE_INT32
}

func (field Field) IsUint32() bool {
	return field.desc.GetType() == gogodesc.FieldDescriptorProto_TYPE_UINT32
}

func (field Field) IsRepeated() bool {
	return field.desc.IsRepeated()
}

func (field Field) IsRequired() bool {
	return field.desc.IsRequired()
}

func (field Field) IsOptional() bool {
	return field.desc.GetLabel() == gogodesc.FieldDescriptorProto_LABEL_OPTIONAL
}

func (field Field) IsNullable() bool {
	return proto.GetBoolExtension(field.desc.GetOptions(), gogoproto.E_Nullable, true)
}

func (field Field) LeadingComments() string {
	return field.File().locations[field.Path()].LeadingComments()
}

func (location Location) LeadingComments() string {
	lines := strings.Split(location.loc.GetLeadingComments(), "\n")
	n := len(lines)
	if n == 0 {
		return ""
	}
	if lines[n-1] == "" {
		lines = lines[:n-1]
	}
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	return strings.Join(lines, " ")
}

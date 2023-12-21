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

package eventschema

import (
	"strings"

	"github.com/gravitational/trace"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	tree "github.com/gravitational/teleport/build.assets/tooling/lib/protobuf-tree"
)

// SchemaGenerator generates the OpenAPI v3 schema from a proto file.
type SchemaGenerator struct {
	memo  map[string]*Schema
	roots map[string]*Schema
}

// Schema is a set of object properties.
type Schema struct {
	apiextv1.JSONSchemaProps
	built bool
}

func (s *Schema) DeepCopy() *Schema {
	return &Schema{
		JSONSchemaProps: *s.JSONSchemaProps.DeepCopy(),
		built:           s.built,
	}
}

func NewSchemaGenerator() *SchemaGenerator {
	return &SchemaGenerator{
		memo:  make(map[string]*Schema),
		roots: make(map[string]*Schema),
	}
}

func NewSchema(description string) *Schema {
	return &Schema{JSONSchemaProps: apiextv1.JSONSchemaProps{
		Type:        "object",
		Description: description,
		Properties:  make(map[string]apiextv1.JSONSchemaProps),
	}}
}

func (generator *SchemaGenerator) GetRoots() map[string]*Schema {
	return generator.roots
}

func (generator *SchemaGenerator) Process(file *tree.File) error {
	// Get the OneOf message
	rootMsg, ok := file.GetMessageByName("OneOf")
	if !ok {
		return trace.NotFound("resource %q is not found", "OneOf")
	}

	// list all events part of OneOf
	for _, field := range rootMsg.Fields {
		eventMessage := field.TypeMessage()
		schema, err := generator.traverseInner(eventMessage)
		if err != nil {
			return trace.Wrap(err)
		}
		schema.ID = field.Name()
		generator.roots[field.Name()] = schema
	}

	// iterate over all event messages
	// call traverseInner() to build the schemas
	// add the schemas to the roots
	return nil
}

func (generator *SchemaGenerator) traverseInner(message *tree.Message) (*Schema, error) {
	name := message.Name()
	if schema, ok := generator.memo[name]; ok {
		if !schema.built {
			return nil, trace.Errorf("circular dependency in the %s", message.Name())
		}
		return schema, nil
	}
	schema := NewSchema(descriptionFromComment(message.LeadingComments()))
	generator.memo[name] = schema

	for _, field := range message.Fields {
		if _, ok := ignoredFields[message.Name()][field.Name()]; ok {
			continue
		}

		switch jsonName := field.JSONName(); jsonName {
		case "-":
			// We skip this field
		case "":
			// The field is an embedded message, we traverse it and copy its
			// properties to the parent
			if !field.IsMessage() {
				return nil, trace.BadParameter("Embedded field is not a message")
			}
			embeddedSchema, err := generator.traverseInner(field.TypeMessage())
			if err != nil {
				return nil, trace.Wrap(err)
			}
			for propName, prop := range embeddedSchema.Properties {
				schema.Properties[propName] = prop
			}
		default:
			// This is a regular non-embedded field
			var err error
			schema.Properties[jsonName], err = generator.prop(field)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
	}
	schema.built = true

	return schema, nil
}

func (generator *SchemaGenerator) prop(field *tree.Field) (apiextv1.JSONSchemaProps, error) {
	prop := apiextv1.JSONSchemaProps{Description: descriptionFromComment(field.LeadingComments())}

	if field.IsRepeated() && !field.IsMap() {
		prop.Type = "array"
		prop.Items = &apiextv1.JSONSchemaPropsOrArray{
			Schema: &apiextv1.JSONSchemaProps{},
		}
		if err := generator.singularProp(field, prop.Items.Schema); err != nil {
			return prop, trace.Wrap(err)
		}
	} else {
		if err := generator.singularProp(field, &prop); err != nil {
			return prop, trace.Wrap(err)
		}
	}

	if field.IsNullable() && (prop.Type == "array" || prop.Type == "object") {
		prop.Nullable = true
	}

	// Labels are relying on `utils.Strings`, which can either marshall as an array of strings or a single string
	// This does not pass Schema validation from the apiserver, to workaround we don't specify type for those fields
	// and ask Kubernetes to preserve unknown fields.
	if field.CustomType() == "Labels" {
		prop.Type = "object"
		preserveUnknownFields := true
		prop.AdditionalProperties = &apiextv1.JSONSchemaPropsOrBool{
			Schema: &apiextv1.JSONSchemaProps{
				XPreserveUnknownFields: &preserveUnknownFields,
			},
		}
	}

	return prop, nil
}

func (generator *SchemaGenerator) singularProp(field *tree.Field, prop *apiextv1.JSONSchemaProps) error {
	switch {
	case field.IsBool():
		prop.Type = "boolean"
	case field.IsString():
		prop.Type = "string"
	case field.IsDuration():
		prop.Type = "string"
		prop.Format = "duration"
	case field.IsTime():
		prop.Type = "string"
		prop.Format = "date-time"
	case field.IsInt32() || field.IsUint32() || field.IsEnum():
		prop.Type = "integer"
		prop.Format = "int32"
	case field.IsInt64() || field.IsUint64():
		prop.Type = "integer"
		prop.Format = "int64"
	case field.IsBytes():
		// We ignore the bytes?
		prop.Type = "null"
	case field.TypeName() == ".wrappers.LabelValues":
		prop.Type = "object"
		prop.AdditionalProperties = &apiextv1.JSONSchemaPropsOrBool{
			Schema: &apiextv1.JSONSchemaProps{
				Type:  "array",
				Items: &apiextv1.JSONSchemaPropsOrArray{Schema: &apiextv1.JSONSchemaProps{Type: "string"}},
			},
		}
	case field.TypeName() == ".wrappers.StringValues":
		prop.Type = "array"
		prop.Items = &apiextv1.JSONSchemaPropsOrArray{
			Schema: &apiextv1.JSONSchemaProps{Type: "string"},
		}
	case field.TypeName() == ".types.CertExtensionType" || field.TypeName() == ".types.CertExtensionMode":
		prop.Type = "integer"
		prop.Format = "int32"
		// We have no idea of the structure of the Struct item
	case field.TypeName() == ".google.protobuf.Struct":
		prop.Type = "object"
	case strings.HasSuffix(field.TypeName(), ".v1.LoginRule.TraitsMapEntry"):
		prop.Type = "object"
		prop.AdditionalProperties = &apiextv1.JSONSchemaPropsOrBool{
			Schema: &apiextv1.JSONSchemaProps{
				Type:  "array",
				Items: &apiextv1.JSONSchemaPropsOrArray{Schema: &apiextv1.JSONSchemaProps{Type: "string"}},
			},
		}
	case field.IsMessage():
		inner := field.TypeMessage()
		if inner == nil {
			return trace.Errorf("failed to get type for %s.%s", field.Message().Name(), field.Name())
		}
		schema, err := generator.traverseInner(inner)
		if err != nil {
			return trace.Wrap(err)
		}
		prop.Type = "object"
		prop.Properties = schema.Properties
	case field.CastType() != "":
		return trace.Errorf("unsupported casttype %s.%s", field.Message().Name(), field.Name())
	case field.CustomType() != "":
		return trace.Errorf("unsupported customtype %s.%s", field.Message().Name(), field.Name())
	default:
		return trace.Errorf("unsupported %s.%s", field.Message().Name(), field.Name())
	}

	return nil
}

func descriptionFromComment(comment string) string {
	parts := strings.SplitN(comment, " ", 2)
	if len(parts) < 2 {
		return ""
	}
	return strings.TrimRight(parts[1], ". ")
}

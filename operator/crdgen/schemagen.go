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
	"regexp"
	"strings"

	"github.com/gobuffalo/flect"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gravitational/trace"
)

var regexpResourceName = regexp.MustCompile(`^([A-Za-z]+)(V[0-9]+)$`)

// SchemaGenerator generates the OpenAPI v3 schema from a proto file.
type SchemaGenerator struct {
	groupName string
	memo      map[string]*Schema
	roots     map[string]*RootSchema
}

// RootSchema is a wrapper for a message we are generating a schema for.
type RootSchema struct {
	groupName  string
	versions   map[string]*Schema
	name       string
	pluralName string
	kind       string
}

// Schema is a set of object properties.
type Schema struct {
	apiextv1.JSONSchemaProps
	built bool
}

func NewSchemaGenerator(groupName string) *SchemaGenerator {
	return &SchemaGenerator{
		groupName: groupName,
		memo:      make(map[string]*Schema),
		roots:     make(map[string]*RootSchema),
	}
}

func NewSchema() *Schema {
	return &Schema{JSONSchemaProps: apiextv1.JSONSchemaProps{
		Type:       "object",
		Properties: make(map[string]apiextv1.JSONSchemaProps),
	}}
}

func (generator *SchemaGenerator) addResource(file *File, name string) error {
	rootMsg, ok := file.messageByName[name]
	if !ok {
		return trace.NotFound("resource %q is not found", name)
	}

	specField, ok := rootMsg.GetField("Spec")
	if !ok {
		return trace.NotFound("message %q does not have Spec field", name)
	}

	specMsg := specField.TypeMessage()
	if specMsg == nil {
		return trace.NotFound("message %q Spec type is not a message", name)
	}

	schema, err := generator.traverseInner(specMsg)
	if err != nil {
		return trace.Wrap(err)
	}
	resourceKind, resourceVersion, err := parseKindAndVersion(rootMsg)
	if err != nil {
		return trace.Wrap(err)
	}
	schema.Description = fmt.Sprintf("%s resource definition %s from Teleport", resourceKind, resourceVersion)

	root, ok := generator.roots[resourceKind]
	if !ok {
		root = &RootSchema{
			groupName:  generator.groupName,
			versions:   make(map[string]*Schema),
			kind:       resourceKind,
			name:       strings.ToLower(resourceKind),
			pluralName: strings.ToLower(flect.Pluralize(resourceKind)),
		}
		generator.roots[resourceKind] = root
	}

	root.versions[resourceVersion] = schema

	return nil
}

func parseKindAndVersion(message *Message) (string, string, error) {
	msgName := message.Name()
	res := regexpResourceName.FindStringSubmatch(msgName)
	if len(res) == 0 {
		return "", "", trace.Errorf("failed to parse resource name and version from %s message name", msgName)
	}
	return res[1], strings.ToLower(res[2]), nil
}

func (generator *SchemaGenerator) traverseInner(message *Message) (*Schema, error) {
	name := message.Name()
	if schema, ok := generator.memo[name]; ok {
		if !schema.built {
			return nil, trace.Errorf("circular dependency in the %s", message.Name())
		}
		return schema, nil
	}
	schema := NewSchema()
	generator.memo[name] = schema

	for _, field := range message.Fields {
		if ignoredFields[message.Name()].Contains(field.Name()) {
			continue
		}

		jsonName := field.JSONName()
		if jsonName == "" {
			return nil, trace.Errorf("empty json tag for %s.%s", message.Name(), field.Name())
		}
		if jsonName == "-" {
			continue
		}

		prop := apiextv1.JSONSchemaProps{Description: field.LeadingComments()}

		if field.IsRepeated() {
			prop.Type = "array"
			prop.Items = &apiextv1.JSONSchemaPropsOrArray{
				Schema: &apiextv1.JSONSchemaProps{},
			}
			generator.singularProp(field, prop.Items.Schema)
		} else {
			generator.singularProp(field, &prop)
		}

		if field.IsNullable() && (prop.Type == "array" || prop.Type == "object") {
			prop.Nullable = true
		}

		schema.Properties[jsonName] = prop
	}
	schema.built = true

	return schema, nil
}

func (generator *SchemaGenerator) singularProp(field *Field, prop *apiextv1.JSONSchemaProps) error {
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
	case field.IsInt32() || field.IsUint32():
		prop.Type = "integer"
		prop.Format = "int32"
	case field.IsInt64() || field.IsUint64():
		prop.Type = "integer"
		prop.Format = "int64"
	case field.TypeName() == ".wrappers.LabelValues":
		prop.Type = "object"
		prop.AdditionalProperties = &apiextv1.JSONSchemaPropsOrBool{
			Schema: &apiextv1.JSONSchemaProps{
				Type:  "array",
				Items: &apiextv1.JSONSchemaPropsOrArray{Schema: &apiextv1.JSONSchemaProps{Type: "string"}},
			},
		}
	case field.TypeName() == ".types.CertExtensionType" || field.TypeName() == ".types.CertExtensionMode":
		prop.Type = "integer"
		prop.Format = "int32"
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
	case field.IsMap():
		return trace.Errorf("maps are not supported %s.%s", field.Message().Name(), field.Name())
	default:
		return trace.Errorf("unsupported %s.%s", field.Message().Name(), field.Name())
	}

	return nil
}

func (root RootSchema) CustomResourceDefinition() apiextv1.CustomResourceDefinition {
	crd := apiextv1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiextv1.SchemeGroupVersion.String(),
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s.%s", root.pluralName, root.groupName),
		},
		Spec: apiextv1.CustomResourceDefinitionSpec{
			Group: root.groupName,
			Names: apiextv1.CustomResourceDefinitionNames{
				Kind:     root.kind,
				ListKind: root.kind + "List",
				Plural:   root.pluralName,
				Singular: root.name,
			},
			Scope: apiextv1.NamespaceScoped,
		},
	}
	for versionName, schema := range root.versions {
		crd.Spec.Versions = append(crd.Spec.Versions, apiextv1.CustomResourceDefinitionVersion{
			Name:    versionName,
			Served:  true,
			Storage: true,
			Subresources: &apiextv1.CustomResourceSubresources{
				Status: &apiextv1.CustomResourceSubresourceStatus{},
			},
			Schema: &apiextv1.CustomResourceValidation{
				OpenAPIV3Schema: &apiextv1.JSONSchemaProps{
					Type:        "object",
					Description: fmt.Sprintf("%s is the Schema for the %s API", root.kind, root.pluralName),
					Properties: map[string]apiextv1.JSONSchemaProps{
						"apiVersion": {
							Type:        "string",
							Description: "APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources",
						},
						"kind": {
							Type:        "string",
							Description: "Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds",
						},
						"metadata": {Type: "object"},
						"spec":     schema.JSONSchemaProps,
						"status": {
							Type:        "object",
							Description: fmt.Sprintf("%s resource status", root.kind),
							Properties: map[string]apiextv1.JSONSchemaProps{
								"lastError": {
									Type: "object",
									Properties: map[string]apiextv1.JSONSchemaProps{
										"kind": {
											Type:        "string",
											Description: "Error kind",
										},
										"text": {
											Type:        "string",
											Description: "Error text",
										},
									},
								},
							},
						},
					},
				},
			},
		})
	}
	return crd
}

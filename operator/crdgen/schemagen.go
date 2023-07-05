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

	"github.com/dustin/go-humanize/english"
	"github.com/gravitational/trace"
	"golang.org/x/exp/slices"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crdtools "sigs.k8s.io/controller-tools/pkg/crd"
	crdmarkers "sigs.k8s.io/controller-tools/pkg/crd/markers"
	"sigs.k8s.io/controller-tools/pkg/loader"
	"sigs.k8s.io/controller-tools/pkg/markers"
)

const k8sKindPrefix = "Teleport"

// Add names to this array when adding support to new Teleport resources that could conflict with Kubernetes
var kubernetesReservedNames = []string{"role"}
var regexpResourceName = regexp.MustCompile(`^([A-Za-z]+)(V[0-9]+)?$`)

// SchemaGenerator generates the OpenAPI v3 schema from a proto file.
type SchemaGenerator struct {
	groupName string
	memo      map[string]*Schema
	roots     map[string]*RootSchema
}

// RootSchema is a wrapper for a message we are generating a schema for.
type RootSchema struct {
	groupName  string
	versions   []SchemaVersion
	name       string
	pluralName string
	kind       string
}

type SchemaVersion struct {
	Version string
	Schema  *Schema
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

type resourceSchemaConfig struct {
	versionOverride  string
	customSpecFields []string
}

type resourceSchemaOption func(*resourceSchemaConfig)

func withVersionOverride(version string) resourceSchemaOption {
	return func(cfg *resourceSchemaConfig) {
		cfg.versionOverride = version
	}
}

func withCustomSpecFields(customSpecFields []string) resourceSchemaOption {
	return func(cfg *resourceSchemaConfig) {
		cfg.customSpecFields = customSpecFields
	}
}

func (generator *SchemaGenerator) addResource(file *File, name string, opts ...resourceSchemaOption) error {
	var cfg resourceSchemaConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	rootMsg, ok := file.messageByName[name]
	if !ok {
		return trace.NotFound("resource %q is not found", name)
	}

	var schema *Schema
	if len(cfg.customSpecFields) > 0 {
		schema = NewSchema()
		for _, fieldName := range cfg.customSpecFields {
			field, ok := rootMsg.GetField(fieldName)
			if !ok {
				return trace.NotFound("field %q not found", fieldName)
			}
			var err error
			schema.Properties[fieldName], err = generator.prop(field)
			if err != nil {
				return trace.Wrap(err)
			}
		}
	} else {
		specField, ok := rootMsg.GetField("Spec")
		if !ok {
			return trace.NotFound("message %q does not have Spec field", name)
		}

		specMsg := specField.TypeMessage()
		if specMsg == nil {
			return trace.NotFound("message %q Spec type is not a message", name)
		}

		var err error
		schema, err = generator.traverseInner(specMsg)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	schema = schema.DeepCopy()
	resourceKind, resourceVersion, err := parseKindAndVersion(rootMsg)
	if err != nil {
		return trace.Wrap(err)
	}
	if cfg.versionOverride != "" {
		resourceVersion = cfg.versionOverride
	}
	schema.Description = fmt.Sprintf("%s resource definition %s from Teleport", resourceKind, resourceVersion)

	root, ok := generator.roots[resourceKind]
	if !ok {
		root = &RootSchema{
			groupName:  generator.groupName,
			kind:       resourceKind,
			name:       strings.ToLower(resourceKind),
			pluralName: strings.ToLower(english.PluralWord(2, resourceKind, "")),
		}
		generator.roots[resourceKind] = root
	}
	root.versions = append(root.versions, SchemaVersion{
		Version: resourceVersion,
		Schema:  schema,
	})

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
		if _, ok := ignoredFields[message.Name()][field.Name()]; ok {
			continue
		}

		jsonName := field.JSONName()
		if jsonName == "" {
			return nil, trace.Errorf("empty json tag for %s.%s", message.Name(), field.Name())
		}
		if jsonName == "-" {
			continue
		}

		var err error
		schema.Properties[jsonName], err = generator.prop(field)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	schema.built = true

	return schema, nil
}

func (generator *SchemaGenerator) prop(field *Field) (apiextv1.JSONSchemaProps, error) {
	prop := apiextv1.JSONSchemaProps{Description: field.LeadingComments()}

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
	case field.IsInt32() || field.IsUint32() || field.desc.IsEnum():
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
	case field.TypeName() == ".wrappers.StringValues":
		prop.Type = "array"
		prop.Items = &apiextv1.JSONSchemaPropsOrArray{
			Schema: &apiextv1.JSONSchemaProps{Type: "string"},
		}
	case field.TypeName() == ".types.CertExtensionType" || field.TypeName() == ".types.CertExtensionMode":
		prop.Type = "integer"
		prop.Format = "int32"
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

func (root RootSchema) CustomResourceDefinition() apiextv1.CustomResourceDefinition {
	crd := apiextv1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiextv1.SchemeGroupVersion.String(),
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s.%s", strings.ToLower(k8sKindPrefix+root.pluralName), root.groupName),
		},
		Spec: apiextv1.CustomResourceDefinitionSpec{
			Group: root.groupName,
			Names: apiextv1.CustomResourceDefinitionNames{
				Kind:       k8sKindPrefix + root.kind,
				ListKind:   k8sKindPrefix + root.kind + "List",
				Plural:     strings.ToLower(k8sKindPrefix + root.pluralName),
				Singular:   strings.ToLower(k8sKindPrefix + root.name),
				ShortNames: root.getShortNames(),
			},
			Scope: apiextv1.NamespaceScoped,
		},
	}

	// This part parses the types not coming from the protobuf (the status)
	// We instantiate a parser, load the relevant packages in it and look for
	// the package we need. The package is then loaded to the parser, a schema is
	// generated and used in the CRD

	registry := &markers.Registry{}
	// CRD markers contain special markers used by the parser to discover properties
	// e.g. `+kubebuilder:validation:Minimum=0`
	crdmarkers.Register(registry)
	parser := &crdtools.Parser{
		Collector: &markers.Collector{Registry: registry},
		Checker:   &loader.TypeChecker{},
	}

	// Some types are special and require manual overrides, like metav1.Time.
	crdtools.AddKnownTypes(parser)

	pkgs, err := loader.LoadRoots("github.com/gravitational/teleport/operator/apis/...")
	if err != nil {
		fmt.Printf("parser error: %s", err)
	}

	for i, schemaVersion := range root.versions {

		var statusType crdtools.TypeIdent
		versionName := schemaVersion.Version
		schema := schemaVersion.Schema
		for _, pkg := range pkgs {
			// This if is a bit janky, condition checking should be stronger
			if pkg.Name == versionName {
				parser.NeedPackage(pkg)
				statusType = crdtools.TypeIdent{
					Package: pkg,
					Name:    fmt.Sprintf("%s%sStatus", k8sKindPrefix, root.kind),
				}
				// Kubernetes CRDs don't support $ref in openapi schemas, we need a flattened schema
				parser.NeedFlattenedSchemaFor(statusType)
			}
		}

		crd.Spec.Versions = append(crd.Spec.Versions, apiextv1.CustomResourceDefinitionVersion{
			Name:   versionName,
			Served: true,
			// Storage the first version available.
			Storage: i == 0,
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
						"status":   parser.FlattenedSchemata[statusType],
					},
				},
			},
		})
	}
	return crd
}

// getShortNames returns the schema short names while ensuring they won't conflict with existing Kubernetes resources
// See https://github.com/gravitational/teleport/issues/17587 and https://github.com/kubernetes/kubernetes/issues/113227
func (root RootSchema) getShortNames() []string {
	if slices.Contains(kubernetesReservedNames, root.name) {
		return []string{}
	}
	return []string{root.name, root.pluralName}
}

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

package main

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/dustin/go-humanize/english"
	"github.com/gravitational/trace"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crdtools "sigs.k8s.io/controller-tools/pkg/crd"
	crdmarkers "sigs.k8s.io/controller-tools/pkg/crd/markers"
	"sigs.k8s.io/controller-tools/pkg/loader"
	"sigs.k8s.io/controller-tools/pkg/markers"
)

const (
	k8sKindPrefix     = "Teleport"
	statusPackagePath = "github.com/gravitational/teleport/integrations/operator/apis"
	statusPackageName = "resources"
	statusPackage     = statusPackagePath + "/" + statusPackageName
	statusTypeName    = "Status"
)

// Add names to this array when adding support to new Teleport resources that could conflict with Kubernetes
var (
	kubernetesReservedNames = []string{"role"}
	regexpResourceName      = regexp.MustCompile(`^([A-Za-z]+)(V[0-9]+)?$`)
)

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
	// teleportKind is the kind of the Teleport resource
	teleportKind string
	// kubernetesKind is the kind of the Kubernetes resource. This is the
	// teleportKind, prefixed by "Teleport" and potentially suffixed by the
	// version. Since v15, resources with multiple versions are exposed through
	// different kinds. At some point we will suffix all kinds by the version
	// and deprecate the old resources.
	kubernetesKind string
}

type SchemaVersion struct {
	// Version is the Kubernetes CR API version. For single-version
	// Teleport resource, this is equal to the Teleport resource Version for
	// compatibility purposes. For multi-version resource, the value is always
	// "v1" as the version is already in the CR kind.
	Version           string
	Schema            *Schema
	additionalColumns []apiextv1.CustomResourceColumnDefinition
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
	nameOverride        string
	versionOverride     string
	customSpecFields    []string
	kindContainsVersion bool
	additionalColumns   []apiextv1.CustomResourceColumnDefinition
}

type resourceSchemaOption func(*resourceSchemaConfig)

func withVersionOverride(version string) resourceSchemaOption {
	return func(cfg *resourceSchemaConfig) {
		cfg.versionOverride = version
	}
}

func withNameOverride(name string) resourceSchemaOption {
	return func(cfg *resourceSchemaConfig) {
		cfg.nameOverride = name
	}
}

// set this onlt on new multi-version resources
func withVersionInKindOverride() resourceSchemaOption {
	return func(cfg *resourceSchemaConfig) {
		cfg.kindContainsVersion = true
	}
}

func withCustomSpecFields(customSpecFields []string) resourceSchemaOption {
	return func(cfg *resourceSchemaConfig) {
		cfg.customSpecFields = customSpecFields
	}
}

var ageColumn = apiextv1.CustomResourceColumnDefinition{
	Name:        "Age",
	Type:        "date",
	Description: "The age of this resource",
	JSONPath:    ".metadata.creationTimestamp",
}

func withAdditionalColumns(additionalColumns []apiextv1.CustomResourceColumnDefinition) resourceSchemaOption {
	// We add the age column back (it's removed if we set additional columns for the CRD).
	// See https://github.com/kubernetes/kubectl/issues/903#issuecomment-669244656.
	columns := make([]apiextv1.CustomResourceColumnDefinition, len(additionalColumns)+1)
	copy(columns, additionalColumns)
	columns[len(additionalColumns)] = ageColumn

	return func(cfg *resourceSchemaConfig) {
		cfg.additionalColumns = columns
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
		// We check both "Spec" with a capital S, and "spec" in lower case.
		specField, ok := rootMsg.GetField("Spec")
		if !ok {
			specField, ok = rootMsg.GetField("spec")
			if !ok {
				return trace.NotFound("message %q does not have Spec field", name)
			}
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
	if cfg.nameOverride != "" {
		resourceKind = cfg.nameOverride
	}
	kubernetesKind := resourceKind
	if cfg.kindContainsVersion {
		kubernetesKind = resourceKind + strings.ToUpper(resourceVersion)
	}
	schema.Description = fmt.Sprintf("%s resource definition %s from Teleport", resourceKind, resourceVersion)

	root, ok := generator.roots[kubernetesKind]
	if !ok {
		pluralName := strings.ToLower(english.PluralWord(2, resourceKind, ""))
		if cfg.kindContainsVersion {
			pluralName = pluralName + resourceVersion
		}
		root = &RootSchema{
			groupName:      generator.groupName,
			teleportKind:   resourceKind,
			kubernetesKind: kubernetesKind,
			name:           strings.ToLower(kubernetesKind),
			pluralName:     pluralName,
		}
		generator.roots[kubernetesKind] = root
	}

	// For legacy CRs with a single version, we use the Teleport version as the
	// Kubernetes API version
	kubernetesVersion := resourceVersion
	if cfg.kindContainsVersion {
		// For new multi-version resources we always set the version to "v1" as
		// the Teleport version is also in the CR kind.
		kubernetesVersion = "v1"
	}
	root.versions = append(root.versions, SchemaVersion{
		Version:           kubernetesVersion,
		Schema:            schema,
		additionalColumns: cfg.additionalColumns,
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
			handled := handleEmptyJSONTag(schema, message, field)
			if !handled {
				return nil, trace.Errorf("empty json tag for %s.%s", message.Name(), field.Name())
			}
			continue
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

// handleEmptyJSONTag attempts to handle special case fields that have
// an empty JSON tag. True is returned if the field was handled and a
// new schema property was created.
func handleEmptyJSONTag(schema *Schema, message *Message, field *Field) bool {
	if field.Name() != "MaxAge" && message.Name() != "OIDCConnectorSpecV3" {
		return false
	}

	// Handle MaxAge as a special case. It's type is a message that is embedded.
	// Because the message is embedded, MaxAge itself explicitly sets its json
	// name to an empty string, but the embedded message type has a single field
	// with a json name, so use that instead.
	schema.Properties["max_age"] = apiextv1.JSONSchemaProps{
		Description: field.LeadingComments(),
		Type:        "string",
		Format:      "duration",
	}

	return true
}

func (generator *SchemaGenerator) prop(field *Field) (apiextv1.JSONSchemaProps, error) {
	prop := apiextv1.JSONSchemaProps{Description: field.LeadingComments()}

	// Known overrides: we broke the link between the go struct and the protobuf message.
	// As we have no guarantee they're identical anymore (they are not) we need
	// to manually maintain a list of mappings. This is not maintainable on the
	// long term and this defeats the purpose of the generators, but we didn't
	// have the time yet to revamp this.

	// Traits are represented as map[string][]string in go,
	// and as []struct{key string, values []string} in protobuf.
	if field.IsRepeated() && field.TypeName() == ".teleport.trait.v1.Trait" {
		prop.Type = "object"
		prop.AdditionalProperties = &apiextv1.JSONSchemaPropsOrBool{
			Schema: &apiextv1.JSONSchemaProps{
				Type:  "array",
				Items: &apiextv1.JSONSchemaPropsOrArray{Schema: &apiextv1.JSONSchemaProps{Type: "string"}},
			},
		}
		return prop, nil
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
		return prop, nil
	}

	// Regular treatment
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
	case field.IsInt32() || field.IsUint32():
		prop.Type = "integer"
		prop.Format = "int32"
	case field.desc.IsEnum():
		prop.XIntOrString = true
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

func (root RootSchema) CustomResourceDefinition() (apiextv1.CustomResourceDefinition, error) {
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
				Kind:       k8sKindPrefix + root.kubernetesKind,
				ListKind:   k8sKindPrefix + root.kubernetesKind + "List",
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
	err := crdmarkers.Register(registry)
	if err != nil {
		return apiextv1.CustomResourceDefinition{},
			trace.Wrap(err, "adding CRD markers to the registry")
	}
	parser := &crdtools.Parser{
		Collector: &markers.Collector{Registry: registry},
		Checker:   &loader.TypeChecker{},
	}

	// Some types are special and require manual overrides, like metav1.Time.
	crdtools.AddKnownTypes(parser)

	// Status does not exist in Teleport, only in the CR.
	// We parse go's AST to find its struct and convert it in a schema.
	statusSchema, err := getStatusSchema(parser)
	if err != nil {
		return apiextv1.CustomResourceDefinition{},
			trace.Wrap(err, "getting status schema from go's AST")
	}

	for i, schemaVersion := range root.versions {

		schema := schemaVersion.Schema

		crd.Spec.Versions = append(crd.Spec.Versions, apiextv1.CustomResourceDefinitionVersion{
			Name:   schemaVersion.Version,
			Served: true,
			// Storage the first version available.
			Storage: i == 0,
			Subresources: &apiextv1.CustomResourceSubresources{
				Status: &apiextv1.CustomResourceSubresourceStatus{},
			},
			Schema: &apiextv1.CustomResourceValidation{
				OpenAPIV3Schema: &apiextv1.JSONSchemaProps{
					Type:        "object",
					Description: fmt.Sprintf("%s is the Schema for the %s API", root.kubernetesKind, root.pluralName),
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
						"status":   statusSchema,
					},
				},
			},
			AdditionalPrinterColumns: schemaVersion.additionalColumns,
		})
	}
	return crd, nil
}

// getShortNames returns the schema short names while ensuring they won't conflict with existing Kubernetes resources
// See https://github.com/gravitational/teleport/issues/17587 and https://github.com/kubernetes/kubernetes/issues/113227
func (root RootSchema) getShortNames() []string {
	if slices.Contains(kubernetesReservedNames, root.name) {
		return []string{}
	}
	return []string{root.name, root.pluralName}
}

func getStatusSchema(parser *crdtools.Parser) (apiextv1.JSONSchemaProps, error) {
	pkgs, err := loader.LoadRoots(statusPackage)
	if err != nil {
		// Loader errors might be non-critical.
		// e.g. the loader complains about the unknown "toolchain" directive in our go mod
		fmt.Printf("loader error: %s", err)
	}
	var statusType crdtools.TypeIdent
	for _, pkg := range pkgs {
		if pkg.Name == "resources" {
			parser.NeedPackage(pkg)
			statusType = crdtools.TypeIdent{
				Package: pkg,
				Name:    statusTypeName,
			}
			parser.NeedFlattenedSchemaFor(statusType)
			return parser.FlattenedSchemata[statusType], nil
		}
	}
	return apiextv1.JSONSchemaProps{}, trace.NotFound("Package %q not found, cannot generate status JSON Schema", statusPackage)
}

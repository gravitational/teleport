package schemagen

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/dustin/go-humanize/english"
	gogodesc "github.com/gogo/protobuf/protoc-gen-gogo/descriptor"
	"github.com/gogo/protobuf/protoc-gen-gogo/generator"
	gogoplugin "github.com/gogo/protobuf/protoc-gen-gogo/plugin"
	"github.com/gravitational/trace"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

type stringSet map[string]struct{}

var regexpResourceName = regexp.MustCompile(`^([A-Za-z]+)(V[0-9]+)$`)

func NewGenerator(req *gogoplugin.CodeGeneratorRequest) (*Forest, error) {
	gen := generator.New()

	gen.Request = req
	gen.CommandLineParameters(gen.Request.GetParameter())
	gen.WrapTypes()
	gen.SetPackageNames()
	gen.BuildTypeNameMap()

	return &Forest{
		Generator:  gen,
		messageMap: make(map[*gogodesc.DescriptorProto]*Message),
	}, nil
}

// SchemaGenerator generates the OpenAPI v3 schema from a proto file.
type SchemaGenerator struct {
	memo  map[string]*Schema
	Roots map[string]*RootSchema
}

// RootSchema is a wrapper for a message we are generating a schema for.
type RootSchema struct {
	Versions   []SchemaVersion
	Name       string
	PluralName string
	Kind       string
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

func NewSchemaGenerator() *SchemaGenerator {
	return &SchemaGenerator{
		memo:  make(map[string]*Schema),
		Roots: make(map[string]*RootSchema),
	}
}

func NewSchema() *Schema {
	return &Schema{JSONSchemaProps: apiextv1.JSONSchemaProps{
		Type:       "object",
		Properties: make(map[string]apiextv1.JSONSchemaProps),
	}}
}

// ParseResourceOptions configures the way a SchemaGenerator parses a specific
// resource.
type ParseResourceOptions struct {
	// A version to assign to a resource instead of the version extracted from
	// the resource's corresponding message
	VersionOverride string

	// Names of fields to ignore when parsing the resource's corresponding
	// message
	IgnoredFields []string
}

func (generator *SchemaGenerator) ParseResource(file *File, name string, opts ParseResourceOptions) error {
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

	i := make(stringSet)
	for _, f := range opts.IgnoredFields {
		i[f] = struct{}{}
	}

	schema, err := generator.traverseInner(specMsg, i)
	if err != nil {
		return trace.Wrap(err)
	}
	schema = schema.DeepCopy()
	resourceKind, resourceVersion, err := parseKindAndVersion(rootMsg)
	if err != nil {
		return trace.Wrap(err)
	}
	if opts.VersionOverride != "" {
		resourceVersion = opts.VersionOverride
	}
	schema.Description = fmt.Sprintf("%s resource definition %s from Teleport", resourceKind, resourceVersion)

	root, ok := generator.Roots[resourceKind]
	if !ok {
		root = &RootSchema{
			Kind:       resourceKind,
			Name:       strings.ToLower(resourceKind),
			PluralName: strings.ToLower(english.PluralWord(2, resourceKind, "")),
		}
		generator.Roots[resourceKind] = root
	}
	root.Versions = append(root.Versions, SchemaVersion{
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

func (generator *SchemaGenerator) traverseInner(message *Message, ignoredFields stringSet) (*Schema, error) {
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
		if _, ok := ignoredFields[field.Name()]; ok {
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
			generator.singularProp(field, prop.Items.Schema, ignoredFields)
		} else {
			generator.singularProp(field, &prop, ignoredFields)
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

		schema.Properties[jsonName] = prop
	}
	schema.built = true

	return schema, nil
}

func (generator *SchemaGenerator) singularProp(field *Field, prop *apiextv1.JSONSchemaProps, ignoredFields stringSet) error {
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
	case field.IsMessage():
		inner := field.TypeMessage()
		if inner == nil {
			return trace.Errorf("failed to get type for %s.%s", field.Message().Name(), field.Name())
		}
		schema, err := generator.traverseInner(inner, ignoredFields)
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

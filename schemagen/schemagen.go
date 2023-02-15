package schemagen

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/dustin/go-humanize/english"
	gogodesc "github.com/gogo/protobuf/protoc-gen-gogo/descriptor"
	"github.com/gogo/protobuf/protoc-gen-gogo/generator"
	gogoplugin "github.com/gogo/protobuf/protoc-gen-gogo/plugin"
	"github.com/gogo/protobuf/vanity/command"
	"github.com/gravitational/trace"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

type stringSet map[string]struct{}

var regexpResourceName = regexp.MustCompile(`^([A-Za-z]+)(V[0-9]+)$`)

func newGenerator(req *gogoplugin.CodeGeneratorRequest) (*Forest, error) {
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

// schemaCollection includes the  OpenAPI v3 schemas parsed from a single proto
// file.
type schemaCollection struct {
	// Map of message names to schemas we have already visited. Used for
	// detecting dependency cycles.
	memo map[string]*Schema
	// Map of resource kinds to RootSchemas
	roots map[string]*RootSchema
}

// TransformedFile includes the name and content of a protobuf file transformed
// by your custom plugin.
type TransformedFile struct {
	Name    string
	Content string
}

func generateSchema(
	file *File,
	config []ParseResourceOptions,
	transformer TransformerFunc,
	resp *gogoplugin.CodeGeneratorResponse,
) error {
	generator := newSchemaCollection()

	for _, c := range config {
		if err := generator.parseResource(file, c); err != nil {
			return trace.Wrap(err)
		}
	}

	tf := make([]*TransformedFile, len(generator.roots))
	var i int
	for _, root := range generator.roots {
		file, err := transformer(root)
		if err != nil {
			return trace.Wrap(err)
		}
		tf[i] = file
		i++
	}

	for _, f := range tf {
		if f.Name == "" || f.Content == "" {
			return trace.Wrap(errors.New("all transformed files must include a name and content"))
		}
		resp.File = append(resp.File, &gogoplugin.CodeGeneratorResponse_File{Name: &f.Name, Content: &f.Content})
	}

	return nil
}

// TransformerFunc uses the provided *SchemaCollection generate the content for
// zero or more files to write out, including their names and contents. It
// returns an error if it fails to generate output.
type TransformerFunc func(c *RootSchema) (*TransformedFile, error)

// RunPlugin loads proto files from standard input, then uses the provided
// config to parse resource definitions from the proto files in OpenAPI v3
// format. The provided transformer
func RunPlugin(
	config []ParseResourceOptions,
	transformer TransformerFunc) error {

	req := command.Read()

	if len(req.FileToGenerate) == 0 {
		return trace.Wrap(errors.New("no input file provided"))
	}
	if len(req.FileToGenerate) > 1 {
		return trace.Wrap(errors.New("too many input files"))
	}

	gen, err := newGenerator(req)
	if err != nil {
		return trace.Wrap(err)
	}

	rootFileName := req.FileToGenerate[0]
	gen.SetFile(rootFileName)
	for _, fileDesc := range gen.AllFiles().File {
		file := gen.AddFile(fileDesc)
		if fileDesc.GetName() == rootFileName {
			if err := generateSchema(file, config, transformer, gen.Response); err != nil {
				return trace.Wrap(err)
			}
		}
	}

	command.Write(gen.Response)

	return nil
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

func (s *Schema) deepCopy() *Schema {
	return &Schema{
		JSONSchemaProps: *s.JSONSchemaProps.DeepCopy(),
		built:           s.built,
	}
}

func newSchemaCollection() *schemaCollection {
	return &schemaCollection{
		memo:  make(map[string]*Schema),
		roots: make(map[string]*RootSchema),
	}
}

func newSchema() *Schema {
	return &Schema{JSONSchemaProps: apiextv1.JSONSchemaProps{
		Type:       "object",
		Properties: make(map[string]apiextv1.JSONSchemaProps),
	}}
}

// ParseResourceOptions configures the way a SchemaGenerator parses a specific
// resource.
type ParseResourceOptions struct {
	// The name of a resource to extract from a proto file
	Name string
	// A version to assign to a resource instead of the version extracted from
	// the resource's corresponding message
	VersionOverride string

	// Names of fields to ignore when parsing the resource's corresponding
	// message
	IgnoredFields []string
}

func (generator *schemaCollection) parseResource(file *File, opts ParseResourceOptions) error {
	rootMsg, ok := file.messageByName[opts.Name]
	if !ok {
		return trace.NotFound("resource %q is not found", opts.Name)
	}

	specField, ok := rootMsg.GetField("Spec")
	if !ok {
		return trace.NotFound("message %q does not have Spec field", opts.Name)
	}

	specMsg := specField.TypeMessage()
	if specMsg == nil {
		return trace.NotFound("message %q Spec type is not a message", opts.Name)
	}

	i := make(stringSet)
	for _, f := range opts.IgnoredFields {
		i[f] = struct{}{}
	}

	schema, err := generator.traverseInner(specMsg, i)
	if err != nil {
		return trace.Wrap(err)
	}
	schema = schema.deepCopy()
	resourceKind, resourceVersion, err := parseKindAndVersion(rootMsg)
	if err != nil {
		return trace.Wrap(err)
	}
	if opts.VersionOverride != "" {
		resourceVersion = opts.VersionOverride
	}
	schema.Description = fmt.Sprintf("%s resource definition %s from Teleport", resourceKind, resourceVersion)

	root, ok := generator.roots[resourceKind]
	if !ok {
		root = &RootSchema{
			Kind:       resourceKind,
			Name:       strings.ToLower(resourceKind),
			PluralName: strings.ToLower(english.PluralWord(2, resourceKind, "")),
		}
		generator.roots[resourceKind] = root
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

func (generator *schemaCollection) traverseInner(message *Message, ignoredFields stringSet) (*Schema, error) {
	name := message.Name()
	if schema, ok := generator.memo[name]; ok {
		if !schema.built {
			return nil, trace.Errorf("circular dependency in the %s", message.Name())
		}
		return schema, nil
	}
	schema := newSchema()
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

func (generator *schemaCollection) singularProp(field *Field, prop *apiextv1.JSONSchemaProps, ignoredFields stringSet) error {
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

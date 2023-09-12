package resource

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"regexp"
	"strings"
)

type Resource struct {
	SectionName string
	Description string
	SourcePath  string
	Fields      []Field
	YAMLExample string
}

type Field struct {
	Name        string
	Description string
	Type        string
}

type yamlKind int

const (
	unknownKind yamlKind = iota
	sequenceKind
	mappingKind
	stringKind
	numberKind
	boolKind
)

// rawField contains information about a struct field required for downstream
// processing. The intention is to limit raw AST handling to as small a part of
// the source as possible.
type rawField struct {
	doc  string
	kind yamlKindNode
	name string
	// struct tag expression for the field
	tags string
}

// rawNamedStruct contains information about a struct field required for
// downstream processing. The intention is to limit raw AST handling to as small
// a part of the source as possible.
type rawNamedStruct struct {
	doc    string
	name   string
	fields []rawField
}

// yamlKindNode represents a node in a potentially recursive YAML type, such as
// an integer, a map of integers to strings, a sequence of maps of strings to
// strings, etc. Used for printing example YAML documents and tables of fields.
// This is not intended to be a comprehensive YAML AST.
type yamlKindNode struct {
	kind     yamlKind
	children []yamlKindNode
}

// getRawNamedStruct returns the type spec to use for further processing. Returns an
// error if there is either no type spec or more than one.
func getRawNamedStruct(decl *ast.GenDecl) (rawNamedStruct, error) {
	if len(decl.Specs) == 0 {
		return rawNamedStruct{}, errors.New("declaration has no specs")
	}

	// Name the section after the first type declaration found. We expect
	// there to be one type spec.
	var t *ast.TypeSpec
	for _, s := range decl.Specs {
		ts, ok := s.(*ast.TypeSpec)
		if !ok {
			continue
		}
		if t != nil {
			return rawNamedStruct{}, errors.New("declaration contains more than one type spec")
		}
		t = ts
	}

	if t == nil {
		return rawNamedStruct{}, errors.New("no type spec found")
	}

	str, ok := t.Type.(*ast.StructType)
	if !ok {
		return rawNamedStruct{}, errors.New("the declaration is not a struct")
	}

	var rawFields []rawField

	for _, field := range str.Fields.List {
		f, err := makeRawField(field)
		// We shouldn't skip fields at this point since we are only
		// representing essential information about each field.
		// Downstream consumers decide whether to skip a field.
		if err != nil {
			return rawNamedStruct{}, err
		}

		rawFields = append(rawFields, f)
	}

	result := rawNamedStruct{
		name: t.Name.Name,
		// Preserving newlines for downstream processing
		doc:    decl.Doc.Text(),
		fields: rawFields,
	}

	return result, nil
}

// makeYAMLExample creates an example YAML document illustrating the fields
// within the declaration.
func makeYAMLExample(fields []rawField) (string, error) {
	// Write part of a potentially complex type to the YAML example.
	// Assumes that the part will be on the same line as its predecessor.
	addNodeToExample := func(example bytes.Buffer, node yamlKindNode) error {
		// TODO: In the recursive function:
		// TODO: handle custom fields per the "Custom fields" section of the RFD
		// TODO: handle predeclared composite types per the relevant section of the
		// RFD.
		// TODO: handle named types per the relevant section of the RFD

		switch node.kind {
		case stringKind:
			example.WriteString("string")
		case numberKind:
			example.WriteString("1")
		case boolKind:
			example.WriteString("true")
		}
		return nil
	}

	var buf bytes.Buffer

	for _, field := range fields {
		buf.WriteString("- " + getJSONTag(field.tags) + ":")
		if err := addNodeToExample(buf, field.kind); err != nil {
			return "", err
		}
	}

	return buf.String(), nil
}

// Key-value pair for the "json" tag within a struct tag. Keys and values are
// separated by colons. Values are surrounded by double quotes.
// See: https://pkg.go.dev/reflect#StructTag
var jsonTagKeyValue = regexp.MustCompile(`json:"([^"]+)"`)

// getYAMLTag returns the "json" tag value from the struct tag expression in
// tags.
func getJSONTag(tags string) string {
	kv := jsonTagKeyValue.FindStringSubmatch(tags)

	// No "yaml" tag, or a "yaml" tag with no value.
	if len(kv) != 2 {
		return ""
	}

	return strings.TrimSuffix(kv[1], ",omitempty")
}

// getYAMLType returns a name for field that is suitable for printing within the
// resource reference.
func getYAMLType(field *ast.Field) (yamlKindNode, error) {
	switch t := field.Type.(type) {
	// TODO: Handle fields with manually overriden types per the
	// "Predeclared scalar types" section of the RFD.
	case *ast.Ident:
		switch t.Name {
		case "string":
			return yamlKindNode{
				kind: stringKind,
			}, nil
		case "uint8", "uint16", "uint32", "uint64", "int8", "int16", "int32", "int64", "float32", "float64":
			return yamlKindNode{
				kind: numberKind,
			}, nil
		case "bool":
			return yamlKindNode{
					kind: boolKind,
				},
				nil
		default:
			return yamlKindNode{}, fmt.Errorf("unsupported type: %+v", t.Name)
		}
		// TODO: Handle slices, maps, and structs
	// TODO: For declared types, field.Type is an *ast.SelectorExpr.
	// Figure out how to handle this case.
	default:
		return yamlKindNode{}, nil
	}
}

// tableValueFor returns a summary of a YAML type suitable for printing in a
// table of fields within the resource reference. For composite types,
// recursively traverses the children of node.
func tableValueFor(node yamlKindNode) (string, error) {
	traverseNode := func(node yamlKindNode, partialReturn string) (string, error) {
		switch node.kind {
		case stringKind:
			return "string", nil
		case numberKind:
			return "number", nil
		case boolKind:
			return "Boolean", nil
		default:
			return "", fmt.Errorf("we cannot find a value to print to the field table for type %v", node.kind)

		}
	}
	return traverseNode(node, "")
}

// makeRawField translates an *ast.Field into a rawField for downstream
// processing.
func makeRawField(field *ast.Field) (rawField, error) {
	doc := field.Doc.Text()
	if len(field.Names) > 1 {
		return rawField{}, fmt.Errorf("field %+v contains more than one name", field)
	}

	if len(field.Names) == 0 {
		return rawField{}, fmt.Errorf("field %+v has no names", field)
	}

	tn, err := getYAMLType(field)
	if err != nil {
		return rawField{}, err
	}

	return rawField{
		doc:  doc,
		kind: tn,
		name: field.Names[0].Name,
		tags: field.Tag.Value,
	}, nil
}

// makeFieldTableInfo assembles a slice of human-readable information about fields
// within a Go struct.
func makeFieldTableInfo(fields []rawField) ([]Field, error) {
	var result []Field
	for _, field := range fields {
		desc := strings.Trim(strings.ReplaceAll(field.doc, "\n", " "), " ")
		jsonName := getJSONTag(field.tags)

		// This field is ignored, so skip it.
		// See: https://pkg.go.dev/encoding/json#Marshal
		if jsonName == "-" {
			continue
		}
		// Using the exported field declaration name as the field name
		// per JSON marshaling rules.
		if jsonName == "" {
			jsonName = field.name
		}

		tv, err := tableValueFor(field.kind)
		if err != nil {
			return nil, err
		}
		result = append(result, Field{
			Description: descriptionWithoutName(desc, field.name),
			Name:        jsonName,
			Type:        tv,
		})
	}
	return result, nil
}

// descriptionWithoutName takes a description that contains name and removes
// name, fixing capitalization. The best practice for adding comments to
// exported Go declarations is to begin the comment with the name of the
// declaration. This function removes the declaration name since it won't mean
// anything to readers of the user-facing documentation.
func descriptionWithoutName(description, name string) string {
	// Not possible to trim the name from description
	if len(name) > len(description) {
		return description
	}

	var result = description
	switch {
	case strings.HasPrefix(description, name+" are "):
		result = strings.TrimPrefix(description, name+" are ")
	case strings.HasPrefix(description, name+" is "):
		result = strings.TrimPrefix(description, name+" is ")
	case strings.HasPrefix(description, name+" "):
		result = strings.TrimPrefix(description, name+" ")
	case strings.HasPrefix(description, name):
		result = strings.TrimPrefix(description, name)
	}

	// Make sure the result begins with a capital letter
	if len(result) > 0 {
		result = strings.ToUpper(result[:1]) + result[1:]
	}

	return result
}

// NewFromDecl creates a Resource object from the provided *GenDecl. filepath is
// the Go source file where the declaration was made, and is used only for
// printing.
func NewFromDecl(decl *ast.GenDecl, filepath string) (Resource, error) {
	rs, err := getRawNamedStruct(decl)
	if err != nil {
		return Resource{}, err
	}

	yml, err := makeYAMLExample(rs.fields)
	if err != nil {
		return Resource{}, err
	}

	fld, err := makeFieldTableInfo(rs.fields)
	if err != nil {
		return Resource{}, err
	}

	desc := strings.Trim(strings.ReplaceAll(rs.doc, "\n", " "), " ")
	return Resource{
		SectionName: rs.name,
		Description: descriptionWithoutName(desc, rs.name),
		SourcePath:  filepath,
		Fields:      fld,
		YAMLExample: yml,
	}, nil
}

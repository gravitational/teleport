package resource

import (
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

// getTypeSpec returns the type spec to use for further processing. Returns an
// error if there is either no type spec or more than one.
func getTypeSpec(decl *ast.GenDecl) (*ast.TypeSpec, error) {
	if len(decl.Specs) == 0 {
		return nil, errors.New("declaration has no specs")
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
			return nil, errors.New("declaration contains more than one type spec")
		}
		t = ts
	}

	if t == nil {
		return nil, errors.New("no type spec found")
	}

	return t, nil
}

// getSectionName determines how to name a section of the resource reference
// after the provided declaration.
func getSectionName(spec *ast.TypeSpec) string {
	return spec.Name.Name
}

// makeYAMLExample creates an example YAML document illustrating the fields
// within the declaration.
func makeYAMLExample(fields *ast.FieldList) (string, error) {
	// TODO: make the YAML example
	return "", nil
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

// getTypeName returns a name for field that is suitable for printing within the
// resource reference.
func getTypeName(field *ast.Field) (string, error) {
	switch t := field.Type.(type) {
	// TODO: Handle fields with manually overriden types per the
	// "Predeclared scalar types" section of the RFD.
	case *ast.Ident:
		switch t.Name {
		case "string":
			return "string", nil
		case "uint8", "uint16", "uint32", "uint64", "int8", "int16", "int32", "int64", "float32", "float64":
			return "number", nil
		case "bool":
			return "Boolean", nil
		default:
			return "", fmt.Errorf("unsupported type: %+v", t.Name)
		}
		// TODO: Handle slices, maps, and structs
	// TODO: For declared types, field.Type is an *ast.SelectorExpr.
	// Figure out how to handle this case.
	default:
		return "", nil
	}
}

// makeFields assembles a slice of human-readable information about fields
// within a Go struct.
func makeFields(fields *ast.FieldList) ([]Field, error) {
	var result []Field
	for _, field := range fields.List {
		desc := strings.Trim(strings.ReplaceAll(field.Doc.Text(), "\n", " "), " ")
		if len(field.Names) > 1 {
			return nil, fmt.Errorf("field %+v contains more than one name", field)
		}

		if len(field.Names) == 0 {
			return nil, fmt.Errorf("field %+v has no names", field)
		}
		name := field.Names[0].Name
		jsonName := getJSONTag(field.Tag.Value)
		// This field is ignored, so skip it.
		// See: https://pkg.go.dev/encoding/json#Marshal
		if jsonName == "-" {
			continue
		}
		// Using the exported field declaration name as the field name
		// per JSON marshaling rules.
		if jsonName == "" {
			jsonName = name
		}

		tn, err := getTypeName(field)
		if err != nil {
			return nil, err
		}

		result = append(result, Field{
			Description: descriptionWithoutName(desc, name),
			Name:        jsonName,
			Type:        tn,
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
	ts, err := getTypeSpec(decl)
	if err != nil {
		return Resource{}, err
	}

	str, ok := ts.Type.(*ast.StructType)
	if !ok {
		return Resource{}, errors.New("the declaration is not a struct")
	}

	yml, err := makeYAMLExample(str.Fields)
	if err != nil {
		return Resource{}, err
	}

	fld, err := makeFields(str.Fields)
	if err != nil {
		return Resource{}, err
	}

	section := getSectionName(ts)
	desc := strings.Trim(strings.ReplaceAll(decl.Doc.Text(), "\n", " "), " ")
	return Resource{
		SectionName: section,
		Description: descriptionWithoutName(desc, section),
		SourcePath:  filepath,
		Fields:      fld,
		YAMLExample: yml,
	}, nil
}

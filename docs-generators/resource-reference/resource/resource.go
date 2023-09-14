package resource

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"regexp"
	"strings"
)

// Package is used to look up a Go declaration in a map of declaration names to
// resource data.
type PackageInfo struct {
	TypeName    string
	PackageName string
}

type ReferenceEntry struct {
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
	// Original name of the field
	name string
	// Name as it appears in YAML, based on the json tag and json
	// encoding/marshaling rules.
	jsonName string
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
type yamlKindNode interface {
	// Generate a string representation to include in a table of fields
	formatForTable() string
	// Generate an example YAML value for the type with the provided number
	// of indendations.
	formatForExampleYAML(indents int) string
}

type yamlSequence struct {
	elementKind yamlKindNode
}

func (y yamlSequence) formatForTable() string {
	return `[]` + y.elementKind.formatForTable()
}

func (y yamlSequence) formatForExampleYAML(indents int) string {
	var leading string
	for i := 0; i < indents; i++ {
		leading += "  "
	}
	// Always start a sequence on a new line
	return fmt.Sprintf(`
%v- %v
%v- %v
%v- %v`,
		leading, y.elementKind.formatForExampleYAML(indents+1),
		leading, y.elementKind.formatForExampleYAML(indents+1),
		leading, y.elementKind.formatForExampleYAML(indents+1),
	)
}

type yamlMapping struct {
	keyKind   yamlKindNode
	valueKind yamlKindNode
}

func (y yamlMapping) formatForExampleYAML(indents int) string {
	var leading string
	// Add an extra indent for mappings
	indents = indents + 1
	for i := 0; i < indents; i++ {
		leading += "  "
	}

	kv := fmt.Sprintf("%v%v: %v", leading, y.keyKind.formatForExampleYAML(0), y.valueKind.formatForExampleYAML(indents+1))
	return fmt.Sprintf("\n%v\n%v\n%v", kv, kv, kv)
}

func (y yamlMapping) formatForTable() string {
	return fmt.Sprintf("map[%v]%v", y.keyKind.formatForTable(), y.valueKind.formatForTable())
}

type yamlString struct{}

func (y yamlString) formatForTable() string {
	return "string"
}

func (y yamlString) formatForExampleYAML(indents int) string {
	return `"string"`
}

type yamlNumber struct{}

func (y yamlNumber) formatForTable() string {
	return "number"
}

func (y yamlNumber) formatForExampleYAML(indents int) string {
	return "1"
}

type yamlBool struct{}

func (y yamlBool) formatForTable() string {
	return "Boolean"
}

func (y yamlBool) formatForExampleYAML(indents int) string {
	return "true"
}

// A type declared by the program, i.e., not one of Go's predeclared types.
type yamlCustomType struct {
	name string
}

func (y yamlCustomType) formatForExampleYAML(indents int) string {
	var leading string
	for i := 0; i < indents; i++ {
		leading += "  "
	}

	return "\n" + leading + "# [...]"
}

func (y yamlCustomType) formatForTable() string {
	return fmt.Sprintf(
		"[%v](#%v)",
		y.name,
		strings.ReplaceAll(strings.ToLower(y.name), " ", "-"),
	)
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

		if err != nil {
			return rawNamedStruct{}, err
		}

		jsonName := getJSONTag(f.tags)

		// This field is ignored, so skip it.
		// See: https://pkg.go.dev/encoding/json#Marshal
		if jsonName == "-" {
			continue
		}
		// Using the exported field declaration name as the field name
		// per JSON marshaling rules.
		if jsonName == "" {
			f.jsonName = f.name
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
	var buf bytes.Buffer

	for _, field := range fields {
		buf.WriteString(getJSONTag(field.tags) + ": ")
		buf.WriteString(field.kind.formatForExampleYAML(0))
		buf.WriteString("\n")
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

var camelCaseWordBoundary *regexp.Regexp = regexp.MustCompile("([a-z]+)([A-Z])")
var versionNumber *regexp.Regexp = regexp.MustCompile("V([0-9]+)")

// makeSectionName edits the original name of a declaration to make it more
// suitable as a section within the resource reference.
func makeSectionName(original string) string {
	s := camelCaseWordBoundary.ReplaceAllString(original, "$1 $2")
	return versionNumber.ReplaceAllString(s, "v$1")
}

// getYAMLTypeForExpr takes an AST type expression and recursively
// traverses it to populate a yamlKindNode. Each iteration converts a
// single *ast.Expr into a single yamlKindNode, returning the new node.
func getYAMLTypeForExpr(exp ast.Expr) (yamlKindNode, error) {
	switch t := exp.(type) {
	// TODO: Handle fields with manually overriden types per the
	// "Predeclared scalar types" section of the RFD.
	case *ast.Ident:
		switch t.Name {
		case "string":
			return yamlString{}, nil
		case "uint", "uint8", "uint16", "uint32", "uint64", "int", "int8", "int16", "int32", "int64", "float32", "float64":
			return yamlNumber{}, nil
		case "bool":
			return yamlBool{}, nil
		default:
			return nil, fmt.Errorf("unsupported type: %+v", t.Name)
		}
	case *ast.MapType:
		k, err := getYAMLTypeForExpr(t.Key)
		if err != nil {
			return nil, err
		}

		v, err := getYAMLTypeForExpr(t.Value)
		if err != nil {
			return nil, err
		}
		return yamlMapping{
			keyKind:   k,
			valueKind: v,
		}, nil
	case *ast.ArrayType:
		e, err := getYAMLTypeForExpr(t.Elt)
		if err != nil {
			return nil, err
		}
		return yamlSequence{
			elementKind: e,
		}, nil
	case *ast.SelectorExpr:
		return yamlCustomType{
			name: makeSectionName(t.Sel.Name),
		}, nil
	default:
		return nil, fmt.Errorf("unexpected type: %v", t)
	}

}

// getYAMLType returns a name for field that is suitable for printing within the
// resource reference.
func getYAMLType(field *ast.Field) (yamlKindNode, error) {
	return getYAMLTypeForExpr(field.Type)
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
		doc:      doc,
		kind:     tn,
		name:     field.Names[0].Name,
		jsonName: getJSONTag(field.Tag.Value),
		tags:     field.Tag.Value,
	}, nil
}

// makeFieldTableInfo assembles a slice of human-readable information about fields
// within a Go struct.
func makeFieldTableInfo(fields []rawField) ([]Field, error) {
	var result []Field
	for _, field := range fields {
		desc := strings.Trim(strings.ReplaceAll(field.doc, "\n", " "), " ")

		result = append(result, Field{
			Description: descriptionWithoutName(desc, field.name),
			Name:        field.jsonName,
			Type:        field.kind.formatForTable(),
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

const yamlExampleDelimeter string = "Example YAML:\n---\n"

// NewFromDecl creates a Resource object from the provided *GenDecl. filepath is
// the Go source file where the declaration was made, and is used only for
// printing. NewFromDecl uses allResources to look up custom fields.
func NewFromDecl(decl *ast.GenDecl, filepath string) (ReferenceEntry, error) {
	rs, err := getRawNamedStruct(decl)
	if err != nil {
		return ReferenceEntry{}, err
	}

	description := rs.doc
	var example string

	if strings.Contains(rs.doc, yamlExampleDelimeter) {
		sides := strings.Split(rs.doc, yamlExampleDelimeter)
		if len(sides) != 2 {
			return ReferenceEntry{}, errors.New("malformed example YAML in description: " + rs.doc)
		}
		example = sides[1]
		description = sides[0]
	} else {

		example, err = makeYAMLExample(rs.fields)
		if err != nil {
			return ReferenceEntry{}, err
		}
	}
	fld, err := makeFieldTableInfo(rs.fields)
	if err != nil {
		return ReferenceEntry{}, err
	}

	description = strings.Trim(strings.ReplaceAll(description, "\n", " "), " ")
	return ReferenceEntry{
		SectionName: rs.name,
		Description: descriptionWithoutName(description, rs.name),
		SourcePath:  filepath,
		Fields:      fld,
		YAMLExample: example,
	}, nil
}

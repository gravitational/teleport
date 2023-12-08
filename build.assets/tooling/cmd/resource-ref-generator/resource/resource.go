// Teleport
// Copyright (C) 2023  Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package resource

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"regexp"
	"strings"
)

// Package is used to look up a Go declaration in a map of declaration names to
// resource data.
type PackageInfo struct {
	DeclName    string
	PackageName string
}

// ReferenceEntry represents a section in the resource reference docs.
type ReferenceEntry struct {
	SectionName string
	Description string
	SourcePath  string
	Fields      []Field
	YAMLExample string
}

// DeclarationInfo includes data about a declaration so the generator can
// convert it into a ReferenceEntry.
type DeclarationInfo struct {
	FilePath    string
	Decl        ast.Decl
	PackageName string
}

// Field represents a row in a table that provides information about a field in
// the resource reference.
type Field struct {
	Name        string
	Description string
	Type        string
}

// yamlKind is the type of a field as represented in YAML. Used for determining
// how to document a field in the resource reference.
type yamlKind int

const (
	unknownKind yamlKind = iota
	sequenceKind
	mappingKind
	stringKind
	numberKind
	boolKind
)

// rawField contains simplified information about a struct field type. This
// prevents passing around AST nodes and makes testing easier.
type rawField struct {
	// Package that declares the field type
	packageName string
	// A declaration's GoDoc, including newline characters but not comment
	// characters.
	doc string
	// The type of the field.
	kind yamlKindNode
	// Original name of the field.
	name string
	// Name as it appears in YAML, based on the "json" struct tag and
	// marshaling rules in the encoding/json package.
	jsonName string
	// The entire struct tag expression for the field.
	tags string
}

// rawType contains simplified information about a type, which may or may not be
// a struct. This prevents passing around AST nodes and makes testing easier.
type rawType struct {
	// A declaration's GoDoc, including newline characters but not comment
	// characters.
	doc string
	// The name of the type declaration.
	name string
	// Struct fields within the type. Empty if not a struct.
	fields []rawField
}

// yamlKindNode represents a node in a potentially recursive YAML type, such as
// an integer, a map of integers to strings, a sequence of maps of strings to
// strings, etc. Used for printing example YAML documents and tables of fields.
// This is not intended to be a comprehensive YAML AST.
type yamlKindNode interface {
	// Generate a string representation to include in a table of fields.
	formatForTable() string
	// Generate an example YAML value for the type with the provided number
	// of indendations.
	formatForExampleYAML(indents int) string
	// Get the custom children of this yamlKindNode. Must call
	// customFieldData on its own children before returning.
	customFieldData() []PackageInfo
}

// nonYAMLKind represents a field type that we cannot convert to YAML. Consumers
// should return an error if there is no way to avoid creating a reference entry
// for this kind.
type nonYAMLKind struct{}

func (n nonYAMLKind) formatForTable() string {
	return ""
}

func (n nonYAMLKind) formatForExampleYAML(indents int) string {
	return ""
}

func (n nonYAMLKind) customFieldData() []PackageInfo {
	return []PackageInfo{}
}

// yamlSequence is a list of elements.
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

func (y yamlSequence) customFieldData() []PackageInfo {
	return y.elementKind.customFieldData()
}

// yamlMapping is a mapping of keys to values.
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

func (y yamlMapping) customFieldData() []PackageInfo {
	k := y.keyKind.customFieldData()
	v := y.valueKind.customFieldData()
	return append(k, v...)
}

type yamlString struct{}

func (y yamlString) formatForTable() string {
	return "string"
}

func (y yamlString) formatForExampleYAML(indents int) string {
	return `"string"`
}

func (y yamlString) customFieldData() []PackageInfo {
	return []PackageInfo{}
}

type yamlNumber struct{}

func (y yamlNumber) formatForTable() string {
	return "number"
}

func (y yamlNumber) formatForExampleYAML(indents int) string {
	return "1"
}

func (y yamlNumber) customFieldData() []PackageInfo {
	return []PackageInfo{}
}

type yamlBool struct{}

func (y yamlBool) formatForTable() string {
	return "Boolean"
}

func (y yamlBool) formatForExampleYAML(indents int) string {
	return "true"
}

func (y yamlBool) customFieldData() []PackageInfo {
	return []PackageInfo{}
}

// A type declared by the program, i.e., not one of Go's predeclared types.
type yamlCustomType struct {
	name string
	// Used to look up more information about the declaration of the custom
	// type so we can populate additional reference entries.
	declarationInfo PackageInfo
}

func (y yamlCustomType) customFieldData() []PackageInfo {
	return []PackageInfo{
		y.declarationInfo,
	}
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

type NotAGenDeclError struct{}

func (e NotAGenDeclError) Error() string {
	return "the declaration is not a GenDecl"
}

// getRawTypes returns a representation of the type spec of decl to use for
// further processing. Returns an error if there is either no type spec or more
// than one.
func getRawTypes(decl DeclarationInfo) (rawType, error) {
	gendecl, ok := decl.Decl.(*ast.GenDecl)
	if !ok {
		return rawType{}, NotAGenDeclError{}
	}

	if len(gendecl.Specs) == 0 {
		return rawType{}, errors.New("declaration has no specs")
	}

	if len(gendecl.Specs) > 1 {
		return rawType{}, errors.New("declaration contains more than one type spec")
	}

	if gendecl.Specs[0] == nil {
		return rawType{}, errors.New("no spec found")
	}

	t, ok := gendecl.Specs[0].(*ast.TypeSpec)
	if !ok {
		return rawType{}, errors.New("no type spec found")
	}

	str, ok := t.Type.(*ast.StructType)
	// The declaration is not a struct, but we may still want to include it
	// in the reference. Return a rawType with no fields.
	if !ok {
		return rawType{
			name:   t.Name.Name,
			doc:    gendecl.Doc.Text(),
			fields: []rawField{},
		}, nil
	}

	// We have determined that decl is a struct type, so collect its fields.
	var rawFields []rawField
	for _, field := range str.Fields.List {
		f, err := makeRawField(field, decl.PackageName)
		if err != nil {
			return rawType{}, err
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

	result := rawType{
		name: t.Name.Name,
		// Preserving newlines for downstream processing
		doc:    gendecl.Doc.Text(),
		fields: rawFields,
	}

	return result, nil
}

// makeYAMLExample creates an example YAML document illustrating the fields
// within a declaration. This appears at the end of a section within the
// reference.
func makeYAMLExample(fields []rawField) (string, error) {
	var buf bytes.Buffer

	for _, field := range fields {
		var example string
		// There is a predefined YAML example in the field comment, so
		// use that.
		if strings.Contains(field.doc, yamlExampleDelimeter) {
			sides := strings.Split(field.doc, yamlExampleDelimeter)
			if len(sides) != 2 {
				return "", errors.New("malformed example YAML in description: " + field.doc)
			}
			example = "\n" + sides[1]
		} else {
			example = field.kind.formatForExampleYAML(0) + "\n"
		}
		buf.WriteString(getJSONTag(field.tags) + ": ")
		buf.WriteString(example)
	}

	return buf.String(), nil
}

// Key-value pair for the "json" tag within a struct tag.  See:
// https://pkg.go.dev/reflect#StructTag
var jsonTagKeyValue = regexp.MustCompile(`json:"([^"]+)"`)

// getYAMLTag returns the "json" tag value from the provided struct tag
// expression.
func getJSONTag(tags string) string {
	kv := jsonTagKeyValue.FindStringSubmatch(tags)

	// No "json" tag, or a "json" tag with no value.
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
func getYAMLTypeForExpr(exp ast.Expr, pkg string) (yamlKindNode, error) {
	switch t := exp.(type) {
	case *ast.StarExpr:
		// Ignore the star, since YAML fields are unmarshaled as the
		// values they point to.
		return getYAMLTypeForExpr(t.X, pkg)
	case *ast.Ident:
		switch t.Name {
		case "string":
			return yamlString{}, nil
		case "uint", "uint8", "uint16", "uint32", "uint64", "int", "int8", "int16", "int32", "int64", "float32", "float64":
			return yamlNumber{}, nil
		case "bool":
			return yamlBool{}, nil
		default:
			return yamlCustomType{
				name: makeSectionName(t.Name),
				declarationInfo: PackageInfo{
					DeclName:    t.Name,
					PackageName: pkg,
				},
			}, nil
		}
	case *ast.MapType:
		k, err := getYAMLTypeForExpr(t.Key, pkg)
		if err != nil {
			return nil, err
		}

		v, err := getYAMLTypeForExpr(t.Value, pkg)
		if err != nil {
			return nil, err
		}
		return yamlMapping{
			keyKind:   k,
			valueKind: v,
		}, nil
	case *ast.ArrayType:
		e, err := getYAMLTypeForExpr(t.Elt, pkg)
		if err != nil {
			return nil, err
		}
		return yamlSequence{
			elementKind: e,
		}, nil
	case *ast.SelectorExpr:
		var pkg string
		x, ok := t.X.(*ast.Ident)
		if ok {
			pkg = x.Name
		}
		return yamlCustomType{
			name: makeSectionName(t.Sel.Name),
			declarationInfo: PackageInfo{
				DeclName:    t.Sel.Name,
				PackageName: pkg,
			},
		}, nil
	default:
		return nonYAMLKind{}, nil
	}
}

// getYAMLType returns YAML type information for a struct field so we can print
// information about it in the resource reference.
func getYAMLType(field *ast.Field, pkg string) (yamlKindNode, error) {
	return getYAMLTypeForExpr(field.Type, pkg)
}

// makeRawField translates an *ast.Field into a rawField for downstream
// processing. packageName is the name of the package that includes this name in
// a struct declaration.
func makeRawField(field *ast.Field, packageName string) (rawField, error) {
	doc := field.Doc.Text()
	if len(field.Names) > 1 {
		return rawField{}, fmt.Errorf("field %+v in %v contains more than one name", field, packageName)
	}

	var name string
	// Otherwise, the field is likely an embedded struct.
	if len(field.Names) == 1 {
		name = field.Names[0].Name
	}

	tn, err := getYAMLType(field, packageName)
	if err != nil {
		return rawField{}, err
	}

	// Indicate which package declared this field depending on whether the
	// field's type name includes the name of another package.
	pkg := packageName
	s, ok := field.Type.(*ast.SelectorExpr)
	if ok {
		i, ok := s.X.(*ast.Ident)
		if ok {
			pkg = i.Name
		}
	}

	var tag string
	if field.Tag != nil {
		tag = field.Tag.Value
	}

	return rawField{
		packageName: pkg,
		doc:         doc,
		kind:        tn,
		name:        name,
		jsonName:    getJSONTag(tag),
		tags:        tag,
	}, nil
}

// makeFieldTableInfo assembles a slice of human-readable information about fields
// within a Go struct to include within the resource reference.
func makeFieldTableInfo(fields []rawField) ([]Field, error) {
	var result []Field
	for _, field := range fields {
		var desc string
		var typ string
		// If there is a predefined YAML example, we don't attempt to
		// create a field table, since it will probably be inaccurate.
		// Instead, refer readers to the YAML example.
		if strings.Contains(field.doc, yamlExampleDelimeter) {
			sides := strings.Split(field.doc, yamlExampleDelimeter)
			if len(sides) != 2 {
				return nil, errors.New("malformed example YAML in description: " + field.doc)
			}
			desc = sides[0]
			typ = "See YAML example."
		} else {
			desc = field.doc
			typ = field.kind.formatForTable()
		}
		desc = strings.Trim(strings.ReplaceAll(desc, "\n", " "), " ")

		result = append(result, Field{
			Description: descriptionWithoutName(desc, field.name),
			Name:        field.jsonName,
			Type:        typ,
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

// handleEmbeddedStructFields finds embedded structs within fld and recursively
// processes the fields of those structs as though the fields belonged to the
// containing struct. Uses decl and allDecls to look up fields within the base
// structs. Returns a modified slice of fields that include all non-embedded
// fields within fld.
func handleEmbeddedStructFields(decl DeclarationInfo, fld []rawField, allDecls map[PackageInfo]DeclarationInfo) ([]rawField, error) {
	fieldsToProcess := []rawField{}
	for _, l := range fld {
		if l.name != "" {
			fieldsToProcess = append(fieldsToProcess, l)
			continue
		}
		c, ok := l.kind.(yamlCustomType)
		// Not an embedded struct since it's not a custom type
		if !ok {
			continue
		}

		// If the field's type has no package name, assume the field's
		// package name is the same as the one for decl.
		var pkg string
		if l.packageName != "" {
			pkg = l.packageName
		} else {
			pkg = decl.PackageName
		}
		p := PackageInfo{
			DeclName:    c.declarationInfo.DeclName,
			PackageName: pkg,
		}
		d, ok := allDecls[p]
		if !ok {
			return nil, fmt.Errorf(
				"%v: field %v.%v is not declared anywhere",
				decl.FilePath,
				l.packageName,
				c.name,
			)
		}
		e, err := getRawTypes(d)
		if err != nil && !errors.Is(err, NotAGenDeclError{}) {
			return nil, err
		}

		nf, err := handleEmbeddedStructFields(decl, e.fields, allDecls)
		if err != nil {
			return nil, err
		}

		fieldsToProcess = append(fieldsToProcess, nf...)
	}
	return fieldsToProcess, nil

}

// ReferenceDataFromDeclaration uses allResources to look up custom fields.
func ReferenceDataFromDeclaration(decl DeclarationInfo, allDecls map[PackageInfo]DeclarationInfo, allMethods map[PackageInfo][]MethodInfo) (map[PackageInfo]ReferenceEntry, error) {
	rs, err := getRawTypes(decl)
	if err != nil {
		return nil, err
	}

	fieldsToProcess, err := handleEmbeddedStructFields(decl, rs.fields, allDecls)
	if err != nil {
		return nil, err
	}

	var overridden bool

	// Handle example YAML within the declaration's GoDoc.
	description := rs.doc
	var example string
	if strings.Contains(rs.doc, yamlExampleDelimeter) {
		sides := strings.Split(rs.doc, yamlExampleDelimeter)
		if len(sides) != 2 {
			return nil, errors.New("malformed example YAML in description: " + rs.doc)
		}
		example = sides[1]
		description = sides[0]
		overridden = true
	} else {

		m := allMethods[PackageInfo{
			DeclName:    rs.name,
			PackageName: decl.PackageName,
		}]
		for _, e := range m {
			if e.Name == "UnmarshalYAML" || e.Name == "UnmarshalJSON" {
				return nil, fmt.Errorf("%v: type %v.%v has a custom unmarshaler, so it needs a custom YAML example, a comment beginning %q", decl.FilePath, decl.PackageName, rs.name, yamlExampleDelimeter)
			}
		}

		if len(rs.fields) == 0 {
			return nil, fmt.Errorf("%v: declaration %v has no fields and no example YAML in the GoDoc", decl.FilePath, rs.name)
		}
		example, err = makeYAMLExample(fieldsToProcess)
		if err != nil {
			return nil, err
		}
	}

	// Initialize the return value and insert the root reference entry
	// provided by decl.
	refs := make(map[PackageInfo]ReferenceEntry)

	description = strings.Trim(strings.ReplaceAll(description, "\n", " "), " ")
	entry := ReferenceEntry{
		SectionName: makeSectionName(rs.name),
		Description: descriptionWithoutName(description, rs.name),
		SourcePath:  decl.FilePath,
		YAMLExample: example,
		Fields:      []Field{},
	}
	key := PackageInfo{
		DeclName:    rs.name,
		PackageName: decl.PackageName,
	}

	// We are describing this reference entry via a YAML example override,
	// so we don't create reference entries for its fields.
	if overridden {
		refs[key] = entry
		return refs, nil
	}

	fld, err := makeFieldTableInfo(fieldsToProcess)
	if err != nil {
		return nil, err
	}
	entry.Fields = fld
	refs[key] = entry

	// For any fields within decl that have a custom type, look up the
	// declaration for that type and create a separate reference entry for
	// it.
	deps := []PackageInfo{}
	for _, f := range rs.fields {
		// Don't make separate reference entries for embedded structs
		// since they are part of the containing struct for the purposes
		// of unmarshaling YAML.
		//
		// Also ignore fields we are overriding with a custom YAML
		// example because the actual type is something we want to hide
		// from docs readers.
		if f.name == "" || strings.Contains(f.doc, yamlExampleDelimeter) {
			continue
		}
		deps = append(deps, f.kind.customFieldData()...)
	}
	for _, d := range deps {
		gd, ok := allDecls[d]
		if !ok {
			continue
		}
		r, err := ReferenceDataFromDeclaration(gd, allDecls, allMethods)
		if errors.Is(err, NotAGenDeclError{}) {
			continue
		}
		if err != nil {
			return nil, err
		}

		for k, v := range r {
			refs[k] = v
		}
	}
	return refs, nil
}

// MethodInfo is a simplified representation of a Go method.
type MethodInfo struct {
	// The name of the method
	Name string
	// Any field assignments within the main body of the method. Keys
	// represent fields of the receiver. Values are the values the
	// assignments assign.
	FieldAssignments map[string]string
}

func GetTopLevelStringAssignments(decls []ast.Decl, pkg string) (map[PackageInfo]string, error) {
	result := make(map[PackageInfo]string)

	// var and const assignments are GenDecls, so ignore any input Decls that
	// don't meet this criterion by making a slice of GenDecls.
	gd := []*ast.GenDecl{}
	for _, d := range decls {
		g, ok := d.(*ast.GenDecl)
		if !ok {
			continue
		}
		gd = append(gd, g)
	}

	// Whether in the "var =" format or "var (" format, each assignment is
	// an *ast.ValueSpec. Round up all ValueSpecs within a GenDecl that
	// declares a var or a const.
	vs := []*ast.ValueSpec{}
	for _, g := range gd {
		if g.Tok != token.VAR && g.Tok != token.CONST {
			continue
		}
		for _, s := range g.Specs {
			s, ok := s.(*ast.ValueSpec)
			if !ok {
				continue
			}
			vs = append(vs, s)
		}
	}

	// Add the name and value of each var/const to the return as long as
	// there is one name and the value is a string literal.
	for _, v := range vs {
		if len(v.Names) != 1 {
			continue
		}
		if len(v.Values) != 1 {
			continue
		}

		l, ok := v.Values[0].(*ast.BasicLit)
		if !ok {
			continue
		}
		if l.Kind != token.STRING {
			continue
		}
		// String literal values are quoted. Remove the quotes so we can
		// compare values downstream.
		result[PackageInfo{
			DeclName:    v.Names[0].Name,
			PackageName: pkg,
		}] = strings.Trim(l.Value, "\"")

	}
	return result, nil
}

func getMethodName(exp ast.Expr) (string, error) {
	switch t := exp.(type) {
	case *ast.IndexExpr:
		return getMethodName(t.X)
	case *ast.IndexListExpr:
		return getMethodName(t.X)
	case *ast.StarExpr:
		return getMethodName(t.X)
	case *ast.Ident:
		return t.Name, nil
	default:
		return "", errors.New("method has an unexpected receiver type")
	}
}

func GetMethodInfo(decls []DeclarationInfo) (map[PackageInfo][]MethodInfo, error) {
	if decls == nil || len(decls) == 0 {
		return map[PackageInfo][]MethodInfo{}, nil
	}

	result := make(map[PackageInfo][]MethodInfo)

	for _, decl := range decls {
		f, ok := decl.Decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		// Not a method
		if f.Recv == nil {
			continue
		}

		if len(f.Recv.List) != 1 {
			return nil, fmt.Errorf("%v: method %v.%v has an unexpected number of receivers",
				decl.FilePath,
				decl.PackageName,
				f.Name.Name,
			)
		}

		i, err := getMethodName(f.Recv.List[0].Type)
		if err != nil {
			return nil, fmt.Errorf("%v: method %v.%v has an unexpected receiver type",
				decl.FilePath,
				decl.PackageName,
				f.Name.Name,
			)

		}

		pi := PackageInfo{
			PackageName: decl.PackageName,
			DeclName:    i,
		}

		mi := MethodInfo{
			Name:             f.Name.Name,
			FieldAssignments: map[string]string{},
		}

		a, ok := result[pi]
		if !ok {
			result[pi] = []MethodInfo{}
		}

		for _, l := range f.Body.List {
			n, ok := l.(*ast.AssignStmt)
			if !ok {
				continue
			}

			// Do not collect assignments with more than one value.
			// These are done done in the relevant parts of the Teleport
			// source, and handling them complicates things.
			if len(n.Rhs) != 1 || len(n.Lhs) != 1 {
				continue
			}

			nt, ok := n.Rhs[0].(*ast.Ident)
			// We don't need to process other types, such as
			// selector expressions, on the right hand side yet.
			if !ok {
				continue
			}
			rhs := nt.Name

			sel, ok := n.Lhs[0].(*ast.SelectorExpr)
			// Does not assign one of the method receiver's
			// fields, since it's not a selector expression.
			if !ok {
				continue
			}

			id, ok := sel.X.(*ast.Ident)
			if !ok {
				continue
			}

			// There is no method receiver name to assign to, so we
			// won't track assignments made in this method.
			if len(f.Recv.List[0].Names) != 1 {
				continue
			}

			// This is not an assignment of a field within
			// the method receiver.
			if id.Name != f.Recv.List[0].Names[0].Name {
				continue
			}

			mi.FieldAssignments[sel.Sel.Name] = rhs
		}

		result[pi] = append(a, mi)
	}
	return result, nil
}

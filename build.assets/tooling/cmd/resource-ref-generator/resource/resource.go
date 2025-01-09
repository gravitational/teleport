// Teleport
// Copyright (C) 2025  Gravitational, Inc.
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
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	gofs "io/fs"

	"github.com/spf13/afero"
	"golang.org/x/tools/go/ast/astutil"
)

// PackageInfo is used to look up a Go declaration in a map of declaration names
// to resource data.
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

// Len returns the number of fields in the ReferenceEntry. Required for sorting.
func (re ReferenceEntry) Len() int {
	return len(re.Fields)
}

// Less compares field names in order to sort them.
func (re ReferenceEntry) Less(i, j int) bool {
	return re.Fields[j].Name < re.Fields[i].Name
}

// Swap swaps the order of reference fields in order to sort them.
func (re ReferenceEntry) Swap(i, j int) {
	re.Fields[i], re.Fields[j] = re.Fields[j], re.Fields[i]
}

// DeclarationInfo includes data about a declaration so the generator can
// convert it into a ReferenceEntry.
type DeclarationInfo struct {
	FilePath    string
	Decl        ast.Decl
	PackageName string
	// Maps the file-scoped name of each import (if given) to the
	// corresponding full package path.
	NamedImports map[string]string
}

// Field represents a row in a table that provides information about a field in
// the resource reference.
type Field struct {
	Name        string
	Description string
	Type        string
}

type SourceData struct {
	// TypeDecls maps package and declaration names to data that the generator
	// uses to format documentation for dynamic resource fields.
	TypeDecls map[PackageInfo]DeclarationInfo
	// PossibleFuncDecls are declarations that are not import, constant,
	// type or variable declarations.
	PossibleFuncDecls []DeclarationInfo
	// StringAssignments is used to look up the values of constants declared
	// in the source tree.
	StringAssignments map[PackageInfo]string
}

func NewSourceData(fs afero.Fs, rootPath string) (SourceData, error) {
	// All declarations within the source tree. We use this to extract
	// information about dynamic resource fields, which we can look up by
	// package and declaration name.
	typeDecls := make(map[PackageInfo]DeclarationInfo)
	possibleFuncDecls := []DeclarationInfo{}
	stringAssignments := make(map[PackageInfo]string)

	// Load each file in the source directory individually. Not using
	// packages.Load here since the resulting []*Package does not expose
	// individual file names, which we need so contributors who want to edit
	// the resulting docs page know which files to modify.
	err := afero.Walk(fs, rootPath, func(currentPath string, info gofs.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("loading Go source: %w", err)
		}

		if info.IsDir() {
			return nil
		}

		if filepath.Ext(info.Name()) != ".go" {
			return nil
		}

		// Open the file so we can pass it to ParseFile. Otherwise,
		// ParseFile always reads from the OS FS, not from fs.
		f, err := fs.Open(currentPath)
		if err != nil {
			return err
		}
		defer f.Close()
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, currentPath, f, parser.ParseComments)
		if err != nil {
			return err
		}

		str, err := GetTopLevelStringAssignments(file.Decls, file.Name.Name)
		if err != nil {
			return err
		}

		for k, v := range str {
			stringAssignments[k] = v
		}

		// Collect information from each file:
		// - Imported packages and their aliases
		// - Possible function declarations (for identifying relevant
		//   methods later)
		// - Type declarations
		pn := NamedImports(file)
		for _, decl := range file.Decls {
			di := DeclarationInfo{
				Decl:         decl,
				FilePath:     currentPath,
				PackageName:  file.Name.Name,
				NamedImports: pn,
			}
			l, ok := decl.(*ast.GenDecl)
			if !ok {
				possibleFuncDecls = append(possibleFuncDecls, di)
				continue
			}
			if len(l.Specs) != 1 {
				continue
			}
			spec, ok := l.Specs[0].(*ast.TypeSpec)
			if !ok {
				continue
			}

			typeDecls[PackageInfo{
				DeclName:    spec.Name.Name,
				PackageName: file.Name.Name,
			}] = DeclarationInfo{
				Decl:         l,
				FilePath:     currentPath,
				PackageName:  file.Name.Name,
				NamedImports: pn,
			}
		}
		return nil
	})
	if err != nil {
		return SourceData{}, fmt.Errorf("loading Go source files: %w", err)
	}
	return SourceData{
		TypeDecls:         typeDecls,
		PossibleFuncDecls: possibleFuncDecls,
		StringAssignments: stringAssignments,
	}, nil
}

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
	return "# See description"
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
	indents++
	for i := 0; i < indents; i++ {
		leading += "  "
	}
	el := y.elementKind.formatForExampleYAML(indents)
	// Trim leading indentation since each element is already indented.
	el = strings.TrimLeft(el, " ")
	// Always start a sequence on a new line
	return fmt.Sprintf(`
%v- %v
%v- %v
%v- %v`,
		leading, el,
		leading, el,
		leading, el,
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

	val := y.valueKind.formatForExampleYAML(indents)
	// Remove leading indentation on the first line of the value since the
	// key/value pair is already indented. This does not affect subsequent
	// lines of the value.
	val = strings.TrimLeft(val, " ")

	kv := fmt.Sprintf("%v%v: %v", leading, y.keyKind.formatForExampleYAML(0), val)
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

type yamlBase64 struct{}

func (y yamlBase64) formatForTable() string {
	return "base64-encoded string"
}

func (y yamlBase64) formatForExampleYAML(indents int) string {
	return "BASE64_STRING"
}

func (y yamlBase64) customFieldData() []PackageInfo {
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

	return leading + "# [...]"
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
func getRawTypes(decl DeclarationInfo, allDecls map[PackageInfo]DeclarationInfo) (rawType, error) {
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
		f, err := makeRawField(field, decl.PackageName, allDecls, decl.NamedImports)
		if err != nil {
			return rawType{}, err
		}

		// The struct field name is lowercased, so the field is not
		// exported. Ignore it.
		if f.name != "" && f.name[0] >= 'a' && f.name[0] <= 122 {
			continue
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
		example = field.kind.formatForExampleYAML(0) + "\n"
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
var versionNumber *regexp.Regexp = regexp.MustCompile("V([0-9]+)$")

// makeSectionName edits the original name of a declaration to make it more
// suitable as a section within the resource reference.
func makeSectionName(original string) string {
	s := versionNumber.ReplaceAllString(original, "")
	s = camelCaseWordBoundary.ReplaceAllString(s, "$1 $2")
	return s
}

// isByteSlice returns whether t is a []byte.
func isByteSlice(t *ast.ArrayType) bool {
	i, ok := t.Elt.(*ast.Ident)
	if !ok {
		return false
	}
	return i.Name == "byte"
}

// getYAMLTypeForExpr takes an AST type expression and recursively
// traverses it to populate a yamlKindNode. Each iteration converts a
// single *ast.Expr into a single yamlKindNode, returning the new node.
func getYAMLTypeForExpr(exp ast.Expr, pkg string, allDecls map[PackageInfo]DeclarationInfo, namedImports map[string]string) (yamlKindNode, error) {
	switch t := exp.(type) {
	case *ast.StarExpr:
		// Ignore the star, since YAML fields are unmarshaled as the
		// values they point to.
		return getYAMLTypeForExpr(t.X, pkg, allDecls, namedImports)
	case *ast.Ident:
		switch t.Name {
		case "string":
			return yamlString{}, nil
		case "uint", "uint8", "uint16", "uint32", "uint64", "int", "int8", "int16", "int32", "int64", "float32", "float64":
			return yamlNumber{}, nil
		case "bool":
			return yamlBool{}, nil
		default:
			info := PackageInfo{
				DeclName:    t.Name,
				PackageName: pkg,
			}
			if _, ok := allDecls[info]; !ok {
				return nonYAMLKind{}, nil
			}

			return yamlCustomType{
				name:            makeSectionName(t.Name),
				declarationInfo: info,
			}, nil
		}
	case *ast.MapType:
		k, err := getYAMLTypeForExpr(t.Key, pkg, allDecls, namedImports)
		if err != nil {
			return nil, err
		}

		v, err := getYAMLTypeForExpr(t.Value, pkg, allDecls, namedImports)
		if err != nil {
			return nil, err
		}
		return yamlMapping{
			keyKind:   k,
			valueKind: v,
		}, nil
	case *ast.ArrayType:
		// Bite slices marshal to base64 strings
		if isByteSlice(t) {
			return yamlBase64{}, nil
		}
		e, err := getYAMLTypeForExpr(t.Elt, pkg, allDecls, namedImports)
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
			if i, ok := namedImports[x.Name]; ok {
				pkg = i
			}
		}
		info := PackageInfo{
			DeclName:    t.Sel.Name,
			PackageName: pkg,
		}
		if _, ok := allDecls[info]; !ok {
			return nonYAMLKind{}, nil
		}

		return yamlCustomType{
			name:            makeSectionName(t.Sel.Name),
			declarationInfo: info,
		}, nil
	default:
		return nonYAMLKind{}, nil
	}
}

// getYAMLType returns YAML type information for a struct field so we can print
// information about it in the resource reference.
func getYAMLType(field *ast.Field, pkg string, allDecls map[PackageInfo]DeclarationInfo, namedImports map[string]string) (yamlKindNode, error) {
	return getYAMLTypeForExpr(field.Type, pkg, allDecls, namedImports)
}

// makeRawField translates an *ast.Field into a rawField for downstream
// processing. packageName is the name of the package that includes this name in
// a struct declaration.
func makeRawField(field *ast.Field, packageName string, allDecls map[PackageInfo]DeclarationInfo, namedImports map[string]string) (rawField, error) {
	doc := field.Doc.Text()
	if len(field.Names) > 1 {
		return rawField{}, fmt.Errorf("field %+v in %v contains more than one name", field, packageName)
	}

	var name string
	// Otherwise, the field is likely an embedded struct.
	if len(field.Names) == 1 {
		name = field.Names[0].Name
	}

	tn, err := getYAMLType(field, packageName, allDecls, namedImports)
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

// makeFieldTableInfo assembles a slice of human-readable information about
// fields within a Go struct to include within the resource reference.
func makeFieldTableInfo(fields []rawField) ([]Field, error) {
	var result []Field
	for _, field := range fields {
		var desc string
		var typ string

		desc = field.doc
		typ = field.kind.formatForTable()
		// Escape pipes so they do not affect table rendering.
		desc = strings.ReplaceAll(desc, "|", `\|`)
		// Remove surrounding spaces and inner line breaks.
		desc = strings.Trim(strings.ReplaceAll(desc, "\n", " "), " ")

		// Escape angle brackets so the docs engine handles them as
		// strings instead of HTML tags.
		desc = strings.ReplaceAll(desc, "<", `\<`)
		desc = strings.ReplaceAll(desc, ">", `\>`)

		result = append(result, Field{
			Description: descriptionWithoutName(desc, field.name),
			Name:        field.jsonName,
			Type:        typ,
		})
	}
	return result, nil
}

// descriptionWithoutName takes a description that contains an identifier name
// and removes the name so we can include it within the resource reference,
// fixing capitalization issues resulting from removing the name. Since the
// identifier's name within the source won't mean anything to a docs reader,
// removing it makes the description easier to read.
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

// handleEmbeddedStructFields finds embedded structs within fld and recursively
// processes the fields of those structs as though the fields belonged to the
// containing struct. Uses decl and allDecls to look up fields within the base
// structs. Returns a modified slice of fields that include all non-embedded
// fields within fld.
func handleEmbeddedStructFields(decl DeclarationInfo, fld []rawField, allDecls map[PackageInfo]DeclarationInfo) ([]rawField, error) {
	fieldsToProcess := []rawField{}
	for _, l := range fld {
		// Not an embedded struct field, so append it to the final
		// result.
		if l.name != "" {
			fieldsToProcess = append(fieldsToProcess, l)
			continue
		}
		c, ok := l.kind.(yamlCustomType)
		// Not an embedded struct since it's not a declared type.
		if !ok {
			continue
		}

		// Find the package name to use to look up the declaration from
		// its identifier.
		var pkg string
		i, ok := decl.NamedImports[l.packageName]
		switch {
		// The file that made the declaration provided a name for the
		// package associated with the identifier, so find the full
		// package path and use that to look up the declaration.
		case ok:
			pkg = i
		case l.packageName != "":
			pkg = l.packageName
		// If the field's type has no package name, assume the field's
		// package name is the same as the one for decl.
		default:
			pkg = decl.PackageName

		}
		p := PackageInfo{
			DeclName:    c.declarationInfo.DeclName,
			PackageName: pkg,
		}

		// We expect to find a declaration of the embedded struct.
		d, ok := allDecls[p]
		if !ok {
			return nil, fmt.Errorf(
				"%v: field %v.%v is not declared anywhere",
				decl.FilePath,
				l.packageName,
				c.name,
			)
		}
		e, err := getRawTypes(d, allDecls)
		if err != nil && !errors.Is(err, NotAGenDeclError{}) {
			return nil, err
		}

		// The embedded struct field may have its own embedded struct
		// fields.
		nf, err := handleEmbeddedStructFields(decl, e.fields, allDecls)
		if err != nil {
			return nil, err
		}

		fieldsToProcess = append(fieldsToProcess, nf...)
	}
	return fieldsToProcess, nil
}

// NamedImports creates a mapping from the provided name of each package import
// to the original package path.
func NamedImports(file *ast.File) map[string]string {
	m := make(map[string]string)
	for _, i := range file.Imports {
		if i.Name == nil {
			continue
		}
		s := strings.Trim(i.Path.Value, "\"")
		p := strings.Split(s, "/")
		// Consumers check the named imports map against the final path
		// segment of a package path.
		if len(p) > 1 {
			s = p[len(p)-1]
		}
		m[i.Name.Name] = s
	}
	return m
}

// ReferenceDataFromDeclaration gets data for the reference by examining decl.
// Looks up decl's fields in allDecls and methods in allMethods.
func ReferenceDataFromDeclaration(decl DeclarationInfo, allDecls map[PackageInfo]DeclarationInfo) (map[PackageInfo]ReferenceEntry, error) {
	rs, err := getRawTypes(decl, allDecls)
	if err != nil {
		return nil, err
	}

	fieldsToProcess, err := handleEmbeddedStructFields(decl, rs.fields, allDecls)
	if err != nil {
		return nil, err
	}

	description := rs.doc
	var example string

	example, err = makeYAMLExample(fieldsToProcess)
	if err != nil {
		return nil, err
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

	fld, err := makeFieldTableInfo(fieldsToProcess)
	if err != nil {
		return nil, err
	}
	entry.Fields = fld
	sort.Sort(entry)
	refs[key] = entry

	// For any fields within decl that have a custom type, look up the
	// declaration for that type and create a separate reference entry for
	// it.
	for _, f := range rs.fields {
		// Don't make separate reference entries for embedded structs
		// since they are part of the containing struct for the purposes
		// of unmarshaling YAML.
		//
		if f.name == "" {
			continue
		}

		c := f.kind.customFieldData()

		for _, d := range c {
			// Find the package name to use to look up the declaration from
			// its identifier.
			i, ok := decl.NamedImports[d.PackageName]
			// The file that made the declaration provided a name for the
			// package associated with the identifier, so find the full
			// package path and use that to look up the declaration.
			if ok {
				d.PackageName = i
			}

			// Get information about the field type's declaration.
			// If we can't find it, it means the field type was
			// probably declared in the standard library or
			// third-party package. In this case, leave it to the
			// GoDoc to describe the field type.
			gd, ok := allDecls[d]
			if !ok {
				continue
			}
			r, err := ReferenceDataFromDeclaration(gd, allDecls)
			if errors.Is(err, NotAGenDeclError{}) {
				continue
			}
			if err != nil {
				return nil, err
			}

			for k, v := range r {
				sort.Sort(v)
				refs[k] = v
			}
		}
	}
	return refs, nil
}

// GetTopLevelStringAssignments collects all declarations of a var or a const
// within decls that assign a string value. Used to look up the values of these
// declarations.
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
	// an *ast.ValueSpec. Collect all ValueSpecs within a GenDecl that
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

func getReceiverName(exp ast.Expr) (string, error) {
	switch t := exp.(type) {
	case *ast.IndexExpr:
		return getReceiverName(t.X)
	case *ast.IndexListExpr:
		return getReceiverName(t.X)
	case *ast.StarExpr:
		return getReceiverName(t.X)
	case *ast.Ident:
		return t.Name, nil
	default:
		return "", errors.New("method has an unexpected receiver type")
	}
}

// getAssignments collects all of the assignments made to fields of a method
// reciever. Assumes that n is the function body of the method. In the resulting
// map, each key is a field name and each value is the assignment value.
func getAssignments(receiver string, n ast.Node) map[string]string {
	result := make(map[string]string)
	astutil.Apply(n, func(c *astutil.Cursor) bool {
		n, ok := c.Node().(*ast.AssignStmt)
		if !ok {
			return true
		}
		// Do not collect assignments with more than one value.
		// These are done done in the relevant parts of the Teleport
		// source, and handling them complicates things.
		if len(n.Rhs) != 1 || len(n.Lhs) != 1 {
			return true
		}

		// We don't need to process non-identifier expressions,
		// such as selector expressions, on the right hand side
		// yet. See if there is an identifier on the right hand
		// side.
		nt, ok := n.Rhs[0].(*ast.Ident)
		if !ok {
			return true
		}
		rhs := nt.Name

		// We expect the left hand side to be a selector expression,
		// since it assigns one of the receiver's fields.
		sel, ok := n.Lhs[0].(*ast.SelectorExpr)
		// Does not assign one of the method receiver's
		// fields, since it's not a selector expression.
		if !ok {
			return true
		}

		id, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}

		// This is not an assignment of a field within
		// the method receiver.
		if id.Name != receiver {
			return true
		}

		result[sel.Sel.Name] = rhs

		return true
	}, nil)
	return result
}

type VersionKindAssignment struct {
	Version string
	Kind    string
}

const versionField = "Version"
const kindField = "Kind"

// VersionKindAssignments finds all methods with methodName, which we expect to
// assign the version and kind fields within the receiver, and returns a map of
// receiver PackageInfos to the version and kind fields they assign.
func VersionKindAssignments(decls []DeclarationInfo, methodName string) (map[PackageInfo]VersionKindAssignment, error) {
	if decls == nil || len(decls) == 0 {
		return map[PackageInfo]VersionKindAssignment{}, nil
	}

	result := make(map[PackageInfo]VersionKindAssignment)

	for _, decl := range decls {
		f, ok := decl.Decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		if f.Name.Name != methodName {
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
		receiver := f.Recv.List[0]

		// There is no method receiver name to assign to, so we
		// won't track assignments made in this method.
		if len(receiver.Names) != 1 {
			continue
		}

		recName, err := getReceiverName(receiver.Type)
		if err != nil {
			return nil, fmt.Errorf("%v: method %v.%v has an unexpected receiver type",
				decl.FilePath,
				decl.PackageName,
				f.Name.Name,
			)
		}

		pi := PackageInfo{
			PackageName: decl.PackageName,
			DeclName:    recName,
		}

		_, ok = result[pi]
		if ok {
			continue
		}

		// Not all resource types have a version and a kind. If these
		// are empty, handle this downstream.
		s := getAssignments(receiver.Names[0].Name, f)
		result[pi] = VersionKindAssignment{
			Version: s[versionField],
			Kind:    s[kindField],
		}
	}

	return result, nil
}

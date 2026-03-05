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
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// PackageInfo is used to look up a Go declaration in a map of declaration names
// to resource data.
type PackageInfo struct {
	// DeclName is the name of a Go declaration.
	DeclName string
	// PackagePath is the full path of a Go package containing the
	// declaration, e.g.,
	// "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	PackagePath string
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

// sortFieldsByName sorts a and b ascending by Name.
func sortFieldsByName(a, b Field) int {
	return strings.Compare(a.Name, b.Name)
}

type SourceData struct {
	// TypeDecls maps package and declaration names to data that the generator
	// uses to format documentation for dynamic resource fields.
	TypeDecls map[PackageInfo]DeclarationInfo
}

// NewSourceData extracts type declarations from the Go files rooted at
// rootPath. Uses prefix, e.g., github.com/gravitational/teleport, to construct
// package paths.
func NewSourceData(prefix string, rootPath string) (SourceData, error) {
	// All declarations within the source tree. We use this to extract
	// information about dynamic resource fields, which we can look up by
	// package and declaration name.
	typeDecls := make(map[PackageInfo]DeclarationInfo)

	// Load each file in the source directory individually. Not using
	// packages.Load here since the resulting []*Package does not expose
	// individual file names, which we need so contributors who want to edit
	// the resulting docs page know which files to modify.
	err := filepath.Walk(rootPath, func(currentPath string, info fs.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("loading Go source: %w", err)
		}

		if info.IsDir() {
			return nil
		}

		if filepath.Ext(info.Name()) != ".go" {
			return nil
		}

		// Find the Go package path corresponding to the current file.
		rel, err := filepath.Rel(rootPath, currentPath)
		if err != nil {
			return fmt.Errorf("unable to find a relative path between %v and %v: %w", rootPath, currentPath, err)
		}
		pkg := path.Join(
			prefix,
			filepath.Base(rootPath),
			filepath.Dir(rel),
		)

		// Open the file so we can pass it to ParseFile. Otherwise,
		// ParseFile always reads from the OS FS, not from fs.
		f, err := os.Open(currentPath)
		if err != nil {
			return err
		}
		defer f.Close()
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, currentPath, f, parser.ParseComments)
		if err != nil {
			return err
		}

		// Use a relative path from the source directory for cleaner
		// paths
		relDeclPath, err := filepath.Rel(rootPath, currentPath)
		if err != nil {
			return err
		}

		// Collect information from each file:
		// - Imported packages and their aliases
		// - Possible function declarations (for identifying relevant
		//   methods later)
		// - Type declarations
		pn := NamedImports(file)
		for _, decl := range file.Decls {
			l, ok := decl.(*ast.GenDecl)
			if !ok {
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
				PackagePath: pkg,
			}] = DeclarationInfo{
				Decl:         l,
				FilePath:     relDeclPath,
				PackageName:  pkg,
				NamedImports: pn,
			}
		}
		return nil
	})
	if err != nil {
		return SourceData{}, fmt.Errorf("loading Go source files: %w", err)
	}
	return SourceData{
		TypeDecls: typeDecls,
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

// kindTableFormatOptions configures the way the generator formats YAML kinds
// for the field table in a reference page.
type kindTableFormatOptions struct {
	// camelCaseExceptions is a list of strings to exempt when splitting
	// camel-case words.
	camelCaseExceptions []string
}

// yamlKindNode represents a node in a potentially recursive YAML type, such as
// an integer, a map of integers to strings, a sequence of maps of strings to
// strings, etc. Used for printing example YAML documents and tables of fields.
// This is not intended to be a comprehensive YAML AST.
type yamlKindNode interface {
	// Generate a string representation to include in a table of fields.
	formatForTable(kindTableFormatOptions) string
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

func (n nonYAMLKind) formatForTable(opts kindTableFormatOptions) string {
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

func (y yamlSequence) formatForTable(opts kindTableFormatOptions) string {
	return `[]` + y.elementKind.formatForTable(opts)
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

func (y yamlMapping) formatForTable(opts kindTableFormatOptions) string {
	return fmt.Sprintf("map[%v]%v", y.keyKind.formatForTable(opts), y.valueKind.formatForTable(opts))
}

func (y yamlMapping) customFieldData() []PackageInfo {
	k := y.keyKind.customFieldData()
	v := y.valueKind.customFieldData()
	return append(k, v...)
}

type yamlString struct{}

func (y yamlString) formatForTable(opts kindTableFormatOptions) string {
	return "string"
}

func (y yamlString) formatForExampleYAML(indents int) string {
	return `"string"`
}

func (y yamlString) customFieldData() []PackageInfo {
	return []PackageInfo{}
}

type yamlBase64 struct{}

func (y yamlBase64) formatForTable(opts kindTableFormatOptions) string {
	return "base64-encoded string"
}

func (y yamlBase64) formatForExampleYAML(indents int) string {
	return "BASE64_STRING"
}

func (y yamlBase64) customFieldData() []PackageInfo {
	return []PackageInfo{}
}

type yamlNumber struct{}

func (y yamlNumber) formatForTable(opts kindTableFormatOptions) string {
	return "number"
}

func (y yamlNumber) formatForExampleYAML(indents int) string {
	return "1"
}

func (y yamlNumber) customFieldData() []PackageInfo {
	return []PackageInfo{}
}

type yamlBool struct{}

func (y yamlBool) formatForTable(opts kindTableFormatOptions) string {
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

func (y yamlCustomType) formatForTable(opts kindTableFormatOptions) string {
	name := splitCamelCase(y.name, opts.camelCaseExceptions)
	return fmt.Sprintf(
		"[%v](#%v)",
		name,
		strings.ReplaceAll(strings.ToLower(name), " ", "-"),
	)
}

type NotAGenDeclError struct{}

func (e NotAGenDeclError) Error() string {
	return "the declaration is not a GenDecl"
}

// typeForDecl returns a representation of the type spec of decl to use for
// further processing. Returns an error if there is either no type spec or more
// than one.
func typeForDecl(decl DeclarationInfo, allDecls map[PackageInfo]DeclarationInfo) (rawType, error) {
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
		example := field.kind.formatForExampleYAML(0) + "\n"
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

var camelCaseWordBoundary = regexp.MustCompile(`([a-z0-9])([A-Z][a-z0-9])`)

// splitCamelCase edits the original name of a declaration to make it more
// suitable as a section within the resource reference.
func splitCamelCase(original string, camelCaseExceptions []string) string {
	exceptionMap := make(map[string]struct{})
	for _, e := range camelCaseExceptions {
		exceptionMap[e] = struct{}{}
	}

	// Ensure that each exception occupies its own word. This way, we can
	// feed each exception-only word to the result and split the remaining
	// camel case boundaries.
	exceptions := regexp.MustCompile(
		fmt.Sprintf("(%v)", strings.Join(camelCaseExceptions, "|")),
	)
	split := exceptions.ReplaceAllString(original, " $1 ")
	words := bufio.NewScanner(strings.NewReader(split))

	// Iterate through the words we have so far, preserving exceptions and
	// splitting the remaining camel-cased words.
	var result bytes.Buffer
	words.Split(bufio.ScanWords)
	for words.Scan() {
		word := words.Text()
		if _, ok := exceptionMap[word]; ok {
			result.WriteString(word + " ")
			continue
		}
		result.WriteString(camelCaseWordBoundary.ReplaceAllString(word, "$1 $2") + " ")
	}

	return strings.Trim(result.String(), " ")
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
				PackagePath: pkg,
			}
			if _, ok := allDecls[info]; !ok {
				return nonYAMLKind{}, nil
			}

			return yamlCustomType{
				name:            t.Name,
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
			PackagePath: pkg,
		}
		if _, ok := allDecls[info]; !ok {
			return nonYAMLKind{}, nil
		}

		return yamlCustomType{
			name:            t.Sel.Name,
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
		// Not an identifier, so don't look up imports
		if !ok {
			goto assignTag
		}

		p, imp := namedImports[i.Name]
		if !imp {
			return rawField{}, fmt.Errorf("package %v does not include an import with name %v", packageName, i.Name)
		}

		pkg = p
	}

assignTag:
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
func makeFieldTableInfo(fields []rawField, camelCaseExceptions []string) ([]Field, error) {
	var result []Field
	for _, field := range fields {
		var desc string
		var typ string

		desc = field.doc
		typ = field.kind.formatForTable(kindTableFormatOptions{
			camelCaseExceptions: camelCaseExceptions,
		})
		// Escape pipes so they do not affect table rendering.
		desc = strings.ReplaceAll(desc, "|", `\|`)
		// Remove surrounding spaces and inner line breaks.
		desc = strings.Trim(strings.ReplaceAll(desc, "\n", " "), " ")

		// Escape angle brackets so the docs engine handles them as
		// strings instead of HTML tags.
		desc = strings.ReplaceAll(desc, "<", `\<`)
		desc = strings.ReplaceAll(desc, ">", `\>`)

		result = append(result, Field{
			Description: printableDescription(desc, field.name),
			Name:        field.jsonName,
			Type:        typ,
		})
	}
	return result, nil
}

// curlyBracePairPattern matches a pair of curly braces, with a capture group
// for the content enclosed by the braces.
var curlyBracePairPattern = regexp.MustCompile(`\{([^}]*)\}`)

// printableDescription modifies a field or type description to make it suitable
// for reading on a docs page.
//
// ident is the name of a Go identifier. printableDescription removes the name
// from the description so we can include it within the resource reference,
// fixing capitalization issues resulting from removing the name. Since the
// identifier's name within the source won't mean anything to a docs reader,
// removing it makes the description easier to read.
//
// Since curly brace pairs break docs site builds, printableDescription also
// encloses any curly brace pairs with backticks.
func printableDescription(description, ident string) string {
	result := curlyBracePairPattern.ReplaceAllString(description, "`{$1}`")
	// Replace any double-backticks resulting from the previous operation.
	// This is a hack to avoid the need for more complex logic. Double
	// backticks won't render as expected in the docs anyway, so it's fine
	// to replace ones that don't result from the earlier replacement.
	result = strings.ReplaceAll(result, "``", "`")

	// Not possible to trim the name from description
	if len(ident) > len(result) {
		return result
	}

	switch {
	case strings.HasPrefix(result, ident+" are "):
		result = strings.TrimPrefix(result, ident+" are ")
	case strings.HasPrefix(result, ident+" is "):
		result = strings.TrimPrefix(result, ident+" is ")
	case strings.HasPrefix(result, ident+" "):
		result = strings.TrimPrefix(result, ident+" ")
	case strings.HasPrefix(result, ident):
		result = strings.TrimPrefix(result, ident)
	}

	// Make sure the result begins with a capital letter
	if len(result) > 0 {
		result = strings.ToUpper(result[:1]) + result[1:]
	}

	return result
}

// allFieldsForDecl finds embedded structs within fld and recursively
// processes the fields of those structs as though the fields belonged to the
// containing struct. Uses decl and allDecls to look up fields within the base
// structs. Returns a modified slice of fields that include all non-embedded
// fields within fld.
func allFieldsForDecl(decl DeclarationInfo, fld []rawField, allDecls map[PackageInfo]DeclarationInfo) ([]rawField, error) {
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
			PackagePath: pkg,
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
		e, err := typeForDecl(d, allDecls)
		if err != nil && !errors.As(err, &NotAGenDeclError{}) {
			return nil, err
		}

		// The embedded struct field may have its own embedded struct
		// fields.
		nf, err := allFieldsForDecl(decl, e.fields, allDecls)
		if err != nil {
			return nil, err
		}

		fieldsToProcess = append(fieldsToProcess, nf...)
	}
	return fieldsToProcess, nil
}

// NamedImports creates a mapping from the provided name of each package import
// to the original package path. If the package does not have an explicit name,
// map the full path to the final path segment instead.
func NamedImports(file *ast.File) map[string]string {
	m := make(map[string]string)
	for _, i := range file.Imports {
		pkgPath := strings.Trim(i.Path.Value, "\"")
		if i.Name == nil {
			m[path.Base(pkgPath)] = pkgPath
		} else {
			m[i.Name.Name] = pkgPath
		}
	}
	return m
}

// ReferenceDataFromDeclaration gets data for the reference by examining decl.
// Looks up decl's fields in allDecls and methods in allMethods. Uses prefix,
// e.g., "github.com/gravitational/teleport", to construct package paths
func ReferenceDataFromDeclaration(
	prefix string,
	decl DeclarationInfo,
	allDecls map[PackageInfo]DeclarationInfo,
	camelCaseExceptions []string,
) (map[PackageInfo]ReferenceEntry, error) {
	rs, err := typeForDecl(decl, allDecls)
	if err != nil {
		return nil, err
	}

	fieldsToProcess, err := allFieldsForDecl(decl, rs.fields, allDecls)
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
		SectionName: splitCamelCase(rs.name, camelCaseExceptions),
		Description: printableDescription(description, rs.name),
		SourcePath:  decl.FilePath,
		YAMLExample: example,
		Fields:      []Field{},
	}
	key := PackageInfo{
		DeclName:    rs.name,
		PackagePath: decl.PackageName,
	}

	fld, err := makeFieldTableInfo(fieldsToProcess, camelCaseExceptions)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(fld, sortFieldsByName)
	entry.Fields = fld
	refs[key] = entry

	// For any fields within decl that have a custom type, look up the
	// declaration for that type and create a separate reference entry for
	// it.
	for _, f := range fieldsToProcess {
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
			i, ok := decl.NamedImports[d.PackagePath]
			// The file that made the declaration provided a name for the
			// package associated with the identifier, so find the full
			// package path and use that to look up the declaration.
			if ok {
				d.PackagePath = i
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
			r, err := ReferenceDataFromDeclaration(prefix, gd, allDecls, camelCaseExceptions)
			if errors.As(err, &NotAGenDeclError{}) {
				continue
			}
			if err != nil {
				return nil, err
			}

			for k, v := range r {
				slices.SortFunc(v.Fields, sortFieldsByName)
				refs[k] = v
			}
		}
	}
	return refs, nil
}

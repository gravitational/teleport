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

package generator

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
	"text/template"
)

// PackageInfo identifies a Go type declaration by name and package.
type PackageInfo struct {
	DeclName    string
	PackagePath string
}

// ReferenceEntry represents a section in the config reference documentation.
type ReferenceEntry struct {
	SectionName string
	Description string
	SourcePath  string
	Fields      []Field
	YAMLExample string
}

// DeclarationInfo holds data about a type declaration.
type DeclarationInfo struct {
	FilePath     string
	Decl         ast.Decl
	PackageName  string
	NamedImports map[string]string
}

// Field represents a row in a config reference table.
type Field struct {
	Name        string
	Description string
	Type        string
}

// rawField contains simplified information about a struct field.
type rawField struct {
	packageName string
	doc         string
	kind        yamlKindNode
	name        string
	yamlName    string
	inline      bool
}

// rawType contains simplified information about a type declaration.
type rawType struct {
	doc    string
	name   string
	fields []rawField
}

// kindTableFormatOptions configures table formatting for YAML kinds.
type kindTableFormatOptions struct {
	camelCaseExceptions []string
}

// yamlKindNode represents a node in a potentially recursive YAML type such as
// an integer, a map of integers to strings, or a sequence of custom objects.
// This is used for printing example YAML documents and field type tables.
type yamlKindNode interface {
	formatForTable(kindTableFormatOptions) string
	formatForExampleYAML(indents int) string
	customFieldData() []PackageInfo
}

// nonYAMLKind represents a field type that cannot be expressed as YAML.
type nonYAMLKind struct{}

func (n nonYAMLKind) formatForTable(kindTableFormatOptions) string { return "" }
func (n nonYAMLKind) formatForExampleYAML(int) string              { return "# See description" }
func (n nonYAMLKind) customFieldData() []PackageInfo               { return nil }

// yamlSequence is a list of elements.
type yamlSequence struct{ elementKind yamlKindNode }

func (y yamlSequence) formatForTable(opts kindTableFormatOptions) string {
	return "[]" + y.elementKind.formatForTable(opts)
}

func (y yamlSequence) formatForExampleYAML(indents int) string {
	var leading string
	indents++
	for i := 0; i < indents; i++ {
		leading += "  "
	}
	el := strings.TrimLeft(y.elementKind.formatForExampleYAML(indents), " ")
	return fmt.Sprintf("\n%v- %v\n%v- %v\n%v- %v", leading, el, leading, el, leading, el)
}

func (y yamlSequence) customFieldData() []PackageInfo { return y.elementKind.customFieldData() }

// yamlMapping is a mapping of keys to values.
type yamlMapping struct {
	keyKind   yamlKindNode
	valueKind yamlKindNode
}

func (y yamlMapping) formatForTable(opts kindTableFormatOptions) string {
	return fmt.Sprintf("map[%v]%v", y.keyKind.formatForTable(opts), y.valueKind.formatForTable(opts))
}

func (y yamlMapping) formatForExampleYAML(indents int) string {
	var leading string
	indents++
	for i := 0; i < indents; i++ {
		leading += "  "
	}
	val := strings.TrimLeft(y.valueKind.formatForExampleYAML(indents), " ")
	kv := fmt.Sprintf("%v%v: %v", leading, y.keyKind.formatForExampleYAML(0), val)
	return fmt.Sprintf("\n%v\n%v\n%v", kv, kv, kv)
}

func (y yamlMapping) customFieldData() []PackageInfo {
	return append(y.keyKind.customFieldData(), y.valueKind.customFieldData()...)
}

type yamlString struct{}

func (y yamlString) formatForTable(kindTableFormatOptions) string { return "string" }
func (y yamlString) formatForExampleYAML(int) string              { return `"string"` }
func (y yamlString) customFieldData() []PackageInfo               { return nil }

type yamlBase64 struct{}

func (y yamlBase64) formatForTable(kindTableFormatOptions) string { return "base64-encoded string" }
func (y yamlBase64) formatForExampleYAML(int) string              { return "BASE64_STRING" }
func (y yamlBase64) customFieldData() []PackageInfo               { return nil }

type yamlNumber struct{}

func (y yamlNumber) formatForTable(kindTableFormatOptions) string { return "number" }
func (y yamlNumber) formatForExampleYAML(int) string              { return "1" }
func (y yamlNumber) customFieldData() []PackageInfo               { return nil }

type yamlBool struct{}

func (y yamlBool) formatForTable(kindTableFormatOptions) string { return "Boolean" }
func (y yamlBool) formatForExampleYAML(int) string              { return "true" }
func (y yamlBool) customFieldData() []PackageInfo               { return nil }

// yamlCustomType represents a program-declared type (not a Go predeclared type).
type yamlCustomType struct {
	name            string
	declarationInfo PackageInfo
}

func (y yamlCustomType) customFieldData() []PackageInfo { return []PackageInfo{y.declarationInfo} }

func (y yamlCustomType) formatForExampleYAML(indents int) string {
	var leading string
	for i := 0; i < indents; i++ {
		leading += "  "
	}
	return leading + "# [...]"
}

func (y yamlCustomType) formatForTable(opts kindTableFormatOptions) string {
	name := splitCamelCase(y.name, opts.camelCaseExceptions)
	return fmt.Sprintf("[%v](#%v)", name, strings.ReplaceAll(strings.ToLower(name), " ", "-"))
}

// NotAGenDeclError is returned when the given declaration is not a GenDecl.
type NotAGenDeclError struct{}

func (e NotAGenDeclError) Error() string { return "the declaration is not a GenDecl" }

// SourceData holds indexed type declarations from the Go source tree.
type SourceData struct {
	TypeDecls map[PackageInfo]DeclarationInfo
}

// GeneratorConfig is the user-facing configuration for the config reference generator.
type GeneratorConfig struct {
	// SourcePath is the directory tree to scan for Go source files.
	SourcePath string `yaml:"source"`
	// ModulePath is the path to the Go module root directory (where go.mod lives).
	ModulePath string `yaml:"module_root"`
	// ModulePrefix is the Go module import path prefix
	// (e.g., "github.com/gravitational/teleport").
	ModulePrefix string `yaml:"module_prefix"`
	// DestinationDirectory is where the generator writes output MDX files.
	DestinationDirectory string `yaml:"destination"`
	// EntryType is the name of the top-level config struct to enumerate
	// (e.g., "FileConfig").
	EntryType string `yaml:"entry_type"`
	// ServiceSuffix is the yaml-tag suffix used to select which fields of
	// EntryType become output pages. Defaults to "_service".
	ServiceSuffix string `yaml:"service_suffix"`
	// CamelCaseExceptions lists strings to preserve intact when splitting
	// camelCase type names into words (e.g., "AWS", "TLS").
	CamelCaseExceptions []string `yaml:"camel_case_exceptions"`
}

// pageContent is the data passed to the MDX template for each service section.
type pageContent struct {
	YAMLKey     string
	Root        ReferenceEntry
	SubSections []ReferenceEntry
	SourcePath  string
}

// NewSourceData scans Go source files under sourcePath and indexes their type
// declarations. Package import paths are computed relative to modulePath using
// modulePrefix (e.g., "github.com/gravitational/teleport").
func NewSourceData(modulePrefix, modulePath, sourcePath string) (SourceData, error) {
	typeDecls := make(map[PackageInfo]DeclarationInfo)

	absModulePath, err := filepath.Abs(modulePath)
	if err != nil {
		return SourceData{}, fmt.Errorf("resolving module path %q: %w", modulePath, err)
	}

	err = filepath.Walk(sourcePath, func(currentPath string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(info.Name()) != ".go" {
			return nil
		}

		// Compute the Go package import path from the module root.
		dir := filepath.Dir(currentPath)
		absDir, err := filepath.Abs(dir)
		if err != nil {
			return fmt.Errorf("resolving %v: %w", dir, err)
		}
		rel, err := filepath.Rel(absModulePath, absDir)
		if err != nil {
			return fmt.Errorf("computing relative path from %v to %v: %w", absModulePath, absDir, err)
		}
		pkg := path.Join(modulePrefix, filepath.ToSlash(rel))

		f, err := os.Open(currentPath)
		if err != nil {
			return err
		}
		defer f.Close()

		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, currentPath, f, parser.ParseComments)
		if err != nil {
			// Skip files that fail to parse (e.g., build-tag-constrained files).
			return nil
		}

		relDeclPath, err := filepath.Rel(sourcePath, currentPath)
		if err != nil {
			return err
		}

		ni := namedImports(file)
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
				NamedImports: ni,
			}
		}
		return nil
	})
	if err != nil {
		return SourceData{}, fmt.Errorf("scanning source files: %w", err)
	}
	return SourceData{TypeDecls: typeDecls}, nil
}

// namedImports returns a map from the import alias (or base name) of each
// package imported by file to the full import path.
func namedImports(file *ast.File) map[string]string {
	m := make(map[string]string)
	for _, i := range file.Imports {
		pkgPath := strings.Trim(i.Path.Value, `"`)
		if i.Name == nil {
			m[path.Base(pkgPath)] = pkgPath
		} else {
			m[i.Name.Name] = pkgPath
		}
	}
	return m
}

var yamlTagPattern = regexp.MustCompile(`yaml:"([^"]+)"`)

// getYAMLTag extracts the field name from a yaml struct tag expression.
// Returns the empty string if there is no yaml tag or the tag has no explicit
// field name (e.g., yaml:",omitempty" or yaml:",inline").
func getYAMLTag(tags string) string {
	kv := yamlTagPattern.FindStringSubmatch(tags)
	if len(kv) != 2 {
		return ""
	}
	parts := strings.SplitN(kv[1], ",", 2)
	return parts[0]
}

// isYAMLInline reports whether the struct tag specifies the "inline" yaml option.
func isYAMLInline(tags string) bool {
	kv := yamlTagPattern.FindStringSubmatch(tags)
	if len(kv) != 2 {
		return false
	}
	parts := strings.SplitN(kv[1], ",", 2)
	if len(parts) < 2 {
		return false
	}
	for _, opt := range strings.Split(parts[1], ",") {
		if opt == "inline" {
			return true
		}
	}
	return false
}

// typeForDecl extracts a rawType from a DeclarationInfo.
func typeForDecl(decl DeclarationInfo, allDecls map[PackageInfo]DeclarationInfo) (rawType, error) {
	gendecl, ok := decl.Decl.(*ast.GenDecl)
	if !ok {
		return rawType{}, NotAGenDeclError{}
	}
	if len(gendecl.Specs) == 0 {
		return rawType{}, errors.New("declaration has no specs")
	}
	if len(gendecl.Specs) > 1 {
		return rawType{}, errors.New("declaration has more than one type spec")
	}
	t, ok := gendecl.Specs[0].(*ast.TypeSpec)
	if !ok {
		return rawType{}, errors.New("no type spec found")
	}

	str, ok := t.Type.(*ast.StructType)
	if !ok {
		return rawType{
			name: t.Name.Name,
			doc:  gendecl.Doc.Text(),
		}, nil
	}

	var rawFields []rawField
	for _, field := range str.Fields.List {
		f, err := makeRawField(field, decl.PackageName, allDecls, decl.NamedImports)
		if err != nil {
			return rawType{}, err
		}
		// Skip unexported fields.
		if f.name != "" && f.name[0] >= 'a' && f.name[0] <= 'z' {
			continue
		}
		// Skip fields tagged yaml:"-".
		if f.yamlName == "-" {
			continue
		}
		rawFields = append(rawFields, f)
	}

	return rawType{
		name:   t.Name.Name,
		doc:    gendecl.Doc.Text(),
		fields: rawFields,
	}, nil
}

// isByteSlice reports whether t is []byte.
func isByteSlice(t *ast.ArrayType) bool {
	i, ok := t.Elt.(*ast.Ident)
	return ok && i.Name == "byte"
}

// getYAMLTypeForExpr recursively converts an AST expression to a yamlKindNode.
func getYAMLTypeForExpr(
	exp ast.Expr,
	pkg string,
	allDecls map[PackageInfo]DeclarationInfo,
	imports map[string]string,
) (yamlKindNode, error) {
	switch t := exp.(type) {
	case *ast.StarExpr:
		// Pointer types marshal as the pointed-to type.
		return getYAMLTypeForExpr(t.X, pkg, allDecls, imports)
	case *ast.Ident:
		switch t.Name {
		case "string":
			return yamlString{}, nil
		case "uint", "uint8", "uint16", "uint32", "uint64",
			"int", "int8", "int16", "int32", "int64",
			"float32", "float64":
			return yamlNumber{}, nil
		case "bool":
			return yamlBool{}, nil
		default:
			info := PackageInfo{DeclName: t.Name, PackagePath: pkg}
			if _, ok := allDecls[info]; !ok {
				return nonYAMLKind{}, nil
			}
			return yamlCustomType{name: t.Name, declarationInfo: info}, nil
		}
	case *ast.MapType:
		k, err := getYAMLTypeForExpr(t.Key, pkg, allDecls, imports)
		if err != nil {
			return nil, err
		}
		v, err := getYAMLTypeForExpr(t.Value, pkg, allDecls, imports)
		if err != nil {
			return nil, err
		}
		return yamlMapping{keyKind: k, valueKind: v}, nil
	case *ast.ArrayType:
		if isByteSlice(t) {
			return yamlBase64{}, nil
		}
		e, err := getYAMLTypeForExpr(t.Elt, pkg, allDecls, imports)
		if err != nil {
			return nil, err
		}
		return yamlSequence{elementKind: e}, nil
	case *ast.SelectorExpr:
		var pkgPath string
		if x, ok := t.X.(*ast.Ident); ok {
			pkgPath = x.Name
			if full, ok := imports[x.Name]; ok {
				pkgPath = full
			}
		}
		info := PackageInfo{DeclName: t.Sel.Name, PackagePath: pkgPath}
		if _, ok := allDecls[info]; !ok {
			return nonYAMLKind{}, nil
		}
		return yamlCustomType{name: t.Sel.Name, declarationInfo: info}, nil
	default:
		return nonYAMLKind{}, nil
	}
}

// makeRawField converts an *ast.Field to a rawField for downstream processing.
func makeRawField(
	field *ast.Field,
	packageName string,
	allDecls map[PackageInfo]DeclarationInfo,
	imports map[string]string,
) (rawField, error) {
	doc := field.Doc.Text()
	if len(field.Names) > 1 {
		return rawField{}, fmt.Errorf("field in %v has more than one name", packageName)
	}

	var name string
	if len(field.Names) == 1 {
		name = field.Names[0].Name
	}

	kind, err := getYAMLTypeForExpr(field.Type, packageName, allDecls, imports)
	if err != nil {
		return rawField{}, err
	}

	// Determine the package for selector-typed fields.
	pkg := packageName
	if s, ok := field.Type.(*ast.SelectorExpr); ok {
		if ident, ok := s.X.(*ast.Ident); ok {
			if full, ok := imports[ident.Name]; ok {
				pkg = full
			} else {
				pkg = ident.Name
			}
		}
	}

	var tags string
	if field.Tag != nil {
		tags = field.Tag.Value
	}

	yamlName := getYAMLTag(tags)
	inline := isYAMLInline(tags)
	// Anonymous embedded fields without an explicit inline tag are also
	// inlined by gopkg.in/yaml.v2 (and v3).
	if name == "" && !inline && yamlName == "" {
		inline = true
	}

	return rawField{
		packageName: pkg,
		doc:         doc,
		kind:        kind,
		name:        name,
		yamlName:    yamlName,
		inline:      inline,
	}, nil
}

// allFieldsForDecl expands inline and anonymous embedded struct fields into the
// parent's field list, recursively.
func allFieldsForDecl(
	decl DeclarationInfo,
	fld []rawField,
	allDecls map[PackageInfo]DeclarationInfo,
) ([]rawField, error) {
	var result []rawField
	for _, f := range fld {
		if !f.inline && f.name != "" {
			result = append(result, f)
			continue
		}
		// Inline or anonymous embedded field: recursively include its fields.
		c, ok := f.kind.(yamlCustomType)
		if !ok {
			continue
		}

		// Resolve the full package path for the embedded type.
		pkg := decl.PackageName
		if full, ok := decl.NamedImports[f.packageName]; ok {
			pkg = full
		} else if f.packageName != "" {
			pkg = f.packageName
		}

		p := PackageInfo{DeclName: c.declarationInfo.DeclName, PackagePath: pkg}
		d, ok := allDecls[p]
		if !ok {
			return nil, fmt.Errorf("%v: embedded field %v not declared anywhere", decl.FilePath, c.name)
		}

		e, err := typeForDecl(d, allDecls)
		if err != nil && !errors.As(err, &NotAGenDeclError{}) {
			return nil, err
		}

		expanded, err := allFieldsForDecl(decl, e.fields, allDecls)
		if err != nil {
			return nil, err
		}
		result = append(result, expanded...)
	}
	return result, nil
}

// makeYAMLExample generates an example YAML document for a list of struct fields.
func makeYAMLExample(fields []rawField) string {
	var buf bytes.Buffer
	for _, f := range fields {
		name := f.yamlName
		if name == "" {
			name = strings.ToLower(f.name)
		}
		if name == "" || name == "-" {
			continue
		}
		buf.WriteString(name + ": ")
		buf.WriteString(f.kind.formatForExampleYAML(0) + "\n")
	}
	return buf.String()
}

var (
	curlyBracePairPattern  = regexp.MustCompile(`\{([^}]*)\}`)
	camelCaseWordBoundary  = regexp.MustCompile(`([a-z0-9])([A-Z][a-z0-9])`)
)

// splitCamelCase converts a camelCase identifier to a space-separated string,
// preserving any substrings listed in camelCaseExceptions.
func splitCamelCase(original string, camelCaseExceptions []string) string {
	if len(camelCaseExceptions) == 0 {
		return strings.Trim(camelCaseWordBoundary.ReplaceAllString(original, "$1 $2"), " ")
	}

	exceptionMap := make(map[string]struct{})
	for _, e := range camelCaseExceptions {
		exceptionMap[e] = struct{}{}
	}
	exceptions := regexp.MustCompile(fmt.Sprintf("(%v)", strings.Join(camelCaseExceptions, "|")))
	split := exceptions.ReplaceAllString(original, " $1 ")

	var result bytes.Buffer
	words := bufio.NewScanner(strings.NewReader(split))
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

// printableDescription cleans up a Go doc comment for use on a docs page.
// It removes the leading identifier name and fixes capitalization.
func printableDescription(description, ident string) string {
	result := curlyBracePairPattern.ReplaceAllString(description, "`{$1}`")
	result = strings.ReplaceAll(result, "``", "`")

	if len(ident) <= len(result) {
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
	}

	if len(result) > 0 {
		result = strings.ToUpper(result[:1]) + result[1:]
	}
	return result
}

// makeFieldTableInfo assembles human-readable field information for the reference table.
func makeFieldTableInfo(fields []rawField, camelCaseExceptions []string) []Field {
	var result []Field
	for _, f := range fields {
		name := f.yamlName
		if name == "" {
			name = strings.ToLower(f.name)
		}
		if name == "" || name == "-" {
			continue
		}

		desc := strings.Trim(strings.ReplaceAll(f.doc, "\n", " "), " ")
		desc = strings.ReplaceAll(desc, "|", `\|`)
		desc = strings.ReplaceAll(desc, "<", `\<`)
		desc = strings.ReplaceAll(desc, ">", `\>`)

		typ := f.kind.formatForTable(kindTableFormatOptions{
			camelCaseExceptions: camelCaseExceptions,
		})

		result = append(result, Field{
			Name:        name,
			Description: printableDescription(desc, f.name),
			Type:        typ,
		})
	}
	return result
}

func sortFieldsByName(a, b Field) int { return strings.Compare(a.Name, b.Name) }

// ReferenceDataFromDeclaration generates reference entries for decl and all
// types it references. Returns a map from PackageInfo to ReferenceEntry.
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

	example := makeYAMLExample(fieldsToProcess)
	description := strings.Trim(strings.ReplaceAll(rs.doc, "\n", " "), " ")
	key := PackageInfo{DeclName: rs.name, PackagePath: decl.PackageName}

	flds := makeFieldTableInfo(fieldsToProcess, camelCaseExceptions)
	slices.SortFunc(flds, sortFieldsByName)

	refs := make(map[PackageInfo]ReferenceEntry)
	refs[key] = ReferenceEntry{
		SectionName: splitCamelCase(rs.name, camelCaseExceptions),
		Description: printableDescription(description, rs.name),
		SourcePath:  decl.FilePath,
		YAMLExample: example,
		Fields:      flds,
	}

	// Recursively add reference entries for custom field types.
	for _, f := range fieldsToProcess {
		// Don't create sub-entries for inline fields; they are part of the
		// containing struct for YAML unmarshaling.
		if f.name == "" {
			continue
		}
		for _, d := range f.kind.customFieldData() {
			// Resolve the import alias to a full package path.
			if full, ok := decl.NamedImports[d.PackagePath]; ok {
				d.PackagePath = full
			}
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

// yamlKeyToTitle converts a yaml key like "auth_service" to "Auth Service".
func yamlKeyToTitle(key string) string {
	words := strings.Split(key, "_")
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// indentLines prepends prefix to every non-empty line of s.
func indentLines(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = prefix + line
		}
	}
	return strings.Join(lines, "\n")
}

const tmplBase = "generator.tmpl"

// Generate reads Go source data, finds the entry type, and writes one MDX file
// per service section (fields whose yaml tag ends with ServiceSuffix).
func Generate(conf GeneratorConfig, tmpl *template.Template) error {
	suffix := conf.ServiceSuffix
	if suffix == "" {
		suffix = "_service"
	}

	sourceData, err := NewSourceData(conf.ModulePrefix, conf.ModulePath, conf.SourcePath)
	if err != nil {
		return fmt.Errorf("loading Go source: %w", err)
	}

	// Find the entry type (e.g., FileConfig).
	var entryDecl DeclarationInfo
	var found bool
	for k, v := range sourceData.TypeDecls {
		if k.DeclName == conf.EntryType {
			entryDecl = v
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("cannot find entry type %q in source tree", conf.EntryType)
	}

	entryType, err := typeForDecl(entryDecl, sourceData.TypeDecls)
	if err != nil {
		return fmt.Errorf("reading entry type %q: %w", conf.EntryType, err)
	}

	var errs []error
	for _, field := range entryType.fields {
		if !strings.HasSuffix(field.yamlName, suffix) {
			continue
		}

		c, ok := field.kind.(yamlCustomType)
		if !ok {
			continue
		}

		decl, ok := sourceData.TypeDecls[c.declarationInfo]
		if !ok {
			errs = append(errs, fmt.Errorf("field %q: cannot find declaration for type %v in %v",
				field.yamlName, c.declarationInfo.DeclName, c.declarationInfo.PackagePath))
			continue
		}

		entries, err := ReferenceDataFromDeclaration(conf.ModulePrefix, decl, sourceData.TypeDecls, conf.CamelCaseExceptions)
		if errors.As(err, &NotAGenDeclError{}) {
			continue
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("field %q: %w", field.yamlName, err))
			continue
		}

		rootKey := c.declarationInfo
		rootEntry := entries[rootKey]
		delete(entries, rootKey)

		// Sort sub-sections for deterministic output.
		var subSections []ReferenceEntry
		for _, e := range entries {
			subSections = append(subSections, e)
		}
		slices.SortFunc(subSections, func(a, b ReferenceEntry) int {
			return strings.Compare(a.SectionName, b.SectionName)
		})

		pc := pageContent{
			YAMLKey:     field.yamlName,
			Root:        rootEntry,
			SubSections: subSections,
			SourcePath:  decl.FilePath,
		}

		filename := strings.ReplaceAll(field.yamlName, "_", "-") + ".mdx"
		docpath := filepath.Join(conf.DestinationDirectory, filename)
		doc, err := os.Create(docpath)
		if err != nil {
			errs = append(errs, fmt.Errorf("cannot create %v: %w", docpath, err))
			continue
		}
		defer doc.Close()

		if err := tmpl.Execute(doc, pc); err != nil {
			errs = append(errs, fmt.Errorf("cannot render template for %v: %w", field.yamlName, err))
		}
	}
	return errors.Join(errs...)
}

// NewTemplate loads and returns the MDX template with helper functions registered.
func NewTemplate(tmplPath string) (*template.Template, error) {
	return template.New(tmplBase).Funcs(template.FuncMap{
		"title": yamlKeyToTitle,
		"indent": func(s, prefix string) string {
			return indentLines(s, prefix)
		},
	}).ParseFiles(tmplPath)
}

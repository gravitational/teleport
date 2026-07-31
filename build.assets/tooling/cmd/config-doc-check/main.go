// Teleport
// Copyright (C) 2026  Gravitational, Inc.
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

package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	configHelp            = "path to a YAML configuration file (see the README)"
	teleportPackagePrefix = "github.com/gravitational/teleport"
)

// serviceSectionInfo configures the documentation YAML sections checked in one example file.
type serviceSectionInfo struct {
	// Name is a human-readable label printed in output.
	Name string `yaml:"name"`
	// ExamplePath is the path to the doc example YAML, relative to the repo root.
	ExamplePath  string        `yaml:"example_path"`
	KeyTypePairs []KeyTypePair `yaml:"key_type_pairs"`
	// DismissedKeys are exact YAML key tree paths, which should be ignored when comparing the example YAML with the struct.
	DismissedKeys []string `yaml:"dismissed_keys"`
}

// KeyTypePair pairs a top-level configuration YAML section key with its corresponding Go struct type name.
type KeyTypePair struct {
	SectionKey string `yaml:"section_key"`
	TypeName   string `yaml:"type_name"`
}

// checkerConfig is the user-facing configuration of the configuration reference checker.
type checkerConfig struct {
	// Path to the root of the Go project directory.
	SourcePath string `yaml:"source_path"`
	// List of the configuration sections to check.
	ServiceSections []serviceSectionInfo `yaml:"service_sections"`
}

// loadConfigFile reads the checker config from YAML and validates its contents.
func loadConfigFile(path string) (*checkerConfig, error) {
	conffile, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening configuration file %q: %w", path, err)
	}
	defer conffile.Close()

	var config checkerConfig
	if err := yaml.NewDecoder(conffile).Decode(&config); err != nil {
		return nil, fmt.Errorf("parsing configuration file %q: %w", path, err)
	}
	if config.SourcePath == "" {
		return nil, fmt.Errorf("checker config has no source path")
	}

	if len(config.ServiceSections) == 0 {
		return nil, fmt.Errorf("checker config has no service sections")
	}

	for index, section := range config.ServiceSections {
		if section.Name == "" || section.ExamplePath == "" {
			return nil, fmt.Errorf("service section %d must define name and example_path", index)
		}
		if len(section.KeyTypePairs) == 0 {
			return nil, fmt.Errorf("service section %d must define key_type_pairs", index)
		}
		sectionKeys := make(map[string]struct{}, len(section.KeyTypePairs))
		for _, pair := range section.KeyTypePairs {
			if pair.SectionKey == "" || pair.TypeName == "" {
				return nil, fmt.Errorf("service section %d has a section without section_key or type_name", index)
			}
			if _, ok := sectionKeys[pair.SectionKey]; ok {
				return nil, fmt.Errorf("duplicate service section key %q", pair.SectionKey)
			}
			sectionKeys[pair.SectionKey] = struct{}{}
		}
	}

	return &config, nil
}

var valueOptionsPattern = regexp.MustCompile(`("[^"]*")(?:\|"[^"]*")+`)

// preprocessExampleYAML matches documentation notation to display value options
// for a field like "a"|"b"|"c". As this is not valid YAML, we replace it with just
// the first value.
func preprocessExampleYAML(data []byte) []byte {
	return valueOptionsPattern.ReplaceAll(data, []byte(`$1`))
}

// tree is a node in the expected YAML key tree. A nil *tree means the key is a leaf (a scalar, a list of scalars, etc.).
type yamlKeyTree struct {
	// children maps yaml field names to their sub-trees.
	// A nil value means the child is a leaf.
	children map[string]*yamlKeyTree
	// hasInlineMap is true when the struct has a map field tagged yaml:",inline",
	// meaning arbitrary extra keys are valid at this level.
	hasInlineMap bool
}

var yamlTagKeyValue = regexp.MustCompile(`(?:^|\s)yaml:"([^"]*)"`)

// typeDecl represents a type declaration in a Go source file, including its AST expression and imports.
type typeDecl struct {
	expr    ast.Expr
	imports map[string]string
}

// sourcePackage represents a Go package with its type declarations and types that implement MarshalYAML returning a scalar.
type sourcePackage struct {
	types           map[string]typeDecl
	yamlScalarTypes map[string]bool
}

// configKeyTreeBuilder builds a tree of YAML keys from Go struct types in the source code.
type configKeyTreeBuilder struct {
	// absolute path to the project root
	rootPath string
	// cache of loaded source packages
	sourcePackages map[string]*sourcePackage
	// cache of built YAML key trees for type names
	treeCache map[string]*yamlKeyTree
	// tracks types currently being processed (avoids infinite recursion)
	inProgressTypes map[string]bool
}

// treeForTypeName builds a YAML key tree for the given type name in the specified import path.
func (b *configKeyTreeBuilder) treeForTypeName(importPath, typeName string) (*yamlKeyTree, error) {
	cacheKey := importPath + "." + typeName
	if result, ok := b.treeCache[cacheKey]; ok {
		return result, nil
	}
	if b.inProgressTypes[cacheKey] {
		return nil, nil
	}
	b.inProgressTypes[cacheKey] = true
	defer delete(b.inProgressTypes, cacheKey)

	pkg, err := b.loadSourcePackage(importPath)
	if err != nil {
		return nil, err
	}
	decl, ok := pkg.types[typeName]
	if !ok {
		return nil, nil
	}
	if pkg.yamlScalarTypes[typeName] {
		return nil, nil
	}

	result, err := b.treeFromExpr(importPath, decl.imports, decl.expr)
	if err != nil {
		return nil, err
	}
	b.treeCache[cacheKey] = result
	return result, nil
}

// loadSourcePackage loads the Go source package at the given import path and caches its type declarations.
func (b *configKeyTreeBuilder) loadSourcePackage(importPath string) (*sourcePackage, error) {
	// Return cached package if already loaded.
	if pkg, ok := b.sourcePackages[importPath]; ok {
		return pkg, nil
	}

	if importPath != teleportPackagePrefix && !strings.HasPrefix(importPath, teleportPackagePrefix+"/") {
		return &sourcePackage{
			types:           map[string]typeDecl{},
			yamlScalarTypes: map[string]bool{},
		}, nil
	}
	dir := filepath.Join(b.rootPath, strings.TrimPrefix(importPath, teleportPackagePrefix))

	fset := token.NewFileSet()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading package directory %s: %w", importPath, err)
	}

	var files []*ast.File
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		filePath := filepath.Join(dir, entry.Name())
		file, err := parser.ParseFile(fset, filePath, nil, 0)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", filePath, err)
		}
		files = append(files, file)
	}

	pkg := &sourcePackage{
		types:           make(map[string]typeDecl),
		yamlScalarTypes: make(map[string]bool),
	}
	for _, file := range files {
		imports := make(map[string]string)
		for _, spec := range file.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return nil, fmt.Errorf("parsing import in %s: %w", importPath, err)
			}
			name := filepath.Base(path)
			if spec.Name != nil {
				name = spec.Name.Name
			}
			imports[name] = path
		}
		for _, decl := range file.Decls {
			if funcDecl, ok := decl.(*ast.FuncDecl); ok {
				receiverName := receiverTypeName(funcDecl)
				if receiverName != "" && funcDecl.Name.Name == "MarshalYAML" && marshalYAMLReturnsScalar(funcDecl.Body) {
					pkg.yamlScalarTypes[receiverName] = true
				}
				continue
			}
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}
			for _, spec := range genDecl.Specs {
				typeSpec := spec.(*ast.TypeSpec)
				pkg.types[typeSpec.Name.Name] = typeDecl{expr: typeSpec.Type, imports: imports}
			}
		}
	}
	b.sourcePackages[importPath] = pkg
	return pkg, nil
}

func receiverTypeName(decl *ast.FuncDecl) string {
	if decl.Recv == nil || len(decl.Recv.List) != 1 {
		return ""
	}
	return embeddedFieldName(decl.Recv.List[0].Type)
}

func marshalYAMLReturnsScalar(body *ast.BlockStmt) bool {
	if body == nil {
		return false
	}
	foundValue := false
	returnsScalar := true
	ast.Inspect(body, func(node ast.Node) bool {
		returnStmt, ok := node.(*ast.ReturnStmt)
		if !ok || len(returnStmt.Results) == 0 {
			return true
		}
		if ident, ok := returnStmt.Results[0].(*ast.Ident); ok && ident.Name == "nil" {
			return true
		}
		foundValue = true
		if call, ok := returnStmt.Results[0].(*ast.CallExpr); ok {
			if selector, ok := call.Fun.(*ast.SelectorExpr); ok && strings.Contains(strings.ToLower(selector.Sel.Name), "map") {
				returnsScalar = false
			}
		}
		if composite, ok := returnStmt.Results[0].(*ast.CompositeLit); ok {
			switch composite.Type.(type) {
			case *ast.MapType, *ast.StructType, *ast.ArrayType:
				returnsScalar = false
			}
		}
		return true
	})
	return foundValue && returnsScalar
}

// treeFromExpr builds the YAML key tree represented by a Go type expression.
func (b *configKeyTreeBuilder) treeFromExpr(importPath string, imports map[string]string, expr ast.Expr) (*yamlKeyTree, error) {
	switch typedExpr := expr.(type) {
	case *ast.StarExpr:
		return b.treeFromExpr(importPath, imports, typedExpr.X)
	case *ast.ArrayType:
		return b.treeFromExpr(importPath, imports, typedExpr.Elt)
	case *ast.MapType:
		return &yamlKeyTree{hasInlineMap: true}, nil
	case *ast.StructType:
		return b.treeFromStruct(importPath, imports, typedExpr)
	case *ast.Ident:
		return b.treeForTypeName(importPath, typedExpr.Name)
	case *ast.SelectorExpr:
		packageName, ok := typedExpr.X.(*ast.Ident)
		if !ok {
			return nil, nil
		}
		selectedImportPath, ok := imports[packageName.Name]
		if !ok || !strings.HasPrefix(selectedImportPath, teleportPackagePrefix) {
			return nil, nil
		}
		return b.treeForTypeName(selectedImportPath, typedExpr.Sel.Name)
	default:
		return nil, nil
	}
}

// treeFromStruct builds a YAML key tree from a Go struct type.
func (b *configKeyTreeBuilder) treeFromStruct(importPath string, imports map[string]string, structType *ast.StructType) (*yamlKeyTree, error) {
	result := &yamlKeyTree{children: make(map[string]*yamlKeyTree)}
	for _, field := range structType.Fields.List {
		fieldName := embeddedFieldName(field.Type)
		if len(field.Names) > 0 {
			fieldName = field.Names[0].Name
		}
		if !ast.IsExported(fieldName) || strings.HasPrefix(fieldName, "XXX_") {
			continue
		}

		tag := ""
		if field.Tag != nil {
			structTag, err := strconv.Unquote(field.Tag.Value)
			if err != nil {
				return nil, fmt.Errorf("parsing struct tag on %s: %w", fieldName, err)
			}
			matches := yamlTagKeyValue.FindStringSubmatch(structTag)
			if len(matches) == 2 {
				tag = matches[1]
			}
		}
		if tag == "-" {
			continue
		}
		parts := strings.SplitN(tag, ",", 2)
		name := parts[0]
		inline := len(parts) == 2 && strings.Contains(parts[1], "inline")

		child, err := b.treeFromExpr(importPath, imports, field.Type)
		if err != nil {
			return nil, err
		}
		if inline {
			if child != nil {
				for key, value := range child.children {
					result.children[key] = value
				}
				result.hasInlineMap = result.hasInlineMap || child.hasInlineMap
			}
			continue
		}
		if name == "" {
			name = strings.ToLower(fieldName)
		}
		result.children[name] = child
	}
	if len(result.children) == 0 && !result.hasInlineMap {
		return nil, nil
	}
	return result, nil
}

// embeddedFieldName returns the name of an embedded field type.
func embeddedFieldName(expr ast.Expr) string {
	switch typedExpr := expr.(type) {
	case *ast.Ident:
		return typedExpr.Name
	case *ast.SelectorExpr:
		return typedExpr.Sel.Name
	case *ast.StarExpr:
		return embeddedFieldName(typedExpr.X)
	default:
		return ""
	}
}

// exampleTreeFromAny builds a key tree from a parsed YAML value.
// For sequences it uses the first map element to infer key structure.
func exampleTreeFromAny(v any) *yamlKeyTree {
	switch m := v.(type) {
	case map[string]any:
		result := &yamlKeyTree{children: make(map[string]*yamlKeyTree)}
		for k, val := range m {
			result.children[k] = exampleTreeFromAny(val)
		}
		return result
	case []any:
		// Sequence: infer keys from the first map element, if any.
		for _, elem := range m {
			if sub, ok := elem.(map[string]any); ok {
				return exampleTreeFromAny(sub)
			}
		}
		return nil
	default:
		return nil
	}
}

// difference is a set of key differences at a particular YAML path.
type difference struct {
	Path         string
	OnlyInStruct []string // defined in struct but absent from doc
	OnlyInDoc    []string // present in doc but absent from struct
}

// compareTrees recursively compares structTree and exampleTree, ignoring keys in dismissedKeys.
func compareTrees(structTree, exampleTree *yamlKeyTree, path string, dismissedKeys map[string]struct{}) []difference {
	// Normalise nils to empty trees for uniform handling.
	if structTree == nil {
		structTree = &yamlKeyTree{}
	}
	if exampleTree == nil {
		exampleTree = &yamlKeyTree{}
	}

	structChildren := structTree.children
	docChildren := exampleTree.children
	if structChildren == nil {
		structChildren = map[string]*yamlKeyTree{}
	}
	if docChildren == nil {
		docChildren = map[string]*yamlKeyTree{}
	}

	var differences []difference

	// Keys defined in struct but absent from doc.
	var onlyInStruct []string
	for k := range structChildren {
		if keyShouldBeDismissed(path, k, dismissedKeys) {
			continue
		}
		if _, ok := docChildren[k]; !ok {
			onlyInStruct = append(onlyInStruct, k)
		}
	}
	sort.Strings(onlyInStruct)

	// Keys present in doc but absent from struct.
	// Skip this check when the struct level has an inline map (arbitrary keys are valid).
	var onlyInDoc []string
	if !structTree.hasInlineMap {
		for k := range docChildren {
			if keyShouldBeDismissed(path, k, dismissedKeys) {
				continue
			}
			if _, ok := structChildren[k]; !ok {
				onlyInDoc = append(onlyInDoc, k)
			}
		}
		sort.Strings(onlyInDoc)
	}

	if len(onlyInStruct) > 0 || len(onlyInDoc) > 0 {
		differences = append(differences, difference{
			Path:         path,
			OnlyInStruct: onlyInStruct,
			OnlyInDoc:    onlyInDoc,
		})
	}

	// Recurse into keys present in both trees.
	// Collect keys to ensure deterministic ordering.
	commonKeys := make([]string, 0, len(structChildren))
	for k := range structChildren {
		if keyShouldBeDismissed(path, k, dismissedKeys) {
			continue
		}
		if _, ok := docChildren[k]; ok {
			commonKeys = append(commonKeys, k)
		}
	}
	sort.Strings(commonKeys)

	for _, k := range commonKeys {
		sChild := structChildren[k]
		dChild := docChildren[k]
		childPath := path + "." + k

		// If the struct child is an opaque map, skip recursion.
		if sChild != nil && sChild.hasInlineMap && len(sChild.children) == 0 {
			continue
		}

		childFindings := compareTrees(sChild, dChild, childPath, dismissedKeys)
		differences = append(differences, childFindings...)
	}

	return differences
}

func keyShouldBeDismissed(path, key string, dismissedKeys map[string]struct{}) bool {
	_, dismiss := dismissedKeys[path+"."+key]
	return dismiss
}

func main() {
	configPath := flag.String("config", "config.yaml", configHelp)
	flag.Parse()

	config, err := loadConfigFile(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	rootAbs, err := filepath.Abs(config.SourcePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error resolving absolute root path: %v\n", err)
		os.Exit(1)
	}

	// Keep track of whether any differences were reported between the documented configuration examples
	// and the actual configuration structs.
	hasDifferences := false
	treeBuilder := &configKeyTreeBuilder{
		rootPath:        rootAbs,
		sourcePackages:  make(map[string]*sourcePackage),
		treeCache:       make(map[string]*yamlKeyTree),
		inProgressTypes: make(map[string]bool),
	}
	// For each service section, unmarshal the example YAML and compare with the actual configuration struct.
	for _, section := range config.ServiceSections {
		dismissedKeys := make(map[string]struct{}, len(section.DismissedKeys))
		for _, key := range section.DismissedKeys {
			dismissedKeys[key] = struct{}{}
		}

		examplePath := filepath.Join(rootAbs, section.ExamplePath)

		data, err := os.ReadFile(examplePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: cannot read the configuration example %s: %v\n", section.ExamplePath, err)
			continue
		}

		var unmarshaledExample map[string]any
		if err := yaml.Unmarshal(preprocessExampleYAML(data), &unmarshaledExample); err != nil {
			fmt.Fprintf(os.Stderr, "warning: cannot parse the configuration example %s: %v\n", section.ExamplePath, err)
			continue
		}

		var differences []difference
		failedToProcessSection := false
		for _, pair := range section.KeyTypePairs {
			// Look up the section key in the unmarshaled example YAML.
			exampleSectionValue, ok := unmarshaledExample[pair.SectionKey]
			if !ok {
				fmt.Printf("*** %s (%s):  WARNING: section %q not found ***\n\n",
					section.Name, section.ExamplePath, pair.SectionKey)
				failedToProcessSection = true
				continue
			}

			exampleTree := exampleTreeFromAny(exampleSectionValue)

			// Build the YAML key tree for the struct type corresponding to this service section.
			structTree, err := treeBuilder.treeForTypeName(fmt.Sprintf("%s/lib/config", teleportPackagePrefix), pair.TypeName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: cannot inspect config type %s: %v\n", pair.TypeName, err)
				failedToProcessSection = true
				continue
			}

			differences = append(differences, compareTrees(
				structTree,
				exampleTree,
				pair.SectionKey,
				dismissedKeys,
			)...)
		}
		if failedToProcessSection {
			continue
		}

		if len(differences) == 0 {
			fmt.Printf("*** %s (%s): OK ***\n", section.Name, section.ExamplePath)
			continue
		}

		hasDifferences = true
		fmt.Printf("*** %s (%s) ***\n", section.Name, section.ExamplePath)
		for _, f := range differences {
			if len(f.OnlyInStruct) > 0 {
				fmt.Printf("  missing from the example at %s:\n", f.Path)
				for _, k := range f.OnlyInStruct {
					fmt.Printf("    - %s.%s\n", f.Path, k)
				}
			}
			if len(f.OnlyInDoc) > 0 {
				fmt.Printf("  extra in the example (not in struct) at %s:\n", f.Path)
				for _, k := range f.OnlyInDoc {
					fmt.Printf("    - %s.%s\n", f.Path, k)
				}
			}
		}
		fmt.Println()
	}

	if hasDifferences {
		os.Exit(1)
	}
}

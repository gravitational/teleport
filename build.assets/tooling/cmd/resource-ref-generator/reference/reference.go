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

package reference

import (
	"errors"
	"fmt"
	"gen-resource-ref/resource"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"text/template"
)

// content represents the sections of the resource reference.
// Fields must be exported so we can use them in templates.
type content struct {
	Resources map[resource.PackageInfo]resourceSection
	Fields    map[resource.PackageInfo]resource.ReferenceEntry
}

// resourceSection represents a top-level section of the resource reference
// dedicated to a dynamic resource.
type resourceSection struct {
	Version string
	Kind    string
	resource.ReferenceEntry
}

// Intended to be executed with a ReferenceContent.
// Ampersands are replaced with backticks.
var referenceTemplate string = strings.ReplaceAll(`
## Resources

{{ range .Resources }}
### {{ .SectionName }}

**Kind**: &{{ .Kind }}&
**Version**: &{{ .Version}}&

{{ .Description }}

{/*Automatically generated from: {{ .SourcePath}}*/}

{{ if gt (len .Fields) 0 }}
|Field Name|Description|Type|
|---|---|---|
{{ range .Fields -}}
|{{.Name}}|{{.Description}}|{{.Type}}|
{{ end }} 
{{ end }}

Example:

&&&yaml
{{ .YAMLExample }}
&&&
{{ end }}

## Resource fields

{{ range .Fields }}
### {{ .SectionName }}

{{ .Description }}

{/*Automatically generated from: {{ .SourcePath}}*/}

{{ if gt (len .Fields) 0 }}
|Field Name|Description|Type|
|---|---|---|
{{ range .Fields -}}
|{{.Name}}|{{.Description}}|&{{.Type}}&|
{{ end }} 
{{ end }}

Example:

&&&yaml
{{ .YAMLExample }}
&&&
{{ end }}
`, "&", "`")

// TypeInfo represents the name and package name of an exported Go type. It
// makes no guarantees about whether the type was actually declared within the
// package.
type TypeInfo struct {
	// Go package path (not a file path)
	Package string `yaml:"package"`
	// Name of the type, e.g., Metadata
	Name string `yaml:"name"`
}

// GeneratorConfig is the user-facing configuration for the resource reference
// generator.
type GeneratorConfig struct {
	// Field types that a type must have to be included in the reference. A
	// type must have one of these field types to be included in the
	// reference. The fields named here can be embedded fields.
	RequiredFieldTypes []TypeInfo `yaml:"required_field_types"`
	// Path to the root of the Go project directory.
	SourcePath string `yaml:"source"`
	// Path of the resource reference.
	DestinationPath string `yaml:"destination"`
	// Struct types to exclude from the reference.
	ExcludedResourceTypes []TypeInfo `yaml:"excluded_resource_types"`
	// The name of the method that assigns values to the required fields
	// within a dynamic resource. The generator determines that a type is a
	// dynamic resource if it has this method.
	FieldAssignmentMethodName string `yaml:"field_assignment_method"`
}

// Validate checks that the GeneratorConfig includes all required fields and, if
// not, returns the first error it encounters.
func (c GeneratorConfig) Validate() error {
	switch {
	case c.DestinationPath == "":
		return errors.New("no destination path provided")
	case c.FieldAssignmentMethodName == "":
		return errors.New("must provide a field assignment method name")
	case c.RequiredFieldTypes == nil || len(c.RequiredFieldTypes) == 0:
		return errors.New("must provide a list of required field types")
	case c.SourcePath == "":
		return errors.New("must provide a source path")
	default:
		return nil
	}
}

// shouldProcess indicates whether we should generate reference entries from the
// type declaration represented in d (i.e., whether this is a dynamic resource
// type). To do so, it checks whether d:
//   - is a struct type
//   - has fields with the required types
//   - does not belong to the list of excluded resources
func shouldProcess(d resource.DeclarationInfo, requiredTypes, excludedResources []TypeInfo) bool {
	// We expect the declaration to be a type declaration with one spec.
	gendecl, ok := d.Decl.(*ast.GenDecl)
	if !ok {
		return false
	}

	if len(gendecl.Specs) != 1 {
		return false
	}

	t, ok := gendecl.Specs[0].(*ast.TypeSpec)
	if !ok {
		return false
	}

	if t == nil {
		return false
	}

	// If the declaration type is not a struct, we can't process it as a
	// root resource entry.
	str, ok := t.Type.(*ast.StructType)
	if !ok {
		return false
	}

	// If the configured excluded resources include this type declaration,
	// don't process it.
	for _, r := range excludedResources {
		if t.Name.Name == r.Name && d.PackageName == r.Package {
			return false
		}
	}
	// Use only the final segment of each desired package path
	// in the comparison, since that is what is preserved in the
	// AST.
	finalTypes := make([]TypeInfo, len(requiredTypes))
	for i, t := range requiredTypes {
		segs := strings.Split(t.Package, "/")
		finalTypes[i] = TypeInfo{
			Package: segs[len(segs)-1],
			Name:    t.Name,
		}
	}

	// Compare the types of fields in the struct type with the required
	// fields types. Only one required field type must be present.
	var m bool
	for _, fld := range str.Fields.List {
		if len(fld.Names) != 1 {
			continue
		}

		// Identify a package for the field type so we can check it
		// against the required field list. Begin by assuming the field
		// comes from the same package as the outer struct type, then
		// assign a package name depending on the expression used to
		// declare the field type.
		gopkg := d.PackageName
		var fldname string
		switch t := fld.Type.(type) {
		case *ast.SelectorExpr:
			// If the type of the field is an *ast.SelectorExpr,
			// it's of the form <package>.<type name>.
			g, ok := t.X.(*ast.Ident)
			if ok {
				gopkg = g.Name
			}
			fldname = t.Sel.Name

		// There's no package, so only assign a name.
		case *ast.Ident:
			fldname = t.Name
		}

		for _, ti := range finalTypes {
			if gopkg == ti.Package && fldname == ti.Name {
				m = true
				break
			}
		}
	}

	return m
}

// Generate uses the provided user-facing configuration to write the resource
// reference to out.
func Generate(out io.Writer, conf GeneratorConfig) error {
	typeDecls := make(map[resource.PackageInfo]resource.DeclarationInfo)
	possibleFuncDecls := []resource.DeclarationInfo{}
	stringAssignments := make(map[resource.PackageInfo]string)

	// Load each file in the source directory individually. Not using
	// packages.Load here since the resulting []*Package does not expose
	// individual file names, which we need so contributors who want to edit
	// the resulting docs page know which files to modify.
	err := filepath.WalkDir(conf.SourcePath, func(path string, info fs.DirEntry, err error) error {
		// There is an error with the path, so we can't load Go source.
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if filepath.Ext(info.Name()) != ".go" {
			return nil
		}

		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return err
		}

		str, err := resource.GetTopLevelStringAssignments(file.Decls, file.Name.Name)
		if err != nil {
			return err
		}

		for k, v := range str {
			stringAssignments[k] = v
		}

		for _, decl := range file.Decls {
			di := resource.DeclarationInfo{
				Decl:        decl,
				FilePath:    info.Name(),
				PackageName: file.Name.Name,
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

			typeDecls[resource.PackageInfo{
				DeclName:    spec.Name.Name,
				PackageName: file.Name.Name,
			}] = resource.DeclarationInfo{
				Decl:        l,
				FilePath:    info.Name(),
				PackageName: file.Name.Name,
			}

		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("can't load Go source files: %v", err)
	}

	methods, err := resource.GetMethodInfo(possibleFuncDecls)
	if err != nil {
		return err
	}

	content := content{
		Resources: make(map[resource.PackageInfo]resourceSection),
		Fields:    make(map[resource.PackageInfo]resource.ReferenceEntry),
	}

	for k, decl := range typeDecls {
		if !shouldProcess(decl, conf.RequiredFieldTypes, conf.ExcludedResourceTypes) {
			continue
		}

		// decl is a dynamic resource type, so get data for the type and
		// its dependencies.
		entries, err := resource.ReferenceDataFromDeclaration(decl, typeDecls, methods)
		if errors.Is(err, resource.NotAGenDeclError{}) {
			continue
		}
		if err != nil {
			return fmt.Errorf("issue creating a reference entry for declaration %v.%v in file %v: %v", k.PackageName, k.DeclName, decl.FilePath, err)
		}

		// Add each reference entry to its appropriate place in the
		// reference, either as a resource or as a field. Resources
		// require a version number and `kind` value, so we search the
		// methods of the resource type for the one that specifies these
		// values.
		for pi, e := range entries {
			entryMethods, ok := methods[pi]
			// Can't be a resource since it does not have methods.
			if !ok {
				content.Fields[pi] = e
				continue
			}
			var foundMethods bool
			for _, method := range entryMethods {
				if method.Name != conf.FieldAssignmentMethodName {
					continue
				}

				ver, ok1 := method.FieldAssignments["Version"]
				kind, ok2 := method.FieldAssignments["Kind"]

				// The version and kind weren't assigned
				if !ok1 || !ok2 {
					continue
				}

				// So far, all values of "Kind" and "Version"
				// are declared in the same package as the types
				// that include these fields.
				verName, ok1 := stringAssignments[resource.PackageInfo{
					DeclName:    ver,
					PackageName: pi.PackageName,
				}]

				kindName, ok2 := stringAssignments[resource.PackageInfo{
					DeclName:    kind,
					PackageName: pi.PackageName,
				}]

				if !ok1 || !ok2 {
					continue
				}

				ref := resourceSection{
					ReferenceEntry: e,
					Version:        verName,
					Kind:           kindName,
				}

				content.Resources[pi] = ref
				foundMethods = true
				break
			}
			if !foundMethods {
				content.Fields[pi] = e
			}
		}
	}

	err = template.Must(template.New("Main reference").Parse(referenceTemplate)).Execute(out, content)
	return nil
}

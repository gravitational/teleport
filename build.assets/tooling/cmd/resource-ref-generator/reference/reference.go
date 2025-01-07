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
	"go/ast"
	"path"
	"strings"
	"text/template"

	"github.com/gravitational/teleport/build.assets/tooling/cmd/resource-ref-generator/resource"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
)

// pageContent represents a reference page for a single resource and its related
// fields. Fields must be exported so we can use them in templates.
type pageContent struct {
	Resource resourceSection
	Fields   map[resource.PackageInfo]resource.ReferenceEntry
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
var referenceTemplate string = strings.ReplaceAll(`---
title: {{ .Resource.SectionName }} Reference
description: Provides a reference of fields within the {{ .Resource.SectionName }} resource, which you can manage with tctl.
sidebar_title: {{ .Resource.SectionName }}
---
{{- if ne .Resource.Kind "" }}

**Kind**: {{ .Resource.Kind }}
{{- end }}

{{- if ne .Resource.Version "" }}
**Version**: {{ .Resource.Version }}
{{- end }}

{{ .Resource.Description }}

{/*Automatically generated from: {{ .Resource.SourcePath}}*/}

Example:

&&&yaml
{{ .Resource.YAMLExample -}}
&&&

{{- if gt (len .Resource.Fields) 0 }}
|Field Name|Description|Type|
|---|---|---|
{{ range .Resource.Fields -}}
|{{.Name}}|{{.Description}}|{{.Type}}|
{{ end }} 
{{- end }}

{{- range .Fields }}
## {{ .SectionName }}

{{ .Description }}

{/*Automatically generated from: {{ .SourcePath}}*/}
{{ if ne .YAMLExample "" }}
Example:

&&&yaml
{{ .YAMLExample -}}
&&&
{{ end }}
{{- if gt (len .Fields) 0 }}
|Field Name|Description|Type|
|---|---|---|
{{ range .Fields -}}
|{{.Name}}|{{.Description}}|{{.Type}}|
{{ end }} 
{{- end }}
{{- end }}
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
	// Directory where the generator writes reference pages.
	DestinationDirectory string `yaml:"destination"`
	// Struct types to exclude from the reference.
	ExcludedResourceTypes []TypeInfo `yaml:"excluded_resource_types"`
	// The name of the method that assigns values to the required fields
	// within a dynamic resource. The generator determines that a type is a
	// dynamic resource if it has this method.
	FieldAssignmentMethodName string `yaml:"field_assignment_method"`
}

// UnmarshalYAML checks that the GeneratorConfig includes all required fields and, if
// not, returns the first error it encounters.
func (c GeneratorConfig) UnmarshalYAML(value *yaml.Node) error {
	if err := value.Decode(&c); err != nil {
		return fmt.Errorf("could not parse the configuration file as YAML: %v\n", err)
	}

	switch {
	case c.DestinationDirectory == "":
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

// getPackageInfoFromExpr extracts a package name and declaration name from an
// arbitrary expression. If the expression is not an expected kind,
// getPackageInfoFromExpr returns an empty PackageInfo.
func getPackageInfoFromExpr(expr ast.Expr) resource.PackageInfo {
	var gopkg, fldname string
	switch t := expr.(type) {
	case *ast.StarExpr:
		return getPackageInfoFromExpr(t.X)
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
	return resource.PackageInfo{
		DeclName:    fldname,
		PackageName: gopkg,
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
		pi := getPackageInfoFromExpr(fld.Type)
		if pi.PackageName != "" {
			gopkg = pi.PackageName
		}

		for _, ti := range finalTypes {
			if gopkg == ti.Package && pi.DeclName == ti.Name {
				m = true
				break
			}
		}
	}

	return m
}

type GenerationError struct {
	messages []error
}

func (g GenerationError) Error() string {
	// Begin with a newline to format the first list item below the outer
	// error.
	final := "\n"
	for _, e := range g.messages {
		final += fmt.Sprintf("- %v\n", e)
	}
	return final
}

// Generate uses the provided user-facing configuration to write the resource
// reference to fs.
func Generate(srcFS, destFS afero.Fs, conf GeneratorConfig) error {
	sourceData, err := resource.NewSourceData(srcFS, conf.SourcePath)
	if err != nil {
		return fmt.Errorf("can't load Go source files: %v", err)
	}

	versionKindAssignments, err := resource.VersionKindAssignments(sourceData.PossibleFuncDecls, conf.FieldAssignmentMethodName)
	if err != nil {
		return err
	}

	// Extract data from a declaration to transform it into a reference
	// entry later
	errs := GenerationError{messages: []error{}}
	for k, decl := range sourceData.TypeDecls {
		if !shouldProcess(decl, conf.RequiredFieldTypes, conf.ExcludedResourceTypes) {
			continue
		}

		pc := pageContent{}

		// decl is a dynamic resource type, so get data for the type and
		// its dependencies.
		entries, err := resource.ReferenceDataFromDeclaration(decl, sourceData.TypeDecls)
		if errors.Is(err, resource.NotAGenDeclError{}) {
			continue
		}
		if err != nil {
			errs.messages = append(errs.messages, fmt.Errorf("issue creating a reference entry for declaration %v.%v in file %v: %v", k.PackageName, k.DeclName, decl.FilePath, err))
		}

		pc.Resource.ReferenceEntry = entries[k]
		delete(entries, k)
		pc.Fields = entries

		vk, ok := versionKindAssignments[k]
		var verName, kindName string
		if ok {
			// So far, all values of "Kind" and "Version"
			// are declared in the same package as the types
			// that include these fields.
			verName = sourceData.StringAssignments[resource.PackageInfo{
				DeclName:    vk.Version,
				PackageName: k.PackageName,
			}]

			kindName = sourceData.StringAssignments[resource.PackageInfo{
				DeclName:    vk.Kind,
				PackageName: k.PackageName,
			}]
		}
		pc.Resource.Kind = kindName
		pc.Resource.Version = verName

		filename := strings.ReplaceAll(strings.ToLower(pc.Resource.SectionName), " ", "-")
		doc, err := destFS.Create(path.Join(conf.DestinationDirectory, filename+".mdx"))
		if err != nil {
			errs.messages = append(errs.messages, err)
		}

		if err := template.Must(template.New("Main reference").Parse(referenceTemplate)).Execute(doc, pc); err != nil {
			errs.messages = append(errs.messages, err)
		}
	}
	if len(errs.messages) > 0 {
		return errs
	}

	return nil
}

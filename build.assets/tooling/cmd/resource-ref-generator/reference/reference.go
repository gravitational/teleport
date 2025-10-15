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

package reference

import (
	_ "embed"
	"errors"
	"fmt"
	"go/ast"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/build.assets/tooling/cmd/resource-ref-generator/resource"
)

var tmpl *template.Template

func init() {
	tmpl = template.Must(template.New("Main reference").Parse(referenceTemplate))
}

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
//
//go:embed reference.tmpl
var referenceTemplate string

// TypeInfo represents the name and package name of an exported Go type. It
// makes no guarantees about whether the type was actually declared within the
// package.
type TypeInfo struct {
	// Go package path (not a file path)
	Package string `yaml:"package"`
	// Name of the type, e.g., Metadata
	Name string `yaml:"name"`
}

// ResourceConfig describes a resource type to include in the reference.
type ResourceConfig struct {
	// The name of the struct type as declared in the Go source, e.g.,
	// RoleV6.
	TypeName string
	// The name of the resource to include in the docs, e.g., Role v6.
	FriendlyName string
	// The value of the "kind" field within a YAML manifest for this
	// resource, e.g., "role".
	KindValue string
	// The value of the "version" field within a YAML manifest for this
	// resource, e.g., "v6".
	VersionValue string
}

// GeneratorConfig is the user-facing configuration for the resource reference
// generator.
type GeneratorConfig struct {
	Resources []ResourceConfig
	// Path to the root of the Go project directory.
	SourcePath string `yaml:"source"`
	// Directory where the generator writes reference pages.
	DestinationDirectory string `yaml:"destination"`
}

// UnmarshalYAML checks that the GeneratorConfig includes all required fields and, if
// not, returns the first error it encounters.
func (c GeneratorConfig) UnmarshalYAML(value *yaml.Node) error {
	if err := value.Decode(&c); err != nil {
		return fmt.Errorf("parsing the configuration file as YAML: %w", err)
	}

	switch {
	case c.DestinationDirectory == "":
		return errors.New("no destination path provided")
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
// type). To do so, it checks whether d has one of the resource types specified
// in resourceTypeNames.
func shouldProcess(d resource.DeclarationInfo, resourceTypeNames map[string]struct{}) bool {
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
	_, ok = t.Type.(*ast.StructType)
	if !ok {
		return false
	}

	// We have not configured the generator to use this resource type.
	if _, ok = resourceTypeNames[t.Name.Name]; !ok {
		return false
	}

	return true
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
func Generate(conf GeneratorConfig) error {
	sourceData, err := resource.NewSourceData(conf.SourcePath)
	if err != nil {
		return fmt.Errorf("loading Go source files: %w", err)
	}

	allowedResourceTypes := make(map[string]struct{})
	for _, r := range conf.Resources {
		allowedResourceTypes[r.TypeName] = struct{}{}
	}

	// Extract data from a declaration to transform it into a reference
	// entry later
	errs := GenerationError{messages: []error{}}
	for k, decl := range sourceData.TypeDecls {
		if !shouldProcess(decl, allowedResourceTypes) {
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
			errs.messages = append(errs.messages, fmt.Errorf("creating a reference entry for declaration %v.%v in file %v: %w", k.PackageName, k.DeclName, decl.FilePath, err))
		}

		pc.Resource.ReferenceEntry = entries[k]
		delete(entries, k)
		pc.Fields = entries

		filename := strings.ReplaceAll(strings.ToLower(pc.Resource.SectionName), " ", "-")
		docpath := filepath.Join(conf.DestinationDirectory, filename+".mdx")
		doc, err := os.Create(docpath)
		if err != nil {
			errs.messages = append(errs.messages, fmt.Errorf("cannot create page at %v: %w", docpath, err))
			continue
		}

		if err := tmpl.Execute(doc, pc); err != nil {
			errs.messages = append(errs.messages, fmt.Errorf("cannot populate the resource reference template: %w", err))
		}
	}
	if len(errs.messages) > 0 {
		return errs
	}

	return nil
}

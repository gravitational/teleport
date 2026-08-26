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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/gravitational/teleport/build.assets/tooling/cmd/resource-ref-generator/resource"
)

// pageContent represents a reference page for a single resource and its related
// fields. Fields must be exported so we can use them in templates.
type pageContent struct {
	// Introduction is optional text to override the default introduction, taken
	// from Resource.
	Introduction string
	// Resource represents the dynamic Teleport resource that is the subject
	// of the page.
	Resource resourceSection
	// Fields are the top-level fields of the dynamic resource for this
	// page.
	Fields map[resource.PackageInfo]resource.ReferenceEntry
}

// resourceSection represents a top-level section of the resource reference
// dedicated to a dynamic resource.
type resourceSection struct {
	Version         string
	Kind            string
	ResourceExample string
	resource.ReferenceEntry
}

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
	TypeName string `yaml:"type"`
	// The full path of the Go package containing this type declaration,
	// e.g.,"github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1".
	PackagePath string `yaml:"package"`
	// The value of the "kind" field within a YAML manifest for this
	// resource, e.g., "role".
	KindValue string `yaml:"yaml_kind"`
	// The value of the "version" field within a YAML manifest for this
	// resource, e.g., "v6".
	VersionValue string `yaml:"yaml_version"`
	// Introduction paragraph(s) to add to the template in place of the
	// default, which is the GoDoc for the resource type.
	Introduction string `yaml:"introduction"`
}

// GeneratorConfig is the user-facing configuration for the resource reference
// generator.
type GeneratorConfig struct {
	Resources []ResourceConfig `yaml:"resources"`
	// Path to the root of the Go project directory.
	SourcePath string `yaml:"source"`
	// Directory where the generator writes reference pages.
	DestinationDirectory string   `yaml:"destination"`
	CamelCaseExceptions  []string `yaml:"camel_case_exceptions"`
	// Directory where example YAML files are located.
	ExamplesDirectory string `yaml:"examples_directory"`
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
// reference to fs. Uses prefix, e.g., github.com/gravitational/teleport, to
// construct package paths.
func Generate(prefix string, conf GeneratorConfig, tmpl *template.Template) error {
	sourceData, err := resource.NewSourceData(prefix, conf.SourcePath)
	if err != nil {
		return fmt.Errorf("loading Go source files: %w", err)
	}

	var errs GenerationError
	for _, r := range conf.Resources {
		k := resource.PackageInfo{
			DeclName:    r.TypeName,
			PackagePath: r.PackagePath,
		}

		decl, ok := sourceData.TypeDecls[k]
		if !ok {
			errs.messages = append(errs.messages, fmt.Errorf("creating a reference entry for declaration %v.%v: cannot find a declaration of this resource type", k.PackagePath, k.DeclName))
			continue
		}

		pc := pageContent{}
		pc.Introduction = r.Introduction

		// decl is a dynamic resource type, so get data for the type and
		// its dependencies.
		entries, err := resource.ReferenceDataFromDeclaration(prefix, decl, sourceData.TypeDecls, conf.CamelCaseExceptions)
		if errors.As(err, &resource.NotAGenDeclError{}) {
			continue
		}
		if err != nil {
			errs.messages = append(errs.messages, fmt.Errorf("creating a reference entry for declaration %v.%v in file %v: %w", k.PackagePath, k.DeclName, decl.FilePath, err))
		}

		pc.Resource.ReferenceEntry = entries[k]
		delete(entries, k)
		pc.Fields = entries

		pc.Resource.Kind = r.KindValue
		pc.Resource.Version = r.VersionValue

		resourceExampleLocation := filepath.Join(conf.ExamplesDirectory, r.KindValue+".yaml")
		if exampleBytes, err := os.ReadFile(resourceExampleLocation); err == nil {
			pc.Resource.ResourceExample = string(exampleBytes)
		}

		filename := strings.ReplaceAll(strings.ToLower(pc.Resource.SectionName), " ", "-")
		docpath := filepath.Join(conf.DestinationDirectory, filename+".mdx")
		doc, err := os.Create(docpath)
		if err != nil {
			errs.messages = append(errs.messages, fmt.Errorf("cannot create page at %v: %w", docpath, err))
			continue
		}
		defer doc.Close()

		if err := tmpl.Execute(doc, pc); err != nil {
			errs.messages = append(errs.messages, fmt.Errorf("cannot populate the resource reference template: %w", err))
		}
	}
	if len(errs.messages) > 0 {
		return errs
	}

	return nil
}

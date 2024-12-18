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
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"testing"

	"github.com/gravitational/teleport/build.assets/tooling/cmd/resource-ref-generator/resource"
	"github.com/spf13/afero"

	"github.com/stretchr/testify/assert"
)

func TestShouldProcess(t *testing.T) {
	src := `package testpkg
type MyStruct struct{
  Metadata       types.Metadata
  AlsoMetadata   Metadata
  Name           otherpkg.TypeName
}
`
	// Parse the fixture as an AST node so we can use it in shouldProcess.
	fset := token.NewFileSet()
	d, err := parser.ParseFile(fset,
		"myfile.go",
		src,
		parser.ParseComments,
	)
	if err != nil {
		t.Fatalf("test fixture contains invalid Go source: %v\n", err)
	}

	if len(d.Decls) != 1 {
		t.Fatal("the source fixture must contain a single *ast.GenDec (this is a problem with the test)")

	}

	l, ok := d.Decls[0].(*ast.GenDecl)
	if !ok {
		t.Fatal("the source fixture must contain a single *ast.GenDec (this is a problem with the test)")

	}

	cases := []struct {
		description       string
		requiredFields    []TypeInfo
		excludedResources []TypeInfo
		expected          bool
	}{
		{
			description: "one required type from a separate package",
			requiredFields: []TypeInfo{
				{
					Package: "types",
					Name:    "Metadata",
				},
			},
			expected: true,
		},
		{
			description: "two required types from separate packages",
			requiredFields: []TypeInfo{
				{
					Package: "types",
					Name:    "Metadata",
				},
				{
					Package: "otherpkg",
					Name:    "TypeName",
				},
			},
			expected: true,
		},
		{
			description: "field from another package is not present",
			requiredFields: []TypeInfo{
				{
					Package: "types",
					Name:    "AbsentType",
				},
			},
			expected: false,
		},
		{
			description: "field from the same package",
			requiredFields: []TypeInfo{
				{
					Package: "testpkg",
					Name:    "Metadata",
				},
			},
			expected: true,
		},
		{
			description: "one required type with long path from a separate package",
			requiredFields: []TypeInfo{
				{
					Package: "long/package/path/types",
					Name:    "Metadata",
				},
			},
			expected: true,
		},
		{
			description: "one required type with long path from the current  package",
			requiredFields: []TypeInfo{
				{
					Package: "long/package/path/testpkg",
					Name:    "Metadata",
				},
			},
			expected: true,
		},
		{
			description: "excluded type",
			requiredFields: []TypeInfo{
				{
					Package: "testpkg",
					Name:    "Metadata",
				},
			},
			excludedResources: []TypeInfo{
				{
					Package: "testpkg",
					Name:    "MyStruct",
				},
			},
			expected: false,
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			assert.Equal(t, c.expected, shouldProcess(resource.DeclarationInfo{
				FilePath:    "myfile.go",
				Decl:        l,
				PackageName: "testpkg",
			}, c.requiredFields, c.excludedResources))
		})
	}
}

// This test reads the golden files at the destination directory and compares
// the generated resource reference docs with them. To regenerate the golden
// files, delete the destination directory (reference/testdata) and run the test
// again.
func TestGenerate(t *testing.T) {
	config := GeneratorConfig{
		RequiredFieldTypes: []TypeInfo{
			{
				Name:    "ResourceHeader",
				Package: "typestest",
			},
			{
				Name:    "Metadata",
				Package: "typestest",
			},
		},
		SourcePath:           "testdata/src",
		DestinationDirectory: "testdata/dest",
		ExcludedResourceTypes: []TypeInfo{
			{
				Name:    "ResourceHeader",
				Package: "typestest",
			},
		},
		FieldAssignmentMethodName: "setStaticFields",
	}

	dir, err := os.ReadDir(config.DestinationDirectory)

	// Recreate the golden file directory if it is missing.
	if errors.Is(err, os.ErrNotExist) {
		if err := os.Mkdir(config.DestinationDirectory, 0777); err != nil {
			t.Fatal(err)
		}
		if err := Generate(afero.NewOsFs(), config); err != nil {
			t.Fatal(err)
		}
	}
	if err != nil {
		t.Fatal(err)
	}

	actual := afero.NewMemMapFs()
	if err := Generate(actual, config); err != nil {
		t.Fatal(err)
	}

	// TODO: create a map with all files in the in-memory directory
	// TODO: range through the files  in the map and check each one against the one in
	// the real directory
	// TODO: if there are any leftover files, fail
}

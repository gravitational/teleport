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
	"bytes"
	"gen-resource-ref/resource"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
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

// This test reads the file at the destination path and compares the
// generated resource reference with it. The test does not regenerate
// the file at the destination path. To do so, navigate to the
// "docs-generators/resource-reference/reference" directory and run the
// following command:
//
// go run gen-resource-ref -config=testdata/config.yaml
func TestGenerate(t *testing.T) {
	cf, err := os.Open(path.Join("testdata", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	config := GeneratorConfig{}
	if err := yaml.NewDecoder(cf).Decode(&config); err != nil {
		t.Fatal(err)
	}

	golden, err := os.Open(path.Join(path.Split(config.DestinationPath)))
	if err != nil {
		t.Fatal(err)
	}

	var expected bytes.Buffer
	_, err = io.Copy(&expected, golden)
	if err != nil {
		t.Fatal(err)
	}

	var actual bytes.Buffer
	assert.NoError(t, Generate(&actual, config))
	assert.Equal(t, expected.String(), actual.String())
}

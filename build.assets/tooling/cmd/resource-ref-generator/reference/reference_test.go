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
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gravitational/teleport/build.assets/tooling/cmd/resource-ref-generator/resource"
	"github.com/spf13/afero"

	"github.com/stretchr/testify/assert"
)

func TestShouldProcess(t *testing.T) {
	cases := []struct {
		description       string
		src               string
		requiredFields    []TypeInfo
		excludedResources []TypeInfo
		expected          bool
	}{
		{
			description: "one required type from a separate package",
			src: `package testpkg
type MyStruct struct{
  Metadata       types.Metadata
  AlsoMetadata   Metadata
  Name           otherpkg.TypeName
}
`,
			requiredFields: []TypeInfo{
				{
					Package: "types",
					Name:    "Metadata",
				},
			},
			expected: true,
		},
		{
			description: "one required type with pointer field",
			src: `package testpkg
type MyStruct struct{
  Metadata       *types.Metadata
  AlsoMetadata   Metadata
  Name           otherpkg.TypeName
}
`,
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
			src: `package testpkg
type MyStruct struct{
  Metadata       types.Metadata
  AlsoMetadata   Metadata
  Name           otherpkg.TypeName
}
`,
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
			src: `package testpkg
type MyStruct struct{
  Metadata       types.Metadata
  AlsoMetadata   Metadata
  Name           otherpkg.TypeName
}
`,
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
			src: `package testpkg
type MyStruct struct{
  Metadata       types.Metadata
  AlsoMetadata   Metadata
  Name           otherpkg.TypeName
}
`,
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
			src: `package testpkg
type MyStruct struct{
  Metadata       types.Metadata
  AlsoMetadata   Metadata
  Name           otherpkg.TypeName
}
`,
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
			src: `package testpkg
type MyStruct struct{
  Metadata       types.Metadata
  AlsoMetadata   Metadata
  Name           otherpkg.TypeName
}
`,
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
			src: `package testpkg
type MyStruct struct{
  Metadata       types.Metadata
  AlsoMetadata   Metadata
  Name           otherpkg.TypeName
}
`,
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
		// Parse the fixture as an AST node so we can use it in shouldProcess.
		fset := token.NewFileSet()
		d, err := parser.ParseFile(fset,
			"myfile.go",
			c.src,
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
// files, delete the destination directory (reference/testdata/dest) and run the
// test again.
func TestGenerate(t *testing.T) {
	// Define paths based on relative paths from this Go source file to
	// guarantee consistent results regardless of where a user runs "go test".
	_, callerPath, _, _ := runtime.Caller(0)
	fmt.Println(callerPath)
	tdPath, err := filepath.Rel(filepath.Dir(callerPath), filepath.Join(filepath.Dir(callerPath), "testdata"))
	if err != nil {
		t.Fatal(err)
	}
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
			{
				Name:    "Metadata",
				Package: "v1",
			},
		},
		SourcePath: path.Join(
			tdPath, "src",
		),
		DestinationDirectory: path.Join(
			tdPath, "dest",
		),
		ExcludedResourceTypes: []TypeInfo{
			{
				Name:    "ResourceHeader",
				Package: "typestest",
			},
		},
		FieldAssignmentMethodName: "setStaticFields",
	}

	dirExpected, err := os.ReadDir(config.DestinationDirectory)
	osFS := afero.NewOsFs()

	switch {
	// Recreate the golden file directory if it is missing.
	case errors.Is(err, os.ErrNotExist):
		if err := os.Mkdir(config.DestinationDirectory, 0777); err != nil {
			t.Fatal(err)
		}
		if err := Generate(osFS, osFS, config); err != nil {
			t.Fatal(err)
		}
		return
	case err != nil:
		t.Fatal(err)
	}

	memfs := afero.NewMemMapFs()
	if err := Generate(osFS, memfs, config); err != nil {
		t.Fatal(err)
	}

	if err := memfs.MkdirAll(config.DestinationDirectory, 0777); err != nil {
		t.Fatal(err)
	}

	fileActual, err := memfs.Open(config.DestinationDirectory)
	if err != nil {
		t.Fatal(err)
	}

	dirActual, err := fileActual.Readdir(-1)
	if err != nil {
		t.Fatal(err)
	}

	expectedFiles := make(map[string]struct{})
	for _, f := range dirExpected {
		expectedFiles[path.Join(config.DestinationDirectory, f.Name())] = struct{}{}
	}

	actualFiles := make(map[string]struct{})
	for _, f := range dirActual {
		actualFiles[path.Join(config.DestinationDirectory, f.Name())] = struct{}{}
	}

	for f := range actualFiles {
		if _, ok := expectedFiles[f]; !ok {
			t.Fatalf(
				"file %v created after running the generator but is not in %v",
				f,
				config.DestinationDirectory,
			)
		}
	}
	for f := range expectedFiles {
		if _, ok := actualFiles[f]; !ok {
			t.Fatalf(
				"file %v in %v was not created after running the generator",
				f,
				config.DestinationDirectory,
			)
		}
	}

	// Actual file names and expected file names match, so we can compare
	// each file.
	for f := range actualFiles {
		actual, err := memfs.Open(f)
		if err != nil {
			t.Fatal(err)
		}
		actualContent, err := io.ReadAll(actual)
		if err != nil {
			t.Fatal(err)
		}

		expected, err := os.Open(f)
		if err != nil {
			t.Fatal(err)
		}

		expectedContent, err := io.ReadAll(expected)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, string(expectedContent), string(actualContent))
	}
}

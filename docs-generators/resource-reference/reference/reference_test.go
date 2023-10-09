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
	cases := []struct {
		description string
		input       []TypeInfo
		expected    bool
	}{
		{
			description: "one required type from a separate package",
			input: []TypeInfo{
				{
					Package: "types",
					Name:    "Metadata",
				},
			},
			expected: true,
		},
		{
			description: "two required types from separate packages",
			input: []TypeInfo{
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
			input: []TypeInfo{
				{
					Package: "types",
					Name:    "AbsentType",
				},
			},
			expected: false,
		},
		{
			description: "field from the same package",
			input: []TypeInfo{
				{
					Package: "testpkg",
					Name:    "Metadata",
				},
			},
			expected: true,
		},
		{
			description: "one required type with long path from a separate package",
			input: []TypeInfo{
				{
					Package: "long/package/path/types",
					Name:    "Metadata",
				},
			},
			expected: true,
		},
		{
			description: "one required type with long path from the current  package",
			input: []TypeInfo{
				{
					Package: "long/package/path/testpkg",
					Name:    "Metadata",
				},
			},
			expected: true,
		},
	}

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

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			assert.Equal(t, c.expected, shouldProcess(resource.DeclarationInfo{
				FilePath:    "myfile.go",
				Decl:        l,
				PackageName: "testpkg",
			}, c.input))
		})
	}
}

func TestGenerate(t *testing.T) {
	// This test reads the file at the destination path and compares the
	// generated resource reference with it. The test does not regenerate
	// the file at the destination path. To do so, navigate to the
	// "docs-generators/resource-reference/reference" directory and run the
	// following command:
	//
	// go run gen-resource-ref -config=reference/testdata/conf.yaml
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

package reference

import (
	"bytes"
	"gen-resource-ref/resource"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

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
	// TODO: Read a golden file instead to get the expected value
	var expected string
	conf := GeneratorConfig{
		RequiredTypes: []TypeInfo{
			{
				Package: "typestest",
				Name:    "ResourceHeader",
			},
			{
				Package: "typestest",
				Name:    "Metadata",
			},
		},
		SourcePath: "testdata",
		// No-op in this case
		DestinationPath: "",
	}

	var buf bytes.Buffer
	assert.NoError(t, Generate(&buf, conf))
	assert.Equal(t, expected, buf.String())
}

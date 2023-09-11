package resource

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// replaceBackticks replaces the "BACKTICK" placeholder text with backticks so
// we can include struct tags within source fixtures.
func replaceBackticks(source string) string {
	return strings.ReplaceAll(source, "BACKTICK", "`")
}

func TestGenerate(t *testing.T) {
	cases := []struct {
		description string
		// Source fixture. Replace backticks with the "BACKTICK"
		// placeholder.
		source   string
		expected Resource
	}{
		// TODO: Add other scalar fields: number, boolean
		{
			description: "Only string fields, one level deep",
			source: `
package mypkg

// Metadata describes information about a dynamic resource. Every dynamic
// resource in Teleport has a metadata object.
type Metadata struct {
    // Name is the name of the resource.
    Name string BACKTICKprotobuf:"bytes,1,opt,name=Name,proto3" json:"name"BACKTICK
    // Namespace is the resource's namespace
    Namespace string BACKTICKprotobuf:"bytes,2,opt,name=Namespace,proto3" json:"-"BACKTICK
    // Description is the resource's description.
    Description string BACKTICKprotobuf:"bytes,3,opt,name=Description,proto3" json:"description,omitempty"BACKTICK
}
`,
			expected: Resource{
				SectionName: "Metadata",
				Description: "Metadata describes information about a dynamic resource. Every dynamic resource in Teleport has a metadata object.",
				SourcePath:  "myfile.go",
				YAMLExample: `name: "string"
namespace: "string"
description: "string"`,
				Fields: []Field{
					Field{
						Name:        "name",
						Description: "The name of the resource.",
						Type:        "string",
					},
					Field{
						Name:        "namespace",
						Description: "The resource's namespace.",
						Type:        "string",
					},
					Field{
						Name:        "description",
						Description: "The resource's description.",
						Type:        "string",
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, "myfile.go", replaceBackticks(tc.source), parser.ParseComments)
			if err != nil {
				t.Fatalf("test fixture contains invalid Go source: %v\n", err)
			}

			if len(f.Decls) != 1 {
				t.Fatalf("test fixture contains an unexpected number of declarations. want 1. got: %v", len(f.Decls))
			}

			gd, ok := f.Decls[0].(*ast.GenDecl)
			if !ok {
				t.Fatalf("test fixture declaration is not a GenDecl")
			}

			r, err := NewFromDecl(gd, "myfile.go")
			assert.NoError(t, err)

			assert.Equal(t, tc.expected, r)
		})
	}
}

func TestGetJSONTag(t *testing.T) {
	cases := []struct {
		description string
		input       string
		expected    string
	}{
		{
			description: "one well-formed struct tag",
			input:       `json:"my_tag"`,
			expected:    "my_tag",
		},
		{
			description: "multiple well-formed struct tags",
			input:       `json:"json_tag" yaml:"yaml_tag" other:"other-tag"`,
			expected:    "json_tag",
		},
		{
			description: "empty JSON tag",
			input:       `json:"" yaml:"yaml_tag" other:"other-tag"`,
			expected:    "",
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			g := getJSONTag(c.input)
			assert.Equal(t, c.expected, g)
		})
	}
}

func TestDescriptionWithoutName(t *testing.T) {
	cases := []struct {
		description string
		input       string
		name        string
		expected    string
	}{
		{
			description: "short description",
			input:       "A",
			name:        "MyDecl",
			expected:    "A",
		},
		{
			description: "no description",
			input:       "",
			name:        "MyDecl",
			expected:    "",
		},
		{
			description: "GoDoc consists only of declaration name",
			input:       "MyDecl",
			name:        "MyDecl",
			expected:    "",
		},
		{
			description: "description containing name",
			input:       "MyDecl is a declaration that we will describe in the docs.",
			name:        "MyDecl",
			expected:    "A declaration that we will describe in the docs.",
		},
		{
			description: "description containing name and \"are\"",
			input:       "MyDecls are things that we will describe in the docs.",
			name:        "MyDecls",
			expected:    "Things that we will describe in the docs.",
		},

		{
			description: "description with no name",
			input:       "Declaration that we will describe in the docs.",
			name:        "MyDecl",
			expected:    "Declaration that we will describe in the docs.",
		},
		{
			description: "description beginning with name and non-is verb",
			input:       "MyDecl performs an action.",
			name:        "MyDecl",
			expected:    "Performs an action.",
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			assert.Equal(t, c.expected, descriptionWithoutName(c.input, c.name))
		})
	}
}

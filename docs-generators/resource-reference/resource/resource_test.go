package resource

import (
	"fmt"
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

func TestNewFromDecl(t *testing.T) {
	cases := []struct {
		description string
		// Go source fixture. Replace backticks with the "BACKTICK"
		// placeholder.
		source   string
		expected []ReferenceEntry
		// Go source fixtures that the test uses for named type fields.
		declSources []string
	}{
		{
			description: "scalar fields with one field ignored",
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
    // Age is the resource's age in seconds.
    Age uint BACKTICKjson:"age"BACKTICK
    // Active indicates whether the resource is currently in use.
    Active bool BACKTICKjson:"active"BACKTICK
}
`,
			expected: []ReferenceEntry{{
				SectionName: "Metadata",
				Description: "Describes information about a dynamic resource. Every dynamic resource in Teleport has a metadata object.",
				SourcePath:  "myfile.go",
				YAMLExample: `name: "string"
description: "string"
age: 1
active: true
`,
				Fields: []Field{
					Field{
						Name:        "name",
						Description: "The name of the resource.",
						Type:        "string",
					},
					Field{
						Name:        "description",
						Description: "The resource's description.",
						Type:        "string",
					},
					Field{
						Name:        "age",
						Description: "The resource's age in seconds.",
						Type:        "number",
					},
					Field{
						Name:        "active",
						Description: "Indicates whether the resource is currently in use.",
						Type:        "Boolean",
					},
				},
			},
			}},
		{
			description: "sequences of scalars",
			source: `
package mypkg

// Metadata describes information about a dynamic resource. Every dynamic
// resource in Teleport has a metadata object.
type Metadata struct {
    // Names is a list of names.
    Names []string BACKTICKjson:"names"BACKTICK
    // Numbers is a list of numbers.
    Numbers []int BACKTICKjson:"numbers"BACKTICK
    // Booleans is a list of Booleans.
    Booleans []bool BACKTICKjson:"booleans"BACKTICK
}
`,
			expected: []ReferenceEntry{{
				SectionName: "Metadata",
				Description: "Describes information about a dynamic resource. Every dynamic resource in Teleport has a metadata object.",
				SourcePath:  "myfile.go",
				YAMLExample: `names: 
- "string"
- "string"
- "string"
numbers: 
- 1
- 1
- 1
booleans: 
- true
- true
- true
`,
				Fields: []Field{
					Field{
						Name:        "names",
						Description: "A list of names.",
						Type:        "[]string",
					},
					Field{
						Name:        "numbers",
						Description: "A list of numbers.",
						Type:        "[]number",
					},
					Field{
						Name:        "booleans",
						Description: "A list of Booleans.",
						Type:        "[]Boolean",
					},
				},
			},
			}},
		{
			description: "a map of strings to sequences",
			source: `
package mypkg

// Metadata describes information about a dynamic resource. Every dynamic
// resource in Teleport has a metadata object.
type Metadata struct {
  // Attributes indicates additional data for the resource.
  Attributes map[string][]string BACKTICKjson:"attributes"BACKTICK
}
`,
			expected: []ReferenceEntry{{
				SectionName: "Metadata",
				Description: "Describes information about a dynamic resource. Every dynamic resource in Teleport has a metadata object.",
				SourcePath:  "myfile.go",
				YAMLExample: `attributes: 
  "string": 
    - "string"
    - "string"
    - "string"
  "string": 
    - "string"
    - "string"
    - "string"
  "string": 
    - "string"
    - "string"
    - "string"
`,
				Fields: []Field{
					Field{
						Name:        "attributes",
						Description: "Indicates additional data for the resource.",
						Type:        "map[string][]string",
					},
				},
			},
			}},
		{
			description: "a custom type field with no override",
			source: `
package mypkg

// Server includes information about a server registered with Teleport.
type Server struct {
    // Name is the name of the resource.
    Name string BACKTICKprotobuf:"bytes,1,opt,name=Name,proto3" json:"name"BACKTICK
    // Spec contains information about the server.
    Spec types.ServerSpecV1 BACKTICKjson:"spec"BACKTICK
}
`,
			expected: []ReferenceEntry{{
				SectionName: "Server",
				Description: "Includes information about a server registered with Teleport.",
				SourcePath:  "myfile.go",
				YAMLExample: `name: "string"
spec: 
# [...]
`,
				Fields: []Field{
					Field{
						Name:        "name",
						Description: "The name of the resource.",
						Type:        "string",
					},
					Field{
						Name:        "spec",
						Description: "Contains information about the server.",
						Type:        "[Server Spec v1](#server-spec-v1)"},
				},
			}},
		},
		{
			description: "example YAML block",
			source: `
package mypkg

// Server includes information about a server registered with Teleport.
// Example YAML:
// ---
// qualities:
//    - "region:us-east-1"
//    - team:security
//      env:dev
//      role:primary
type Server struct {
  // Qualities is a list of either maps or "key:value" strings.
  Qualities types.CustomAttributes BACKTICKjson:"qualities"BACKTICK
}
`,

			expected: []ReferenceEntry{
				{
					SectionName: "Server",
					Description: "Includes information about a server registered with Teleport.",
					SourcePath:  "myfile.go",
					YAMLExample: `qualities:
   - "region:us-east-1"
   - team:security
     env:dev
     role:primary
`,
					Fields: []Field{
						Field{
							Name:        "qualities",
							Description: "A list of either maps or \"key:value\" strings.",
							Type:        "[Custom Attributes](#custom-attributes)",
						},
					},
				},
			},
		},
		{
			description: "a custom type field with no override and a second source file",
			source: `
package mypkg

// Server includes information about a server registered with Teleport.
type Server struct {
    // Name is the name of the resource.
    Name string BACKTICKprotobuf:"bytes,1,opt,name=Name,proto3" json:"name"BACKTICK
    // Spec contains information about the server.
    Spec types.ServerSpecV1 BACKTICKjson:"spec"BACKTICK
}
`,
			declSources: []string{`package types
// ServerSpecV1 includes aspects of a proxied server.
type ServerSpecV1 struct {
    // The address of the server.
    Address string BACKTICKjson:"address"BACKTICK
    // How long the resource is valid.
    TTL int BACKTICKjson:"ttl"BACKTICK
    // Whether the server is active.
    IsActive bool BACKTICKjson:"is_active"BACKTICK
}
`,
			},
			expected: []ReferenceEntry{
				{
					SectionName: "Server",
					Description: "Includes information about a server registered with Teleport.",
					SourcePath:  "myfile.go",
					YAMLExample: `name: "string"
spec: 
# [...]
`,
					Fields: []Field{
						Field{
							Name:        "name",
							Description: "The name of the resource.",
							Type:        "string",
						},
						Field{
							Name:        "spec",
							Description: "Contains information about the server.",
							Type:        "[Server Spec v1](#server-spec-v1)"},
					},
				},
				{
					SectionName: "Server Spec v1",
					Description: "Includes aspects of a proxied server.",
					SourcePath:  "myfile0.go",
					YAMLExample: `address: "string"
ttl: 1
is_active: true
`,
					Fields: []Field{
						Field{
							Name:        "address",
							Description: "The address of the server.",
							Type:        "string",
						},
						Field{
							Name:        "ttl",
							Description: "How long the resource is valid.",
							Type:        "number",
						},
						Field{
							Name:        "is_active",
							Description: "Whether the server is active.",
							Type:        "Boolean",
						},
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

			allDecls := make(map[PackageInfo]*ast.GenDecl)
			// Assemble map of PackageInfo to *ast.GenDecl for
			// source fixtures the test case depends on.
			// TODO: This is getting lengthy, so consider extracting
			// it into separate function--possibly to use in the
			// program itself--or changing the function signature of
			// NewFromDecl to make things less awkward.
			for n, dep := range tc.declSources {
				d, err := parser.ParseFile(fset,
					fmt.Sprintf("myfile%v.go", n),
					replaceBackticks(dep),
					parser.ParseComments,
				)
				if err != nil {
					t.Fatalf("test fixture contains invalid Go source: %v\n", err)
				}

				// Store type declarations in the map.
				for _, def := range d.Decls {
					l, ok := def.(*ast.GenDecl)
					if !ok {
						continue
					}
					if len(l.Specs) != 1 {
						continue
					}
					spec, ok := l.Specs[0].(*ast.TypeSpec)
					if !ok {
						continue
					}

					allDecls[PackageInfo{
						TypeName:    spec.Name.Name,
						PackageName: d.Name.Name,
					}] = l

				}

			}

			if len(f.Decls) != 1 {
				t.Fatalf("test fixture contains an unexpected number of declarations. want 1. got: %v", len(f.Decls))
			}

			gd, ok := f.Decls[0].(*ast.GenDecl)
			if !ok {
				t.Fatalf("test fixture declaration is not a GenDecl")
			}

			r, err := NewFromDecl(gd, "myfile.go", allDecls)
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
			description: "omitempty option in tag value",
			input:       `json:"json_tag,omitempty" yaml:"yaml_tag" other:"other-tag"`,
			expected:    "json_tag",
		},
		{
			description: "No JSON tag",
			input:       `other:"other-tag"`,
			expected:    "",
		},
		{
			description: "Empty JSON tag with the omitempty option",
			input:       `json:",omitempty" other:"other-tag"`,
			expected:    "",
		},
		{
			description: "Ignored JSON field",
			input:       `json:"-" other:"other-tag"`,
			expected:    "-",
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

func TestMakeYAMLExample(t *testing.T) {
	cases := []struct {
		description string
		input       []rawField
		expected    string
	}{
		{
			description: "all scalars",
			input: []rawField{
				rawField{
					doc:  "myInt is an int",
					kind: yamlNumber{},
					name: "myInt",
					tags: `json:"my_int"`,
				},
				rawField{
					doc:  "myBool is a Boolean",
					kind: yamlBool{},
					name: "myBool",
					tags: `json:"my_bool"`,
				},
				rawField{
					doc:  "myString is a string",
					kind: yamlString{},
					tags: `json:"my_string"`,
				},
			},
			expected: `my_int: 1
my_bool: true
my_string: "string"
`,
		},
		{
			description: "sequence of sequence of strings",
			input: []rawField{
				rawField{
					name:     "mySeq",
					jsonName: "my_seq",
					doc:      "mySeq is a sequence of sequences of strings",
					tags:     `json:"my_seq"`,
					kind: yamlSequence{
						elementKind: yamlSequence{
							elementKind: yamlString{},
						},
					},
				},
			},
			expected: `my_seq: 
- 
  - "string"
  - "string"
  - "string"
- 
  - "string"
  - "string"
  - "string"
- 
  - "string"
  - "string"
  - "string"
`,
		},
		{
			description: "maps of numbers to strings",
			input: []rawField{
				rawField{
					name:     "myMap",
					jsonName: "my_map",
					doc:      "myMap is a map of ints to strings",
					tags:     `json:"my_map"`,
					kind: yamlMapping{
						keyKind:   yamlNumber{},
						valueKind: yamlString{},
					},
				},
			},
			expected: `my_map: 
  1: "string"
  1: "string"
  1: "string"
`,
		},
		{
			description: "sequence of maps of strings to Booleans",
			input: []rawField{
				rawField{
					name:     "mySeq",
					jsonName: "my_seq",
					doc:      "mySeq is a complex type",
					tags:     `json:"my_seq"`,
					kind: yamlSequence{
						elementKind: yamlMapping{
							keyKind:   yamlString{},
							valueKind: yamlBool{},
						},
					},
				},
			},
			expected: `my_seq: 
- 
    "string": true
    "string": true
    "string": true
- 
    "string": true
    "string": true
    "string": true
- 
    "string": true
    "string": true
    "string": true
`,
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			e, err := makeYAMLExample(c.input)
			assert.NoError(t, err)
			assert.Equal(t, c.expected, e)
		})
	}
}

func TestMakeSectionName(t *testing.T) {
	cases := []struct {
		description string
		original    string
		expected    string
	}{
		{
			description: "camel-case name",
			original:    "ServerSpec",
			expected:    "Server Spec",
		},
		{
			description: "camel-case name with three words",
			original:    "MyExcellentResource",
			expected:    "My Excellent Resource",
		},
		{
			description: "camel-case name with version",
			original:    "ServerSpecV2",
			expected:    "Server Spec v2",
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			assert.Equal(t, c.expected, makeSectionName(c.original))
		})
	}
}

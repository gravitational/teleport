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

package resource

import (
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

// replaceBackticks replaces the "BACKTICK" placeholder text with backticks so
// we can include struct tags within source fixtures.
func replaceBackticks(source string) string {
	return strings.ReplaceAll(source, "BACKTICK", "`")
}

func TestReferenceDataFromDeclaration(t *testing.T) {
	cases := []struct {
		description string
		source      string
		expected    map[PackageInfo]ReferenceEntry
		// Go source fixtures that the test uses for named type fields.
		declSources []string
		// Substring to expect in a resulting error message
		errorSubstring string
		declInfo       PackageInfo
	}{
		{
			description: "scalar fields with one field ignored",
			declInfo: PackageInfo{
				DeclName:    "Metadata",
				PackageName: "mypkg",
			},
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
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "Metadata",
					PackageName: "mypkg",
				}: {
					SectionName: "Metadata",
					Description: "Describes information about a dynamic resource. Every dynamic resource in Teleport has a metadata object.",
					SourcePath:  "/src/myfile.go",
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
			},
		},
		{
			description: "sequences of scalars",
			declInfo: PackageInfo{
				DeclName:    "Metadata",
				PackageName: "mypkg",
			},
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
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "Metadata",
					PackageName: "mypkg",
				}: {
					SectionName: "Metadata",
					Description: "Describes information about a dynamic resource. Every dynamic resource in Teleport has a metadata object.",
					SourcePath:  "/src/myfile.go",
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
							Name:        "numbers",
							Description: "A list of numbers.",
							Type:        "[]number",
						},
						Field{
							Name:        "names",
							Description: "A list of names.",
							Type:        "[]string",
						},
						Field{
							Name:        "booleans",
							Description: "A list of Booleans.",
							Type:        "[]Boolean",
						},
					},
				},
			},
		},
		{
			description: "a map of strings to sequences",
			declInfo: PackageInfo{
				DeclName:    "Metadata",
				PackageName: "mypkg",
			},
			source: `
package mypkg

// Metadata describes information about a dynamic resource. Every dynamic
// resource in Teleport has a metadata object.
type Metadata struct {
  // Attributes indicates additional data for the resource.
  Attributes map[string][]string BACKTICKjson:"attributes"BACKTICK
}
`,
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "Metadata",
					PackageName: "mypkg",
				}: {
					SectionName: "Metadata",
					Description: "Describes information about a dynamic resource. Every dynamic resource in Teleport has a metadata object.",
					SourcePath:  "/src/myfile.go",
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
			},
		},
		{
			description: "an undeclared custom type field",
			declInfo: PackageInfo{
				DeclName:    "Server",
				PackageName: "mypkg",
			},
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
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "Server",
					PackageName: "mypkg",
				}: ReferenceEntry{
					SectionName: "Server",
					Description: "Includes information about a server registered with Teleport.",
					SourcePath:  "/src/myfile.go",
					Fields: []Field{
						{
							Name:        "spec",
							Description: "Contains information about the server.",
							Type:        "",
						},
						{
							Name:        "name",
							Description: "The name of the resource.",
							Type:        "string",
						},
					},
					YAMLExample: "name: \"string\"\nspec: # See description\n",
				},
			},
		},
		{
			description: "named scalar type",
			declInfo: PackageInfo{
				PackageName: "mypkg",
				DeclName:    "Server",
			},
			source: `package mypkg
// Server includes information about a server registered with Teleport.
type Server struct {
    // Name is the name of the resource.
    Name string BACKTICKprotobuf:"bytes,1,opt,name=Name,proto3" json:"name"BACKTICK
    // Spec contains information about the server.
    Spec types.ServerSpecV1 BACKTICKjson:"spec"BACKTICK
    // Label specifies labels for the server.
    Label Labels BACKTICKjson:"labels"BACKTICK
}
`,
			declSources: []string{
				`package mypkg

// Labels is a slice of strings that we'll process downstream
type Labels []string
`,
			},
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "Server",
					PackageName: "mypkg",
				}: ReferenceEntry{
					SectionName: "Server",
					Description: "Includes information about a server registered with Teleport.",
					SourcePath:  "/src/myfile.go",
					Fields: []Field{
						{
							Name:        "spec",
							Description: "Contains information about the server.",
							Type:        "",
						},

						{
							Name:        "name",
							Description: "The name of the resource.",
							Type:        "string",
						},
						{
							Name:        "labels",
							Description: "Specifies labels for the server.",
							Type:        "[Labels](#labels)",
						},
					},
					YAMLExample: "name: \"string\"\nspec: # See description\nlabels: # [...]\n",
				},
				PackageInfo{
					DeclName:    "Labels",
					PackageName: "mypkg",
				}: ReferenceEntry{
					SectionName: "Labels",
					Description: "A slice of strings that we'll process downstream",
					SourcePath:  "/src/myfile0.go",
					Fields:      nil,
					YAMLExample: "",
				},
			},
		},
		{
			description: "custom type fields with a custom JSON unmarshaller",
			declInfo: PackageInfo{
				DeclName:    "Server",
				PackageName: "mypkg",
			},
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
			declSources: []string{
				`package mypkg

func (s *Server) UnmarshalJSON (b []byte) error {
  return nil
}
`,
			},
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "Server",
					PackageName: "mypkg",
				}: ReferenceEntry{
					SectionName: "Server",
					Description: "Includes information about a server registered with Teleport.",
					SourcePath:  "/src/myfile.go",
					Fields: []Field{
						{
							Name:        "spec",
							Description: "Contains information about the server.",
							Type:        "",
						},
						{
							Name:        "name",
							Description: "The name of the resource.",
							Type:        "string",
						},
					},
					YAMLExample: "name: \"string\"\nspec: # See description\n",
				},
			},
		},
		{
			description: "custom type with custom YAML unmarshaller",
			declInfo: PackageInfo{
				DeclName:    "Application",
				PackageName: "mypkg",
			},
			source: `
package mypkg

// Application includes information about an application registered with Teleport.
type Application struct {
    // Name is the name of the resource.
    Name string BACKTICKprotobuf:"bytes,1,opt,name=Name,proto3" json:"name"BACKTICK
    // Spec contains information about the application.
    Spec types.AppSpecV1 BACKTICKjson:"spec"BACKTICK
}
`,
			declSources: []string{
				`package mypkg

func (a *Application) UnmarshalYAML(value *yaml.Node) error {
  return nil
}
`,
			},
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "Application",
					PackageName: "mypkg",
				}: ReferenceEntry{
					SectionName: "Application",
					Description: "Includes information about an application registered with Teleport.",
					SourcePath:  "/src/myfile.go",
					Fields: []Field{
						{
							Name:        "spec",
							Description: "Contains information about the application.",
							Type:        "",
						},
						{
							Name:        "name",
							Description: "The name of the resource.",
							Type:        "string",
						},
					},
					YAMLExample: "name: \"string\"\nspec: # See description\n",
				},
			},
		},
		{
			description: "a custom type field declared in a second source file",
			declInfo: PackageInfo{
				DeclName:    "Server",
				PackageName: "mypkg",
			},
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
}`,
			},
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "Server",
					PackageName: "mypkg",
				}: {
					SectionName: "Server",
					Description: "Includes information about a server registered with Teleport.",
					SourcePath:  "/src/myfile.go",
					YAMLExample: `name: "string"
spec: # [...]
`,
					Fields: []Field{
						Field{
							Name:        "spec",
							Description: "Contains information about the server.",
							Type:        "[Server Spec](#server-spec)",
						},
						Field{
							Name:        "name",
							Description: "The name of the resource.",
							Type:        "string",
						},
					},
				},
				PackageInfo{
					DeclName:    "ServerSpecV1",
					PackageName: "types",
				}: {
					SectionName: "Server Spec",
					Description: "Includes aspects of a proxied server.",
					SourcePath:  "/src/myfile0.go",
					YAMLExample: `address: "string"
ttl: 1
is_active: true
`,
					Fields: []Field{

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
						Field{
							Name:        "address",
							Description: "The address of the server.",
							Type:        "string",
						},
					},
				},
			},
		},
		{
			description: "composite field type with named scalar type",
			declInfo: PackageInfo{
				DeclName:    "Server",
				PackageName: "mypkg",
			},
			source: `
package mypkg

// Server includes information about a server registered with Teleport.
type Server struct {
    // Spec contains information about the server.
    Spec types.ServerSpecV1 BACKTICKjson:"spec"BACKTICK
    // LabelMaps includes a map of strings to labels.
    LabelMaps []map[string]types.Label BACKTICKjson:"label_maps"BACKTICK
}
`,
			declSources: []string{`package types
// ServerSpecV1 includes aspects of a proxied server.
type ServerSpecV1 struct {
    // The address of the server.
    Address string BACKTICKjson:"address"BACKTICK
}`,
				`package types

// Label is a custom type that we unmarshal in a non-default way.
type Label string
`,
			},
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "Server",
					PackageName: "mypkg",
				}: {
					SectionName: "Server",
					Description: "Includes information about a server registered with Teleport.",
					SourcePath:  "/src/myfile.go",
					YAMLExample: `spec: # [...]
label_maps: 
  - 
    "string": # [...]
    "string": # [...]
    "string": # [...]
  - 
    "string": # [...]
    "string": # [...]
    "string": # [...]
  - 
    "string": # [...]
    "string": # [...]
    "string": # [...]
`,
					Fields: []Field{
						Field{
							Name:        "spec",
							Description: "Contains information about the server.",
							Type:        "[Server Spec](#server-spec)"},
						Field{
							Name:        "label_maps",
							Description: "Includes a map of strings to labels.",
							Type:        "[]map[string][Label](#label)",
						},
					},
				},
				PackageInfo{
					DeclName:    "ServerSpecV1",
					PackageName: "types",
				}: {
					SectionName: "Server Spec",
					Description: "Includes aspects of a proxied server.",
					SourcePath:  "/src/myfile0.go",
					YAMLExample: `address: "string"
`,
					Fields: []Field{
						Field{
							Name:        "address",
							Description: "The address of the server.",
							Type:        "string",
						},
					},
				},
				PackageInfo{
					DeclName:    "Label",
					PackageName: "types",
				}: {
					SectionName: "Label",
					Description: "A custom type that we unmarshal in a non-default way.",
					SourcePath:  "/src/myfile1.go",
					Fields:      nil,
				},
			},
		},
		{
			description: "struct type with an interface field",
			declInfo: PackageInfo{
				DeclName:    "Server",
				PackageName: "mypkg",
			},
			source: `
package mypkg

// Server includes information about a server registered with Teleport.
type Server struct {
  // The name of the server.
  Name string BACKTICK:json:"name"BACKTICK
  // Impl is the implementation of the server.
  Impl ServerImplementation BACKTICK:json:"impl"BACKTICK
}
`,
			declSources: []string{`package mypkg
// ServerImplementation is a remote service with a URL.
type ServerImplementation interface{
  GetURL() string
}
`,
			},
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "Server",
					PackageName: "mypkg",
				}: {
					SectionName: "Server",
					Description: "Includes information about a server registered with Teleport.",
					SourcePath:  "/src/myfile.go",
					Fields: []Field{
						{
							Name:        "name",
							Description: "The name of the server.",
							Type:        "string",
						},
						{
							Name:        "impl",
							Description: "The implementation of the server.",
							Type:        "[Server Implementation](#server-implementation)",
						},
					},
					YAMLExample: `name: "string"
impl: # [...]
`,
				},
				PackageInfo{
					DeclName:    "ServerImplementation",
					PackageName: "mypkg",
				}: ReferenceEntry{
					SectionName: "Server Implementation",
					Description: "A remote service with a URL.",
					SourcePath:  "/src/myfile0.go",
					Fields:      nil,
					YAMLExample: "",
				},
			},
		},
		{
			description: "embedded struct",
			declInfo: PackageInfo{
				DeclName:    "MyResource",
				PackageName: "mypkg",
			},
			source: `package mypkg
// MyResource is a resource declared for testing.
type MyResource struct{
  // Alias is another name to call the resource.
  Alias string BACKTICKjson:"alias"BACKTICK
  types.Metadata
}
`,
			declSources: []string{
				`package types

// Metadata describes information about a dynamic resource. Every dynamic
// resource in Teleport has a metadata object.
type Metadata struct {
    // Name is the name of the resource.
    Name string BACKTICKprotobuf:"bytes,1,opt,name=Name,proto3" json:"name"BACKTICK
    // Active indicates whether the resource is currently in use.
    Active bool BACKTICKjson:"active"BACKTICK
}`,
			},
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "MyResource",
					PackageName: "mypkg",
				}: {
					SectionName: "My Resource",
					Description: "A resource declared for testing.",
					SourcePath:  "/src/myfile.go",
					Fields: []Field{

						{
							Name:        "name",
							Description: "The name of the resource.",
							Type:        "string",
						},
						{
							Name:        "alias",
							Description: "Another name to call the resource.",
							Type:        "string",
						},
						{
							Name:        "active",
							Description: "Indicates whether the resource is currently in use.",
							Type:        "Boolean",
						},
					},
					YAMLExample: `alias: "string"
name: "string"
active: true
`,
				},
			},
		},
		{
			description: "embedded struct with base in the same package",
			declInfo: PackageInfo{
				DeclName:    "MyResource",
				PackageName: "mypkg",
			},
			source: `package mypkg
// MyResource is a resource declared for testing.
type MyResource struct{
  // Alias is another name to call the resource.
  Alias string BACKTICKjson:"alias"BACKTICK
  Metadata
}
`,
			declSources: []string{
				`package mypkg

// Metadata describes information about a dynamic resource. Every dynamic
// resource in Teleport has a metadata object.
type Metadata struct {
    // Name is the name of the resource.
    Name string BACKTICKprotobuf:"bytes,1,opt,name=Name,proto3" json:"name"BACKTICK
    // Active indicates whether the resource is currently in use.
    Active bool BACKTICKjson:"active"BACKTICK
}`,
			},
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "MyResource",
					PackageName: "mypkg",
				}: {
					SectionName: "My Resource",
					Description: "A resource declared for testing.",
					SourcePath:  "/src/myfile.go",
					Fields: []Field{

						{
							Name:        "name",
							Description: "The name of the resource.",
							Type:        "string",
						},
						{
							Name:        "alias",
							Description: "Another name to call the resource.",
							Type:        "string",
						},
						{
							Name:        "active",
							Description: "Indicates whether the resource is currently in use.",
							Type:        "Boolean",
						},
					},
					YAMLExample: `alias: "string"
name: "string"
active: true
`,
				},
			},
		},
		{
			description: "struct with two embedded structs",
			declInfo: PackageInfo{
				DeclName:    "MyResource",
				PackageName: "mypkg",
			},
			source: `package mypkg
// MyResource is a resource declared for testing.
type MyResource struct{
  // Alias is another name to call the resource.
  Alias string BACKTICKjson:"alias"BACKTICK
  types.Metadata
  moretypes.ActivityStatus
}
`,
			declSources: []string{
				`package types

// Metadata describes information about a dynamic resource. Every dynamic
// resource in Teleport has a metadata object.
type Metadata struct {
    // Name is the name of the resource.
    Name string BACKTICKprotobuf:"bytes,1,opt,name=Name,proto3" json:"name"BACKTICK
}`,
				`package moretypes

// ActivityStatus indicates the status of a resource
type ActivityStatus struct{
    // Active indicates whether the resource is currently in use.
    Active bool BACKTICKjson:"active"BACKTICK
}`,
			},
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "MyResource",
					PackageName: "mypkg",
				}: {
					SectionName: "My Resource",
					Description: "A resource declared for testing.",
					SourcePath:  "/src/myfile.go",
					Fields: []Field{
						{
							Name:        "name",
							Description: "The name of the resource.",
							Type:        "string",
						},
						{
							Name:        "alias",
							Description: "Another name to call the resource.",
							Type:        "string",
						},
						{
							Name:        "active",
							Description: "Indicates whether the resource is currently in use.",
							Type:        "Boolean",
						},
					},
					YAMLExample: `alias: "string"
name: "string"
active: true
`,
				},
			},
		},
		{
			description: "embedded struct with an embedded struct",
			declInfo: PackageInfo{
				DeclName:    "MyResource",
				PackageName: "mypkg",
			},
			source: `package mypkg
// MyResource is a resource declared for testing.
type MyResource struct{
  // Alias is another name to call the resource.
  Alias string BACKTICKjson:"alias"BACKTICK
  types.Metadata
}
`,
			declSources: []string{
				`package types

// Metadata describes information about a dynamic resource. Every dynamic
// resource in Teleport has a metadata object.
type Metadata struct {
    // Name is the name of the resource.
    Name string BACKTICKprotobuf:"bytes,1,opt,name=Name,proto3" json:"name"BACKTICK
    moretypes.ActivityStatus
}`,
				`package moretypes

// ActivityStatus indicates the status of a resource
type ActivityStatus struct{
    // Active indicates whether the resource is currently in use.
    Active bool BACKTICKjson:"active"BACKTICK
}`,
			},
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "MyResource",
					PackageName: "mypkg",
				}: {
					SectionName: "My Resource",
					Description: "A resource declared for testing.",
					SourcePath:  "/src/myfile.go",
					Fields: []Field{
						{
							Name:        "name",
							Description: "The name of the resource.",
							Type:        "string",
						},
						{
							Name:        "alias",
							Description: "Another name to call the resource.",
							Type:        "string",
						},
						{
							Name:        "active",
							Description: "Indicates whether the resource is currently in use.",
							Type:        "Boolean",
						},
					},
					YAMLExample: `alias: "string"
name: "string"
active: true
`,
				},
			},
		},
		{
			description: "ignored fields with non-YAML-comptabible types",
			declInfo: PackageInfo{
				DeclName:    "Metadata",
				PackageName: "mypkg",
			},
			source: `
package mypkg

// Metadata describes information about a dynamic resource. Every dynamic
// resource in Teleport has a metadata object.
type Metadata struct {
    // Name is the name of the resource.
    Name string BACKTICKprotobuf:"bytes,1,opt,name=Name,proto3" json:"name"BACKTICK
    XXX_NoUnkeyedLiteral struct{} BACKTICKjson:"-"BACKTICK
    XXX_unrecognized     []byte   BACKTICKjson:"-"BACKTICK

}
`,
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "Metadata",
					PackageName: "mypkg",
				}: {
					SectionName: "Metadata",
					Description: "Describes information about a dynamic resource. Every dynamic resource in Teleport has a metadata object.",
					SourcePath:  "/src/myfile.go",
					YAMLExample: `name: "string"
`,
					Fields: []Field{
						Field{
							Name:        "name",
							Description: "The name of the resource.",
							Type:        "string",
						},
					},
				},
			},
		},
		{
			description: "non-embedded custom field type declared in the same package as the containing struct",

			declInfo: PackageInfo{
				DeclName:    "DatabaseServerV3",
				PackageName: "typestest",
			},
			source: `package typestest

// DatabaseServerV3 represents a database access server.
type DatabaseServerV3 struct {
	// Kind is the database server resource kind.
	Kind string BACKTICKprotobuf:"bytes,1,opt,name=Kind,proto3" json:"kind"BACKTICK
	// Metadata is the database server metadata.
	Metadata Metadata BACKTICKprotobuf:"bytes,4,opt,name=Metadata,proto3" json:"metadata"BACKTICK
}
`,
			declSources: []string{
				`package typestest

// Metadata is resource metadata
type Metadata struct {
	// Name is an object name
	Name string BACKTICKprotobuf:"bytes,1,opt,name=Name,proto3" json:"name"BACKTICK
	// Description is object description
	Description string BACKTICKprotobuf:"bytes,3,opt,name=Description,proto3" json:"description,omitempty"BACKTICK
}`,
			},
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "DatabaseServerV3",
					PackageName: "typestest",
				}: ReferenceEntry{
					SectionName: "Database Server",
					Description: "Represents a database access server.",
					SourcePath:  "/src/myfile.go",
					Fields: []Field{
						Field{
							Name:        "metadata",
							Description: "The database server metadata.",
							Type:        "[Metadata](#metadata)",
						},
						Field{
							Name:        "kind",
							Description: "The database server resource kind.",
							Type:        "string",
						},
					},
					YAMLExample: `kind: "string"
metadata: # [...]
`,
				},
				PackageInfo{
					DeclName:    "Metadata",
					PackageName: "typestest",
				}: ReferenceEntry{
					SectionName: "Metadata",
					Description: "Resource metadata",
					SourcePath:  "/src/myfile0.go",
					Fields: []Field{
						{
							Name:        "name",
							Description: "An object name",
							Type:        "string",
						},
						{
							Name:        "description",
							Description: "Object description",
							Type:        "string",
						},
					},
					YAMLExample: `name: "string"
description: "string"
`,
				},
			},
		},
		{
			description: "pointer field",
			declInfo: PackageInfo{
				DeclName:    "DatabaseServerV3",
				PackageName: "typestest",
			},
			source: `package typestest

// DatabaseServerV3 represents a database access server.
type DatabaseServerV3 struct {
	// Metadata is the database server metadata.
	Metadata *Metadata BACKTICKprotobuf:"bytes,4,opt,name=Metadata,proto3" json:"metadata"BACKTICK
}
`,
			declSources: []string{
				`package typestest

// Metadata is resource metadata
type Metadata struct {
	// Name is an object name
	Name string BACKTICKprotobuf:"bytes,1,opt,name=Name,proto3" json:"name"BACKTICK
}`,
			},
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "DatabaseServerV3",
					PackageName: "typestest",
				}: ReferenceEntry{
					SectionName: "Database Server",
					Description: "Represents a database access server.",
					SourcePath:  "/src/myfile.go",
					Fields: []Field{
						Field{
							Name:        "metadata",
							Description: "The database server metadata.",
							Type:        "[Metadata](#metadata)",
						},
					},
					YAMLExample: `metadata: # [...]
`,
				},
				PackageInfo{
					DeclName:    "Metadata",
					PackageName: "typestest",
				}: ReferenceEntry{
					SectionName: "Metadata",
					Description: "Resource metadata",
					SourcePath:  "/src/myfile0.go",
					Fields: []Field{
						{
							Name:        "name",
							Description: "An object name",
							Type:        "string",
						},
					},
					YAMLExample: `name: "string"
`,
				},
			},
		},
		{
			description: "map of strings to an undeclared field",
			declInfo: PackageInfo{
				PackageName: "mypkg",
				DeclName:    "Server",
			},
			source: `
package mypkg

// Server includes information about a server registered with Teleport.
type Server struct {
    // Name is the name of the server.
    Name string BACKTICKjson:"name"BACKTICK
    // LabelMaps includes a map of strings to labels.
    LabelMaps []map[string]types.Label BACKTICKjson:"label_maps"BACKTICK
}
`,
			declSources: []string{`package types
// ServerSpecV1 includes aspects of a proxied server.
type ServerSpecV1 struct {
    // The address of the server.
    Address string BACKTICKjson:"address"BACKTICK
}`,
			},
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "Server",
					PackageName: "mypkg",
				}: {
					SectionName: "Server",
					Description: "Includes information about a server registered with Teleport.",
					SourcePath:  "/src/myfile.go",
					YAMLExample: `name: "string"
label_maps: 
  - 
    "string": # See description
    "string": # See description
    "string": # See description
  - 
    "string": # See description
    "string": # See description
    "string": # See description
  - 
    "string": # See description
    "string": # See description
    "string": # See description
`,
					Fields: []Field{
						Field{
							Name:        "name",
							Description: "The name of the server.",
							Type:        "string"},
						Field{
							Name:        "label_maps",
							Description: "Includes a map of strings to labels.",
							Type:        "[]map[string]",
						},
					},
				},
			},
		},
		{
			description: "type parameter",
			declInfo: PackageInfo{
				PackageName: "mypkg",
				DeclName:    "Resource",
			},
			source: `package mypkg
// Resource is a resource.
type Resource struct {
  // The name of the resource.
  Name string BACKTICKjson:"name"BACKTICK
}
`,
			declSources: []string{
				`package mypkg
// streamFunc is a wrapper that converts a closure into a stream.
type streamFunc[T any] struct {
	fn        func() (T, error)
	doneFuncs []func()
	item      T
	err       error
}

func (stream *streamFunc[T]) Next() bool {
	stream.item, stream.err = stream.fn()
	return stream.err == nil
}
`,
			},
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					PackageName: "mypkg",
					DeclName:    "Resource",
				}: ReferenceEntry{
					SectionName: "Resource",
					Description: "A resource.",
					SourcePath:  "/src/myfile.go",
					YAMLExample: `name: "string"
`,
					Fields: []Field{
						Field{
							Name:        "name",
							Description: "The name of the resource.",
							Type:        "string",
						},
					},
				},
			},
		},
		{
			description: "field type not declared in a loaded package",
			declInfo: PackageInfo{
				PackageName: "mypkg",
				DeclName:    "Resource",
			},
			source: `package mypkg

// Resource is a resource.
type Resource struct {
  // The name of the resource.
  Name string BACKTICKjson:"name"BACKTICK
  // How much time must elapse before the resource expires.
  Expiry time.Time BACKTICKjson:"expiry"BACKTICK
}
`,
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					PackageName: "mypkg",
					DeclName:    "Resource",
				}: ReferenceEntry{
					SectionName: "Resource",
					Description: "A resource.",
					SourcePath:  "/src/myfile.go",
					YAMLExample: `name: "string"
expiry: # See description
`,
					Fields: []Field{
						Field{
							Name:        "name",
							Description: "The name of the resource.",
							Type:        "string",
						},
						Field{
							Name:        "expiry",
							Description: "How much time must elapse before the resource expires.",
							Type:        "",
						},
					},
				},
			},
			declSources: []string{},
		},
		{
			description: "byte slice",
			declInfo: PackageInfo{
				PackageName: "mypkg",
				DeclName:    "Metadata",
			},
			source: `
package mypkg

// Metadata describes information about a dynamic resource. Every dynamic
// resource in Teleport has a metadata object.
type Metadata struct {
    // Name is the name of the resource.
    Name string BACKTICKprotobuf:"bytes,1,opt,name=Name,proto3" json:"name"BACKTICK
    // PrivateKey is the private key of the resource.
    PrivateKey []byte BACKTICKjson:"private_key"BACKTICK
}
`,
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "Metadata",
					PackageName: "mypkg",
				}: {
					SectionName: "Metadata",
					Description: "Describes information about a dynamic resource. Every dynamic resource in Teleport has a metadata object.",
					SourcePath:  "/src/myfile.go",
					YAMLExample: `name: "string"
private_key: BASE64_STRING
`,
					Fields: []Field{
						Field{
							Name:        "private_key",
							Description: "The private key of the resource.",
							Type:        "base64-encoded string",
						},
						Field{
							Name:        "name",
							Description: "The name of the resource.",
							Type:        "string",
						},
					},
				},
			},
		},
		{
			description: "named import in embedded struct field",
			declInfo: PackageInfo{
				PackageName: "mypkg",
				DeclName:    "Server",
			},
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

import alias "otherpkg"

// ServerSpecV1 includes aspects of a proxied server.
type ServerSpecV1 struct {
  alias.ServerSpec
}`,
				`package otherpkg

type ServerSpec struct {
    // The address of the server.
    Address string BACKTICKjson:"address"BACKTICK
}`,
			},
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "Server",
					PackageName: "mypkg",
				}: {
					SectionName: "Server",
					Description: "Includes information about a server registered with Teleport.",
					SourcePath:  "/src/myfile.go",
					YAMLExample: `name: "string"
spec: # [...]
`,
					Fields: []Field{
						Field{
							Name:        "spec",
							Description: "Contains information about the server.",
							Type:        "[Server Spec](#server-spec)",
						},
						Field{
							Name:        "name",
							Description: "The name of the resource.",
							Type:        "string",
						},
					},
				},
				PackageInfo{
					DeclName:    "ServerSpecV1",
					PackageName: "types",
				}: {
					SectionName: "Server Spec",
					Description: "Includes aspects of a proxied server.",
					SourcePath:  "/src/myfile0.go",
					YAMLExample: `address: "string"
`,
					Fields: []Field{
						Field{
							Name:        "address",
							Description: "The address of the server.",
							Type:        "string",
						},
					},
				},
			},
		},
		{
			description: "named import in named struct field",
			declInfo: PackageInfo{
				PackageName: "mypkg",
				DeclName:    "Server",
			},
			source: `
package mypkg

// Server includes information about a server registered with Teleport.
type Server struct {
    // Spec contains information about the server.
    Spec types.ServerSpecV1 BACKTICKjson:"spec"BACKTICK
}
`,
			declSources: []string{`package types
import alias "otherpkg"

// ServerSpecV1 includes aspects of a proxied server.
type ServerSpecV1 struct {
  // Address information.
  Info alias.AddressInfo BACKTICKjson:"info"BACKTICK
}`,

				`package otherpkg
// AddressInfo provides information about an address.
type AddressInfo struct {
    // The address of the server.
    Address string BACKTICKjson:"address"BACKTICK
}`,
			},
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "AddressInfo",
					PackageName: "otherpkg",
				}: {
					SectionName: "Address Info",
					Description: "Provides information about an address.",
					SourcePath:  "/src/myfile1.go",
					Fields: []Field{
						{
							Name:        "address",
							Description: "The address of the server.",
							Type:        "string",
						},
					},
					YAMLExample: "address: \"string\"\n",
				},
				PackageInfo{
					DeclName:    "Server",
					PackageName: "mypkg",
				}: {
					SectionName: "Server",
					Description: "Includes information about a server registered with Teleport.",
					SourcePath:  "/src/myfile.go",
					YAMLExample: `spec: # [...]
`,
					Fields: []Field{
						Field{
							Name:        "spec",
							Description: "Contains information about the server.",
							Type:        "[Server Spec](#server-spec)"},
					},
				},
				PackageInfo{
					DeclName:    "ServerSpecV1",
					PackageName: "types",
				}: {
					SectionName: "Server Spec",
					Description: "Includes aspects of a proxied server.",
					SourcePath:  "/src/myfile0.go",
					YAMLExample: `info: # [...]
`,
					Fields: []Field{
						Field{
							Name:        "info",
							Description: "Address information.",
							Type:        "[Address Info](#address-info)",
						},
					},
				},
			},
		},
		{
			description: "scalar fields with two unexported fields",
			declInfo: PackageInfo{
				DeclName:    "Metadata",
				PackageName: "mypkg",
			},
			source: `
package mypkg

// Metadata describes information about a dynamic resource. Every dynamic
// resource in Teleport has a metadata object.
type Metadata struct {
    // Name is the name of the resource.
    Name string BACKTICKprotobuf:"bytes,1,opt,name=Name,proto3" json:"name"BACKTICK
    // Description is the resource's description.
    Description string BACKTICKprotobuf:"bytes,3,opt,name=Description,proto3" json:"description,omitempty"BACKTICK
    state protoimpl.MessageState BACKTICKprotogen:"open.v1"BACKTICK
    unknownFields protoimpl.UnknownFields
    sizeCache     protoimpl.SizeCache
}
`,
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "Metadata",
					PackageName: "mypkg",
				}: {
					SectionName: "Metadata",
					Description: "Describes information about a dynamic resource. Every dynamic resource in Teleport has a metadata object.",
					SourcePath:  "/src/myfile.go",
					YAMLExample: `name: "string"
description: "string"
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
					},
				},
			},
		},
		{
			description: "curly braces in descriptions",
			declInfo: PackageInfo{
				DeclName:    "Metadata",
				PackageName: "mypkg",
			},
			source: `
package mypkg

// Metadata describes information about a {dynamic resource}. Every dynamic
// resource in Teleport has a metadata object.
type Metadata struct {
    // Name is the {name of the resource}.
    Name string BACKTICKprotobuf:"bytes,1,opt,name=Name,proto3" json:"name"BACKTICK
}
`,
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "Metadata",
					PackageName: "mypkg",
				}: {
					SectionName: "Metadata",
					Description: "Describes information about a `{dynamic resource}`. Every dynamic resource in Teleport has a metadata object.",
					SourcePath:  "/src/myfile.go",
					YAMLExample: `name: "string"
`,
					Fields: []Field{
						Field{
							Name:        "name",
							Description: "The `{name of the resource}`.",
							Type:        "string",
						},
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			// Make a map of filenames to content
			sources := make(map[string][]byte)
			sources["/src/myfile.go"] = []byte(replaceBackticks(tc.source))
			for i, s := range tc.declSources {
				sources["/src/myfile"+strconv.Itoa(i)+".go"] = []byte(replaceBackticks(s))
			}

			memfs := afero.NewMemMapFs()
			if err := memfs.MkdirAll("/src", 0777); err != nil {
				t.Fatal(err)
			}
			for n, v := range sources {
				if err := afero.WriteFile(memfs, n, v, 0777); err != nil {
					t.Fatal(err)
				}
			}

			sourceData, err := NewSourceData(memfs, "/src")
			if err != nil {
				t.Fatal(err)
			}

			di, ok := sourceData.TypeDecls[tc.declInfo]
			if !ok {
				t.Fatalf("expected data for %v.%v not found in the source", tc.declInfo.PackageName, tc.declInfo.DeclName)
			}

			r, err := ReferenceDataFromDeclaration(di, sourceData.TypeDecls)
			if tc.errorSubstring == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tc.errorSubstring)
			}

			assert.Equal(t, tc.expected, r)
		})
	}
}

func TestNamedImports(t *testing.T) {
	cases := []struct {
		description string
		input       string
		expected    map[string]string
	}{
		{
			description: "single-line format",
			input: `package mypkg
import alias "otherpkg"
`,
			expected: map[string]string{
				"alias": "otherpkg",
			},
		},
		{
			description: "multi-line format",
			input: `package mypkg
import (
    alias "first"
    alias2 "second"
)
`,
			expected: map[string]string{
				"alias":  "first",
				"alias2": "second",
			},
		},
		{
			description: "multi-segment package path",
			input: `package mypkg
import alias "my/multi/segment/package"
`,
			expected: map[string]string{
				"alias": "package",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset,
				"myfile.go",
				c.input,
				parser.ParseComments,
			)
			assert.NoError(t, err)
			assert.Equal(t, c.expected, NamedImports(f))
		})
	}
}

func TestMakeFieldTableInfo(t *testing.T) {
	cases := []struct {
		description string
		input       []rawField
		expected    []Field
	}{
		{
			description: "angle brackets in GoDoc",
			input: []rawField{
				rawField{
					packageName: "mypkg",
					doc:         `An ID, e.g., "<myid>"`,
					kind:        yamlString{},
					name:        "ObjectID",
					jsonName:    "object_id",
					tags:        `json:"object_id"`,
				},
			},
			expected: []Field{
				{
					Name:        "object_id",
					Description: `An ID, e.g., "\<myid\>"`,
					Type:        "string",
				},
			},
		},
		{
			description: "pipe in field description",
			input: []rawField{
				{
					packageName: "mypkg",
					doc:         "Specifies the locking mode (strict|best_effort) to be applied with the role.",
					kind: yamlCustomType{
						name: "LockingMode",
						declarationInfo: PackageInfo{
							DeclName:    "LockingMode",
							PackageName: "mypkg",
						},
					},
					name:     "LockingMode",
					jsonName: "locking_mode",
					tags:     "json:\"locking_mode\"",
				},
			},
			expected: []Field{
				{
					Name:        "locking_mode",
					Description: `Specifies the locking mode (strict\|best_effort) to be applied with the role.`,
					Type:        "[LockingMode](#lockingmode)",
				},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			f, err := makeFieldTableInfo(c.input)
			assert.NoError(t, err)
			assert.Equal(t, c.expected, f)
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

func TestPrintableDescription(t *testing.T) {
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
		{
			description: "curly brace pair and identifier name",
			input:       "MyDecl performs an action, such as {updating, deleting}",
			name:        "MyDecl",
			expected:    "Performs an action, such as `{updating, deleting}`",
		},
		{
			description: "curly brace pair and no identifier name",
			input:       "Performs an action, such as {updating, deleting}",
			name:        "MyDecl",
			expected:    "Performs an action, such as `{updating, deleting}`",
		},
		{
			description: "curly brace pair with existing backticks",
			input:       "Performs an action, such as `{updating, deleting}`",
			name:        "MyDecl",
			expected:    "Performs an action, such as `{updating, deleting}`",
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			assert.Equal(t, c.expected, printableDescription(c.input, c.name))
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
		{
			description: "sequences of custom types",
			input: []rawField{
				rawField{
					name:     "labels",
					jsonName: "labels",
					doc:      "labels is a list of labels",
					tags:     `json:"labels"`,
					kind: yamlSequence{
						elementKind: yamlCustomType{
							name: "label",
							declarationInfo: PackageInfo{
								DeclName:    "label",
								PackageName: "mypkg",
							},
						},
					},
				},
			},
			expected: `labels: 
  - # [...]
  - # [...]
  - # [...]
`,
		},
		{
			description: "maps of strings to custom types",
			input: []rawField{
				rawField{
					name:     "labels",
					jsonName: "labels",
					doc:      "labels is a map of strings to labels",
					tags:     `json:"labels"`,
					kind: yamlMapping{
						keyKind: yamlString{},
						valueKind: yamlCustomType{
							name: "label",
							declarationInfo: PackageInfo{
								DeclName:    "label",
								PackageName: "mypkg",
							},
						},
					},
				},
			},
			expected: `labels: 
  "string": # [...]
  "string": # [...]
  "string": # [...]
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
			expected:    "Server Spec",
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			assert.Equal(t, c.expected, makeSectionName(c.original))
		})
	}
}

func TestGetTopLevelStringAssignments(t *testing.T) {
	cases := []struct {
		description string
		source      string
		expected    map[PackageInfo]string
	}{
		{

			description: "single-var assignments",
			source: `package mypkg
var myString string = "This is a string"
var otherString string ="This is another string"
`,
			expected: map[PackageInfo]string{
				PackageInfo{
					DeclName:    "myString",
					PackageName: "mypkg",
				}: "This is a string",
				PackageInfo{
					DeclName:    "otherString",
					PackageName: "mypkg",
				}: "This is another string",
			},
		},
		{

			description: "single-const assignments",
			source: `package mypkg
const myString string = "This is a string"
const otherString string ="This is another string"
`,
			expected: map[PackageInfo]string{
				PackageInfo{
					DeclName:    "myString",
					PackageName: "mypkg",
				}: "This is a string",
				PackageInfo{
					DeclName:    "otherString",
					PackageName: "mypkg",
				}: "This is another string",
			},
		},
		{

			description: "multiple-var assignments",
			source: `package mypkg

var (
  myString string = "This is a string"
  otherString string ="This is another string"
)
`,
			expected: map[PackageInfo]string{
				PackageInfo{
					DeclName:    "myString",
					PackageName: "mypkg",
				}: "This is a string",
				PackageInfo{
					DeclName:    "otherString",
					PackageName: "mypkg",
				}: "This is another string",
			},
		},
		{

			description: "multiple-const assignments",
			source: `package mypkg

const (
  myString string = "This is a string"
  otherString string ="This is another string"
)
`,
			expected: map[PackageInfo]string{
				PackageInfo{
					DeclName:    "myString",
					PackageName: "mypkg",
				}: "This is a string",
				PackageInfo{
					DeclName:    "otherString",
					PackageName: "mypkg",
				}: "This is another string",
			},
		},
		{

			description: "mix of string and non-string vars and consts",
			source: `package mypkg

import "strings"

const (
  stringConst string = "This is a string"
  boolConst  bool = false
)

var (
    stringVar string = "This is a string"
    funcConst string = strings.ToLower("HELLO") 
)
`,

			expected: map[PackageInfo]string{
				PackageInfo{
					DeclName:    "stringVar",
					PackageName: "mypkg",
				}: "This is a string",
				PackageInfo{
					DeclName:    "stringConst",
					PackageName: "mypkg",
				}: "This is a string",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			fset := token.NewFileSet()
			d, err := parser.ParseFile(fset,
				"myfile.go",
				replaceBackticks(c.source),
				parser.ParseComments,
			)
			if err != nil {
				t.Fatalf("test fixture contains invalid Go source: %v\n", err)
			}

			actual, err := GetTopLevelStringAssignments(d.Decls, d.Name.Name)
			assert.NoError(t, err)
			assert.Equal(t, c.expected, actual)
		})
	}
}

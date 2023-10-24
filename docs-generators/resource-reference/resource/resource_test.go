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
		expected map[PackageInfo]ReferenceEntry
		// Go source fixtures that the test uses for named type fields.
		declSources []string
		// Substring to expect in a resulting error message
		errorSubstring string
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
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "Metadata",
					PackageName: "mypkg",
				}: {
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
			},
		},
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
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "Metadata",
					PackageName: "mypkg",
				}: {
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
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "Metadata",
					PackageName: "mypkg",
				}: {
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
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "Server",
					PackageName: "mypkg",
				}: {
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
			description: "named scalar type with no override",
			source: `package mypkg
// Server includes information about a server registered with Teleport.
type Server struct {
    // Name is the name of the resource.
    Name string BACKTICKprotobuf:"bytes,1,opt,name=Name,proto3" json:"name"BACKTICK
    // Spec contains information about the server.
    Spec types.ServerSpecV1 BACKTICKjson:"spec"BACKTICK
    Label Labels
}
`,
			declSources: []string{
				`package mypkg

// Labels is a slice of strings that we'll process downstream
type Labels []string
`,
			},
			errorSubstring: "example YAML",
		},
		{
			description: "custom type fields with no override and custom JSON unmarshaller",
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
			errorSubstring: "Example YAML:",
		},
		{
			description: "custom type fields with no override and custom YAML unmarshaller",
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
			errorSubstring: "Example YAML:",
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

			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "Server",
					PackageName: "mypkg",
				}: {
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
}`,
			},
			expected: map[PackageInfo]ReferenceEntry{
				PackageInfo{
					DeclName:    "Server",
					PackageName: "mypkg",
				}: {
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
				PackageInfo{
					DeclName:    "ServerSpecV1",
					PackageName: "types",
				}: {
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
		{
			description: "a composite field type with a custom type and an override",
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
// Example YAML:
// ---
// ["my_value0", "my_value1", "my_value2"]
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
					SourcePath:  "myfile.go",
					YAMLExample: `spec: 
# [...]
label_maps: 
- 
    "string": 
      # [...]
    "string": 
      # [...]
    "string": 
      # [...]
- 
    "string": 
      # [...]
    "string": 
      # [...]
    "string": 
      # [...]
- 
    "string": 
      # [...]
    "string": 
      # [...]
    "string": 
      # [...]
`,
					Fields: []Field{
						Field{
							Name:        "spec",
							Description: "Contains information about the server.",
							Type:        "[Server Spec v1](#server-spec-v1)"},
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
					SectionName: "Server Spec v1",
					Description: "Includes aspects of a proxied server.",
					SourcePath:  "myfile0.go",
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
					SourcePath:  "myfile1.go",
					YAMLExample: `["my_value0", "my_value1", "my_value2"]
`,
				},
			},
		},
		{
			description: "embedded struct",
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
					SourcePath:  "myfile.go",
					Fields: []Field{
						{
							Name:        "alias",
							Description: "Another name to call the resource.",
							Type:        "string",
						},
						{
							Name:        "name",
							Description: "The name of the resource.",
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
					SourcePath:  "myfile.go",
					Fields: []Field{
						{
							Name:        "alias",
							Description: "Another name to call the resource.",
							Type:        "string",
						},
						{
							Name:        "name",
							Description: "The name of the resource.",
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
					SourcePath:  "myfile.go",
					Fields: []Field{
						{
							Name:        "alias",
							Description: "Another name to call the resource.",
							Type:        "string",
						},
						{
							Name:        "name",
							Description: "The name of the resource.",
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
					SourcePath:  "myfile.go",
					Fields: []Field{
						{
							Name:        "alias",
							Description: "Another name to call the resource.",
							Type:        "string",
						},
						{
							Name:        "name",
							Description: "The name of the resource.",
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
					SourcePath:  "myfile.go",
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
					SectionName: "Database Server v3",
					Description: "Represents a database access server.",
					SourcePath:  "myfile.go",
					Fields: []Field{
						Field{
							Name:        "kind",
							Description: "The database server resource kind.",
							Type:        "string",
						},
						Field{
							Name:        "metadata",
							Description: "The database server metadata.",
							Type:        "[Metadata](#metadata)",
						},
					},
					YAMLExample: `kind: "string"
metadata: 
# [...]
`,
				},
				PackageInfo{
					DeclName:    "Metadata",
					PackageName: "typestest",
				}: ReferenceEntry{
					SectionName: "Metadata",
					Description: "Resource metadata",
					SourcePath:  "myfile0.go",
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
					SectionName: "Database Server v3",
					Description: "Represents a database access server.",
					SourcePath:  "myfile.go",
					Fields: []Field{
						Field{
							Name:        "metadata",
							Description: "The database server metadata.",
							Type:        "[Metadata](#metadata)",
						},
					},
					YAMLExample: `metadata: 
# [...]
`,
				},
				PackageInfo{
					DeclName:    "Metadata",
					PackageName: "typestest",
				}: ReferenceEntry{
					SectionName: "Metadata",
					Description: "Resource metadata",
					SourcePath:  "myfile0.go",
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
			description: "struct field with an override",
			source: `
package mypkg

// Server includes information about a server registered with Teleport.
type Server struct {
    // Name is the name of the server.
    Name string BACKTICKjson:"name"BACKTICK
    // LabelMaps includes a map of strings to labels.
    // Example YAML:
    // ---
    // - label1: ["my_value0", "my_value1", "my_value2"]
    //   label2: ["my_value0", "my_value1", "my_value2"]
    // - label3: ["my_value0", "my_value1", "my_value2"]
    //   label4: ["my_value0", "my_value1", "my_value2"]
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
					SourcePath:  "myfile.go",
					YAMLExample: `name: "string"
label_maps: 
- label1: ["my_value0", "my_value1", "my_value2"]
  label2: ["my_value0", "my_value1", "my_value2"]
- label3: ["my_value0", "my_value1", "my_value2"]
  label4: ["my_value0", "my_value1", "my_value2"]
`,
					Fields: []Field{
						Field{
							Name:        "name",
							Description: "The name of the server.",
							Type:        "string"},
						Field{
							Name:        "label_maps",
							Description: "Includes a map of strings to labels.",
							Type:        "See YAML example.",
						},
					},
				},
			},
		},
		{
			description: "struct field with an override for an existing field type definition",
			source: `
package mypkg

// Server includes information about a server registered with Teleport.
type Server struct {
    // Name is the name of the server.
    Name string BACKTICKjson:"name"BACKTICK
    // LabelMaps includes a map of strings to labels.
    // Example YAML:
    // ---
    // - label1: ["my_value0", "my_value1", "my_value2"]
    //   label2: ["my_value0", "my_value1", "my_value2"]
    // - label3: ["my_value0", "my_value1", "my_value2"]
    //   label4: ["my_value0", "my_value1", "my_value2"]
    LabelMaps []map[string]mypkg.Label BACKTICKjson:"label_maps"BACKTICK
}
`,
			declSources: []string{`package types
// ServerSpecV1 includes aspects of a proxied server.
type ServerSpecV1 struct {
    // The address of the server.
    Address string BACKTICKjson:"address"BACKTICK
}`,
				`package mypkg

// Label is some more information you can add to a resource.
type Label struct {
  name string
  number int
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
					SourcePath:  "myfile.go",
					YAMLExample: `name: "string"
label_maps: 
- label1: ["my_value0", "my_value1", "my_value2"]
  label2: ["my_value0", "my_value1", "my_value2"]
- label3: ["my_value0", "my_value1", "my_value2"]
  label4: ["my_value0", "my_value1", "my_value2"]
`,
					Fields: []Field{
						Field{
							Name:        "name",
							Description: "The name of the server.",
							Type:        "string"},
						Field{
							Name:        "label_maps",
							Description: "Includes a map of strings to labels.",
							Type:        "See YAML example.",
						},
					},
				},
			},
		},
		{
			description: "type parameter",
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
					SourcePath:  "myfile.go",
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

			// For generating method information
			allDecls := []DeclarationInfo{}
			pkgToDecl := make(map[PackageInfo]DeclarationInfo)
			// Assemble map of PackageInfo to *ast.GenDecl for
			// source fixtures the test case depends on.
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
					allDecls = append(allDecls, DeclarationInfo{
						Decl:        def,
						FilePath:    fmt.Sprintf("myfile%v.go", n),
						PackageName: d.Name.Name,
					})

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
					pkgToDecl[PackageInfo{
						DeclName:    spec.Name.Name,
						PackageName: d.Name.Name,
					}] = DeclarationInfo{
						Decl:        l,
						FilePath:    fmt.Sprintf("myfile%v.go", n),
						PackageName: d.Name.Name,
					}
				}

			}

			allMethods, err := GetMethodInfo(allDecls)
			assert.NoError(t, err)

			r, err := NewFromDecl(DeclarationInfo{
				FilePath:    "myfile.go",
				Decl:        gd,
				PackageName: f.Name.Name,
			}, pkgToDecl, allMethods)
			if tc.errorSubstring == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tc.errorSubstring)
			}

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

func TestGetMethodInfo(t *testing.T) {
	cases := []struct {
		description string
		source      string
		expected    map[PackageInfo][]MethodInfo
	}{
		{
			description: "multiple struct declarations",
			source: `package testpkg

import "strings"

type myStruct struct {
    name string
}

type otherStruct struct {
    name string
    age int
}

func (m *myStruct) uppercaseName() string {
    return strings.ToUpper(m.name)
}

func notAMethod(s string) string {
    return "Not a method"
}

func (o otherStruct) lowercaseName() string{
    return strings.ToLower(m.name)
}
`,
			expected: map[PackageInfo][]MethodInfo{
				PackageInfo{
					DeclName:    "myStruct",
					PackageName: "testpkg",
				}: []MethodInfo{
					{
						Name:             "uppercaseName",
						FieldAssignments: map[string]string{},
					},
				},
				PackageInfo{
					DeclName:    "otherStruct",
					PackageName: "testpkg",
				}: []MethodInfo{
					{
						Name:             "lowercaseName",
						FieldAssignments: map[string]string{},
					},
				},
			},
		},
		{
			description: "assignments",
			source: `package testpkg

import "strings"

type myStruct struct {
    name string
    age int
}

func (m myStruct) assignName() {
    m.name = nameConst
}

func (o *myStruct) assignAgeAndName(a int) {
    o.age = a
    o.name = nameConst
}
`,
			expected: map[PackageInfo][]MethodInfo{
				PackageInfo{
					DeclName:    "myStruct",
					PackageName: "testpkg",
				}: []MethodInfo{
					{
						Name: "assignName",
						FieldAssignments: map[string]string{
							"name": "nameConst",
						},
					},
					{
						Name: "assignAgeAndName",
						FieldAssignments: map[string]string{
							"age":  "a",
							"name": "nameConst",
						},
					},
				},
			},
		},
		{
			description: "some assignment values are skipped",
			source: `package testpkg

func (o *otherStruct) copyNameFrom(m *myStruct){
    o.name = *m.name
    o.name, o.age = blah, 21
}
`,

			expected: map[PackageInfo][]MethodInfo{
				PackageInfo{
					DeclName:    "otherStruct",
					PackageName: "testpkg",
				}: []MethodInfo{
					{
						Name: "copyNameFrom",
						// Expecting no field assignments, since
						// getMethodInfo does not process
						// assignments to star expressions.
						// Also ignoring multiple assignments
						FieldAssignments: map[string]string{},
					},
				},
			},
		},
		{
			description: "method with no receiver identifier",
			source: `package testpkg
func (mystruct) getMessage() string {
    return "This does not depend on the receiver"
}
`,
			expected: map[PackageInfo][]MethodInfo{
				PackageInfo{
					DeclName:    "mystruct",
					PackageName: "testpkg",
				}: []MethodInfo{
					{
						Name:             "getMessage",
						FieldAssignments: map[string]string{},
					},
				},
			},
		},
		{
			description: "type parameter",
			source: `package mypkg
func (stream *streamFunc[T]) Next() bool {
	stream.item, stream.err = stream.fn()
	return stream.err == nil
}
`,
			expected: map[PackageInfo][]MethodInfo{
				PackageInfo{
					PackageName: "mypkg",
					DeclName:    "streamFunc",
				}: []MethodInfo{
					{
						Name:             "Next",
						FieldAssignments: map[string]string{},
					},
				},
			},
		},
		{
			description: "two type parameters",
			source: `package mypkg
func (stream *filterMap[A, B]) Next() bool {
	for {
		if !stream.inner.Next() {
			return false
		}
		var ok bool
		stream.item, ok = stream.fn(stream.inner.Item())
		if !ok {
			continue
		}
		return true
	}
}`,
			expected: map[PackageInfo][]MethodInfo{
				PackageInfo{
					PackageName: "mypkg",
					DeclName:    "filterMap",
				}: []MethodInfo{
					{
						Name:             "Next",
						FieldAssignments: map[string]string{},
					},
				},
			},
		},
		{
			description: "type params with value receiver",
			source: `package mypkg

func (stream MyStream[T]) Next() bool {
	return false
}

func (stream MyStream[A, B]) Finish() bool {
	return false
}
`,
			expected: map[PackageInfo][]MethodInfo{
				PackageInfo{
					PackageName: "mypkg",
					DeclName:    "MyStream",
				}: []MethodInfo{
					{
						Name:             "Next",
						FieldAssignments: map[string]string{},
					},
					{
						Name:             "Finish",
						FieldAssignments: map[string]string{},
					},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			fset := token.NewFileSet()
			allDecls := []DeclarationInfo{}
			d, err := parser.ParseFile(fset,
				"myfile.go",
				replaceBackticks(c.source),
				parser.ParseComments,
			)
			if err != nil {
				t.Fatalf("test fixture contains invalid Go source: %v\n", err)
			}

			for _, l := range d.Decls {
				allDecls = append(allDecls, DeclarationInfo{
					FilePath:    "myfile.go",
					Decl:        l,
					PackageName: d.Name.Name,
				})
			}
			actual, err := GetMethodInfo(allDecls)
			assert.NoError(t, err)
			assert.Equal(t, c.expected, actual)
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

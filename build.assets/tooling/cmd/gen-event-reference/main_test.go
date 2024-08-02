package main

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/gravitational/teleport/gen/go/eventschema"
	"github.com/stretchr/testify/assert"
)

func Test_stringConstantDecls(t *testing.T) {
	cases := []struct {
		description string
		input       io.Reader
		expected    map[string]string
	}{
		{
			description: "bracket-syntax constant decls",
			input: strings.NewReader(`
package main

const (
  const1 = "val1"
  const2 = "val2"
  const3 = "val3"
)
`),
			expected: map[string]string{
				"const1": "val1",
				"const2": "val2",
				"const3": "val3",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			actual, err := stringConstantDecls(c.input)
			assert.NoError(t, err)
			assert.Equal(t, c.expected, actual)
		})
	}
}

func Test_isEventCode(t *testing.T) {
	cases := []struct {
		input    string
		expected bool
	}{
		{
			input:    "AccessListCreateSuccessCode",
			expected: true,
		},
		{
			input:    "AccessListCreateFailureCode",
			expected: true,
		},
		{
			input:    "NodeName",
			expected: false,
		},
	}

	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			actual := isEventCode(c.input)
			assert.Equal(t, c.expected, actual)
		})
	}
}

func Test_typeNameFromCodeName(t *testing.T) {
	cases := []struct {
		description string
		input       string
		expected    string
	}{
		{
			description: "code name ends with SuccessCode",
			input:       "AccessListCreateSuccessCode",
			expected:    "AccessListCreateEvent",
		},
		{
			description: "code name ends with FailureCode",
			input:       "AccessListCreateFailureCode",
			expected:    "AccessListCreateEvent",
		},
		{
			description: "code name ends with Code",
			input:       "AccessListCreateCode",
			expected:    "AccessListCreateEvent",
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			actual := typeNameFromCodeName(c.input)
			assert.Equal(t, c.expected, actual)
		})
	}
}

func Test_schemaNameFromCodeName(t *testing.T) {
	cases := []struct {
		description string
		input       string
		expected    string
	}{
		{
			description: "code name ends with SuccessCode",
			input:       "AccessListCreateSuccessCode",
			expected:    "AccessListCreate",
		},
		{
			description: "code name ends with FailureCode",
			input:       "AccessListCreateFailureCode",
			expected:    "AccessListCreate",
		},
		{
			description: "code name ends with Code",
			input:       "AccessListCreateCode",
			expected:    "AccessListCreate",
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			actual := schemaNameFromCodeName(c.input)
			assert.Equal(t, c.expected, actual)
		})
	}
}

var loginSchema = eventschema.Event{
	Description: "Records a user login event.",
	Fields: []*eventschema.EventField{
		{
			Name:        "cluster_name",
			Description: "Identifies the originating Teleport cluster.",
			Type:        "string",
		},
		{
			Name:        "login",
			Description: "OS login.",
			Type:        "string",
		},
	},
}

var multiLevelSchema = eventschema.Event{
	Description: "Is an event with complex fields.",
	Fields: []*eventschema.EventField{
		{
			Name:        "cluster",
			Description: "Identifies the originating Teleport cluster.",
			Type:        "object",
			Fields: []*eventschema.EventField{
				{
					Name:        "name",
					Description: "Is the cluster name",
					Type:        "string",
				},
				{
					Name:        "address",
					Description: "Is the cluster address",
					Type:        "object",
					Fields: []*eventschema.EventField{
						{
							Name:        "host",
							Description: "Cluster address host.",
							Type:        "string",
						},
						{
							Name:        "port",
							Description: "Cluster address port.",
							Type:        "number",
						},
					},
				},
			},
		},
		{
			Name:        "login",
			Description: "OS login.",
			Type:        "string",
		},
	},
}

func Test_makeEventSectionName(t *testing.T) {
	cases := []struct {
		description string
		typeValue   string
		codeName    string
		expected    string
	}{
		{
			description: "user login",
			codeName:    "UserLoginCode",
			typeValue:   "user.login",
			expected:    "user.login",
		},
		{
			description: "user login failure",
			codeName:    "UserLoginFailureCode",
			typeValue:   "user.login",
			expected:    "user.login (failure)",
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			actual := makeEventSectionName(c.codeName, c.typeValue)
			assert.Equal(t, c.expected, actual)
		})
	}
}

func Test_makeEventEntries(t *testing.T) {
	cases := []struct {
		description string
		codeDecls   io.Reader
		typeDecls   io.Reader
		schemas     map[string]*eventschema.Event
		expected    []EventEntry
	}{
		{
			description: "minimal happy path",
			codeDecls: strings.NewReader(`package main

const (
	// UserLoginCode is the successful local user login event code.
	UserLoginCode = "T1000I"
	// UserLoginFailureCode is the unsuccessful local user login event code.
	UserLoginFailureCode = "T1000W"
	Addr = "https://example.com"
)`),
			typeDecls: strings.NewReader(`package main

const (
       UserLoginEvent = "user.login"
)
`),
			schemas: map[string]*eventschema.Event{
				"UserLogin": &loginSchema,
			},
			expected: []EventEntry{
				{
					Name:   "user.login",
					Code:   "T1000I",
					Type:   "user.login",
					Schema: loginSchema,
				},
				{
					Name:   "user.login (failure)",
					Code:   "T1000W",
					Type:   "user.login",
					Schema: loginSchema,
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			actual, err := makeEventEntries(c.codeDecls, c.typeDecls, c.schemas)
			assert.NoError(t, err)
			assert.Equal(t, c.expected, actual)
		})
	}
}

func Test_makeReferenceTables(t *testing.T) {
	cases := []struct {
		description string
		input       []EventEntry
		expected    string
	}{
		{
			description: "two simple entries",
			input: []EventEntry{
				{
					Name:   "User Login",
					Code:   "T1000I",
					Type:   "user.login",
					Schema: loginSchema,
				},
				{
					Name:   "User Login Failure",
					Code:   "T1000W",
					Type:   "user.login",
					Schema: loginSchema,
				},
			},
			expected: `{/* 
AUTOMATICALLY GENERATED FILE. Edit at:
build.assets/tooling/cmd/gen-event-reference
*/}

### User Login

Records a user login event.

**Event:** user.login

**Code:** T1000I

|Field name|Type|Description|
|---|---|---|
|cluster_name|string|Identifies the originating Teleport cluster.|
|login|string|OS login.|

### User Login Failure

Records a user login event.

**Event:** user.login

**Code:** T1000W

|Field name|Type|Description|
|---|---|---|
|cluster_name|string|Identifies the originating Teleport cluster.|
|login|string|OS login.|
`,
		},
		{
			description: "entry with multiple levels",
			input: []EventEntry{
				{
					Name:   "Complex Entry",
					Code:   "99999",
					Type:   "complex.entry",
					Schema: multiLevelSchema,
				},
			},
			expected: `{/* 
AUTOMATICALLY GENERATED FILE. Edit at:
build.assets/tooling/cmd/gen-event-reference
*/}

### Complex Entry

Is an event with complex fields.

**Event:** complex.entry

**Code:** 99999

|Field name|Type|Description|
|---|---|---|
|cluster|object|Identifies the originating Teleport cluster.|
|cluster.name|string|Is the cluster name|
|cluster.address|object|Is the cluster address|
|cluster.address.host|string|Cluster address host.|
|cluster.address.port|number|Cluster address port.|
|login|string|OS login.|
`,
		},
	}
	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			var buf bytes.Buffer
			err := makeReferenceTables(&buf, c.input)
			assert.NoError(t, err)
			assert.Equal(t, c.expected, buf.String())
		})
	}
}

func TestEventEntryCollection_flatten(t *testing.T) {
	expected := []eventschema.EventField{
		{
			Name:        "cluster",
			Description: "Identifies the originating Teleport cluster.",
			Type:        "object",
			Fields:      []*eventschema.EventField{},
		},
		{
			Name:        "cluster.name",
			Description: "Is the cluster name",
			Type:        "string",
			Fields:      []*eventschema.EventField{},
		},
		{
			Name:        "cluster.address",
			Description: "Is the cluster address",
			Type:        "object",
			Fields:      []*eventschema.EventField{},
		},
		{
			Name:        "cluster.address.host",
			Description: "Cluster address host.",
			Type:        "string",
			Fields:      []*eventschema.EventField{},
		},
		{
			Name:        "cluster.address.port",
			Description: "Cluster address port.",
			Type:        "number",
			Fields:      []*eventschema.EventField{},
		},
		{
			Name:        "login",
			Description: "OS login.",
			Type:        "string",
			Fields:      []*eventschema.EventField{},
		},
	}

	var actual []eventschema.EventField
	e := flattenEvent(multiLevelSchema)
	for _, f := range e.Fields {
		actual = append(actual, *f)
	}
	assert.Equal(t, expected, actual)
}

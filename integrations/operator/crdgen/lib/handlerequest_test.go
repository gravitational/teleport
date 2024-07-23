package lib

import (
	"testing"

	"github.com/stretchr/testify/assert"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func Test_propertyTable(t *testing.T) {
	cases := []struct {
		description string
		input       apiextv1.JSONSchemaProps
		expected    []PropertyTable
	}{
		{
			description: "one scalar",
			input: apiextv1.JSONSchemaProps{
				Type:        "object",
				Description: "Attributes of the server",
				Properties: map[string]apiextv1.JSONSchemaProps{
					"domain": apiextv1.JSONSchemaProps{
						Type:        "string",
						Description: "The domain name of the server",
					},
					"port": apiextv1.JSONSchemaProps{
						Type:        "integer",
						Description: "The port on which the server listens",
					},
				},
			},
			expected: []PropertyTable{
				{
					Name: "",
					Fields: []PropertyTableField{
						{
							Name:        "domain",
							Type:        "string",
							Description: "The domain name of the server",
						},
						{
							Name:        "port",
							Type:        "integer",
							Description: "The port on which the server listens",
						},
					},
				},
			},
		},
		{
			description: "scalar and object",
			input: apiextv1.JSONSchemaProps{
				Type:        "object",
				Description: "Attributes of the server",
				Properties: map[string]apiextv1.JSONSchemaProps{
					"domain": apiextv1.JSONSchemaProps{
						Type:        "string",
						Description: "The domain name of the server",
					},
					"port_info": apiextv1.JSONSchemaProps{
						Type:        "object",
						Description: "Information about the port on which the server listens",
						Properties: map[string]apiextv1.JSONSchemaProps{
							"port": apiextv1.JSONSchemaProps{
								Type:        "integer",
								Description: "The port on which the server listens",
							},
							"protocol": apiextv1.JSONSchemaProps{
								Type:        "string",
								Description: "The protocol the server handles",
							},
						},
					},
				},
			},
			expected: []PropertyTable{
				{
					Name: "",
					Fields: []PropertyTableField{
						{
							Name:        "domain",
							Type:        "string",
							Description: "The domain name of the server",
						},
						{
							Name:        "port_info",
							Type:        "[object](#port_info)",
							Description: "Information about the port on which the server listens",
						},
					},
				},
				{
					Name: "`port_info`",
					Fields: []PropertyTableField{
						{
							Name:        "port",
							Type:        "integer",
							Description: "The port on which the server listens",
						},
						{
							Name:        "protocol",
							Type:        "string",
							Description: "The protocol the server handles",
						},
					},
				},
			},
		},
		{
			description: "scalar and array of scalars",
			input: apiextv1.JSONSchemaProps{
				Type:        "object",
				Description: "Attributes of the server",
				Properties: map[string]apiextv1.JSONSchemaProps{
					"domain": apiextv1.JSONSchemaProps{
						Type:        "string",
						Description: "The domain name of the server",
					},
					"ports": apiextv1.JSONSchemaProps{
						Type:        "array",
						Description: "Ports on which the server listens",
						Items: &apiextv1.JSONSchemaPropsOrArray{
							Schema: &apiextv1.JSONSchemaProps{
								Type: "integer",
							},
						},
					},
				},
			},
			expected: []PropertyTable{
				{
					Name: "",
					Fields: []PropertyTableField{
						{
							Name:        "domain",
							Type:        "string",
							Description: "The domain name of the server",
						},
						{
							Name:        "ports",
							Type:        "[]integer",
							Description: "Ports on which the server listens",
						},
					},
				},
			},
		},
		{
			description: "scalar and array of objects",
			input: apiextv1.JSONSchemaProps{
				Type:        "object",
				Description: "Attributes of the server",
				Properties: map[string]apiextv1.JSONSchemaProps{
					"domain": apiextv1.JSONSchemaProps{
						Type:        "string",
						Description: "The domain name of the server",
					},
					"ports": apiextv1.JSONSchemaProps{
						Type:        "array",
						Description: "Ports on which the server listens",
						Items: &apiextv1.JSONSchemaPropsOrArray{
							Schema: &apiextv1.JSONSchemaProps{
								Type: "object",
								Properties: map[string]apiextv1.JSONSchemaProps{
									"port": apiextv1.JSONSchemaProps{
										Type:        "integer",
										Description: "The port number",
									},
									"protocol": apiextv1.JSONSchemaProps{
										Type:        "string",
										Description: "The protocol the server listens for on the port",
									},
								},
							},
						},
					},
				},
			},
			expected: []PropertyTable{
				{
					Name: "",
					Fields: []PropertyTableField{
						{
							Name:        "domain",
							Type:        "string",
							Description: "The domain name of the server",
						},
						{
							Name:        "ports",
							Type:        "[][object](#ports-values)",
							Description: "Ports on which the server listens",
						},
					},
				},
				{
					Name: "`ports` values",
					Fields: []PropertyTableField{
						{
							Name:        "port",
							Type:        "integer",
							Description: "The port number",
						},
						{
							Name:        "protocol",
							Type:        "string",
							Description: "The protocol the server listens for on the port",
						},
					},
				},
			},
		},
		{
			description: "property with only object type",
			input: apiextv1.JSONSchemaProps{
				Type:        "object",
				Description: "Attributes of the server",
				Properties: map[string]apiextv1.JSONSchemaProps{
					"metadata": apiextv1.JSONSchemaProps{
						Type: "object",
					},
					"name": apiextv1.JSONSchemaProps{
						Type:        "string",
						Description: "The name of the server",
					},
				},
			},
			expected: []PropertyTable{
				{
					Name: "",
					Fields: []PropertyTableField{
						{
							Name:        "metadata",
							Type:        "object",
							Description: "",
						},
						{
							Name:        "name",
							Type:        "string",
							Description: "The name of the server",
						},
					},
				},
			},
		},
		{
			description: "scalar and status with conditions",
			input: apiextv1.JSONSchemaProps{
				Type:        "object",
				Description: "Attributes of the server",
				Properties: map[string]apiextv1.JSONSchemaProps{
					"name": apiextv1.JSONSchemaProps{
						Type:        "string",
						Description: "The name of the server",
					},
					"status": apiextv1.JSONSchemaProps{
						Description: "Status defines the observed state of the Teleport resource",
						Properties: map[string]apiextv1.JSONSchemaProps{
							"conditions": apiextv1.JSONSchemaProps{
								Type:        "array",
								Description: "Conditions represent the latest available observations of an object's state",
								Items: &apiextv1.JSONSchemaPropsOrArray{
									Schema: &apiextv1.JSONSchemaProps{
										Description: `"Condition contains details for one aspect of the current
state of this API Resource.\n---\nThis struct is intended for direct use as an array at the field path .status.conditions.`,
										Type: "object",
									},
								},
							},
						},
					},
				},
			},
			expected: []PropertyTable{
				{
					Name: "",
					Fields: []PropertyTableField{
						{
							Name:        "name",
							Type:        "string",
							Description: "The name of the server",
						},
					},
				},
			},
		},
		{
			description: "int or string enum",
			input: apiextv1.JSONSchemaProps{
				Type:        "object",
				Description: "Attributes of the server",
				Properties: map[string]apiextv1.JSONSchemaProps{
					"host_user_creation_mode": apiextv1.JSONSchemaProps{
						Type:         "",
						Description:  "Host user creation mode. 1 is \"drop\", 2 is \"keep\"",
						XIntOrString: true,
					},
				},
			},
			expected: []PropertyTable{
				{
					Name: "",
					Fields: []PropertyTableField{
						{
							Name:        "host_user_creation_mode",
							Type:        "string or integer",
							Description: "Host user creation mode. 1 is \"drop\", 2 is \"keep\". Can be either the string or the integer representation of each option.",
						},
					},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			actual, err := propertyTable("", &c.input)
			assert.NoError(t, err)
			assert.Equal(t, c.expected, actual)
		})
	}
}

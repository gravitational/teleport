// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package crdgen

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
					Name: "port_info",
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
							Type:        "[][object](#ports-items)",
							Description: "Ports on which the server listens",
						},
					},
				},
				{
					Name: "ports items",
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
			description: "property with only object type and no fields",
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
			description: "repeated property names",
			input: apiextv1.JSONSchemaProps{
				Type:        "object",
				Description: "Attributes of the server",
				Properties: map[string]apiextv1.JSONSchemaProps{
					"metadata": apiextv1.JSONSchemaProps{
						Type:        "object",
						Description: "The server metadata",
						Properties: map[string]apiextv1.JSONSchemaProps{
							"name": apiextv1.JSONSchemaProps{
								Type:        "string",
								Description: "The server name",
							},
						},
					},
					"datacenter": apiextv1.JSONSchemaProps{
						Type:        "object",
						Description: "Information about the data center",
						Properties: map[string]apiextv1.JSONSchemaProps{
							"metadata": apiextv1.JSONSchemaProps{
								Type:        "object",
								Description: "The datacenter metadata",
								Properties: map[string]apiextv1.JSONSchemaProps{
									"name": apiextv1.JSONSchemaProps{
										Type:        "string",
										Description: "The datacenter name",
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
							Name:        "datacenter",
							Type:        "[object](#datacenter)",
							Description: "Information about the data center",
						},
						{
							Name:        "metadata",
							Type:        "[object](#metadata)",
							Description: "The server metadata",
						},
					},
				},
				{
					Name: "metadata",
					Fields: []PropertyTableField{
						{
							Name:        "name",
							Type:        "string",
							Description: "The server name",
						},
					},
				},
				{
					Name: "datacenter",
					Fields: []PropertyTableField{
						{
							Name:        "metadata",
							Type:        "[object](#datacentermetadata)",
							Description: "The datacenter metadata",
						},
					},
				},
				{
					Name: "datacenter.metadata",
					Fields: []PropertyTableField{
						{
							Name:        "name",
							Type:        "string",
							Description: "The datacenter name",
						},
					},
				},
			},
		},
		{
			description: "repeated item property names",
			input: apiextv1.JSONSchemaProps{
				Type:        "object",
				Description: "Attributes of the server",
				Properties: map[string]apiextv1.JSONSchemaProps{
					"circleci": apiextv1.JSONSchemaProps{
						Type:        "object",
						Description: "Allows CircleCI-specific configuration",
						Properties: map[string]apiextv1.JSONSchemaProps{
							"allow": apiextv1.JSONSchemaProps{
								Type:        "array",
								Description: "What to allow",
								Items: &apiextv1.JSONSchemaPropsOrArray{
									Schema: &apiextv1.JSONSchemaProps{
										Type:        "object",
										Description: "Project information",
										Properties: map[string]apiextv1.JSONSchemaProps{
											"project_id": apiextv1.JSONSchemaProps{
												Type:        "string",
												Description: "The project ID",
											},
										},
									},
								},
							},
						},
					},
					"gitlab": apiextv1.JSONSchemaProps{
						Type:        "object",
						Description: "Allows GitLab-specific configuration",
						Properties: map[string]apiextv1.JSONSchemaProps{
							"allow": apiextv1.JSONSchemaProps{
								Type:        "array",
								Description: "What to allow",
								Items: &apiextv1.JSONSchemaPropsOrArray{
									Schema: &apiextv1.JSONSchemaProps{
										Type:        "object",
										Description: "Project information",
										Properties: map[string]apiextv1.JSONSchemaProps{
											"project_id": apiextv1.JSONSchemaProps{
												Type:        "string",
												Description: "The project ID",
											},
										},
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
							Name:        "circleci",
							Type:        "[object](#circleci)",
							Description: "Allows CircleCI-specific configuration",
						},
						{
							Name:        "gitlab",
							Type:        "[object](#gitlab)",
							Description: "Allows GitLab-specific configuration",
						},
					},
				},
				{
					Name: "circleci",
					Fields: []PropertyTableField{
						{
							Name:        "allow",
							Type:        "[][object](#circleciallow-items)",
							Description: "What to allow",
						},
					},
				},
				{
					Name: "circleci.allow items",
					Fields: []PropertyTableField{
						{
							Name:        "project_id",
							Type:        "string",
							Description: "The project ID",
						},
					},
				},
				{
					Name: "gitlab",
					Fields: []PropertyTableField{
						{
							Name:        "allow",
							Type:        "[][object](#gitlaballow-items)",
							Description: "What to allow",
						},
					},
				},
				{
					Name: "gitlab.allow items",
					Fields: []PropertyTableField{
						{
							Name:        "project_id",
							Type:        "string",
							Description: "The project ID",
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
		{
			description: "array of objects with object field",
			input: apiextv1.JSONSchemaProps{
				Type: "object",
				Properties: map[string]apiextv1.JSONSchemaProps{
					"mappings": apiextv1.JSONSchemaProps{
						Type:        "array",
						Description: "Mappings is a list of matches that will map match conditions to labels.",
						Items: &apiextv1.JSONSchemaPropsOrArray{
							Schema: &apiextv1.JSONSchemaProps{
								Type: "object",
								Properties: map[string]apiextv1.JSONSchemaProps{
									"add_labels": apiextv1.JSONSchemaProps{
										Type:        "object",
										Description: "AddLabels specifies which labels to add if any of the previous matches match.",
										Nullable:    true,
										Properties: map[string]apiextv1.JSONSchemaProps{
											"key": apiextv1.JSONSchemaProps{
												Type: "string",
											},
											"value": apiextv1.JSONSchemaProps{
												Type: "string",
											},
										},
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
							Name:        "mappings",
							Type:        "[][object](#mappings-items)",
							Description: "Mappings is a list of matches that will map match conditions to labels.",
						},
					},
				},
				{
					Name: "mappings items",
					Fields: []PropertyTableField{
						{
							Name:        "add_labels",
							Type:        "[object](#mappings-itemsadd_labels)",
							Description: "AddLabels specifies which labels to add if any of the previous matches match.",
						},
					},
				},
				{
					Name: "mappings items.add_labels",
					Fields: []PropertyTableField{
						{
							Name: "key",
							Type: "string",
						},
						{
							Name: "value",
							Type: "string",
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
			// Compare the unsorted slices. Sorting takes place
			// on the VersionSection that contains the
			// []PropertyTable.
			assert.ElementsMatch(t, c.expected, actual)
		})
	}
}

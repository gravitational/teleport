// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package eventschema

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Smoke test that checks if we can list all queryable events without error.
func TestQueryableEventList(t *testing.T) {
	list, err := QueryableEventList()
	require.NoError(t, err)
	require.NotEmpty(t, list)
}

// Large smoke test that checks if there are no errors when dumping schemas for
// all events.
func TestAllAthenaSchemas(t *testing.T) {
	for _, eventType := range eventTypes {
		schema, err := GetEventSchemaFromType(eventType)
		require.NoError(t, err)
		viewSchema, err := schema.ViewSchema()
		require.NoError(t, err)
		require.NotEmpty(t, viewSchema)
		tableSchema, err := schema.TableSchema()
		require.NoError(t, err)
		require.NotEmpty(t, tableSchema)
	}
}

// Checks that different kinds of event fields are rendered properly.
func TestEventField_Schemas(t *testing.T) {
	testDescription := "description"
	tests := []struct {
		name                string
		field               EventField
		fieldName           string
		expectedViewSchema  string
		expectedTableSchema string
	}{
		{
			name: "string",
			field: EventField{
				Type:        "string",
				Description: testDescription,
			},
			fieldName:           "foo",
			expectedViewSchema:  "  , CAST(json_extract(event_data, '$[\"foo\"]') AS varchar) as foo\n",
			expectedTableSchema: "foo, varchar, description\n",
		},
		{
			name: "field with a dot in its name",
			field: EventField{
				Type:        "string",
				Description: testDescription,
			},
			fieldName:           "foo.bar",
			expectedViewSchema:  "  , CAST(json_extract(event_data, '$[\"foo.bar\"]') AS varchar) as foo_bar\n",
			expectedTableSchema: "foo_bar, varchar, description\n",
		},
		{
			name: "integer",
			field: EventField{
				Type:        "integer",
				Description: testDescription,
			},
			fieldName:           "foo",
			expectedViewSchema:  "  , CAST(json_extract(event_data, '$[\"foo\"]') AS integer) as foo\n",
			expectedTableSchema: "foo, integer, description\n",
		},
		{
			name: "boolean",
			field: EventField{
				Type:        "boolean",
				Description: testDescription,
			},
			fieldName:           "foo",
			expectedViewSchema:  "  , CAST(json_extract(event_data, '$[\"foo\"]') AS boolean) as foo\n",
			expectedTableSchema: "foo, boolean, description\n",
		},
		{
			name: "1-level object",
			field: EventField{
				Type:        "object",
				Description: testDescription,
				Fields: []*EventField{
					{
						Name:        "bar",
						Type:        "string",
						Description: testDescription,
					},
				},
			},
			fieldName:           "foo",
			expectedViewSchema:  "  , CAST(json_extract(event_data, '$[\"foo\"][\"bar\"]') AS varchar) as foo_bar\n",
			expectedTableSchema: "foo_bar, varchar, description\n",
		},
		{
			name: "2-level object",
			field: EventField{
				Type:        "object",
				Description: testDescription,
				Fields: []*EventField{
					{
						Name:        "bar",
						Type:        "object",
						Description: testDescription,
						Fields: []*EventField{
							{
								Name:        "baz",
								Type:        "string",
								Description: testDescription,
							},
						},
					},
				},
			},
			fieldName:           "foo",
			expectedViewSchema:  "  , CAST(json_extract(event_data, '$[\"foo\"][\"bar\"][\"baz\"]') AS varchar) as foo_bar_baz\n",
			expectedTableSchema: "foo_bar_baz, varchar, description\n",
		},
		{
			name: "array(string)",
			field: EventField{
				Type:        "array",
				Description: testDescription,
				Items: &EventField{
					Type:        "string",
					Description: testDescription,
				},
			},
			fieldName:           "foo",
			expectedViewSchema:  "  , CAST(json_extract(event_data, '$[\"foo\"]') AS array(varchar)) as foo\n",
			expectedTableSchema: "foo, array(varchar), description\n",
		},
		{
			name: "array(object)",
			field: EventField{
				Type:        "array",
				Description: testDescription,
				Items: &EventField{
					Type:        "object",
					Description: testDescription,
					Fields: []*EventField{
						{
							Name:        "bar",
							Type:        "string",
							Description: testDescription,
						},
					},
				},
			},
			fieldName:           "foo",
			expectedViewSchema:  "  , CAST(json_extract(event_data, '$[\"foo\"]') AS array(row(bar varchar))) as foo\n",
			expectedTableSchema: "foo, array(row(bar varchar)), description\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := []string{tt.fieldName}
			viewSchema, err := tt.field.ViewSchema(path)
			require.NoError(t, err)
			require.Equal(t, tt.expectedViewSchema, viewSchema)
			tableSchema, err := tt.field.TableSchema(path)
			require.Equal(t, tt.expectedTableSchema, tableSchema)
		})
	}
}

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

func TestTableSchema(t *testing.T) {
	schema, err := GetEventSchemaFromType("session.start")
	require.NoError(t, err)
	tableSchema, err := schema.TableSchema()
	require.NoError(t, err)
	require.NotEmpty(t, tableSchema)
}

func TestTableList(t *testing.T) {
	list, err := TableList()
	require.NoError(t, err)
	require.NotEmpty(t, list)
}

func TestViewSchema(t *testing.T) {
	schema, err := GetEventSchemaFromType("session.start")
	require.NoError(t, err)
	viewSchema, err := schema.ViewSchema()
	require.NoError(t, err)
	require.NotEmpty(t, viewSchema)
}

func TestAllAthenaSchemas(t *testing.T) {
	for _, eventType := range eventTypes {
		schema, err := GetEventSchemaFromType(eventType)
		require.NoError(t, err)
		viewSchema, err := schema.ViewSchema()
		require.NoError(t, err)
		require.NotEmpty(t, viewSchema)
	}
}

func TestEvent_Schemas(t *testing.T) {
	testDescription := "description"
	prefixSchemaView := "SELECT\n  event_date, event_time\n"
	prefixTableView := "column_name, column_type, description\nevent_date, date, is the event date\nevent_time, timestamp, is the event time\n"
	tests := []struct {
		name                string
		fields              map[string]*EventField
		expectedViewSchema  string
		expectedTableSchema string
	}{
		{
			name: "string",
			fields: map[string]*EventField{
				"foo": {
					Type:        "string",
					Description: testDescription,
				},
			},
			expectedViewSchema:  "  , CAST(json_extract(event_data, '$[\"foo\"]') AS varchar) as foo\n",
			expectedTableSchema: "foo, varchar, description\n",
		},
		{
			name: "field with a dot in its name",
			fields: map[string]*EventField{
				"foo.bar": {
					Type:        "string",
					Description: testDescription,
				},
			},
			expectedViewSchema:  "  , CAST(json_extract(event_data, '$[\"foo.bar\"]') AS varchar) as foo_bar\n",
			expectedTableSchema: "foo_bar, varchar, description\n",
		},
		{
			name: "integer",
			fields: map[string]*EventField{
				"foo": {
					Type:        "integer",
					Description: testDescription,
				},
			},
			expectedViewSchema:  "  , CAST(json_extract(event_data, '$[\"foo\"]') AS integer) as foo\n",
			expectedTableSchema: "foo, integer, description\n",
		},
		{
			name: "boolean",
			fields: map[string]*EventField{
				"foo": {
					Type:        "boolean",
					Description: testDescription,
				},
			},
			expectedViewSchema:  "  , CAST(json_extract(event_data, '$[\"foo\"]') AS boolean) as foo\n",
			expectedTableSchema: "foo, boolean, description\n",
		},
		{
			name: "1-level object",
			fields: map[string]*EventField{
				"foo": {
					Type:        "object",
					Description: testDescription,
					Fields: map[string]*EventField{
						"bar": {
							Type:        "string",
							Description: testDescription,
						},
					},
				},
			},
			expectedViewSchema:  "  , CAST(json_extract(event_data, '$[\"foo\"][\"bar\"]') AS varchar) as foo_bar\n",
			expectedTableSchema: "foo_bar, varchar, description\n",
		},
		{
			name: "2-level object",
			fields: map[string]*EventField{
				"foo": {
					Type:        "object",
					Description: testDescription,
					Fields: map[string]*EventField{
						"bar": {
							Type:        "object",
							Description: testDescription,
							Fields: map[string]*EventField{
								"baz": {
									Type:        "string",
									Description: testDescription,
								},
							},
						},
					},
				},
			},
			expectedViewSchema:  "  , CAST(json_extract(event_data, '$[\"foo\"][\"bar\"][\"baz\"]') AS varchar) as foo_bar_baz\n",
			expectedTableSchema: "foo_bar_baz, varchar, description\n",
		},
		{
			name: "array(string)",
			fields: map[string]*EventField{
				"foo": {
					Type:        "array",
					Description: testDescription,
					Items: &EventField{
						Type:        "string",
						Description: testDescription,
					},
				},
			},
			expectedViewSchema:  "  , CAST(json_extract(event_data, '$[\"foo\"]') AS array(varchar)) as foo\n",
			expectedTableSchema: "foo, array(varchar), description\n",
		},
		{
			name: "array(object)",
			fields: map[string]*EventField{
				"foo": {
					Type:        "array",
					Description: testDescription,
					Items: &EventField{
						Type:        "object",
						Description: testDescription,
						Fields: map[string]*EventField{
							"bar": {
								Type:        "string",
								Description: testDescription,
							},
						},
					},
				},
			},
			expectedViewSchema:  "  , CAST(json_extract(event_data, '$[\"foo\"]') AS array(row(bar varchar))) as foo\n",
			expectedTableSchema: "foo, array(row(bar varchar)), description\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &Event{
				Description: testDescription,
				Fields:      tt.fields,
			}
			viewSchema, err := event.ViewSchema()
			require.NoError(t, err)
			require.Equal(t, prefixSchemaView+tt.expectedViewSchema, viewSchema)
			tableSchema, err := event.TableSchema()
			require.Equal(t, prefixTableView+tt.expectedTableSchema, tableSchema)
		})
	}
}

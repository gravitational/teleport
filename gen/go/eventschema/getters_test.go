/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package eventschema

import (
	"fmt"
	"strings"
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
	details, err := GetViewsDetails()
	require.NoError(t, err)
	require.Len(t, details, len(eventTypes))
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
		{
			name: "2-level object 2-elements",
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
							{
								Name:        "bar",
								Type:        "string",
								Description: testDescription,
							},
						},
					},
				},
			},
			fieldName: "foo",
			expectedViewSchema: strings.Join([]string{
				"  , CAST(json_extract(event_data, '$[\"foo\"][\"bar\"][\"baz\"]') AS varchar) as foo_bar_baz\n",
				"  , CAST(json_extract(event_data, '$[\"foo\"][\"bar\"][\"bar\"]') AS varchar) as foo_bar_bar\n",
			}, ""),
			expectedTableSchema: strings.Join([]string{
				"foo_bar_baz, varchar, description\n",
				"foo_bar_bar, varchar, description\n",
			}, ""),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := []string{tt.fieldName}
			fields, err := tt.field.TableSchemaDetails(path)
			var sb strings.Builder
			for _, v := range fields {
				sb.WriteString(v.viewSchema())
			}

			require.NoError(t, err)
			require.Equal(t, tt.expectedViewSchema, sb.String())
			sb.Reset()
			for _, v := range fields {
				tableSchema := fmt.Sprintf("%v, %v, %v\n", v.NameSQL(), v.Type, v.Description)
				sb.WriteString(tableSchema)
			}
			require.Equal(t, tt.expectedTableSchema, sb.String())
		})
	}
}

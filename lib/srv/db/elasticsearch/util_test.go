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

package elasticsearch

import (
	"log/slog"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/require"
)

func TestEngineGetQueryFromRequestBody(t *testing.T) {
	const jsonSearchAPIQuery = `{
  "query": {
    "bool" : {
      "must" : {
        "term" : { "user.id" : "pam" }
      },
      "filter": {
        "term" : { "tags" : "production" }
      }
    }
  }
}`

	const jsonSearchAPIJustQuery = `{"bool":{"filter":{"term":{"tags":"production"}},"must":{"term":{"user.id":"pam"}}}}`

	const jsonKNNSearchAPIQuery = `{
  "knn": {
    "field": "image_vector",
    "query_vector": [0.3, 0.1, 1.2],
    "k": 10,
    "num_candidates": 100
  },
  "_source": ["name", "file_type"]
}`

	const jsonKNNSearchAPIJustQuery = `{"field":"image_vector","k":10,"num_candidates":100,"query_vector":[0.3,0.1,1.2]}`

	const jsonSQLSearchAPIQuery = `{
  "query": "SELECT * FROM library ORDER BY page_count DESC LIMIT 5"
}`

	const jsonSQLSearchAPIJustQuery = `SELECT * FROM library ORDER BY page_count DESC LIMIT 5`

	toYAML := func(js string) string {
		yamlBytes, err := yaml.JSONToYAML([]byte(js))
		require.NoError(t, err)
		return string(yamlBytes)
	}

	tests := []struct {
		name        string
		contentType string
		body        string
		want        string
	}{
		{
			name:        "empty",
			contentType: "",
			body:        "",
			want:        "",
		},
		// json
		{
			name:        "json query search api",
			contentType: "application/json",
			body:        jsonSearchAPIQuery,
			want:        jsonSearchAPIJustQuery,
		},
		{
			name:        "json query knn",
			contentType: "application/json",
			body:        jsonKNNSearchAPIQuery,
			want:        jsonKNNSearchAPIJustQuery,
		},
		{
			name:        "json query sql",
			contentType: "application/json",
			body:        jsonSQLSearchAPIQuery,
			want:        jsonSQLSearchAPIJustQuery,
		},
		{
			name:        "json bad encoding",
			contentType: "application/json",
			body:        "",
			want:        "",
		},
		// yaml
		{
			name:        "yaml query search api",
			contentType: "application/yaml",
			body:        toYAML(jsonSearchAPIQuery),
			want:        jsonSearchAPIJustQuery,
		},
		{
			name:        "yaml query knn",
			contentType: "application/yaml",
			body:        toYAML(jsonKNNSearchAPIQuery),
			want:        jsonKNNSearchAPIJustQuery,
		},
		{
			name:        "yaml query sql",
			contentType: "application/yaml",
			body:        toYAML(jsonSQLSearchAPIQuery),
			want:        jsonSQLSearchAPIJustQuery,
		},
		{
			name:        "yaml bad encoding",
			contentType: "application/yaml",
			body:        "",
			want:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Engine{}
			e.Log = slog.Default()

			result := GetQueryFromRequestBody(e.EngineConfig, tt.contentType, []byte(tt.body))
			require.Equal(t, tt.want, result)
		})
	}
}

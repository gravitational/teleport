// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package elasticsearch

import (
	"testing"

	"github.com/ghodss/yaml"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/events"
)

func Test_parsePath(t *testing.T) {
	tests := []struct {
		// name string
		path         string
		wantTarget   string
		wantCategory events.ElasticsearchCategory
	}{
		{
			path:         "",
			wantTarget:   "",
			wantCategory: events.ElasticsearchCategory_ELASTICSEARCH_CATEGORY_GENERAL,
		},
		{
			path:         "/",
			wantTarget:   "",
			wantCategory: events.ElasticsearchCategory_ELASTICSEARCH_CATEGORY_GENERAL,
		},
		{
			path:         "/bah",
			wantTarget:   "",
			wantCategory: events.ElasticsearchCategory_ELASTICSEARCH_CATEGORY_GENERAL,
		},
		{
			path:         "/foo/bar/baz",
			wantTarget:   "",
			wantCategory: events.ElasticsearchCategory_ELASTICSEARCH_CATEGORY_GENERAL,
		},

		{
			path:         "/_security",
			wantTarget:   "",
			wantCategory: events.ElasticsearchCategory_ELASTICSEARCH_CATEGORY_SECURITY,
		},
		{
			path:         "/_security/foo",
			wantTarget:   "",
			wantCategory: events.ElasticsearchCategory_ELASTICSEARCH_CATEGORY_SECURITY,
		},

		{
			path:         "/_search",
			wantTarget:   "",
			wantCategory: events.ElasticsearchCategory_ELASTICSEARCH_CATEGORY_SEARCH,
		},
		{
			path:         "/_search/",
			wantTarget:   "",
			wantCategory: events.ElasticsearchCategory_ELASTICSEARCH_CATEGORY_SEARCH,
		},
		{
			path:         "/_search/asd",
			wantTarget:   "",
			wantCategory: events.ElasticsearchCategory_ELASTICSEARCH_CATEGORY_SEARCH,
		},
		{
			path:         "/blah/_search/asd",
			wantTarget:   "blah",
			wantCategory: events.ElasticsearchCategory_ELASTICSEARCH_CATEGORY_SEARCH,
		},

		{
			path:         "/_async_search/",
			wantTarget:   "",
			wantCategory: events.ElasticsearchCategory_ELASTICSEARCH_CATEGORY_SEARCH,
		},
		{
			path:         "/_async_search/asd",
			wantTarget:   "",
			wantCategory: events.ElasticsearchCategory_ELASTICSEARCH_CATEGORY_SEARCH,
		},
		{
			path:         "/blah/_async_search/asd",
			wantTarget:   "blah",
			wantCategory: events.ElasticsearchCategory_ELASTICSEARCH_CATEGORY_SEARCH,
		},

		{
			path:         "/_sql/",
			wantTarget:   "",
			wantCategory: events.ElasticsearchCategory_ELASTICSEARCH_CATEGORY_SQL,
		},
		{
			path:         "/_sql",
			wantTarget:   "",
			wantCategory: events.ElasticsearchCategory_ELASTICSEARCH_CATEGORY_SQL,
		},
		{
			path:         "/_sql/asd",
			wantTarget:   "",
			wantCategory: events.ElasticsearchCategory_ELASTICSEARCH_CATEGORY_SQL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			target, category := parsePath(tt.path)
			require.Equal(t, tt.wantTarget, target)
			require.Equal(t, tt.wantCategory.String(), category.String())
		})
	}
}

func TestEngine_getQueryFromRequestBody(t *testing.T) {
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
			e.Log = logrus.StandardLogger()

			result := e.getQueryFromRequestBody(tt.contentType, []byte(tt.body))
			t.Log(result)
			require.Equal(t, tt.want, result)
		})
	}
}

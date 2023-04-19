// Copyright 2023 Gravitational, Inc
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
	"strings"

	apievents "github.com/gravitational/teleport/api/types/events"
)

// parsePath returns (optional) target of query as well as the event category.
func parsePath(path string) (string, apievents.ElasticsearchCategory) {
	parts := strings.Split(path, "/")

	if len(parts) < 2 {
		return "", apievents.ElasticsearchCategory_ELASTICSEARCH_CATEGORY_GENERAL
	}

	// first term starts with _
	switch parts[1] {
	case "_security", "_ssl":
		return "", apievents.ElasticsearchCategory_ELASTICSEARCH_CATEGORY_SECURITY
	case
		"_search",       // https://www.elastic.co/guide/en/elasticsearch/reference/master/search-search.html
		"_async_search", // https://www.elastic.co/guide/en/elasticsearch/reference/master/async-search.html
		"_pit",          // https://www.elastic.co/guide/en/elasticsearch/reference/master/point-in-time-api.html
		"_msearch",      // https://www.elastic.co/guide/en/elasticsearch/reference/master/multi-search-template.html, https://www.elastic.co/guide/en/elasticsearch/reference/master/search-multi-search.html
		"_render",       // https://www.elastic.co/guide/en/elasticsearch/reference/master/render-search-template-api.html
		"_field_caps":   // https://www.elastic.co/guide/en/elasticsearch/reference/master/search-field-caps.html
		return "", apievents.ElasticsearchCategory_ELASTICSEARCH_CATEGORY_SEARCH
	case "_sql":
		return "", apievents.ElasticsearchCategory_ELASTICSEARCH_CATEGORY_SQL
	}

	// starts with _, but we don't handle it explicitly
	if strings.HasPrefix("_", parts[1]) {
		return "", apievents.ElasticsearchCategory_ELASTICSEARCH_CATEGORY_GENERAL
	}

	if len(parts) < 3 {
		return "", apievents.ElasticsearchCategory_ELASTICSEARCH_CATEGORY_GENERAL
	}

	// a number of APIs are invoked by providing a target first, e.g. /<target>/_search, where <target> is an index or expression matching a group of indices.
	switch parts[2] {
	case
		"_search",        // https://www.elastic.co/guide/en/elasticsearch/reference/master/search-search.html
		"_async_search",  // https://www.elastic.co/guide/en/elasticsearch/reference/master/async-search.html
		"_pit",           // https://www.elastic.co/guide/en/elasticsearch/reference/master/point-in-time-api.html
		"_knn_search",    // https://www.elastic.co/guide/en/elasticsearch/reference/master/knn-search-api.html
		"_msearch",       // https://www.elastic.co/guide/en/elasticsearch/reference/master/multi-search-template.html, https://www.elastic.co/guide/en/elasticsearch/reference/master/search-multi-search.html
		"_search_shards", // https://www.elastic.co/guide/en/elasticsearch/reference/master/search-shards.html
		"_count",         // https://www.elastic.co/guide/en/elasticsearch/reference/master/search-count.html
		"_validate",      // https://www.elastic.co/guide/en/elasticsearch/reference/master/search-validate.html
		"_terms_enum",    // https://www.elastic.co/guide/en/elasticsearch/reference/master/search-terms-enum.html
		"_explain",       // https://www.elastic.co/guide/en/elasticsearch/reference/master/search-explain.html
		"_field_caps",    // https://www.elastic.co/guide/en/elasticsearch/reference/master/search-field-caps.html
		"_rank_eval",     // https://www.elastic.co/guide/en/elasticsearch/reference/master/search-rank-eval.html
		"_mvt":           // https://www.elastic.co/guide/en/elasticsearch/reference/master/search-vector-tile-api.html
		return parts[1], apievents.ElasticsearchCategory_ELASTICSEARCH_CATEGORY_SEARCH
	}

	return "", apievents.ElasticsearchCategory_ELASTICSEARCH_CATEGORY_GENERAL
}

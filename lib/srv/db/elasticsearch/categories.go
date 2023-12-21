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

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

package opensearch

import (
	"strings"

	apievents "github.com/gravitational/teleport/api/types/events"
)

// parsePath returns (optional) target of query as well as the event category.
func parsePath(path string) (string, apievents.OpenSearchCategory) {
	parts := strings.Split(path, "/")

	// empty string or lone /
	if len(parts) < 2 {
		return "", apievents.OpenSearchCategory_OPEN_SEARCH_CATEGORY_GENERAL
	}

	// underscore
	if strings.HasPrefix(parts[1], "_") {
		switch parts[1] {
		case
			// search
			"_search",  // https://opensearch.org/docs/latest/api-reference/search/
			"_count",   // https://opensearch.org/docs/latest/api-reference/count/
			"_msearch", // https://opensearch.org/docs/2.6/api-reference/multi-search/
			"_render",  // https://opensearch.org/docs/2.6/search-plugins/search-template/
			"_validate",
			"_field_caps",
			"_rank_eval",
			"_search_shards":
			return "", apievents.OpenSearchCategory_OPEN_SEARCH_CATEGORY_SEARCH

		case "_plugins":
			// handle plugins

			// length check
			if len(parts) < 3 {
				return "", apievents.OpenSearchCategory_OPEN_SEARCH_CATEGORY_GENERAL
			}

			switch parts[2] {
			case
				"_sql", // https://opensearch.org/docs/2.6/search-plugins/sql/sql-ppl-api/
				"_ppl": // https://opensearch.org/docs/2.6/search-plugins/sql/sql-ppl-api/
				return "", apievents.OpenSearchCategory_OPEN_SEARCH_CATEGORY_SQL

			case
				"_asynchronous_search", // https://opensearch.org/docs/2.6/search-plugins/async/index/
				"_knn":                 // https://opensearch.org/docs/2.6/search-plugins/knn/api/
				return "", apievents.OpenSearchCategory_OPEN_SEARCH_CATEGORY_SEARCH

			case "_security": // https://opensearch.org/docs/2.6/security/index/
				return "", apievents.OpenSearchCategory_OPEN_SEARCH_CATEGORY_SECURITY
			}

		case "_all":
			// fall through

		default:
			// starts with _, but we don't have logic to handle it in a special way.
			return "", apievents.OpenSearchCategory_OPEN_SEARCH_CATEGORY_GENERAL
		}
	}

	// length check
	if len(parts) < 3 {
		return "", apievents.OpenSearchCategory_OPEN_SEARCH_CATEGORY_GENERAL
	}

	// a number of APIs are invoked by providing a target first, e.g. /<target>/_search, where <target> is an index or expression matching a group of indices.
	switch parts[2] {
	case
		// search variants
		"_search",    // https://opensearch.org/docs/2.6/api-reference/search/
		"_count",     // https://opensearch.org/docs/2.6/api-reference/count/
		"_msearch",   // https://opensearch.org/docs/2.6/api-reference/multi-search/
		"_explain",   // https://opensearch.org/docs/2.6/api-reference/explain/
		"_rank_eval", // https://opensearch.org/docs/2.6/api-reference/rank-eval/
		"_search_shards",
		"_validate",
		"_field_caps",
		"_terms_enum":
		return parts[1], apievents.OpenSearchCategory_OPEN_SEARCH_CATEGORY_SEARCH
	}

	// no special handling, general case.
	return "", apievents.OpenSearchCategory_OPEN_SEARCH_CATEGORY_GENERAL
}

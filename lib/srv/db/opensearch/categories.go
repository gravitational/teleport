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

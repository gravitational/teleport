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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/events"
)

func Test_parsePath(t *testing.T) {
	tests := []struct {
		// name string
		path         string
		wantTarget   string
		wantCategory events.OpenSearchCategory
	}{
		{
			path:         "",
			wantTarget:   "",
			wantCategory: events.OpenSearchCategory_OPEN_SEARCH_CATEGORY_GENERAL,
		},
		{
			path:         "/",
			wantTarget:   "",
			wantCategory: events.OpenSearchCategory_OPEN_SEARCH_CATEGORY_GENERAL,
		},
		{
			path:         "/bah",
			wantTarget:   "",
			wantCategory: events.OpenSearchCategory_OPEN_SEARCH_CATEGORY_GENERAL,
		},
		{
			path:         "/foo/bar/baz",
			wantTarget:   "",
			wantCategory: events.OpenSearchCategory_OPEN_SEARCH_CATEGORY_GENERAL,
		},

		{
			path:         "/_plugins/_security",
			wantTarget:   "",
			wantCategory: events.OpenSearchCategory_OPEN_SEARCH_CATEGORY_SECURITY,
		},
		{
			path:         "/_plugins/_security/foo",
			wantTarget:   "",
			wantCategory: events.OpenSearchCategory_OPEN_SEARCH_CATEGORY_SECURITY,
		},

		{
			path:         "/_search",
			wantTarget:   "",
			wantCategory: events.OpenSearchCategory_OPEN_SEARCH_CATEGORY_SEARCH,
		},
		{
			path:         "/_search/",
			wantTarget:   "",
			wantCategory: events.OpenSearchCategory_OPEN_SEARCH_CATEGORY_SEARCH,
		},
		{
			path:         "/_search/asd",
			wantTarget:   "",
			wantCategory: events.OpenSearchCategory_OPEN_SEARCH_CATEGORY_SEARCH,
		},
		{
			path:         "/blah/_search/asd",
			wantTarget:   "blah",
			wantCategory: events.OpenSearchCategory_OPEN_SEARCH_CATEGORY_SEARCH,
		},
		{
			path:         "/_all/_search/asd",
			wantTarget:   "_all",
			wantCategory: events.OpenSearchCategory_OPEN_SEARCH_CATEGORY_SEARCH,
		},

		{
			path:         "/_plugins/_asynchronous_search/",
			wantTarget:   "",
			wantCategory: events.OpenSearchCategory_OPEN_SEARCH_CATEGORY_SEARCH,
		},
		{
			path:         "/_plugins/_knn/",
			wantTarget:   "",
			wantCategory: events.OpenSearchCategory_OPEN_SEARCH_CATEGORY_SEARCH,
		},

		{
			path:         "/_plugins/_sql/",
			wantTarget:   "",
			wantCategory: events.OpenSearchCategory_OPEN_SEARCH_CATEGORY_SQL,
		},
		{
			path:         "/_plugins/_ppl",
			wantTarget:   "",
			wantCategory: events.OpenSearchCategory_OPEN_SEARCH_CATEGORY_SQL,
		},

		{
			path:         "/_plugins",
			wantTarget:   "",
			wantCategory: events.OpenSearchCategory_OPEN_SEARCH_CATEGORY_GENERAL,
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

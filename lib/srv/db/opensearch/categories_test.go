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

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
)

func FuzzPathToMatcher(f *testing.F) {
	f.Add("")
	f.Add("a/b/c")

	f.Add("/_security/foo")
	f.Add("/_ssl/asd")
	f.Add("/_search/")
	f.Add("/_async_search/")
	f.Add("/_pit/")
	f.Add("/_msearch/")
	f.Add("/_render/")
	f.Add("/_field_caps/")
	f.Add("/_sql/")
	f.Add("/_eql/")

	f.Add("/target/_search")
	f.Add("/target/_async_search")
	f.Add("/target/_pit")
	f.Add("/target/_knn_search")
	f.Add("/target/_msearch")
	f.Add("/target/_search_shards")
	f.Add("/target/_count")
	f.Add("/target/_validate")
	f.Add("/target/_terms_enum")
	f.Add("/target/_explain")
	f.Add("/target/_field_caps")
	f.Add("/target/_rank_eval")
	f.Add("/target/_mvt")

	f.Fuzz(func(t *testing.T, path string) {
		require.NotPanics(t, func() {
			parsePath(path)
		})
	})
}

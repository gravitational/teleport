// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package cache

import (
	"testing"
	"time"

	"github.com/gravitational/teleport/api/types"
)

func TestWebTokens(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForProxy)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[types.WebToken]{
		newResource: func(name string) (types.WebToken, error) {
			return types.NewWebToken(time.Now().Add(time.Hour), types.WebTokenSpecV3{
				Token: "test",
				User:  "llama",
			})
		},
		create:    p.webTokenS.Upsert,
		list:      p.webTokenS.List,
		cacheList: p.cache.GetWebTokens,
		deleteAll: p.webTokenS.DeleteAll,
	})
}

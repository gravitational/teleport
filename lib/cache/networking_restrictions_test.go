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
	"testing/synctest"

	"github.com/gravitational/teleport/api/types"
)

func TestNetworkRestrictions(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := newTestPack(t, ForAuth)
		t.Cleanup(p.Close)
		testLegacySingleton(t, p, testLegacySingletonFuncs[types.NetworkRestrictions]{
			newResource: types.NewNetworkRestrictions,
			create:      p.restrictions.SetNetworkRestrictions,
			update:      p.restrictions.SetNetworkRestrictions,
			get:         p.restrictions.GetNetworkRestrictions,
			cacheGet:    p.cache.GetNetworkRestrictions,
			delete:      p.restrictions.DeleteNetworkRestrictions,
		})
	})
}

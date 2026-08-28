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

	autoupdatev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
)

// TestAutoUpdateConfig tests that CRUD operations on AutoUpdateConfig resources are
// replicated from the backend to the cache.
func TestAutoUpdateConfig(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := newTestPack(t, ForAuth)
		t.Cleanup(p.Close)

		testSingleton153(t, p, testSingletonFuncs153[*autoupdatev1.AutoUpdateConfig]{
			newResource: func() *autoupdatev1.AutoUpdateConfig {
				return newAutoUpdateConfig(t)
			},
			create:   p.autoUpdateService.CreateAutoUpdateConfig,
			update:   p.autoUpdateService.UpdateAutoUpdateConfig,
			get:      p.autoUpdateService.GetAutoUpdateConfig,
			cacheGet: p.cache.GetAutoUpdateConfig,
			delete:   p.autoUpdateService.DeleteAutoUpdateConfig,
		})
	})
}

// TestAutoUpdateVersion tests that CRUD operations on AutoUpdateVersion resource are
// replicated from the backend to the cache.
func TestAutoUpdateVersion(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := newTestPack(t, ForAuth)
		t.Cleanup(p.Close)

		testSingleton153(t, p, testSingletonFuncs153[*autoupdatev1.AutoUpdateVersion]{
			newResource: func() *autoupdatev1.AutoUpdateVersion {
				return newAutoUpdateVersion(t)
			},
			create:   p.autoUpdateService.CreateAutoUpdateVersion,
			update:   p.autoUpdateService.UpdateAutoUpdateVersion,
			get:      p.autoUpdateService.GetAutoUpdateVersion,
			cacheGet: p.cache.GetAutoUpdateVersion,
			delete:   p.autoUpdateService.DeleteAutoUpdateVersion,
		})
	})
}

// TestAutoUpdateAgentRollout tests that CRUD operations on AutoUpdateAgentRollout resource are
// replicated from the backend to the cache.
func TestAutoUpdateAgentRollout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := newTestPack(t, ForAuth)
		t.Cleanup(p.Close)

		testSingleton153(t, p, testSingletonFuncs153[*autoupdatev1.AutoUpdateAgentRollout]{
			newResource: func() *autoupdatev1.AutoUpdateAgentRollout {
				return newAutoUpdateAgentRollout(t)
			},
			create:   p.autoUpdateService.CreateAutoUpdateAgentRollout,
			update:   p.autoUpdateService.UpdateAutoUpdateAgentRollout,
			get:      p.autoUpdateService.GetAutoUpdateAgentRollout,
			cacheGet: p.cache.GetAutoUpdateAgentRollout,
			delete:   p.autoUpdateService.DeleteAutoUpdateAgentRollout,
		})
	})
}

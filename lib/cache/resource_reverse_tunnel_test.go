// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

// TestReverseTunnels tests reverse tunnels caching
func TestReverseTunnels(t *testing.T) {
	t.Parallel()

	p, err := newPack(t, ForProxy)
	require.NoError(t, err)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[types.ReverseTunnel]{
		newResource: func(name string) (types.ReverseTunnel, error) {
			return types.NewReverseTunnel(name, []string{"example.com:2023"})
		},
		create:    p.presenceS.UpsertReverseTunnel,
		list:      p.presenceS.ListReverseTunnels,
		cacheList: p.cache.ListReverseTunnels,
		update:    p.presenceS.UpsertReverseTunnel,
		deleteAll: p.presenceS.DeleteAllReverseTunnels,
	})
}

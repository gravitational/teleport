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

package lockmap_test

import (
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/lockmap"
)

func TestLockMap(t *testing.T) {
	expectMap := map[int]int{
		0: 100,
		1: 101,
		2: 102,
		3: 103,
		4: 104,
	}

	// storing results in an array to avoid errors with unguarded concurrent
	// map access
	resultArr := [5]int{}

	var lm lockmap.LockMap[int]

	// spin up a bunch of goroutines to concurrently increment up to the
	// expected values
	synctest.Test(t, func(t *testing.T) {
		for key, expect := range expectMap {
			for range expect {
				go func(idx int) {
					lm.Lock(key)
					resultArr[key]++
					lm.Unlock(key)
				}(key)
			}
		}

		synctest.Wait()
	})

	// make sure final counts match
	for key, expect := range expectMap {
		assert.Equal(t, expect, resultArr[key])
	}

	// make sure the lock map correctly drops all entries
	require.Zero(t, lm.Len())
}

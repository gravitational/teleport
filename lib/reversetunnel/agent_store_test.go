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

package reversetunnel

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAgentStoreRace(t *testing.T) {
	store := newAgentStore()
	agents := []*agent{{}, {}, {}, {}, {}}

	wg := &sync.WaitGroup{}
	for i := range agents {
		wg.Add(1)
		go func(i int) {
			store.add(agents[i])
			wg.Done()
		}(i)
	}

	wg.Wait()

	wg = &sync.WaitGroup{}
	for i := range agents {
		wg.Add(1)
		go func(i int) {
			ok := store.remove(agents[i])
			require.True(t, ok)
			wg.Done()
		}(i)
	}

	wg.Wait()
}

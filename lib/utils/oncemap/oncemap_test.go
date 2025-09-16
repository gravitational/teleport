/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package oncemap

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestOnceMap verifies the basic expected function of OnceMap.
func TestOnceMap(t *testing.T) {

	var callsMu sync.Mutex
	calls := make(map[int]int)

	m := New(func(key int) (string, error) {
		callsMu.Lock()
		defer callsMu.Unlock()
		calls[key]++
		return fmt.Sprintf("value-%d", key), nil
	})

	results := make(chan struct {
		key int
		val string
		err error
	})
	for i := 0; i < 16; i++ {
		key := i % 2
		go func() {
			for j := 0; j < 16; j++ {
				val, err := m.Get(context.Background(), key)
				results <- struct {
					key int
					val string
					err error
				}{key, val, err}
			}
		}()
	}

	for i := 0; i < 256; i++ {
		res := <-results
		require.NoError(t, res.err)
		require.Equal(t, fmt.Sprintf("value-%d", res.key), res.val)
	}

	callsMu.Lock()
	require.Equal(t, map[int]int{0: 1, 1: 1}, calls)
	callsMu.Unlock()
}

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

package utils

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRoundRobinConcurrent(t *testing.T) {
	t.Parallel()

	const workers = 100
	const rounds = 100

	rr := NewRoundRobin([]bool{true, false})

	var tct atomic.Uint64
	var fct atomic.Uint64

	var wg sync.WaitGroup

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range rounds {
				if rr.Next() {
					tct.Add(1)
				} else {
					fct.Add(1)
				}
			}
		}()
	}

	wg.Wait()

	require.Equal(t, workers*rounds, int(tct.Load()+fct.Load()))
	require.InDelta(t, tct.Load(), fct.Load(), 1.0)
}

func TestRoundRobinSequential(t *testing.T) {
	t.Parallel()
	tts := []struct {
		desc   string
		items  []string
		expect []string
	}{
		{
			desc:  "single-item",
			items: []string{"foo"},
			expect: []string{
				"foo",
				"foo",
				"foo",
			},
		},
		{
			desc: "multi-item",
			items: []string{
				"foo",
				"bar",
				"bin",
				"baz",
			},
			expect: []string{
				"foo",
				"bar",
				"bin",
				"baz",
				"foo",
				"bar",
				"bin",
				"baz",
				"foo",
				"bar",
				"bin",
				"baz",
			},
		},
	}
	for _, tt := range tts {
		t.Run(tt.desc, func(t *testing.T) {
			rr := NewRoundRobin(tt.items)
			for _, exp := range tt.expect {
				require.Equal(t, exp, rr.Next())
			}
		})
	}
}

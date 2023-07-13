/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package etcdbk

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

	rr := newRoundRobin([]bool{true, false})

	var tct atomic.Uint64
	var fct atomic.Uint64

	var wg sync.WaitGroup

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for r := 0; r < rounds; r++ {
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
			rr := newRoundRobin(tt.items)
			for _, exp := range tt.expect {
				require.Equal(t, exp, rr.Next())
			}
		})
	}
}

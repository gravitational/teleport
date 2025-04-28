/*
Copyright 2021-2022 Gravitational, Inc.

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

package retryutils

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewJitter(t *testing.T) {
	t.Parallel()

	const baseDuration time.Duration = time.Microsecond
	for _, tc := range []struct {
		desc          string
		jitter        Jitter
		expectFloor   time.Duration
		expectCeiling time.Duration
	}{
		{
			desc:          "FullJitter",
			jitter:        FullJitter,
			expectFloor:   0,
			expectCeiling: baseDuration - 1,
		},
		{
			desc:          "HalfJitter",
			jitter:        HalfJitter,
			expectFloor:   baseDuration / 2,
			expectCeiling: baseDuration - 1,
		},
		{
			desc:          "SeventhJitter",
			jitter:        SeventhJitter,
			expectFloor:   baseDuration - baseDuration/7,
			expectCeiling: baseDuration - 1,
		},
		{
			desc:          "AdditiveSeventhJitter",
			jitter:        AdditiveSeventhJitter,
			expectFloor:   baseDuration,
			expectCeiling: baseDuration + baseDuration/7 - 1,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			var gotFloor, gotCeiling bool
			for !gotFloor || !gotCeiling {
				d := tc.jitter(baseDuration)
				require.GreaterOrEqual(t, d, tc.expectFloor)
				if d == tc.expectFloor {
					gotFloor = true
				}
				require.LessOrEqual(t, d, tc.expectCeiling)
				if d == tc.expectCeiling {
					gotCeiling = true
				}
			}
		})
	}
}

func mutexedSeventhJitter() Jitter {
	var mu sync.Mutex
	return func(d time.Duration) time.Duration {
		mu.Lock()
		defer mu.Unlock()
		return SeventhJitter(d)
	}
}

func shardedSeventhJitter() Jitter {
	const shards = 64

	var jitters [shards]Jitter
	for i := range jitters {
		jitters[i] = mutexedSeventhJitter()
	}
	var ctr atomic.Uint64

	return func(d time.Duration) time.Duration {
		return jitters[ctr.Add(1)%shards](d)
	}
}

func BenchmarkJitter(b *testing.B) {
	impls := map[string]Jitter{
		"old_global":  mutexedSeventhJitter(),
		"old_sharded": shardedSeventhJitter(),
		"new":         SeventhJitter,
	}
	for impl, jitter := range impls {
		b.Run("impl="+impl, func(b *testing.B) {
			for parShift := range 6 {
				par := 1 << (parShift * 4)
				b.Run(fmt.Sprintf("par=%d", par), func(b *testing.B) {
					var wg sync.WaitGroup
					wg.Add(par)
					for range par {
						go func() {
							defer wg.Done()
							for range b.N {
								jitter(time.Hour)
							}
						}()
					}
					wg.Wait()
				})
			}
		})
	}
}

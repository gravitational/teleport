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

package interval

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestVariableDurationScaling(t *testing.T) {
	type cd struct {
		count    int64
		duration time.Duration
	}
	tts := []struct {
		desc   string
		vd     *VariableDuration
		expect []cd
	}{
		{
			desc: "standard presence params",
			vd: &VariableDuration{
				MinDuration: 90 * time.Second,
				MaxDuration: 9 * time.Minute,
				Step:        2048,
			},
			expect: []cd{
				{0, 90 * time.Second},
				{1_000, 90 * time.Second},
				{5_000, 2 * time.Minute},
				{10_000, 3 * time.Minute},
				{20_000, 4 * time.Minute},
				{40_000, 6 * time.Minute},
				{80_000, 9 * time.Minute},
				{160_000, 9 * time.Minute},
			},
		},
		{
			desc: "inventory presence params",
			vd: &VariableDuration{
				MinDuration: 3 * time.Minute,
				MaxDuration: 18 * time.Minute,
				Step:        1024,
			},
			expect: []cd{
				{0, 3 * time.Minute},
				{1_000, 3 * time.Minute},
				{3_000, 5 * time.Minute},
				{6_000, 7 * time.Minute},
				{12_000, 10 * time.Minute},
				{24_000, 14 * time.Minute},
				{48_000, 18 * time.Minute},
				{96_000, 18 * time.Minute},
			},
		},
	}

	for _, tt := range tts {
		for _, cd := range tt.expect {
			tt.vd.Counter.Store(cd.count)
			d := tt.vd.Duration()
			// note that the delta we're using here is pretty big. we don't care about the specific durations created, which are fractional, we
			// just use this test to verify that the duration scales at roughly the rate we intend.
			require.InDelta(t, cd.duration, d, float64(time.Second*45), "count=%d, expected_duration=%s, actual_duration=%s", cd.count, cd.duration, d)
		}
	}
}

func TestVariableDurationIncDec(t *testing.T) {
	vd := &VariableDuration{}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			vd.Inc()
		}()
	}

	wg.Wait()
	require.Equal(t, int64(100), vd.Counter.Load())

	wg = sync.WaitGroup{}

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			vd.Dec()
		}()
	}

	wg.Wait()
	require.Equal(t, int64(50), vd.Counter.Load())

	wg = sync.WaitGroup{}

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			vd.Dec()
		}()
	}

	wg.Wait()
	require.Equal(t, int64(0), vd.Counter.Load())
}

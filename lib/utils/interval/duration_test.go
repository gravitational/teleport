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
		cfg    VariableDurationConfig
		expect []cd
	}{
		{
			desc: "standard presence params",
			cfg: VariableDurationConfig{
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
			cfg: VariableDurationConfig{
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
		t.Run(tt.desc, func(t *testing.T) {
			vd := NewVariableDuration(tt.cfg)
			for _, cd := range tt.expect {
				vd.counter.Store(cd.count)
				d := vd.Duration()
				// note that the delta we're using here is pretty big. we don't care about the specific durations created, which are fractional, we
				// just use this test to verify that the duration scales at roughly the rate we intend.
				require.InDelta(t, cd.duration, d, float64(time.Second*45), "count=%d, expected_duration=%s, actual_duration=%s", cd.count, cd.duration, d)
			}
		})
	}
}

func TestVariableDurationIncDec(t *testing.T) {
	vd := NewVariableDuration(VariableDurationConfig{})

	var wg sync.WaitGroup
	start := make(chan struct{})
	wg.Add(100)
	for i := range 100 {
		go func() {
			defer wg.Done()
			<-start
			if i%2 == 0 {
				vd.Inc()
			} else {
				vd.Add(i)
			}
		}()
	}

	close(start)
	wg.Wait()
	require.Equal(t, int64(50+50*49+50), vd.Count())

	start = make(chan struct{})
	wg.Add(50)
	for i := range 50 {
		go func() {
			defer wg.Done()
			<-start
			if i%2 == 0 {
				vd.Dec()
			} else {
				vd.Add(-i)
			}
		}()
	}

	close(start)
	wg.Wait()
	require.Equal(t, int64(50+50*49+50-(25+25*24+25)), vd.Count())

	start = make(chan struct{})
	wg.Add(50)
	for i := 50; i < 100; i++ {
		go func() {
			defer wg.Done()
			<-start
			if i%2 == 0 {
				vd.Dec()
			} else {
				vd.Add(-i)
			}
		}()
	}

	close(start)
	wg.Wait()
	require.Equal(t, int64(0), vd.Count())
}

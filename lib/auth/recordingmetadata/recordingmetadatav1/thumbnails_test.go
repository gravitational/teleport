/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

package recordingmetadatav1

import (
	"testing"
	"time"

	"github.com/hinshun/vt10x"
	"github.com/stretchr/testify/require"
)

func TestThumbnailBucketSampler_ShouldCapture(t *testing.T) {
	sampler := newThumbnailBucketSampler(10, time.Second)
	baseTime := time.Now()

	require.True(t, sampler.shouldCapture(baseTime))

	sampler.add(&vt10x.TerminalState{}, baseTime)

	require.False(t, sampler.shouldCapture(baseTime.Add(500*time.Millisecond)))
	require.False(t, sampler.shouldCapture(baseTime.Add(999*time.Millisecond)))
	require.True(t, sampler.shouldCapture(baseTime.Add(time.Second)))
	require.True(t, sampler.shouldCapture(baseTime.Add(2*time.Second)))
}

func TestThumbnailBucketSampler_BasicAdd(t *testing.T) {
	sampler := newThumbnailBucketSampler(10, time.Second)
	baseTime := time.Now()

	sampler.add(&vt10x.TerminalState{}, baseTime)
	sampler.add(&vt10x.TerminalState{}, baseTime.Add(time.Second))
	sampler.add(&vt10x.TerminalState{}, baseTime.Add(2*time.Second))

	result := sampler.result()
	require.Len(t, result, 3)

	require.Equal(t, time.Duration(0), result[0].startOffset)
	require.Equal(t, 999*time.Millisecond, result[0].endOffset)
	require.Equal(t, baseTime, result[0].timestamp)

	require.Equal(t, time.Second, result[1].startOffset)
	require.Equal(t, 1999*time.Millisecond, result[1].endOffset)
	require.Equal(t, baseTime.Add(time.Second), result[1].timestamp)

	require.Equal(t, 2*time.Second, result[2].startOffset)
	require.Equal(t, 2999*time.Millisecond, result[2].endOffset)
	require.Equal(t, baseTime.Add(2*time.Second), result[2].timestamp)
}

func TestThumbnailBucketSampler_AdaptInterval(t *testing.T) {
	sampler := newThumbnailBucketSampler(4, time.Second)
	baseTime := time.Now()

	for i := 0; i < 4; i++ {
		sampler.add(&vt10x.TerminalState{}, baseTime.Add(time.Duration(i)*time.Second))
	}

	require.Len(t, sampler.entries, 4)
	require.Equal(t, time.Second, sampler.interval)

	sampler.add(&vt10x.TerminalState{}, baseTime.Add(4*time.Second))

	require.Len(t, sampler.entries, 3)
	require.Equal(t, 2*time.Second, sampler.interval)

	kept := sampler.result()
	require.Equal(t, baseTime, kept[0].timestamp)
	require.Equal(t, baseTime.Add(2*time.Second), kept[1].timestamp)
	require.Equal(t, baseTime.Add(4*time.Second), kept[2].timestamp)

	require.Equal(t, time.Duration(0), kept[0].startOffset)
	require.Equal(t, 1999*time.Millisecond, kept[0].endOffset)

	require.Equal(t, 2*time.Second, kept[1].startOffset)
	require.Equal(t, 3999*time.Millisecond, kept[1].endOffset)
}

func TestThumbnailBucketSampler_MultipleAdaptations(t *testing.T) {
	sampler := newThumbnailBucketSampler(4, 100*time.Millisecond)
	baseTime := time.Now()

	// Adds multiple entries to the sampler at 100ms intervals.
	// The sampler should adapt its interval multiple times as it reaches capacity.
	//
	// Expected behavior:
	// Add 0ms, 100ms, 200ms, 300ms (4 entries, at capacity)
	// Add 400ms -> adapt: interval = 200ms, keep 0ms, 200ms, 400ms (3 entries)
	// Add 500ms (4 entries, at capacity)
	// Add 600ms -> adapt: interval = 400ms, keep 0ms, 400ms, 600ms (3 entries)
	// Add 700ms (4 entries, at capacity)
	// Add 800ms -> adapt: interval = 800ms, keep 0ms, 600ms, 800ms (3 entries)
	// Add 900ms (4 entries, at capacity)
	// Add 1000ms -> adapt: interval = 1600ms, keep 0ms, 800ms, 1000ms (3 entries)
	// Add 1100ms (4 entries, at capacity)
	// Add 1200ms -> adapt: interval = 3200ms, keep 0ms, 1000ms, 1200ms (3 entries)
	// Add 1300ms (4 entries, at capacity)
	// Add 1400ms -> adapt: interval = 6400ms, keep 0ms, 1200ms, 1400ms (3 entries)
	// Add 1500ms (4 entries)

	for i := 0; i < 16; i++ {
		sampler.add(&vt10x.TerminalState{}, baseTime.Add(time.Duration(i)*100*time.Millisecond))
	}

	require.Equal(t, 6400*time.Millisecond, sampler.interval)
	require.Len(t, sampler.entries, 4)

	results := sampler.result()

	require.Equal(t, baseTime, results[0].timestamp)
	require.Equal(t, baseTime.Add(1200*time.Millisecond), results[1].timestamp)
	require.Equal(t, baseTime.Add(1400*time.Millisecond), results[2].timestamp)
	require.Equal(t, baseTime.Add(1500*time.Millisecond), results[3].timestamp)
}

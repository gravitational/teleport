/**
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

func TestThumbnailBucketSampler_Add(t *testing.T) {
	createTerminalState := func(id int) *vt10x.TerminalState {
		return &vt10x.TerminalState{
			Cols: 80,
			Rows: 24,
		}
	}

	createThumbnailEntry := func(id int, start, end time.Duration) *thumbnailEntry {
		return &thumbnailEntry{
			state:       createTerminalState(id),
			startOffset: start,
			endOffset:   end,
		}
	}

	tests := []struct {
		name          string
		max           int
		addOperations []struct {
			elapsed time.Duration
			entry   *thumbnailEntry
		}
		expectedMaxCount int
	}{
		{
			name: "single thumbnail",
			max:  5,
			addOperations: []struct {
				elapsed time.Duration
				entry   *thumbnailEntry
			}{
				{5 * time.Second, createThumbnailEntry(1, 0, 5*time.Second)},
			},
			expectedMaxCount: 1,
		},
		{
			name: "multiple thumbnails within max",
			max:  5,
			addOperations: []struct {
				elapsed time.Duration
				entry   *thumbnailEntry
			}{
				{5 * time.Second, createThumbnailEntry(1, 0, 5*time.Second)},
				{15 * time.Second, createThumbnailEntry(2, 10*time.Second, 15*time.Second)},
				{25 * time.Second, createThumbnailEntry(3, 20*time.Second, 25*time.Second)},
			},
			expectedMaxCount: 3,
		},
		{
			name: "exactly max thumbnails",
			max:  3,
			addOperations: []struct {
				elapsed time.Duration
				entry   *thumbnailEntry
			}{
				{5 * time.Second, createThumbnailEntry(1, 0, 5*time.Second)},
				{15 * time.Second, createThumbnailEntry(2, 10*time.Second, 15*time.Second)},
				{25 * time.Second, createThumbnailEntry(3, 20*time.Second, 25*time.Second)},
			},
			expectedMaxCount: 3,
		},
		{
			name: "more than max thumbnails maintains even spacing",
			max:  3,
			addOperations: []struct {
				elapsed time.Duration
				entry   *thumbnailEntry
			}{
				{5 * time.Second, createThumbnailEntry(1, 0, 5*time.Second)},
				{15 * time.Second, createThumbnailEntry(2, 10*time.Second, 15*time.Second)},
				{25 * time.Second, createThumbnailEntry(3, 20*time.Second, 25*time.Second)},
				{35 * time.Second, createThumbnailEntry(4, 30*time.Second, 35*time.Second)},
				{45 * time.Second, createThumbnailEntry(5, 40*time.Second, 45*time.Second)},
			},
			expectedMaxCount: 3,
		},
		{
			name: "many thumbnails with small max",
			max:  2,
			addOperations: []struct {
				elapsed time.Duration
				entry   *thumbnailEntry
			}{
				{2 * time.Second, createThumbnailEntry(1, 0, 2*time.Second)},
				{7 * time.Second, createThumbnailEntry(2, 5*time.Second, 7*time.Second)},
				{12 * time.Second, createThumbnailEntry(3, 10*time.Second, 12*time.Second)},
				{22 * time.Second, createThumbnailEntry(4, 20*time.Second, 22*time.Second)},
				{42 * time.Second, createThumbnailEntry(5, 40*time.Second, 42*time.Second)},
			},
			expectedMaxCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sampler := newThumbnailBucketSampler(tt.max, 10*time.Second)

			for _, op := range tt.addOperations {
				sampler.add(op.elapsed, op.entry)
			}

			result := sampler.result()
			require.LessOrEqual(t, len(result), tt.expectedMaxCount)
			require.Greater(t, len(result), 0)
		})
	}
}

func TestThumbnailBucketSampler_EvenSpacing(t *testing.T) {
	t.Run("maintains even spacing with many items", func(t *testing.T) {
		sampler := newThumbnailBucketSampler(5, 1*time.Second)

		for i := 0; i < 100; i++ {
			entry := &thumbnailEntry{
				state: &vt10x.TerminalState{
					Cols: 80,
					Rows: 24,
				},
				startOffset: time.Duration(i) * time.Second,
				endOffset:   time.Duration(i+1) * time.Second,
			}
			sampler.add(time.Duration(i)*time.Second, entry)
		}

		result := sampler.result()
		require.LessOrEqual(t, len(result), 5)
		require.Greater(t, len(result), 0)

		if len(result) > 1 {
			// The spacing should be roughly totalDuration / (max-1)
			expectedSpacing := 99 * time.Second / 4 // 99 seconds / 4 gaps

			require.Less(t, result[0].startOffset, 25*time.Second)
			require.Greater(t, result[len(result)-1].startOffset, 74*time.Second)

			for i := 1; i < len(result); i++ {
				gap := result[i].startOffset - result[i-1].startOffset
				require.Less(t, gap, expectedSpacing*2)
			}
		}
	})
}

func TestThumbnailBucketSampler_Result(t *testing.T) {
	createThumbnailEntry := func(id int) *thumbnailEntry {
		return &thumbnailEntry{
			state: &vt10x.TerminalState{
				Cols: 80,
				Rows: 24,
			},
			startOffset: time.Duration(id) * time.Second,
			endOffset:   time.Duration(id+1) * time.Second,
		}
	}

	tests := []struct {
		name          string
		max           int
		addOperations []struct {
			elapsed time.Duration
			entry   *thumbnailEntry
		}
		minExpected int
		maxExpected int
	}{
		{
			name: "empty sampler",
			max:  5,
			addOperations: []struct {
				elapsed time.Duration
				entry   *thumbnailEntry
			}{},
			minExpected: 0,
			maxExpected: 0,
		},
		{
			name: "single item",
			max:  5,
			addOperations: []struct {
				elapsed time.Duration
				entry   *thumbnailEntry
			}{
				{10 * time.Second, createThumbnailEntry(10)},
			},
			minExpected: 1,
			maxExpected: 1,
		},
		{
			name: "fills up to max",
			max:  3,
			addOperations: []struct {
				elapsed time.Duration
				entry   *thumbnailEntry
			}{
				{10 * time.Second, createThumbnailEntry(10)},
				{20 * time.Second, createThumbnailEntry(20)},
				{30 * time.Second, createThumbnailEntry(30)},
				{40 * time.Second, createThumbnailEntry(40)},
				{50 * time.Second, createThumbnailEntry(50)},
			},
			minExpected: 3,
			maxExpected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sampler := newThumbnailBucketSampler(tt.max, 10*time.Second)

			for _, op := range tt.addOperations {
				sampler.add(op.elapsed, op.entry)
			}

			result := sampler.result()
			require.GreaterOrEqual(t, len(result), tt.minExpected)
			require.LessOrEqual(t, len(result), tt.maxExpected)
		})
	}
}

func TestThumbnailBucketSampler_IntegrationScenarios(t *testing.T) {
	t.Run("handles long sessions with many thumbnails", func(t *testing.T) {
		sampler := newThumbnailBucketSampler(100, 1*time.Second)

		for i := 0; i < 1000; i++ {
			entry := &thumbnailEntry{
				state: &vt10x.TerminalState{
					Cols: 80,
					Rows: 24,
				},
				startOffset: time.Duration(i) * time.Second,
				endOffset:   time.Duration(i+1) * time.Second,
			}
			sampler.add(time.Duration(i)*time.Second, entry)
		}

		result := sampler.result()

		require.LessOrEqual(t, len(result), 100)
		require.Greater(t, len(result), 0)

		require.Less(t, result[0].startOffset, 50*time.Second)
		require.Greater(t, result[len(result)-1].startOffset, 950*time.Second)
	})

	t.Run("handles bursty thumbnail additions", func(t *testing.T) {
		sampler := newThumbnailBucketSampler(10, 5*time.Second)

		for i := 0; i < 50; i++ {
			entry := &thumbnailEntry{
				state: &vt10x.TerminalState{
					Cols: 80,
					Rows: 24,
				},
				startOffset: time.Duration(i*100) * time.Millisecond,
				endOffset:   time.Duration((i+1)*100) * time.Millisecond,
			}
			sampler.add(time.Duration(i*100)*time.Millisecond, entry)
		}

		result := sampler.result()

		require.LessOrEqual(t, len(result), 10)
		require.Greater(t, len(result), 0)
	})

	t.Run("handles sessions with long gaps", func(t *testing.T) {
		sampler := newThumbnailBucketSampler(5, 10*time.Second)

		sampler.add(0, &thumbnailEntry{
			state:       &vt10x.TerminalState{Cols: 80, Rows: 24},
			startOffset: 0,
			endOffset:   1 * time.Second,
		})
		sampler.add(100*time.Second, &thumbnailEntry{
			state:       &vt10x.TerminalState{Cols: 80, Rows: 24},
			startOffset: 100 * time.Second,
			endOffset:   101 * time.Second,
		})
		sampler.add(200*time.Second, &thumbnailEntry{
			state:       &vt10x.TerminalState{Cols: 80, Rows: 24},
			startOffset: 200 * time.Second,
			endOffset:   201 * time.Second,
		})

		result := sampler.result()

		require.Equal(t, 3, len(result))
	})

	t.Run("max of 1 keeps only one thumbnail", func(t *testing.T) {
		sampler := newThumbnailBucketSampler(1, 10*time.Second)

		var lastEntry *thumbnailEntry
		for i := 0; i < 10; i++ {
			entry := &thumbnailEntry{
				state: &vt10x.TerminalState{
					Cols: 80,
					Rows: 24,
				},
				startOffset: time.Duration(i*10) * time.Second,
				endOffset:   time.Duration((i+1)*10) * time.Second,
			}
			sampler.add(time.Duration(i*10)*time.Second, entry)
			if i == 0 {
				lastEntry = entry
			}
		}

		result := sampler.result()

		require.Equal(t, 1, len(result))
		require.Equal(t, lastEntry.startOffset, result[0].startOffset)
	})
}

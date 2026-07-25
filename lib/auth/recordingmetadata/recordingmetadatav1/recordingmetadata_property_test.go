/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestProperty_GetRandomThumbnailTime_InDurationRange(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		duration := genDuration(t)
		result := getRandomThumbnailTime(duration)
		require.GreaterOrEqual(t, int64(result), int64(0), "duration=%v result=%v", duration, result)
		require.LessOrEqual(t, int64(result), int64(duration), "duration=%v result=%v", duration, result)
	})
}

func TestProperty_GetRandomThumbnailTime_RespectsTwentyEightyWindow(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		duration := genDuration(t)
		minIdx := int64(0.2 * float64(duration))
		maxIdx := int64(0.8 * float64(duration))
		result := getRandomThumbnailTime(duration)

		if maxIdx > minIdx {
			require.GreaterOrEqual(t, int64(result), minIdx, "duration=%v result=%v", duration, result)
			require.Less(t, int64(result), maxIdx, "duration=%v result=%v", duration, result)
		} else {
			require.Equal(t, int64(duration)/2, int64(result), "duration=%v result=%v", duration, result)
		}
	})
}

func TestProperty_CalculateThumbnailInterval_AtLeastMinInterval(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		duration := genDuration(t)
		maxThumbnails := rapid.IntRange(1, 10_000).Draw(t, "max_thumbnails")
		minInterval := time.Duration(rapid.Int64Range(0, int64(5*time.Minute)).Draw(t, "min_interval"))

		result := calculateThumbnailInterval(duration, maxThumbnails, minInterval)
		require.GreaterOrEqual(t, int64(result), int64(minInterval),
			"duration=%v max=%d minInterval=%v result=%v", duration, maxThumbnails, minInterval, result)
	})
}

func TestProperty_CalculateThumbnailInterval_MonotonicInDuration(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		maxThumbnails := rapid.IntRange(1, 10_000).Draw(t, "max_thumbnails")
		minInterval := time.Duration(rapid.Int64Range(0, int64(5*time.Minute)).Draw(t, "min_interval"))
		d1 := time.Duration(rapid.Int64Range(0, int64(24*time.Hour)).Draw(t, "d1"))
		extra := time.Duration(rapid.Int64Range(0, int64(24*time.Hour)).Draw(t, "extra"))
		d2 := d1 + extra

		r1 := calculateThumbnailInterval(d1, maxThumbnails, minInterval)
		r2 := calculateThumbnailInterval(d2, maxThumbnails, minInterval)

		require.LessOrEqual(t, int64(r1), int64(r2),
			"non-monotonic: d1=%v d2=%v max=%d min=%v r1=%v r2=%v",
			d1, d2, maxThumbnails, minInterval, r1, r2)
	})
}

func TestProperty_CalculateThumbnailInterval_RoundedToSecondsAboveMin(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		duration := time.Duration(rapid.Int64Range(0, int64(48*time.Hour)).Draw(t, "duration"))
		maxThumbnails := rapid.IntRange(1, 10_000).Draw(t, "max_thumbnails")

		minIntervalSec := rapid.IntRange(0, 300).Draw(t, "min_interval_s")
		minInterval := time.Duration(minIntervalSec) * time.Second

		result := calculateThumbnailInterval(duration, maxThumbnails, minInterval)

		require.Equal(t, time.Duration(0), result%time.Second,
			"non-rounded: duration=%v max=%d min=%v result=%v", duration, maxThumbnails, minInterval, result)
	})
}

// genDuration produces durations biased toward edge cases (0, 1ns) and realistic session lengths.
func genDuration(t *rapid.T) time.Duration {
	t.Helper()

	return rapid.OneOf(
		rapid.Just(time.Duration(0)),
		rapid.Just(time.Duration(1)),
		rapid.Just(time.Nanosecond),
		rapid.Just(time.Second),
		rapid.Just(time.Hour),
		rapid.Map(rapid.Int64Range(0, int64(48*time.Hour)), func(n int64) time.Duration {
			return time.Duration(n)
		}),
	).Draw(t, "duration")
}

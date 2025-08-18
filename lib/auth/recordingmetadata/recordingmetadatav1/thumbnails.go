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
	"math"
	"sort"
	"time"

	"github.com/hinshun/vt10x"
)

// thumbnailBucketSampler maintains at most 'max' evenly-spaced thumbnails
// from a stream of thumbnails without storing all of them in memory.
// It dynamically adjusts which thumbnails to keep to maintain even spacing.
type thumbnailBucketSampler struct {
	max       int
	buckets   []*thumbnailEntry
	totalSeen int
	minTime   time.Duration
	maxTime   time.Duration
}

type thumbnailEntry struct {
	state       *vt10x.TerminalState
	startOffset time.Duration
	endOffset   time.Duration
}

func newThumbnailBucketSampler(max int) *thumbnailBucketSampler {
	return &thumbnailBucketSampler{
		max:       max,
		buckets:   make([]*thumbnailEntry, 0, max),
		totalSeen: 0,
		minTime:   time.Duration(math.MaxInt64),
		maxTime:   time.Duration(0),
	}
}

func (s *thumbnailBucketSampler) add(entry *thumbnailEntry) {
	s.totalSeen++

	if entry.startOffset < s.minTime {
		s.minTime = entry.startOffset
	}
	if entry.startOffset > s.maxTime {
		s.maxTime = entry.startOffset
	}

	if len(s.buckets) < s.max {
		s.insertSorted(entry)
		return
	}

	targetIndex := s.getTargetBucket(entry.startOffset)
	s.replaceIfBetter(targetIndex, entry)
}

func (s *thumbnailBucketSampler) getTargetBucket(offset time.Duration) int {
	if s.maxTime == s.minTime {
		return 0
	}

	// Calculate the index based on the offset's position in the time range.
	ratio := float64(offset-s.minTime) / float64(s.maxTime-s.minTime)
	index := int(ratio * float64(s.max))

	if index >= s.max {
		index = s.max - 1
	}

	return index
}

func (s *thumbnailBucketSampler) replaceIfBetter(targetIndex int, entry *thumbnailEntry) {
	timeRange := s.maxTime - s.minTime
	if timeRange == 0 {
		return
	}

	// Calculate the ideal time for the target bucket
	idealTime := s.minTime + time.Duration(
		float64(timeRange)*(float64(targetIndex)+0.5)/float64(s.max),
	)

	// Find the best existing entry in the target bucket
	// that is closest to the ideal time.
	bestIndex := -1
	bestDistance := time.Duration(math.MaxInt64)

	for i, b := range s.buckets {
		bucketIndex := s.getTargetBucket(b.startOffset)
		if bucketIndex == targetIndex {
			distance := absDuration(b.startOffset - idealTime)
			if distance < bestDistance {
				bestDistance = distance
				bestIndex = i
			}
		}
	}

	// If we found a better existing entry, replace it.
	newDistance := absDuration(entry.startOffset - idealTime)

	if bestIndex >= 0 && newDistance < bestDistance {
		// Replace the best existing entry with the new one
		s.buckets[bestIndex] = entry
		sort.Slice(s.buckets, func(i, j int) bool {
			return s.buckets[i].startOffset < s.buckets[j].startOffset
		})
	} else if bestIndex < 0 {
		// Force replace if no existing entry in the target bucket
		// is better than the new entry.
		s.forceReplace(entry)
	}
}

func (s *thumbnailBucketSampler) forceReplace(entry *thumbnailEntry) {
	// Calculate how many entries are in each bucket
	bucketCounts := make([]int, s.max)
	for _, b := range s.buckets {
		index := s.getTargetBucket(b.startOffset)
		bucketCounts[index]++
	}

	// Find the bucket with the most entries
	// and replace one of its entries with the new one.
	maxCount := 0
	overRepIndex := -1
	for i, count := range bucketCounts {
		if count > maxCount {
			maxCount = count
			overRepIndex = i
		}
	}

	// If we found an over-represented bucket, replace one of its entries
	// with the new entry.
	if overRepIndex >= 0 {
		for i, b := range s.buckets {
			if s.getTargetBucket(b.startOffset) == overRepIndex {
				s.buckets[i] = entry
				break
			}
		}

		sort.Slice(s.buckets, func(i, j int) bool {
			return s.buckets[i].startOffset < s.buckets[j].startOffset
		})
	}
}

func (s *thumbnailBucketSampler) insertSorted(entry *thumbnailEntry) {
	s.buckets = append(s.buckets, entry)
	sort.Slice(s.buckets, func(i, j int) bool {
		return s.buckets[i].startOffset < s.buckets[j].startOffset
	})
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

func (s *thumbnailBucketSampler) result() []*thumbnailEntry {
	return s.buckets
}

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
	"time"

	"github.com/hinshun/vt10x"
)

// thumbnailBucketSampler maintains at most 'max' evenly-spaced thumbnails
// from a stream of thumbnails without storing all of them in memory.
// It dynamically adjusts which thumbnails to keep to maintain even spacing, doubling
// the interval when the number of thumbnails exceeds 'max' and removing every second thumbnail
// to keep the total count within 'max'.
type thumbnailBucketSampler struct {
	maxCapacity   int
	entries       []*thumbnailEntry
	interval      time.Duration
	nextTimestamp time.Time
	startTime     time.Time
}

type thumbnailState struct {
	svg           []byte
	cols, rows    int
	cursorVisible bool
	cursor        vt10x.Cursor
}

type thumbnailEntry struct {
	state       *thumbnailState
	startOffset time.Duration
	endOffset   time.Duration
	timestamp   time.Time
}

func newThumbnailBucketSampler(maxCapacity int, interval time.Duration) *thumbnailBucketSampler {
	return &thumbnailBucketSampler{
		maxCapacity: maxCapacity,
		entries:     make([]*thumbnailEntry, 0, maxCapacity),
		interval:    interval,
	}
}

func (s *thumbnailBucketSampler) shouldCapture(timestamp time.Time) bool {
	return !timestamp.Before(s.nextTimestamp)
}

func (s *thumbnailBucketSampler) add(state *thumbnailState, timestamp time.Time) {
	if s.startTime.IsZero() {
		s.startTime = timestamp
	}

	if len(s.entries) >= s.maxCapacity {
		s.adaptInterval()
	}

	entry := &thumbnailEntry{
		state:       state,
		timestamp:   timestamp,
		startOffset: timestamp.Sub(s.startTime),
		endOffset:   timestamp.Add(s.interval).Add(-1 * time.Millisecond).Sub(s.startTime), // subtract 1ms to make the ranges non-overlapping
	}

	s.nextTimestamp = timestamp.Add(s.interval)
	s.entries = append(s.entries, entry)
}

func (s *thumbnailBucketSampler) adaptInterval() {
	s.interval *= 2

	kept := make([]*thumbnailEntry, 0, len(s.entries)/2+1)

	for i := 0; i < len(s.entries); i += 2 {
		kept = append(kept, s.entries[i])

		if i < len(s.entries)-1 {
			s.entries[i].endOffset = s.entries[i+1].endOffset
		}
	}

	s.entries = kept
}

func (s *thumbnailBucketSampler) result() []*thumbnailEntry {
	return s.entries
}

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

	idealTime := s.minTime + time.Duration(
		float64(timeRange)*(float64(targetIndex)+0.5)/float64(s.max),
	)

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

	newDistance := absDuration(entry.startOffset - idealTime)

	if bestIndex >= 0 && newDistance < bestDistance {
		s.buckets[bestIndex] = entry
		sort.Slice(s.buckets, func(i, j int) bool {
			return s.buckets[i].startOffset < s.buckets[j].startOffset
		})
	} else if bestIndex < 0 {
		s.forceReplace(entry)
	}
}

func (s *thumbnailBucketSampler) forceReplace(entry *thumbnailEntry) {
	bucketCounts := make([]int, s.max)
	for _, b := range s.buckets {
		index := s.getTargetBucket(b.startOffset)
		bucketCounts[index]++
	}

	maxCount := 0
	overRepIndex := -1
	for i, count := range bucketCounts {
		if count > maxCount {
			maxCount = count
			overRepIndex = i
		}
	}

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

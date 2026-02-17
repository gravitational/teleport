// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package discovery

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
	libevents "github.com/gravitational/teleport/lib/events"
)

func makeTestEvent(id string, t time.Time, eventType string) apievents.AuditEvent {
	return &apievents.SessionStart{
		Metadata: apievents.Metadata{
			ID:   id,
			Type: eventType,
			Time: t,
		},
	}
}

var (
	t0 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 = time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	t2 = time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)
	t3 = time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC)
	t4 = time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
)

// TestCacheFileName verifies file name generation.
func TestCacheFileName(t *testing.T) {
	got := cacheFileName("ssm.run", t0, t1)
	assert.Equal(t, "ssm_run_2026-01-01T0000_2026-01-02T0000.jsonl", got)

	// Dots are replaced with underscores.
	got = cacheFileName("a.b.c", t0, t1)
	assert.Equal(t, "a_b_c_2026-01-01T0000_2026-01-02T0000.jsonl", got)

	// No dots, name unchanged.
	got = cacheFileName("simple", t0, t1)
	assert.Equal(t, "simple_2026-01-01T0000_2026-01-02T0000.jsonl", got)
}

// TestUncoveredRanges tests the gap computation logic.
func TestUncoveredRanges(t *testing.T) {
	mkFile := func(from, to time.Time) cacheFile {
		return cacheFile{Header: cacheHeader{From: from, To: to}}
	}

	t.Run("no cache files", func(t *testing.T) {
		gaps := uncoveredRanges(t0, t4, nil)
		require.Len(t, gaps, 1)
		assert.Equal(t, t0, gaps[0].From)
		assert.Equal(t, t4, gaps[0].To)
	})

	t.Run("cache covers entire range", func(t *testing.T) {
		files := []cacheFile{mkFile(t0, t4)}
		gaps := uncoveredRanges(t0, t4, files)
		assert.Empty(t, gaps)
	})

	t.Run("cache covers start only", func(t *testing.T) {
		files := []cacheFile{mkFile(t0, t2)}
		gaps := uncoveredRanges(t0, t4, files)
		require.Len(t, gaps, 1)
		assert.Equal(t, t2, gaps[0].From)
		assert.Equal(t, t4, gaps[0].To)
	})

	t.Run("cache covers end only", func(t *testing.T) {
		files := []cacheFile{mkFile(t2, t4)}
		gaps := uncoveredRanges(t0, t4, files)
		require.Len(t, gaps, 1)
		assert.Equal(t, t0, gaps[0].From)
		assert.Equal(t, t2, gaps[0].To)
	})

	t.Run("gap in middle", func(t *testing.T) {
		files := []cacheFile{mkFile(t0, t1), mkFile(t2, t4)}
		gaps := uncoveredRanges(t0, t4, files)
		require.Len(t, gaps, 1)
		assert.Equal(t, t1, gaps[0].From)
		assert.Equal(t, t2, gaps[0].To)
	})

	t.Run("multiple overlapping files", func(t *testing.T) {
		// Two overlapping files that together cover [t0, t3].
		files := []cacheFile{mkFile(t0, t2), mkFile(t1, t3)}
		gaps := uncoveredRanges(t0, t4, files)
		require.Len(t, gaps, 1)
		assert.Equal(t, t3, gaps[0].From)
		assert.Equal(t, t4, gaps[0].To)
	})

	t.Run("cache wider than query", func(t *testing.T) {
		files := []cacheFile{mkFile(t0, t4)}
		gaps := uncoveredRanges(t1, t3, files)
		assert.Empty(t, gaps)
	})
}

// TestDeduplicateEvents tests event deduplication by ID.
func TestDeduplicateEvents(t *testing.T) {
	evTime := t0

	t.Run("no duplicates", func(t *testing.T) {
		events := []apievents.AuditEvent{
			makeTestEvent("a", evTime, "session.start"),
			makeTestEvent("b", evTime, "session.start"),
		}
		got := deduplicateEvents(events)
		require.Len(t, got, 2)
	})

	t.Run("duplicate IDs", func(t *testing.T) {
		events := []apievents.AuditEvent{
			makeTestEvent("a", evTime, "session.start"),
			makeTestEvent("a", evTime.Add(time.Hour), "session.start"),
			makeTestEvent("b", evTime, "session.start"),
		}
		got := deduplicateEvents(events)
		require.Len(t, got, 2)
		assert.Equal(t, "a", got[0].GetID())
		assert.Equal(t, "b", got[1].GetID())
		// First occurrence kept.
		assert.Equal(t, evTime, got[0].GetTime())
	})

	t.Run("empty IDs all kept", func(t *testing.T) {
		events := []apievents.AuditEvent{
			makeTestEvent("", evTime, "session.start"),
			makeTestEvent("", evTime, "session.start"),
		}
		got := deduplicateEvents(events)
		require.Len(t, got, 2)
	})

	t.Run("mix of empty and non-empty IDs", func(t *testing.T) {
		events := []apievents.AuditEvent{
			makeTestEvent("a", evTime, "session.start"),
			makeTestEvent("", evTime, "session.start"),
			makeTestEvent("a", evTime, "session.start"),
			makeTestEvent("", evTime, "session.start"),
		}
		got := deduplicateEvents(events)
		require.Len(t, got, 3) // "a" once + two empty
	})
}

// TestCacheWriteAndRead tests the write/read roundtrip for cache files.
func TestCacheWriteAndRead(t *testing.T) {
	t.Run("write and read back", func(t *testing.T) {
		dir := t.TempDir()
		cache := &eventCache{Dir: dir}

		header := cacheHeader{
			EventType: "session.start",
			From:      t0,
			To:        t1,
			FetchedAt: time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC),
			Count:     2,
		}
		events := []apievents.AuditEvent{
			makeTestEvent("ev1", t0.Add(time.Hour), "session.start"),
			makeTestEvent("ev2", t0.Add(2*time.Hour), "session.start"),
		}

		require.NoError(t, cache.writeCacheFile(header, events))

		// Read header.
		path := filepath.Join(dir, cacheFileName(header.EventType, header.From, header.To))
		gotHeader, err := readCacheHeader(path)
		require.NoError(t, err)
		assert.Equal(t, header.EventType, gotHeader.EventType)
		assert.Equal(t, header.From, gotHeader.From)
		assert.Equal(t, header.To, gotHeader.To)
		assert.Equal(t, header.Count, gotHeader.Count)

		// Read events.
		gotEvents, err := readCacheEvents(path)
		require.NoError(t, err)
		require.Len(t, gotEvents, 2)
		assert.Equal(t, "ev1", gotEvents[0].GetID())
		assert.Equal(t, "ev2", gotEvents[1].GetID())
	})

	t.Run("empty events list", func(t *testing.T) {
		dir := t.TempDir()
		cache := &eventCache{Dir: dir}

		header := cacheHeader{
			EventType: "session.start",
			From:      t0,
			To:        t1,
			FetchedAt: time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC),
			Count:     0,
		}

		require.NoError(t, cache.writeCacheFile(header, nil))

		path := filepath.Join(dir, cacheFileName(header.EventType, header.From, header.To))
		gotEvents, err := readCacheEvents(path)
		require.NoError(t, err)
		assert.Empty(t, gotEvents)
	})
}

// TestCacheListCacheFiles tests listing and filtering cache files.
func TestCacheListCacheFiles(t *testing.T) {
	t.Run("directory does not exist", func(t *testing.T) {
		cache := &eventCache{Dir: filepath.Join(t.TempDir(), "nonexistent")}
		files, err := cache.listCacheFiles("session.start")
		require.NoError(t, err)
		assert.Nil(t, files)
	})

	t.Run("corrupt cache file skipped", func(t *testing.T) {
		dir := t.TempDir()
		cache := &eventCache{Dir: dir}

		// Write a valid file.
		header := cacheHeader{
			EventType: "session.start",
			From:      t0,
			To:        t1,
			Count:     0,
		}
		require.NoError(t, cache.writeCacheFile(header, nil))

		// Write a corrupt file with matching prefix.
		corruptPath := filepath.Join(dir, "session_start_corrupt.jsonl")
		require.NoError(t, os.WriteFile(corruptPath, []byte("not valid json\n"), 0o644))

		files, err := cache.listCacheFiles("session.start")
		require.NoError(t, err)
		// Only the valid file should be returned.
		require.Len(t, files, 1)
		assert.Equal(t, header.EventType, files[0].Header.EventType)
	})

	t.Run("files sorted by From time", func(t *testing.T) {
		dir := t.TempDir()
		cache := &eventCache{Dir: dir}

		// Write in reverse order.
		for _, r := range []struct{ from, to time.Time }{{t2, t3}, {t0, t1}} {
			h := cacheHeader{EventType: "session.start", From: r.from, To: r.to}
			require.NoError(t, cache.writeCacheFile(h, nil))
		}

		files, err := cache.listCacheFiles("session.start")
		require.NoError(t, err)
		require.Len(t, files, 2)
		assert.True(t, files[0].Header.From.Before(files[1].Header.From))
	})
}

// TestCacheMergeAndRewrite tests merging overlapping cache files.
func TestCacheMergeAndRewrite(t *testing.T) {
	t.Run("two overlapping files merged", func(t *testing.T) {
		dir := t.TempDir()
		cache := &eventCache{Dir: dir}

		// Write two overlapping cache files.
		h1 := cacheHeader{EventType: "session.start", From: t0, To: t2, Count: 1, FetchedAt: t0}
		e1 := []apievents.AuditEvent{makeTestEvent("ev1", t0.Add(time.Hour), "session.start")}
		require.NoError(t, cache.writeCacheFile(h1, e1))

		h2 := cacheHeader{EventType: "session.start", From: t1, To: t3, Count: 1, FetchedAt: t0}
		e2 := []apievents.AuditEvent{makeTestEvent("ev2", t1.Add(time.Hour), "session.start")}
		require.NoError(t, cache.writeCacheFile(h2, e2))

		newEvents := []apievents.AuditEvent{makeTestEvent("ev3", t2.Add(time.Hour), "session.start")}
		merged, err := cache.mergeAndRewrite("session.start", t0, t3, newEvents)
		require.NoError(t, err)
		assert.Len(t, merged, 3)

		// Old files removed, only merged file remains.
		files, err := cache.listCacheFiles("session.start")
		require.NoError(t, err)
		require.Len(t, files, 1)
		assert.Equal(t, t0, files[0].Header.From)
		assert.Equal(t, t3, files[0].Header.To)
	})

	t.Run("events deduplicated across files", func(t *testing.T) {
		dir := t.TempDir()
		cache := &eventCache{Dir: dir}

		h1 := cacheHeader{EventType: "session.start", From: t0, To: t2, Count: 1, FetchedAt: t0}
		e1 := []apievents.AuditEvent{makeTestEvent("dup", t0.Add(time.Hour), "session.start")}
		require.NoError(t, cache.writeCacheFile(h1, e1))

		// New events contain same ID.
		newEvents := []apievents.AuditEvent{makeTestEvent("dup", t0.Add(time.Hour), "session.start")}
		merged, err := cache.mergeAndRewrite("session.start", t0, t2, newEvents)
		require.NoError(t, err)
		assert.Len(t, merged, 1)
	})

}

// mockSearchFn creates a mock searchEventsFunc that records calls and returns predefined events.
type mockSearchFn struct {
	calls  []libevents.SearchEventsRequest
	events []apievents.AuditEvent
}

func (m *mockSearchFn) fn(ctx context.Context, req libevents.SearchEventsRequest) ([]apievents.AuditEvent, string, error) {
	m.calls = append(m.calls, req)
	return m.events, "", nil
}

// TestCachedSearchEvents tests the top-level cached search logic.
func TestCachedSearchEvents(t *testing.T) {
	eventType := "session.start"

	t.Run("nil cache falls through to direct fetch", func(t *testing.T) {
		mock := &mockSearchFn{
			events: []apievents.AuditEvent{
				makeTestEvent("ev1", t0.Add(time.Hour), eventType),
			},
		}

		var cache *eventCache // nil
		result, err := cache.cachedSearchEvents(context.Background(), mock.fn, t0, t1, eventType, 100)
		require.NoError(t, err)
		assert.Len(t, result.Events, 1)
		assert.Len(t, mock.calls, 1, "searchFn should be called")
	})

	t.Run("empty cache dir fetches everything and writes cache", func(t *testing.T) {
		dir := t.TempDir()
		cache := &eventCache{Dir: dir}

		mock := &mockSearchFn{
			events: []apievents.AuditEvent{
				makeTestEvent("ev1", t0.Add(time.Hour), eventType),
			},
		}

		result, err := cache.cachedSearchEvents(context.Background(), mock.fn, t0, t1, eventType, 100)
		require.NoError(t, err)
		assert.Len(t, result.Events, 1)
		assert.Len(t, mock.calls, 1, "searchFn should be called for the full range")

		// Cache file should exist now.
		files, err := cache.listCacheFiles(eventType)
		require.NoError(t, err)
		assert.NotEmpty(t, files)
	})

	t.Run("full cache hit", func(t *testing.T) {
		dir := t.TempDir()
		cache := &eventCache{Dir: dir}

		// Pre-populate cache.
		h := cacheHeader{EventType: eventType, From: t0, To: t1, Count: 1, FetchedAt: t0}
		events := []apievents.AuditEvent{makeTestEvent("ev1", t0.Add(time.Hour), eventType)}
		require.NoError(t, cache.writeCacheFile(h, events))

		mock := &mockSearchFn{}
		result, err := cache.cachedSearchEvents(context.Background(), mock.fn, t0, t1, eventType, 100)
		require.NoError(t, err)
		assert.Len(t, result.Events, 1)
		assert.Empty(t, mock.calls, "searchFn should NOT be called for a full cache hit")
		assert.Equal(t, 1, result.CacheHits)
		assert.Equal(t, 0, result.CacheMisses)
	})

	t.Run("partial overlap fetches only gap", func(t *testing.T) {
		dir := t.TempDir()
		cache := &eventCache{Dir: dir}

		// Cache covers [t0, t2), query is [t0, t3).
		h := cacheHeader{EventType: eventType, From: t0, To: t2, Count: 1, FetchedAt: t0}
		cachedEvts := []apievents.AuditEvent{makeTestEvent("cached1", t0.Add(time.Hour), eventType)}
		require.NoError(t, cache.writeCacheFile(h, cachedEvts))

		mock := &mockSearchFn{
			events: []apievents.AuditEvent{
				makeTestEvent("fresh1", t2.Add(time.Hour), eventType),
			},
		}

		result, err := cache.cachedSearchEvents(context.Background(), mock.fn, t0, t3, eventType, 100)
		require.NoError(t, err)
		assert.Len(t, result.Events, 2)
		require.Len(t, mock.calls, 1, "searchFn should be called once for the gap")
		// The gap should be [t2, t3).
		assert.Equal(t, t2, mock.calls[0].From)
		assert.Equal(t, t3, mock.calls[0].To)
	})

	t.Run("limit applied", func(t *testing.T) {
		dir := t.TempDir()
		cache := &eventCache{Dir: dir}

		mock := &mockSearchFn{
			events: []apievents.AuditEvent{
				makeTestEvent("ev1", t0.Add(3*time.Hour), eventType),
				makeTestEvent("ev2", t0.Add(2*time.Hour), eventType),
				makeTestEvent("ev3", t0.Add(time.Hour), eventType),
			},
		}

		result, err := cache.cachedSearchEvents(context.Background(), mock.fn, t0, t1, eventType, 2)
		require.NoError(t, err)
		assert.Len(t, result.Events, 2)
		assert.True(t, result.LimitReached)
	})
}

// TestCacheZeroEventsFromBackend verifies that a cache file is still created
// when the backend returns zero events (marks the range as fetched).
func TestCacheZeroEventsFromBackend(t *testing.T) {
	dir := t.TempDir()
	cache := &eventCache{Dir: dir}
	eventType := "session.start"

	mock := &mockSearchFn{events: nil}

	_, err := cache.cachedSearchEvents(context.Background(), mock.fn, t0, t1, eventType, 100)
	require.NoError(t, err)

	files, err := cache.listCacheFiles(eventType)
	require.NoError(t, err)
	assert.Len(t, files, 1, "cache file should be created even for zero events")
	assert.Equal(t, 0, files[0].Header.Count)
}

// TestCacheWriteAtomic verifies that writeCacheFile does not leave partial
// files when an error occurs, and that the final file only appears after
// a successful write.
func TestCacheWriteAtomic(t *testing.T) {
	t.Run("no temp files left after successful write", func(t *testing.T) {
		dir := t.TempDir()
		cache := &eventCache{Dir: dir}
		eventType := "session.start"

		h := cacheHeader{EventType: eventType, From: t0, To: t1, FetchedAt: t0, Count: 1}
		events := []apievents.AuditEvent{makeTestEvent("ev1", t0.Add(time.Hour), eventType)}
		require.NoError(t, cache.writeCacheFile(h, events))

		// Only the final file should exist, no .cache-tmp-* leftovers.
		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		for _, e := range entries {
			assert.NotContains(t, e.Name(), ".cache-tmp-", "temp file should be cleaned up")
		}

		// The final file should be readable.
		files, err := cache.listCacheFiles(eventType)
		require.NoError(t, err)
		assert.Len(t, files, 1)
	})

	t.Run("write to read-only dir fails without leaving files", func(t *testing.T) {
		dir := t.TempDir()
		roDir := filepath.Join(dir, "readonly")
		require.NoError(t, os.MkdirAll(roDir, 0o500))
		cache := &eventCache{Dir: roDir}
		eventType := "session.start"

		h := cacheHeader{EventType: eventType, From: t0, To: t1, FetchedAt: t0, Count: 1}
		events := []apievents.AuditEvent{makeTestEvent("ev1", t0.Add(time.Hour), eventType)}
		err := cache.writeCacheFile(h, events)
		require.Error(t, err)

		// No files should be left behind.
		entries, err := os.ReadDir(roDir)
		require.NoError(t, err)
		assert.Empty(t, entries, "no files should be left on failed write")
	})
}

func TestFilterEventsInRange(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 2, 16, 12, 0, 0, 0, time.UTC)

	mkEvent := func(offset time.Duration) apievents.AuditEvent {
		return &apievents.SSMRun{Metadata: apievents.Metadata{Time: base.Add(offset)}}
	}

	events := []apievents.AuditEvent{
		mkEvent(-2 * time.Hour),
		mkEvent(-1 * time.Hour),
		mkEvent(0),
		mkEvent(1 * time.Hour),
		mkEvent(2 * time.Hour),
	}

	t.Run("filters to range", func(t *testing.T) {
		from := base.Add(-1 * time.Hour)
		to := base.Add(1 * time.Hour)
		result := filterEventsInRange(events, from, to)
		require.Len(t, result, 3) // -1h, 0, +1h
	})

	t.Run("inclusive boundaries", func(t *testing.T) {
		result := filterEventsInRange(events, base, base)
		require.Len(t, result, 1) // exactly base
	})

	t.Run("empty result", func(t *testing.T) {
		from := base.Add(3 * time.Hour)
		to := base.Add(4 * time.Hour)
		result := filterEventsInRange(events, from, to)
		require.Empty(t, result)
	})

	t.Run("nil input", func(t *testing.T) {
		result := filterEventsInRange(nil, base, base)
		require.Empty(t, result)
	})
}

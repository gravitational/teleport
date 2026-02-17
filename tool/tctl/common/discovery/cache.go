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
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/utils"
)

const cacheTimeFormat = "2006-01-02T1504"

// cacheHeader is the first line of a cache .jsonl file, containing metadata
// about the cached event query.
type cacheHeader struct {
	EventType string    `json:"event_type"`
	From      time.Time `json:"from"`
	To        time.Time `json:"to"`
	FetchedAt time.Time `json:"fetched_at"`
	Count     int       `json:"count"`
}

// cacheFile represents a single cache file on disk.
type cacheFile struct {
	Path   string
	Header cacheHeader
}

// eventCache manages cached audit event files stored in a directory.
// It tracks cumulative stats across all cachedSearchEvents calls.
type eventCache struct {
	Dir         string // the .cache/<cluster-name>/ directory
	totalHits   int    // cumulative events served from cache
	totalMisses int    // cumulative events fetched from backend
	totalFiles  int    // cumulative cache files read
}

// cacheSummary returns a human-readable summary of cache usage so far.
// Returns empty string if no cache activity occurred.
func (c *eventCache) cacheSummary() string {
	if c == nil {
		return ""
	}
	total := c.totalHits + c.totalMisses
	if total == 0 && c.totalFiles == 0 {
		return ""
	}
	if c.totalHits == 0 {
		return fmt.Sprintf("%d events fetched (no cache)", c.totalMisses)
	}
	if c.totalMisses == 0 {
		return fmt.Sprintf("%d events from cache (%d files)", c.totalHits, c.totalFiles)
	}
	return fmt.Sprintf("%d events total (%d cached, %d fetched)", total, c.totalHits, c.totalMisses)
}

// cacheFileName returns the filename for a cache file given an event type and time range.
// Dots in the event type are replaced with underscores, and times are formatted
// with cacheTimeFormat.
func cacheFileName(eventType string, from, to time.Time) string {
	safe := strings.ReplaceAll(eventType, ".", "_")
	return fmt.Sprintf("%s_%s_%s.jsonl", safe, from.Format(cacheTimeFormat), to.Format(cacheTimeFormat))
}

// listCacheFiles reads the cache directory and returns all cache files matching
// the given event type prefix, sorted by From time. Returns nil, nil if the
// directory does not exist. Corrupt files are logged and skipped.
func (c *eventCache) listCacheFiles(eventType string) ([]cacheFile, error) {
	prefix := strings.ReplaceAll(eventType, ".", "_") + "_"

	entries, err := os.ReadDir(c.Dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, trace.Wrap(err)
	}

	var files []cacheFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}
		path := filepath.Join(c.Dir, entry.Name())
		header, err := readCacheHeader(path)
		if err != nil {
			slog.Warn("Skipping corrupt cache file", "path", path, "error", err)
			continue
		}
		files = append(files, cacheFile{
			Path:   path,
			Header: header,
		})
	}

	slices.SortFunc(files, func(a, b cacheFile) int {
		return a.Header.From.Compare(b.Header.From)
	})

	return files, nil
}

// readCacheHeader reads and unmarshals the first line of a cache file as a cacheHeader.
func readCacheHeader(path string) (cacheHeader, error) {
	f, err := os.Open(path)
	if err != nil {
		return cacheHeader{}, trace.Wrap(err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return cacheHeader{}, trace.Wrap(err)
		}
		return cacheHeader{}, trace.BadParameter("empty cache file: %s", path)
	}

	var header cacheHeader
	if err := json.Unmarshal(scanner.Bytes(), &header); err != nil {
		return cacheHeader{}, trace.Wrap(err)
	}

	return header, nil
}

// readCacheEvents reads all lines after the header from a cache file and
// unmarshals each as an audit event. Uses a 1MB scanner buffer.
func readCacheEvents(path string) ([]apievents.AuditEvent, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	// Skip the header line.
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, trace.Wrap(err)
		}
		return nil, trace.BadParameter("empty cache file: %s", path)
	}

	var events []apievents.AuditEvent
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var fields libevents.EventFields
		if err := json.Unmarshal(line, &fields); err != nil {
			return nil, trace.Wrap(err, "unmarshalling event in %s", path)
		}
		event, err := libevents.FromEventFields(fields)
		if err != nil {
			return nil, trace.Wrap(err, "converting event fields in %s", path)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, trace.Wrap(err)
	}

	return events, nil
}

// writeCacheFile writes a cache file with the given header and events.
// It creates the cache directory with 0o700 permissions if it doesn't exist.
// The file format is: header JSON on the first line, followed by one JSON-encoded
// event per line.
func (c *eventCache) writeCacheFile(header cacheHeader, events []apievents.AuditEvent) error {
	if err := os.MkdirAll(c.Dir, 0o700); err != nil {
		return trace.Wrap(err)
	}

	finalPath := filepath.Join(c.Dir, cacheFileName(header.EventType, header.From, header.To))

	// Write to a temp file first, then atomic rename so interrupted writes
	// never leave a partial/corrupt cache file on disk.
	tmp, err := os.CreateTemp(c.Dir, ".cache-tmp-*.jsonl")
	if err != nil {
		return trace.Wrap(err)
	}
	tmpPath := tmp.Name()
	success := false
	defer func() {
		tmp.Close()
		if !success {
			os.Remove(tmpPath)
		}
	}()

	w := bufio.NewWriter(tmp)

	headerBytes, err := json.Marshal(header)
	if err != nil {
		return trace.Wrap(err)
	}
	if _, err := w.Write(headerBytes); err != nil {
		return trace.Wrap(err)
	}
	if err := w.WriteByte('\n'); err != nil {
		return trace.Wrap(err)
	}

	for _, event := range events {
		data, err := utils.FastMarshal(event)
		if err != nil {
			return trace.Wrap(err)
		}
		if _, err := w.Write(data); err != nil {
			return trace.Wrap(err)
		}
		if err := w.WriteByte('\n'); err != nil {
			return trace.Wrap(err)
		}
	}

	if err := w.Flush(); err != nil {
		return trace.Wrap(err)
	}
	if err := tmp.Close(); err != nil {
		return trace.Wrap(err)
	}

	if err := os.Rename(tmpPath, finalPath); err != nil {
		return trace.Wrap(err)
	}
	success = true
	return nil
}

// timeRange represents a half-open time interval [From, To].
type timeRange struct {
	From, To time.Time
}

// uncoveredRanges computes sub-ranges of [from, to] not covered by any cache file.
// files must be sorted by From time (as returned by listCacheFiles).
func uncoveredRanges(from, to time.Time, files []cacheFile) []timeRange {
	cursor := from
	var gaps []timeRange

	for _, f := range files {
		fileFrom := f.Header.From
		fileTo := f.Header.To

		// Skip files entirely before cursor.
		if !fileTo.After(cursor) {
			continue
		}
		// Skip files entirely after to.
		if !fileFrom.Before(to) {
			break
		}

		// If cursor is before the file's From, record a gap.
		if cursor.Before(fileFrom) {
			gaps = append(gaps, timeRange{From: cursor, To: fileFrom})
		}

		// Advance cursor to max(cursor, fileTo).
		if fileTo.After(cursor) {
			cursor = fileTo
		}
	}

	// Final gap if cursor hasn't reached to.
	if cursor.Before(to) {
		gaps = append(gaps, timeRange{From: cursor, To: to})
	}

	return gaps
}

// filterEventsInRange returns events whose GetTime() is within [from, to] (inclusive on both ends).
func filterEventsInRange(events []apievents.AuditEvent, from, to time.Time) []apievents.AuditEvent {
	var filtered []apievents.AuditEvent
	for _, e := range events {
		t := e.GetTime()
		if (t.Equal(from) || t.After(from)) && (t.Equal(to) || t.Before(to)) {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// deduplicateEvents removes duplicates by event ID (GetID()), keeping the first
// occurrence. Events with an empty ID are always kept.
func deduplicateEvents(events []apievents.AuditEvent) []apievents.AuditEvent {
	seen := make(map[string]struct{})
	var result []apievents.AuditEvent
	for _, e := range events {
		id := e.GetID()
		if id == "" {
			result = append(result, e)
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, e)
	}
	return result
}

// mergeAndRewrite lists cache files for eventType, loads events from files
// overlapping or adjacent to [from, to], merges them with newEvents, deduplicates,
// sorts descending by time, writes a single merged file covering the expanded range,
// and removes the old files. Returns the merged events.
func (c *eventCache) mergeAndRewrite(eventType string, from, to time.Time, newEvents []apievents.AuditEvent) ([]apievents.AuditEvent, error) {
	files, err := c.listCacheFiles(eventType)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	mergedFrom := from
	mergedTo := to
	var allEvents []apievents.AuditEvent
	var toRemove []string

	for _, f := range files {
		fileFrom := f.Header.From
		fileTo := f.Header.To

		// Check if the file overlaps or is adjacent to [from, to].
		if fileTo.Before(from) || fileFrom.After(to) {
			continue
		}

		events, err := readCacheEvents(f.Path)
		if err != nil {
			slog.Warn("Skipping unreadable cache file during merge", "path", f.Path, "error", err)
			continue
		}
		allEvents = append(allEvents, events...)

		if fileFrom.Before(mergedFrom) {
			mergedFrom = fileFrom
		}
		if fileTo.After(mergedTo) {
			mergedTo = fileTo
		}

		toRemove = append(toRemove, f.Path)
	}

	allEvents = append(allEvents, newEvents...)
	allEvents = deduplicateEvents(allEvents)

	sort.SliceStable(allEvents, func(i, j int) bool {
		return allEvents[i].GetTime().After(allEvents[j].GetTime())
	})

	header := cacheHeader{
		EventType: eventType,
		From:      mergedFrom,
		To:        mergedTo,
		FetchedAt: time.Now().UTC(),
		Count:     len(allEvents),
	}
	if err := c.writeCacheFile(header, allEvents); err != nil {
		return nil, trace.Wrap(err)
	}

	// Remove old files (except if old path equals new path).
	newPath := filepath.Join(c.Dir, cacheFileName(eventType, mergedFrom, mergedTo))
	for _, path := range toRemove {
		if path == newPath {
			continue
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			slog.Warn("Failed to remove old cache file", "path", path, "error", err)
		}
	}

	return allEvents, nil
}

// searchEventsFunc is a function type matching the client.SearchEvents signature.
type searchEventsFunc func(ctx context.Context, req libevents.SearchEventsRequest) ([]apievents.AuditEvent, string, error)

// cachedSearchResult holds the result of a cached search operation.
type cachedSearchResult struct {
	Events       []apievents.AuditEvent
	LimitReached bool
	CacheHits    int
	CacheMisses  int
	CacheFiles   int
}

// cachedSearchEvents searches for audit events using the cache when available.
// It loads cached events for covered time ranges, fetches fresh events for any
// uncovered gaps, merges them together, and returns the combined result.
// Times are truncated to minute boundaries for stable cache keys.
func (c *eventCache) cachedSearchEvents(ctx context.Context, searchFn searchEventsFunc, from, to time.Time, eventType string, limit int) (cachedSearchResult, error) {
	// Truncate to minute boundaries so that repeated calls within the same
	// minute hit the cache instead of always finding a sub-second gap.
	from = from.Truncate(time.Minute)
	to = to.Truncate(time.Minute)

	// If cache is not configured, fall through to direct fetch.
	if c == nil || c.Dir == "" {
		events, limitReached, err := fetchAllPages(ctx, searchFn, from, to, eventType, limit)
		if err != nil {
			return cachedSearchResult{}, trace.Wrap(err)
		}
		return cachedSearchResult{
			Events:       events,
			LimitReached: limitReached,
			CacheMisses:  len(events),
		}, nil
	}

	// List cache files for this event type.
	files, err := c.listCacheFiles(eventType)
	if err != nil {
		slog.WarnContext(ctx, "Failed to list cache files, falling back to direct fetch", "event_type", eventType, "error", err)
		events, limitReached, err := fetchAllPages(ctx, searchFn, from, to, eventType, limit)
		if err != nil {
			return cachedSearchResult{}, trace.Wrap(err)
		}
		return cachedSearchResult{
			Events:       events,
			LimitReached: limitReached,
			CacheMisses:  len(events),
		}, nil
	}

	// Compute uncovered gaps.
	gaps := uncoveredRanges(from, to, files)

	// Load cached events from overlapping files.
	var cachedEvents []apievents.AuditEvent
	cacheFilesRead := 0
	for _, f := range files {
		fileFrom := f.Header.From
		fileTo := f.Header.To

		// Skip files that don't overlap with [from, to].
		if fileTo.Before(from) || fileFrom.After(to) {
			continue
		}

		events, err := readCacheEvents(f.Path)
		if err != nil {
			slog.WarnContext(ctx, "Skipping unreadable cache file", "path", f.Path, "error", err)
			continue
		}
		filtered := filterEventsInRange(events, from, to)
		cachedEvents = append(cachedEvents, filtered...)
		cacheFilesRead++
	}

	// Fetch fresh events for each gap — always fetch everything (no limit)
	// so the cache captures the full range.
	var freshEvents []apievents.AuditEvent
	for _, gap := range gaps {
		slog.DebugContext(ctx, "Fetching uncached range", "event_type", eventType, "from", gap.From, "to", gap.To)
		events, _, err := fetchAllPages(ctx, searchFn, gap.From, gap.To, eventType, 0)
		if err != nil {
			return cachedSearchResult{}, trace.Wrap(err)
		}
		freshEvents = append(freshEvents, events...)
	}

	// Merge and rewrite cache if we have fresh events or gaps.
	if len(freshEvents) > 0 || len(gaps) > 0 {
		_, err := c.mergeAndRewrite(eventType, from, to, freshEvents)
		if err != nil {
			slog.WarnContext(ctx, "Failed to merge cache files", "event_type", eventType, "error", err)
		}
	}

	// Combine cached + fresh events, deduplicate, sort descending by time.
	allEvents := append(cachedEvents, freshEvents...)
	allEvents = deduplicateEvents(allEvents)
	sort.SliceStable(allEvents, func(i, j int) bool {
		return allEvents[i].GetTime().After(allEvents[j].GetTime())
	})

	// Apply limit (limit <= 0 means unlimited).
	limitReached := false
	if limit > 0 && len(allEvents) > limit {
		allEvents = allEvents[:limit]
		limitReached = true
	}

	// Accumulate stats.
	c.totalHits += len(cachedEvents)
	c.totalMisses += len(freshEvents)
	c.totalFiles += cacheFilesRead

	return cachedSearchResult{
		Events:       allEvents,
		LimitReached: limitReached,
		CacheHits:    len(cachedEvents),
		CacheMisses:  len(freshEvents),
		CacheFiles:   cacheFilesRead,
	}, nil
}

// fetchAllPages performs a standard SearchEvents pagination loop, fetching up to
// limit events using auditEventPageSize per page. Use limit <= 0 for unlimited.
func fetchAllPages(ctx context.Context, searchFn searchEventsFunc, from, to time.Time, eventType string, limit int) ([]apievents.AuditEvent, bool, error) {
	unlimited := limit <= 0
	if unlimited {
		limit = math.MaxInt
	}
	allEvents := make([]apievents.AuditEvent, 0, min(limit, 10000))
	var startKey string
	pages := 0
	for len(allEvents) < limit {
		requestLimit := limit - len(allEvents)
		if requestLimit > auditEventPageSize {
			requestLimit = auditEventPageSize
		}
		pageEvents, nextKey, err := searchFn(ctx, libevents.SearchEventsRequest{
			From:       from,
			To:         to,
			EventTypes: []string{eventType},
			Limit:      requestLimit,
			Order:      types.EventOrderDescending,
			StartKey:   startKey,
		})
		if err != nil {
			return nil, false, trace.Wrap(err)
		}
		pages++
		allEvents = append(allEvents, pageEvents...)
		slog.DebugContext(ctx, "Fetched audit event page", "event_type", eventType, "page", pages, "page_size", len(pageEvents), "total_so_far", len(allEvents))
		if nextKey == "" || len(pageEvents) == 0 {
			break
		}
		startKey = nextKey
	}
	return allEvents, !unlimited && len(allEvents) >= limit, nil
}

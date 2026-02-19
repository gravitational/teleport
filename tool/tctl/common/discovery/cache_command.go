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
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"


	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/trace"
)

func (c *Command) runCacheLoad(ctx context.Context, client discoveryClient) error {
	if c.cache == nil {
		return trace.BadParameter("cache not initialized (could not resolve cluster name)")
	}

	from, to, err := resolveTimeRangeFromFlags(c.cacheLoadLast, c.cacheLoadFromUTC, c.cacheLoadToUTC)
	if err != nil {
		return trace.Wrap(err)
	}

	w := c.output()
	for _, eventType := range []string{libevents.SSMRunEvent, libevents.InstanceJoinEvent} {
		fmt.Fprintf(w, "Loading %s events for %s ...\n", eventType, timeRangeDescriptionFromFlags(c.cacheLoadLast, c.cacheLoadFromUTC, c.cacheLoadToUTC))
		result, err := c.cache.cachedSearchEvents(ctx, client.SearchEvents, from, to, eventType, 0)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Fprintf(w, "  %d events (%d cached, %d fetched)\n", len(result.Events), result.CacheHits, result.CacheMisses)
	}
	fmt.Fprintf(w, "\nCache directory: %s\n", c.cache.Dir)
	return nil
}

func (c *Command) runCacheStatus(_ context.Context, _ discoveryClient) error {
	if c.cache == nil {
		return trace.BadParameter("cache not initialized (could not resolve cluster name)")
	}

	w := c.output()
	style := newTextStyle(w)
	now := time.Now().UTC()

	fmt.Fprintf(w, "Cache directory: %s\n", c.cache.Dir)

	var allFiles []cacheFile
	for _, eventType := range []string{libevents.SSMRunEvent, libevents.InstanceJoinEvent} {
		files, err := c.cache.listCacheFiles(eventType)
		if err != nil {
			fmt.Fprintf(w, "\n%s\n", style.warning(fmt.Sprintf("Error listing %s cache files: %v", eventType, err)))
			continue
		}
		allFiles = append(allFiles, files...)
	}

	if len(allFiles) == 0 {
		fmt.Fprintf(w, "\n%s\n", style.warning("No cached files."))
		return nil
	}

	fmt.Fprintf(w, "\n%s\n", style.section(fmt.Sprintf("Cached Files [%d]", len(allFiles))))
	for i, f := range allFiles {
		if i > 0 {
			fmt.Fprintln(w)
		}
		h := f.Header
		details := []keyValue{
			{Key: "EVENT TYPE", Value: style.section(h.EventType)},
			{Key: "FROM", Value: h.From.Format(cacheTimeFormat)},
			{Key: "TO", Value: h.To.Format(cacheTimeFormat)},
			{Key: "EVENTS", Value: style.good(fmt.Sprintf("%d", h.Count))},
			{Key: "FETCHED", Value: fmt.Sprintf("%s (%s)", h.FetchedAt.Format(time.RFC3339), formatRelativeTime(h.FetchedAt, now))},
			{Key: "FILE", Value: filepath.Base(f.Path)},
		}
		if err := style.numberedBlock(w, i, details); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *Command) runCachePrune(_ context.Context, _ discoveryClient) error {
	if c.cache == nil {
		return trace.BadParameter("cache not initialized (could not resolve cluster name)")
	}

	entries, err := os.ReadDir(c.cache.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(c.output(), "No cache directory found at %s\n", c.cache.Dir)
			return nil
		}
		return trace.Wrap(err)
	}

	removed := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		if err := os.Remove(filepath.Join(c.cache.Dir, e.Name())); err != nil {
			slog.Warn("Failed to remove cache file", "file", e.Name(), "error", err)
			continue
		}
		removed++
	}

	fmt.Fprintf(c.output(), "Removed %d cache file(s) from %s\n", removed, c.cache.Dir)
	return nil
}

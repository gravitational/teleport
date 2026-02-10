/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package test

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	apitypes "github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
)

// WatchEventsHeightVolumeConfig contains configuration for high-volume watcher tests
type WatchEventsHeightVolumeConfig struct {
	// NumEvents is the total number of events to write
	NumEvents int
	// NumWriters is the number of concurrent goroutines writing events
	NumWriters int
	// ItemSize is the size of each item's value in bytes
	ItemSize int
	// EventTimeout is how long to wait for events with no activity before stopping
	EventTimeout time.Duration
}

func (c *WatchEventsHeightVolumeConfig) checkAndSetDefaults() error {
	if c.NumEvents <= 0 {
		c.NumEvents = 1000
	}
	if c.NumWriters <= 0 {
		c.NumWriters = 10
	}
	if c.ItemSize <= 0 {
		c.ItemSize = 1024 * 400 // 400KB
	}
	if c.EventTimeout <= 0 {
		c.EventTimeout = 30 * time.Second
	}
	if c.NumEvents%c.NumWriters != 0 {
		return trace.BadParameter("NumEvents (%d) must be divisible by NumWriters (%d)", c.NumEvents, c.NumWriters)
	}
	return nil
}

// RunWatchEventsHeightVolume tests that a watcher can reliably receive all events
// when the backend is under high write load. This is particularly useful for
// testing backends with complex event streaming mechanisms (e.g., DynamoDB Streams
// with shard splits, or other distributed event systems).
func RunWatchEventsHeightVolume(t *testing.T, newBackend Constructor, cfg WatchEventsHeightVolumeConfig) {
	require.NoError(t, cfg.checkAndSetDefaults())

	uut, _, err := newBackend()
	require.NoError(t, err)
	defer func() { require.NoError(t, uut.Close()) }()

	prefix := MakePrefix()

	watcher, err := uut.NewWatcher(t.Context(), backend.Watch{Prefixes: []backend.Key{prefix("")}})
	require.NoError(t, err)
	defer func() { require.NoError(t, watcher.Close()) }()

	select {
	case event := <-watcher.Events():
		require.Equal(t, apitypes.OpInit, event.Type)
		t.Log("Watcher initialized")
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for watcher init")
	}

	tracker := newEventTracker()
	go func() {
		for {
			select {
			case event := <-watcher.Events():
				if event.Type == apitypes.OpPut {
					tracker.registerReceivedEvent(t, event.Item.Key.String())
				}
			case <-t.Context().Done():
				return
			}
		}
	}()

	t.Logf("Writing %d events with %d concurrent writers (item size: %d bytes)", cfg.NumEvents, cfg.NumWriters, cfg.ItemSize)
	writeHighVolumeEvents(t, uut, tracker, prefix, cfg)
	t.Logf("All writers finished")

	require.NoError(t, waitForAllEvents(t, tracker, cfg.EventTimeout))

	res := tracker.getProgress()
	t.Logf("Final progress: received %d/%d events, last event received %v ago",
		res.receivedCount, res.writtenCount, res.timeSinceLastEvent.Round(time.Second))
	require.Equal(t, res.writtenCount, res.receivedCount, "Not all events were received")
}

// writeHighVolumeEvents writes events concurrently to the backend
func writeHighVolumeEvents(t *testing.T, b backend.Backend, tracker *eventTracker, prefix prefixFunc, cfg WatchEventsHeightVolumeConfig) {
	eventsPerWriter := cfg.NumEvents / cfg.NumWriters
	payload := strings.Repeat("A", cfg.ItemSize)

	var wg errgroup.Group

	wg.SetLimit(cfg.NumWriters)

	for w := 0; w < cfg.NumWriters; w++ {
		wg.Go(func() error {
			return writeHighVolumeEventBatch(t, b, tracker, prefix, w, eventsPerWriter, payload)
		})
	}
	require.NoError(t, wg.Wait())

}

// writeItemWithRetry attempts to write an item with simple retry logic
func writeItemWithRetry(t *testing.T, b backend.Backend, item backend.Item) error {
	ctx := t.Context()
	for {
		if _, err := b.Put(ctx, item); err != nil {
			var throttlingErr *dynamodbtypes.ThrottlingException
			if errors.As(err, &throttlingErr) {
				// Tootling is a transient error, and we want to
				// write all events.
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(time.Second * 10):
					continue
				}
			}
			return trace.Wrap(err)
		}
		return nil
	}
}

type prefixFunc func(components ...string) backend.Key

// writeHighVolumeEventBatch writes a batch of events from a single writer
func writeHighVolumeEventBatch(t *testing.T, b backend.Backend, tracker *eventTracker, prefix prefixFunc, writerID, eventsPerWriter int, payload string) error {
	start := writerID * eventsPerWriter
	end := start + eventsPerWriter

	for i := start; i < end; i++ {
		item := backend.Item{
			Key:   prefix(fmt.Sprintf("event-%d", i)),
			Value: []byte(payload),
		}

		if err := writeItemWithRetry(t, b, item); err != nil {
			return trace.Wrap(err)
		}

		tracker.registerWrittenEvent(item.Key.String())

		if (i-start)%100 == 0 && i > start {
			t.Logf("Writer %d: wrote %d events", writerID, i-start)
		}
	}

	t.Logf("Writer %d completed: wrote %d events", writerID, end-start)
	return nil
}

func newEventTracker() *eventTracker {
	return &eventTracker{
		writtenEvents:  make(map[string]struct{}),
		receivedEvents: make(map[string]struct{}),
		lastEventTime:  time.Now(),
	}
}

// eventTracker tracks written and received events for comparison
type eventTracker struct {
	writtenEvents  map[string]struct{}
	receivedEvents map[string]struct{}
	mu             sync.Mutex
	receivedCount  int
	lastEventTime  time.Time
}

func (e *eventTracker) registerReceivedEvent(t *testing.T, keyStr string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	_, ok := e.receivedEvents[keyStr]
	require.False(t, ok, "Received duplicate event: %s", keyStr)

	e.receivedEvents[keyStr] = struct{}{}
	e.receivedCount++
	e.lastEventTime = time.Now()

	if e.receivedCount%100 == 0 {
		t.Logf("Received %d events so far", e.receivedCount)
	}
}

func (e *eventTracker) registerWrittenEvent(keyStr string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.writtenEvents[keyStr] = struct{}{}
}

type progressSnapshot struct {
	receivedCount      int
	writtenCount       int
	timeSinceLastEvent time.Duration
}

func (e *eventTracker) getProgress() progressSnapshot {
	e.mu.Lock()
	defer e.mu.Unlock()
	return progressSnapshot{
		receivedCount:      len(e.receivedEvents),
		writtenCount:       len(e.writtenEvents),
		timeSinceLastEvent: time.Since(e.lastEventTime),
	}
}

// waitForEvents waits for all events to be received or timeout
func waitForAllEvents(t *testing.T, e *eventTracker, timeout time.Duration) error {
	t.Log("Waiting for events to propagate")

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-t.Context().Done():
			return t.Context().Err()
		case <-ticker.C:
			progress := e.getProgress()
			if progress.receivedCount == progress.writtenCount {
				return nil
			}

			t.Logf("Waiting for events... received %d/%d (%.1f%%), last event %v ago",
				progress.receivedCount, progress.writtenCount,
				float64(progress.receivedCount)/float64(progress.writtenCount)*100,
				progress.timeSinceLastEvent.Round(time.Second))

			if progress.receivedCount >= progress.writtenCount {
				return trace.BadParameter("all events received: %d/%d", progress.receivedCount, progress.writtenCount)
			}

			if progress.timeSinceLastEvent > timeout {
				return trace.NotFound("timeout waiting for events: received %d/%d, last event received %v ago",
					progress.receivedCount, progress.writtenCount, progress.timeSinceLastEvent.Round(time.Second))
			}

		}
	}
}

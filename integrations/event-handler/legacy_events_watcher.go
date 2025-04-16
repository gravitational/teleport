/*
Copyright 2015-2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/time/rate"

	auditlogpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/auditlog/v1"
	"github.com/gravitational/teleport/api/types"
)

const (
	// lockMessage represents a message added to Lock when user is auto-locked
	lockMessage = "User is locked due to too many failed login attempts"
)

// LegacyCursorValue represents the cursor data used by the LegacyEventsWatcher to
// resume from where it left off.
type LegacyCursorValues struct {
	Cursor          string
	ID              string
	WindowStartTime time.Time
}

// IsEmpty returns true if all cursor values are empty.
func (c *LegacyCursorValues) IsEmpty() bool {
	return c.Cursor == "" && c.ID == "" && c.WindowStartTime.IsZero()
}

func (c *LegacyCursorValues) Equals(other LegacyCursorValues) bool {
	return c.Cursor == other.Cursor && c.ID == other.ID && c.WindowStartTime.Equal(other.WindowStartTime)
}

// LegacyEventsWatcher represents wrapper around Teleport client to work with events
type LegacyEventsWatcher struct {
	// client is an instance of GRPC Teleport client
	client TeleportSearchEventsClient
	// cursor current page cursor value
	cursor string
	// nextCursor next page cursor value
	nextCursor string
	// id latest event returned by Next()
	id string
	// pos current virtual cursor position within a batch
	pos int
	// batch current events batch
	batch []*LegacyTeleportEvent
	// config is teleport config
	config *StartCmdConfig
	// export is the callback that is invoked for each event. it is retried indefinitely
	// until it returns nil.
	export func(context.Context, *TeleportEvent) error

	log *slog.Logger

	// exportedCursor is a pointer to the cursor values of the most recently
	// exported event. the values mirror above fields, but those are only accessible
	// to the main event processing goroutine. these values are meant to be read
	// by concurrently.
	exportedCursor atomic.Pointer[LegacyCursorValues]

	// windowStartTime is event time frame start
	windowStartTime   time.Time
	windowStartTimeMu sync.Mutex
}

// NewLegacyEventsWatcher builds Teleport client instance
func NewLegacyEventsWatcher(
	c *StartCmdConfig,
	client TeleportSearchEventsClient,
	cursorValues LegacyCursorValues,
	export func(context.Context, *TeleportEvent) error,
	log *slog.Logger,
) *LegacyEventsWatcher {
	w := &LegacyEventsWatcher{
		client:          client,
		pos:             -1,
		cursor:          cursorValues.Cursor,
		config:          c,
		export:          export,
		id:              cursorValues.ID,
		windowStartTime: cursorValues.WindowStartTime,
		log:             log,
	}

	w.exportedCursor.Store(&cursorValues)

	return w
}

// Close closes connection to Teleport
func (t *LegacyEventsWatcher) Close() {
	t.client.Close()
}

func (t *LegacyEventsWatcher) GetCursorValues() LegacyCursorValues {
	// exported cursor values ptr is never nil
	return *t.exportedCursor.Load()
}

// flipPage flips the current page
func (t *LegacyEventsWatcher) flipPage(ctx context.Context) bool {
	if t.nextCursor == "" {
		t.log.DebugContext(ctx, "not flipping page (no next cursor)")
		return false
	}

	t.log.DebugContext(ctx, "flipping page", "cursor", t.cursor, "next", t.nextCursor)

	t.cursor = t.nextCursor
	t.pos = -1
	t.batch = make([]*LegacyTeleportEvent, 0)

	return true
}

// fetch fetches the page and sets the position to the event after latest known
func (t *LegacyEventsWatcher) fetch(ctx context.Context) error {
	// Zero batch
	t.batch = make([]*LegacyTeleportEvent, 0, t.config.BatchSize)
	nextCursor, err := t.getEvents(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// Save next cursor
	t.nextCursor = nextCursor

	// Mark position as unresolved (the page is empty)
	t.pos = -1

	t.log.DebugContext(
		ctx, "Fetched page",
		"cursor", t.cursor,
		"next", nextCursor,
		"len", len(t.batch),
	)

	// Page is empty: do nothing, return
	if len(t.batch) == 0 {
		t.pos = 0
		return nil
	}

	pos := 0

	// If last known id is not empty, let's try to find its pos
	if t.id != "" {
		for i, e := range t.batch {
			if e.ID == t.id {
				pos = i + 1
			}
		}
	}

	// Set the position of the last known event
	t.pos = pos

	if pos == 0 {
		t.log.DebugContext(ctx, "starting from first event in fetch", "id", t.id, "pos", pos)
	} else {
		t.log.DebugContext(ctx, "advancing past last known event in fetch", "id", t.id, "pos", pos)
	}

	return nil
}

// getEvents iterates over the range of days between the last windowStartTime and now.
// It returns a slice of events, a cursor for the next page and an error.
// If the cursor is out of the range, it advances the windowStartTime to the next day.
// It only advances the windowStartTime if no events are found until the last complete day.
func (t *LegacyEventsWatcher) getEvents(ctx context.Context) (string, error) {
	wst := t.getWindowStartTime()
	rangeSplitByDay := splitRangeByDay(wst, time.Now().UTC(), t.config.WindowSize)
	for i := 1; i < len(rangeSplitByDay); i++ {
		startTime := rangeSplitByDay[i-1]
		endTime := rangeSplitByDay[i]
		t.log.DebugContext(ctx, "Fetching events", "from", startTime, "to", endTime)
		evts, cursor, err := t.getEventsInWindow(ctx, startTime, endTime)
		if err != nil {
			return "", trace.Wrap(err)
		}

		// Convert batch to TeleportEvent
		for _, e := range evts {
			if _, ok := t.config.SkipEventTypes[e.Type]; ok {
				t.log.DebugContext(ctx, "Skipping event", "event", e)
				continue
			}
			evt, err := NewLegacyTeleportEvent(e, t.cursor, wst)
			if err != nil {
				return "", trace.Wrap(err)
			}

			t.batch = append(t.batch, evt)
		}

		// if no events are found, the cursor is out of the range [startTime, endTime]
		// and it's the last complete day, update start time to the next day.
		if t.canSkipToNextWindow(i, rangeSplitByDay, cursor) {
			t.log.InfoContext(
				ctx, "No new events found for the range",
				"from", startTime,
				"to", endTime,
			)
			t.setWindowStartTime(endTime)
			continue
		}
		// if any events are found, return them
		return cursor, nil
	}
	return t.cursor, nil
}

func (t *LegacyEventsWatcher) canSkipToNextWindow(i int, rangeSplitByDay []time.Time, cursor string) bool {
	if cursor != "" {
		return false

	}
	if len(t.batch) == 0 && i < len(rangeSplitByDay)-1 {
		t.log.InfoContext(
			context.TODO(), "No events found for the range",
			"from", rangeSplitByDay[i-1],
			"to", rangeSplitByDay[i],
		)
		return true
	}
	pos := 0
	// If last known id is not empty, let's try to find if all events are already processed
	// and if we can skip to next page
	if t.id != "" {
		for i, e := range t.batch {
			if e.ID == t.id {
				pos = i + 1
			}
		}
	}

	if i < len(rangeSplitByDay)-1 && pos >= len(t.batch) {
		t.log.InfoContext(
			context.TODO(), "No new events found for the range",
			"from", rangeSplitByDay[i-1],
			"to", rangeSplitByDay[i],
			"pos", pos,
			"len", len(t.batch),
		)
		return true
	}
	return false
}

// getEvents calls Teleport client and loads events from the audit log.
// It returns a slice of events, a cursor for the next page and an error.
func (t *LegacyEventsWatcher) getEventsInWindow(ctx context.Context, from, to time.Time) ([]*auditlogpb.EventUnstructured, string, error) {
	evts, cursor, err := t.client.SearchUnstructuredEvents(
		ctx,
		from,
		to,
		"default",
		t.config.Types,
		t.config.BatchSize,
		types.EventOrderAscending,
		t.cursor,
	)
	return evts, cursor, trace.Wrap(err)
}

func splitRangeByDay(from, to time.Time, windowSize time.Duration) []time.Time {
	// splitRangeByDay splits the range into days
	var days []time.Time
	for d := from; d.Before(to); d = d.Add(windowSize) {
		days = append(days, d)
	}
	return append(days, to) // add the last date
}

// pause sleeps for timeout seconds
func (t *LegacyEventsWatcher) pause(ctx context.Context) error {
	t.log.DebugContext(
		ctx, "No new events, pausing",
		"pause_time", t.config.Timeout,
	)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(t.config.Timeout):
		return nil
	}
}

// ExportEvents exports events from Teleport to the export function provided on initialization. The atomic
// cursor value is updated after each successful export call, and failed export calls are retried indefinitely.
func (t *LegacyEventsWatcher) ExportEvents(ctx context.Context) error {
	ch := make(chan *LegacyTeleportEvent, t.config.BatchSize)
	e := make(chan error, 1)

	go func() {
		defer close(e)

		logLimiter := rate.NewLimiter(rate.Every(time.Minute), 6)

		for {
			// If there is nothing in the batch, request
			if len(t.batch) == 0 {
				t.log.DebugContext(ctx, "fetching due to empty batch...")
				err := t.fetch(ctx)
				if err != nil {
					e <- trace.Wrap(err)
					break
				}

				// If there is still nothing, sleep
				if len(t.batch) == 0 && t.nextCursor == "" {
					if t.config.ExitOnLastEvent {
						t.log.InfoContext(ctx, "All events are processed, exiting...")
						break
					}

					err := t.pause(ctx)
					if err != nil {
						e <- trace.Wrap(err)
						break
					}

					continue
				}
			}

			// If we processed the last event on a page
			if t.pos >= len(t.batch) {
				// If there is next page, flip page
				if t.flipPage(ctx) {
					continue
				}

				// If not, update current page
				err := t.fetch(ctx)
				if err != nil {
					e <- trace.Wrap(err)
					break
				}

				// If there is still nothing new on current page, sleep
				if t.pos >= len(t.batch) {
					if t.config.ExitOnLastEvent && t.nextCursor == "" {
						t.log.InfoContext(ctx, "All events are processed, exiting...")
						break
					}

					err := t.pause(ctx)
					if err != nil {
						e <- trace.Wrap(err)
						break
					}

					continue
				}
			}

			event := t.batch[t.pos]
			t.pos++
			t.id = event.ID

			// attempt non-blocking send first, falling back to blocking send
			// if we encounter backpressure.
			select {
			case ch <- event:
				continue
			default:
			}

			if logLimiter.Allow() {
				t.log.WarnContext(ctx, "Encountering backpressure from outbound event processing")
			}

			select {
			case ch <- event:
			case <-ctx.Done():
				e <- ctx.Err()
				return
			}
		}
	}()

	for {
		select {
		case evt := <-ch:
		Export:
			for {
				// retry export of event indefinitely until event export either succeeds or
				// exporter is closed.
				err := t.export(ctx, evt.TeleportEvent)
				if err == nil {
					break Export
				}

				t.log.ErrorContext(ctx, "Failed to export event, retrying...", "error", err)
				select {
				case <-ctx.Done():
					return trace.Wrap(ctx.Err())
				case <-time.After(5 * time.Second): // TODO(fspmarshall): add real backoff
				}
			}
			// store updated cursor values after successful export
			t.exportedCursor.Store(&LegacyCursorValues{
				Cursor:          evt.Cursor,
				ID:              evt.ID,
				WindowStartTime: evt.Window,
			})
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		case err := <-e:
			return trace.Wrap(err)
		}
	}
}

func (t *LegacyEventsWatcher) getWindowStartTime() time.Time {
	t.windowStartTimeMu.Lock()
	defer t.windowStartTimeMu.Unlock()
	return t.windowStartTime
}

func (t *LegacyEventsWatcher) setWindowStartTime(time time.Time) {
	t.windowStartTimeMu.Lock()
	defer t.windowStartTimeMu.Unlock()
	t.windowStartTime = time
}

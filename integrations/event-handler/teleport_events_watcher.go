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
	"fmt"
	"sync"
	"time"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/time/rate"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	auditlogpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/auditlog/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/credentials"
	"github.com/gravitational/teleport/integrations/lib/logger"
)

const (
	// lockMessage represents a message added to Lock when user is auto-locked
	lockMessage = "User is locked due to too many failed login attempts"
)

// TeleportSearchEventsClient is an interface for client.Client, required for testing
type TeleportSearchEventsClient interface {
	// SearchEvents searches for events in the audit log and returns them using their protobuf representation.
	SearchEvents(ctx context.Context, fromUTC, toUTC time.Time, namespace string, eventTypes []string, limit int, order types.EventOrder, startKey string) ([]events.AuditEvent, string, error)
	// StreamSessionEvents returns session events stream for a given session ID using their protobuf representation.
	StreamSessionEvents(ctx context.Context, sessionID string, startIndex int64) (chan events.AuditEvent, chan error)
	// SearchUnstructuredEvents searches for events in the audit log and returns them using an unstructured representation (structpb.Struct).
	SearchUnstructuredEvents(ctx context.Context, fromUTC, toUTC time.Time, namespace string, eventTypes []string, limit int, order types.EventOrder, startKey string) ([]*auditlogpb.EventUnstructured, string, error)
	// StreamUnstructuredSessionEvents returns session events stream for a given session ID using an unstructured representation (structpb.Struct).
	StreamUnstructuredSessionEvents(ctx context.Context, sessionID string, startIndex int64) (chan *auditlogpb.EventUnstructured, chan error)
	UpsertLock(ctx context.Context, lock types.Lock) error
	Ping(ctx context.Context) (proto.PingResponse, error)
	Close() error
}

// TeleportEventsWatcher represents wrapper around Teleport client to work with events
type TeleportEventsWatcher struct {
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
	batch []*TeleportEvent
	// config is teleport config
	config *StartCmdConfig

	// windowStartTime is event time frame start
	windowStartTime   time.Time
	windowStartTimeMu sync.Mutex
}

// NewTeleportEventsWatcher builds Teleport client instance
func NewTeleportEventsWatcher(
	ctx context.Context,
	c *StartCmdConfig,
	windowStartTime time.Time,
	cursor string,
	id string,
) (*TeleportEventsWatcher, error) {
	var creds []client.Credentials
	switch {
	case c.TeleportIdentityFile != "" && !c.TeleportRefreshEnabled:
		creds = []client.Credentials{client.LoadIdentityFile(c.TeleportIdentityFile)}
	case c.TeleportIdentityFile != "" && c.TeleportRefreshEnabled:
		cred, err := lib.NewIdentityFileWatcher(ctx, c.TeleportIdentityFile, c.TeleportRefreshInterval)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		creds = []client.Credentials{cred}
	case c.TeleportCert != "" && c.TeleportKey != "" && c.TeleportCA != "":
		creds = []client.Credentials{client.LoadKeyPair(c.TeleportCert, c.TeleportKey, c.TeleportCA)}
	default:
		return nil, trace.BadParameter("no credentials configured")
	}

	if validCred, err := credentials.CheckIfExpired(creds); err != nil {
		log.Warn(err)
		if !validCred {
			return nil, trace.BadParameter(
				"No valid credentials found, this likely means credentials are expired. In this case, please sign new credentials and increase their TTL if needed.",
			)
		}
		log.Info("At least one non-expired credential has been found, continuing startup")
	}

	config := client.Config{
		Addrs:       []string{c.TeleportAddr},
		Credentials: creds,
	}

	teleportClient, err := client.New(ctx, config)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tc := TeleportEventsWatcher{
		client:          teleportClient,
		pos:             -1,
		cursor:          cursor,
		config:          c,
		id:              id,
		windowStartTime: windowStartTime,
	}

	return &tc, nil
}

// Close closes connection to Teleport
func (t *TeleportEventsWatcher) Close() {
	t.client.Close()
}

// flipPage flips the current page
func (t *TeleportEventsWatcher) flipPage() bool {
	if t.nextCursor == "" {
		return false
	}

	t.cursor = t.nextCursor
	t.pos = -1
	t.batch = make([]*TeleportEvent, 0)

	return true
}

// fetch fetches the page and sets the position to the event after latest known
func (t *TeleportEventsWatcher) fetch(ctx context.Context) error {
	log := logger.Get(ctx)
	// Zero batch
	t.batch = make([]*TeleportEvent, 0, t.config.BatchSize)
	nextCursor, err := t.getEvents(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// Save next cursor
	t.nextCursor = nextCursor

	// Mark position as unresolved (the page is empty)
	t.pos = -1

	log.WithField("cursor", t.cursor).WithField("next", nextCursor).WithField("len", len(t.batch)).Debug("Fetched page")

	// Page is empty: do nothing, return
	if len(t.batch) == 0 {
		t.pos = 0
		return nil
	}

	pos := 0

	// If last known id is not empty, let's try to find it's pos
	if t.id != "" {
		for i, e := range t.batch {
			if e.ID == t.id {
				pos = i + 1
				t.id = e.ID
			}
		}
	}

	// Set the position of the last known event
	t.pos = pos

	log.WithField("id", t.id).WithField("new_pos", t.pos).Debug("Skipping last known event")

	return nil
}

// getEvents iterates over the range of days between the last windowStartTime and now.
// It returns a slice of events, a cursor for the next page and an error.
// If the cursor is out of the range, it advances the windowStartTime to the next day.
// It only advances the windowStartTime if no events are found until the last complete day.
func (t *TeleportEventsWatcher) getEvents(ctx context.Context) (string, error) {
	rangeSplitByDay := splitRangeByDay(t.getWindowStartTime(), time.Now().UTC(), t.config.WindowSize)
	for i := 1; i < len(rangeSplitByDay); i++ {
		startTime := rangeSplitByDay[i-1]
		endTime := rangeSplitByDay[i]
		log.Debugf("Fetching events from %v to %v", startTime, endTime)
		evts, cursor, err := t.getEventsInWindow(ctx, startTime, endTime)
		if err != nil {
			return "", trace.Wrap(err)
		}

		// Convert batch to TeleportEvent
		for _, e := range evts {
			if _, ok := t.config.SkipEventTypes[e.Type]; ok {
				log.WithField("event", e).Debug("Skipping event")
				continue
			}
			evt, err := NewTeleportEvent(e, t.cursor)
			if err != nil {
				return "", trace.Wrap(err)
			}

			t.batch = append(t.batch, evt)
		}

		// if no events are found, the cursor is out of the range [startTime, endTime]
		// and it's the last complete day, update start time to the next day.
		if t.canSkipToNextWindow(i, rangeSplitByDay, cursor) {
			log.Infof("No new events found for the range %v to %v", startTime, endTime)
			t.setWindowStartTime(endTime)
			continue
		}
		// if any events are found, return them
		return cursor, nil
	}
	return t.cursor, nil
}

func (t *TeleportEventsWatcher) canSkipToNextWindow(i int, rangeSplitByDay []time.Time, cursor string) bool {
	if cursor != "" {
		return false

	}
	if len(t.batch) == 0 && i < len(rangeSplitByDay)-1 {
		log.Infof("No events found for the range %v to %v", rangeSplitByDay[i-1], rangeSplitByDay[i])
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
		log.WithField("pos", pos).WithField("len", len(t.batch)).Infof("No new events found for the range %v to %v", rangeSplitByDay[i-1], rangeSplitByDay[i])
		return true
	}
	return false
}

// getEvents calls Teleport client and loads events from the audit log.
// It returns a slice of events, a cursor for the next page and an error.
func (t *TeleportEventsWatcher) getEventsInWindow(ctx context.Context, from, to time.Time) ([]*auditlogpb.EventUnstructured, string, error) {
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
func (t *TeleportEventsWatcher) pause(ctx context.Context) error {
	log := logger.Get(ctx)
	log.Debugf("No new events, pause for %v seconds", t.config.Timeout)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(t.config.Timeout):
		return nil
	}
}

// Next returns next event from a batch or requests next batch if it has been ended
func (t *TeleportEventsWatcher) Events(ctx context.Context) (chan *TeleportEvent, chan error) {
	ch := make(chan *TeleportEvent, t.config.BatchSize)
	e := make(chan error, 1)

	go func() {
		defer close(ch)
		defer close(e)

		logLimiter := rate.NewLimiter(rate.Every(time.Minute), 6)

		for {
			// If there is nothing in the batch, request
			if len(t.batch) == 0 {
				err := t.fetch(ctx)
				if err != nil {
					e <- trace.Wrap(err)
					break
				}

				// If there is still nothing, sleep
				if len(t.batch) == 0 && t.nextCursor == "" {
					if t.config.ExitOnLastEvent {
						log.Info("All events are processed, exiting...")
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
				if t.flipPage() {
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
						log.Info("All events are processed, exiting...")
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
				log.Warn("encountering backpressure from outbound event processing")
			}

			select {
			case ch <- event:
			case <-ctx.Done():
				e <- ctx.Err()
				return
			}
		}
	}()

	return ch, e
}

// StreamSessionEvents returns session event stream, that's the simple delegate to an API function
func (t *TeleportEventsWatcher) StreamUnstructuredSessionEvents(ctx context.Context, id string, index int64) (chan *auditlogpb.EventUnstructured, chan error) {
	return t.client.StreamUnstructuredSessionEvents(ctx, id, index)
}

// UpsertLock upserts user lock
func (t *TeleportEventsWatcher) UpsertLock(ctx context.Context, user string, login string, period time.Duration) error {
	var expires *time.Time

	if period > 0 {
		t := time.Now()
		t = t.Add(period)
		expires = &t
	}

	lock := &types.LockV2{
		Metadata: types.Metadata{
			Name: fmt.Sprintf("event-handler-auto-lock-%v-%v", user, login),
		},
		Spec: types.LockSpecV2{
			Target: types.LockTarget{
				Login: login,
				User:  user,
			},
			Message: lockMessage,
			Expires: expires,
		},
	}

	return t.client.UpsertLock(ctx, lock)
}

func (t *TeleportEventsWatcher) getWindowStartTime() time.Time {
	t.windowStartTimeMu.Lock()
	defer t.windowStartTimeMu.Unlock()
	return t.windowStartTime
}

func (t *TeleportEventsWatcher) setWindowStartTime(time time.Time) {
	t.windowStartTimeMu.Lock()
	defer t.windowStartTimeMu.Unlock()
	t.windowStartTime = time
}

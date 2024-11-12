// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	limiter "github.com/sethvargo/go-limiter"
	"github.com/sethvargo/go-limiter/memorystore"
	"google.golang.org/protobuf/types/known/timestamppb"

	auditlogpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/auditlog/v1"
	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/lib/events/export"
)

// EventsJob incapsulates audit log event consumption logic
type EventsJob struct {
	lib.ServiceJob
	app             *App
	rl              limiter.Store
	eventsProcessed atomic.Uint64
	targetDate      atomic.Pointer[time.Time]
}

// NewEventsJob creates new EventsJob structure
func NewEventsJob(app *App) *EventsJob {
	j := &EventsJob{app: app}
	j.ServiceJob = lib.NewServiceJob(j.run)
	return j
}

// run runs the event consumption logic
func (j *EventsJob) run(ctx context.Context) error {
	// Create cancellable context which handles app termination
	ctx, cancel := context.WithCancel(ctx)
	j.app.Process.OnTerminate(func(_ context.Context) error {
		cancel()
		return nil
	})

	// set up background logging of event processing rate
	go func() {
		logTicker := time.NewTicker(time.Minute)
		defer logTicker.Stop()

		for {
			select {
			case <-logTicker.C:
				ll := j.app.log.With("events_per_minute", j.eventsProcessed.Swap(0))
				if td := j.targetDate.Load(); td != nil {
					ll = ll.With("date", td.Format(time.DateOnly))
				}
				ll.InfoContext(ctx, "Event processing")
			case <-ctx.Done():
				return
			}
		}
	}()

	store, err := memorystore.New(&memorystore.Config{
		Tokens:   uint64(j.app.Config.LockFailedAttemptsCount),
		Interval: j.app.Config.LockPeriod,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	j.rl = store

	j.SetReady(true)
	defer j.app.Terminate()

	for {
		err := j.runPolling(ctx)
		if err == nil || ctx.Err() != nil {
			j.app.log.DebugContext(ctx, "Watch loop exiting")
			return trace.Wrap(err)
		}

		j.app.log.ErrorContext(
			ctx, "Unexpected error in watch loop. Reconnecting in 5s...",
			"error", err,
		)

		select {
		case <-time.After(time.Second * 5):
		case <-ctx.Done():
			return nil
		}
	}
}

// runPolling runs actual event queue polling
func (j *EventsJob) runPolling(ctx context.Context) error {
	cursorV2State := j.app.State.GetCursorV2State()
	if cursorV2State.IsEmpty() {
		// we haven't started using the newer bulk event export API yet. check if the upstream implements it by
		// performing a fake request. if it does not, we'll need to fallback to using the legacy API.
		chunks := j.app.client.GetEventExportChunks(ctx, &auditlogpb.GetEventExportChunksRequest{
			// target a date 2 days in the future to be confident that we're querying a valid but
			// empty date range, even in the context of reasonable clock drift.
			Date: timestamppb.New(time.Now().AddDate(0, 0, 2)),
		})

		if err := stream.Drain(chunks); err != nil {
			if trace.IsNotImplemented(err) {
				// fallback to legacy behavior
				return j.runLegacyPolling(ctx)
			}
			return trace.Wrap(err)
		}

		// the new API is implemented, check if there is preexisting legacy cursor state.
		legacyStates, err := j.app.State.GetLegacyCursorValues()
		if err != nil {
			return trace.Wrap(err)
		}

		if !legacyStates.IsEmpty() {
			// cursorV2 state isn't totally compatible, but we can skip ahead to the same target date as
			// was being tracked by the legacy cursor.
			cursorV2State.Dates[normalizeDate(legacyStates.WindowStartTime)] = export.DateExporterState{}
		}
	}

	startTime, err := j.app.State.GetStartTime()
	if err != nil {
		return trace.Wrap(err)
	}

	idleCh := make(chan struct{}, 1)

	// a minimum concurrency of 3 for the date exporter is enforced because 3 is the largest number of chunks we'd
	// expect to show up simultaneously due to normal ongoing activity. as concurrency is scaled up we apply a quarter
	// of the value of the global concurrency limit to the exporter. this has been shown in manual scale testing to be
	// a fairly optimal ratio relative to session processing concurrency (which uses the concurrency limit directly).
	// using the global concurrency limit directly results in a lot of stalled chunks because each chunk will typically
	// contain a large number of sessions that need to be processed.
	concurrency := max(j.app.Config.Concurrency/4, 3)

	exporter, err := export.NewExporter(export.ExporterConfig{
		Client:    j.app.client,
		StartDate: *startTime,
		Export:    j.handleEventV2,
		OnIdle: func(_ context.Context) {
			select {
			case idleCh <- struct{}{}:
			default:
			}
		},
		PreviousState: cursorV2State,
		Concurrency:   concurrency,
		BacklogSize:   2,
		MaxBackoff:    time.Second * 90,
		PollInterval:  time.Second * 16,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// the exporter manages retries internally once successfully created, so from here on out
	// we just want to periodically sync state to disk until the event exporter closes.
	cursorTicker := time.NewTicker(time.Millisecond * 666)
	defer cursorTicker.Stop()

	for {
		select {
		case <-cursorTicker.C:
			date := exporter.GetCurrentDate()
			j.targetDate.Store(&date)
			if err := j.app.State.SetCursorV2State(exporter.GetState()); err != nil {
				j.app.log.ErrorContext(ctx, "Failed to save cursor_v2 values, will retry", "error", err)
			}
		case <-ctx.Done():
			exporter.Close()
			<-exporter.Done()
			// optimistically attempt one last save
			j.app.State.SetCursorV2State(exporter.GetState())
			return nil
		case <-idleCh:
			// the exporter is idle, which means it has caught up to the present.
			if j.app.Config.ExitOnLastEvent {
				exporter.Close()
				<-exporter.Done()
				// optimistically attempt one last save
				j.app.State.SetCursorV2State(exporter.GetState())
				return nil
			}
		}
	}
}

// runLegacyPolling handles event processing via the old API.
func (j *EventsJob) runLegacyPolling(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	lc, err := j.app.State.GetLegacyCursorValues()
	if err != nil {
		return trace.Wrap(err)
	}

	if lc.IsEmpty() {
		st, err := j.app.State.GetStartTime()
		if err != nil {
			return trace.Wrap(err)
		}
		lc.WindowStartTime = *st
	}

	eventWatcher := NewLegacyEventsWatcher(
		j.app.Config, j.app.client, *lc, j.handleEvent, j.app.log,
	)

	// periodically sync cursor values to disk
	go func() {
		ticker := time.NewTicker(time.Millisecond * 666)
		defer ticker.Stop()

		lastCursorValues := *lc
		for {
			select {
			case <-ticker.C:
				currentCursorValues := eventWatcher.GetCursorValues()
				date := normalizeDate(currentCursorValues.WindowStartTime)
				j.targetDate.Store(&date)
				if currentCursorValues.Equals(lastCursorValues) {
					continue
				}
				if err := j.app.State.SetLegacyCursorValues(currentCursorValues); err != nil {
					j.app.log.ErrorContext(ctx, "Failed to save cursor values, will retry", "error", err)
					continue
				}
				lastCursorValues = currentCursorValues
			case <-ctx.Done():
				// optimistically attempt one last save. this is good practice, but note that
				// we don't promise not to emit duplicate events post-restart, so this we don't
				// bother checking for errors here.
				j.app.State.SetLegacyCursorValues(eventWatcher.GetCursorValues())
				return
			}
		}
	}()

	return eventWatcher.ExportEvents(ctx)
}

// handleEventV2 processes an event from the newer export API.
func (j *EventsJob) handleEventV2(ctx context.Context, evt *auditlogpb.ExportEventUnstructured) error {

	// filter out unwanted event types (in the v1 event export logic this was an internal behavior
	// of the event processing helper since it would perform conversion prior to storing the event
	// in its internal buffer).
	if _, ok := j.app.Config.SkipEventTypes[evt.Event.Type]; ok {
		return nil
	}

	// convert the event to teleport-event-exporter's internal representation
	event, err := NewTeleportEvent(evt.Event)
	if err != nil {
		return trace.Wrap(err)
	}

	// remaining handling logic is common to v1 and v2 event export
	return j.handleEvent(ctx, event)
}

// handleEvent processes an event
func (j *EventsJob) handleEvent(ctx context.Context, evt *TeleportEvent) error {
	// Send event to Teleport
	err := j.sendEvent(ctx, evt)
	if err != nil {
		return trace.Wrap(err)
	}

	// Start session ingestion if needed
	if evt.IsSessionEnd {
		j.app.RegisterSession(ctx, evt)
	}

	// If the event is login event
	if evt.IsFailedLogin {
		err := j.TryLockUser(ctx, evt)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	j.eventsProcessed.Add(1)

	return nil
}

// sendEvent sends an event to Teleport
func (j *EventsJob) sendEvent(ctx context.Context, evt *TeleportEvent) error {
	return j.app.SendEvent(ctx, j.app.Config.FluentdURL, evt)
}

// TryLockUser locks user if they exceeded failed attempts
func (j *EventsJob) TryLockUser(ctx context.Context, evt *TeleportEvent) error {
	if !j.app.Config.LockEnabled || j.app.Config.DryRun {
		return nil
	}

	_, _, _, ok, err := j.rl.Take(ctx, evt.FailedLoginData.Login)
	if err != nil {
		return trace.Wrap(err)
	}
	if ok {
		return nil
	}

	err = upsertLock(ctx, j.app.client, evt.FailedLoginData.User, evt.FailedLoginData.Login, j.app.Config.LockFor)
	if err != nil {
		return trace.Wrap(err)
	}

	j.app.log.InfoContext(ctx, "User login is locked", "data", evt.FailedLoginData)

	return nil
}

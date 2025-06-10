/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package export

import (
	"cmp"
	"context"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/time/rate"

	auditlogpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/auditlog/v1"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/utils/interval"
)

type ExporterState struct {
	// Dates is a map of dates to their respective state. Note that an empty
	// state for a date is still meaningful and either indicates that the date
	// itself contains no events, or that no progress has been made against that
	// date yet.
	Dates map[time.Time]DateExporterState
}

// IsEmpty returns true if no state is defined.
func (s *ExporterState) IsEmpty() bool {
	return len(s.Dates) == 0
}

// Clone creates a deep copy of the exporter state.
func (s *ExporterState) Clone() ExporterState {
	out := ExporterState{
		Dates: make(map[time.Time]DateExporterState, len(s.Dates)),
	}
	for date, state := range s.Dates {
		out.Dates[date] = state.Clone()
	}
	return out
}

// BulkExportResumeState contains the information used to update the resume state
// (specifically the Cursor and Completed flag) for a given Date and Chunk
// on an external system managing the overall bulk export process.
type BulkExportResumeState struct {
	// Date is the specific day for which the resume state is being updated,
	// expected to be normalized to UTC midnight (00:00:00).
	Date time.Time
	// Chunk identifies the specific data segment or partition (within the Date)
	// for which this resume state applies.
	Chunk string
	// Cursor marks the position (e.g., the next event ID or offset) within
	// the Chunk from where the export should resume.
	Cursor string
	// Completed indicates whether processing for this specific Date and Chunk
	// has been fully and successfully exported.
	Completed bool
}

// BatchExportConfig instructs the Exporter to use the
// BatchExportConfig.Callback with events batched up to MaxSize and a maximum
// delay of BatchExportConfig.MaxDelay between calls.
type BatchExportConfig struct {
	// Callback is used to export batched events. It must be safe for concurrent
	// use if the ExportConfig.Concurrency parameter is greater than 1.
	Callback func(ctx context.Context, events []*auditlogpb.EventUnstructured, resumeState BulkExportResumeState) error
	// MaxDelay specifies the maximum duration to wait before invoking the
	// Callback with a batch of events, even if MaxSize has not been reached.
	// If the zero value is provided for this field, a default duration of 5 *
	// time.Second will be used by the exporter.
	MaxDelay time.Duration
	// MaxSize defines the maximum total size in bytes of the events accumulated
	// in a single batch before the Callback is invoked. If the zero value is
	// provided for this field, a default size of 2 MB will be used by the
	// exporter.
	MaxSize int
}

// ExporterConfig configured an exporter.
type ExporterConfig struct {
	// Client is the audit event client used to fetch and export events.
	Client Client
	// StartDate is the date from which to start exporting events.
	StartDate time.Time

	// Export is the callback used to export events. Must be safe for concurrent use if
	// the Concurrency parameter is greater than 1.
	Export func(ctx context.Context, event *auditlogpb.ExportEventUnstructured) error
	// BatchExport is the callback with configuration used to export bulk events in batches.
	BatchExport *BatchExportConfig
	// OnIdle is an optional callback that gets invoked periodically when the exporter is idle. Note that it is
	// safe to close the exporter or inspect its state from within this callback, but waiting on the exporter's
	// Done channel within this callback will deadlock. This callback is an asynchronous signal and additional
	// events may be discovered concurrently with its invocation.
	OnIdle func(ctx context.Context)

	// PreviousState is an optional parameter used to resume from a previous date export run.
	PreviousState ExporterState

	// Concurrency sets the maximum number of event chunks that will be processed concurrently
	// for a given date (defaults to 1). Note that the total number of inflight chunk processing
	// may be up to Conurrency * (BacklogSize + 1).
	Concurrency int
	// BacklogSize optionally overrides the default size of the export backlog (i.e. the number of
	// previous dates for which polling continues after initial idleness). default is 1.
	BacklogSize int
	// MaxBackoff optionally overrides the default maximum backoff applied when errors are hit.
	MaxBackoff time.Duration
	// PollInterval optionally overrides the default poll interval used to fetch event chunks.
	PollInterval time.Duration
}

// CheckAndSetDefaults validates configuration and sets default values for optional parameters.
func (cfg *ExporterConfig) CheckAndSetDefaults() error {
	if cfg.Client == nil {
		return trace.BadParameter("missing required parameter Client in ExporterConfig")
	}
	if cfg.StartDate.IsZero() {
		return trace.BadParameter("missing required parameter StartDate in ExporterConfig")
	}
	if cfg.Export == nil && cfg.BatchExport == nil {
		return trace.BadParameter("missing required parameter Export or BatchExport in ExporterConfig")
	}
	if cfg.Export != nil && cfg.BatchExport != nil {
		return trace.BadParameter("only one of Export or BatchExport may be set in ExporterConfig")
	}
	if cfg.BatchExport != nil && cfg.BatchExport.Callback == nil {
		return trace.BadParameter("missing parameter BatchExport.Callback in ExporterConfig")
	}
	cfg.Concurrency = cmp.Or(cfg.Concurrency, 1)
	cfg.MaxBackoff = cmp.Or(cfg.MaxBackoff, 90*time.Second)
	cfg.PollInterval = cmp.Or(cfg.PollInterval, 16*time.Second)
	if cfg.BatchExport != nil {
		cfg.BatchExport.MaxDelay = cmp.Or(cfg.BatchExport.MaxDelay, 5*time.Second)
		cfg.BatchExport.MaxSize = cmp.Or(cfg.BatchExport.MaxSize, 2*1024*1024 /* 2MiB */)
	}
	return nil
}

// Exporter is a utility for exporting events starting from a given date using the chunked event export APIs. Note that
// it is specifically designed to prioritize performance and ensure that events aren't missed. Events may not be yielded
// in time order. Export of events is performed by consuming all currently available events for a given date, then moving
// to the next date. In order to account for replication delays, a backlog of previous dates are also polled.
type Exporter struct {
	cfg         ExporterConfig
	mu          sync.Mutex
	current     *DateExporter
	currentDate time.Time
	previous    map[time.Time]*DateExporter
	cancel      context.CancelFunc
	idle        chan struct{}
	done        chan struct{}
}

// NewExporter creates a new exporter and begins background processing of events. Processing will continue indefinitely
// until Exporter.Close is called.
func NewExporter(cfg ExporterConfig) (*Exporter, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	e := &Exporter{
		cfg:      cfg,
		cancel:   cancel,
		idle:     make(chan struct{}, 1),
		done:     make(chan struct{}),
		previous: make(map[time.Time]*DateExporter, len(cfg.PreviousState.Dates)),
	}

	// start initial event processing
	var initError error
	e.withLock(func() {
		var resumed int
		for date, state := range cfg.PreviousState.Dates {
			date = normalizeDate(date)
			if cfg.StartDate.After(date) {
				// skip dates that are older than the start date
				continue
			}
			if err := e.resumeExportLocked(ctx, date, state); err != nil {
				initError = err
				return
			}
			slog.InfoContext(ctx, "resumed event export", "date", date.Format(time.DateOnly))
			resumed++
		}

		if resumed == 0 {
			// no previous state at/after start date, start at the beginning
			if err := e.startExportLocked(ctx, cfg.StartDate); err != nil {
				initError = err
				return
			}
			slog.InfoContext(ctx, "started event export", "date", cfg.StartDate.Format(time.DateOnly))
		}
	})
	if initError != nil {
		e.Close()
		return nil, trace.Wrap(initError)
	}

	go e.run(ctx)
	return e, nil

}

// Close terminates all event processing. Note that shutdown is asynchronous. Any operation that needs to wait for export to fully
// terminate should wait on Done after calling Close.
func (e *Exporter) Close() {
	e.cancel()
}

// Done provides a channel that will be closed when the exporter has completed processing all inflight dates. When saving the
// final state of the exporter for future resumption, this channel must be waited upon before state is loaded. Note that the date
// exporter never terminates unless Close is called, so waiting on Done is only meaningful after Close has been called.
func (e *Exporter) Done() <-chan struct{} {
	return e.done
}

// GetCurrentDate returns the current target date. Note that earlier dates may also be being processed concurrently.
func (e *Exporter) GetCurrentDate() time.Time {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.currentDate
}

// GetState loads the current state of the exporter. Note that there may be concurrent export operations
// in progress, meaning that by the time state is observed it may already be outdated.
func (e *Exporter) GetState() ExporterState {
	e.mu.Lock()
	defer e.mu.Unlock()
	state := ExporterState{
		Dates: make(map[time.Time]DateExporterState, len(e.previous)+1),
	}

	// Add the current date state.
	state.Dates[e.currentDate] = e.current.GetState()

	for date, exporter := range e.previous {
		state.Dates[date] = exporter.GetState()
	}

	return state
}

func (e *Exporter) run(ctx context.Context) {
	defer func() {
		// on exit we close all date exporters and block on their completion
		// before signaling that we are done.
		var doneChans []<-chan struct{}
		e.withLock(func() {
			doneChans = make([]<-chan struct{}, 0, len(e.previous)+1)
			e.current.Close()
			doneChans = append(doneChans, e.current.Done())
			for _, exporter := range e.previous {
				exporter.Close()
				doneChans = append(doneChans, exporter.Done())
			}
		})

		for _, done := range doneChans {
			<-done
		}
		close(e.done)
	}()

	poll := interval.New(interval.Config{
		Duration:      e.cfg.PollInterval,
		FirstDuration: retryutils.FullJitter(e.cfg.PollInterval / 2),
		Jitter:        retryutils.SeventhJitter,
	})
	defer poll.Stop()

	logLimiter := rate.NewLimiter(rate.Every(time.Minute), 1)

	for {
		idle, err := e.poll(ctx)
		if err != nil && logLimiter.Allow() {
			var dates []string
			e.withLock(func() {
				dates = make([]string, 0, len(e.previous)+1)
				dates = append(dates, e.currentDate.Format(time.DateOnly))
				for date := range e.previous {
					dates = append(dates, date.Format(time.DateOnly))
				}
			})
			slices.Sort(dates)
			slog.WarnContext(ctx, "event export poll failed", "error", err, "dates", dates)
		}

		if idle && e.cfg.OnIdle != nil {
			e.cfg.OnIdle(ctx)
		}

		select {
		case <-e.idle:
		case <-poll.Next():
		case <-ctx.Done():
			return
		}
	}
}

// poll advances the exporter to the next date if the current date is idle and in the past, and prunes any idle exporters that
// are outside of the target backlog range. if the exporter is caught up with the current date and all sub-exporters are idle,
// poll returns true. otherwise, poll returns false.
func (e *Exporter) poll(ctx context.Context) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	var caughtUp bool
	if e.current.IsIdle() {
		if normalizeDate(time.Now()).After(e.currentDate) {
			nextDate := e.currentDate.AddDate(0, 0, 1)
			// current date is idle and in the past, advance to the next date
			if err := e.startExportLocked(ctx, nextDate); err != nil {
				return false, trace.Wrap(err)
			}
			slog.InfoContext(ctx, "advanced to next event export target", "date", nextDate.Format(time.DateOnly))
		} else {
			caughtUp = true
		}
	}

	// prune any dangling exporters that appear idle
	e.pruneBacklogLocked(ctx)

	if !caughtUp {
		return false, nil
	}

	// check if all backlog exporters are idle
	for _, exporter := range e.previous {
		if !exporter.IsIdle() {
			return false, nil
		}
	}

	// all exporters are idle and we are caught up with the current date
	return true, nil
}

// pruneBacklogLocked prunes any idle exporters that are outside of the target backlog range.
func (e *Exporter) pruneBacklogLocked(ctx context.Context) {
	if len(e.previous) <= e.cfg.BacklogSize {
		return
	}

	dates := make([]time.Time, 0, len(e.previous))
	for date := range e.previous {
		dates = append(dates, date)
	}

	// sort dates with most recent first
	slices.SortFunc(dates, func(a, b time.Time) int {
		if a.After(b) {
			return -1
		}
		if b.After(a) {
			return 1
		}
		return 0
	})

	// close any idle exporters that are older than the backlog size
	for _, date := range dates[e.cfg.BacklogSize:] {
		if !e.previous[date].IsIdle() {
			continue
		}

		e.previous[date].Close()

		doneC := e.previous[date].Done()

		var closing bool
		e.withoutLock(func() {
			select {
			case <-doneC:
			case <-ctx.Done():
				closing = true
			}
		})

		if closing {
			return
		}

		delete(e.previous, date)

		slog.InfoContext(ctx, "halted historical event export", "date", date.Format(time.DateOnly))
	}
}

// startExport starts export of events for the given date.
func (e *Exporter) startExportLocked(ctx context.Context, date time.Time) error {
	return e.resumeExportLocked(ctx, date, DateExporterState{})
}

// resumeExport resumes export of events for the given date with the given state.
func (e *Exporter) resumeExportLocked(ctx context.Context, date time.Time, state DateExporterState) error {
	date = normalizeDate(date)

	// check if the date is already being exported
	if _, ok := e.previous[date]; ok || e.currentDate.Equal(date) {
		return nil
	}

	onIdle := func(ctx context.Context) {
		var isCurrent bool
		e.withLock(func() {
			isCurrent = e.currentDate.Equal(date)
		})
		if !isCurrent {
			// idle callbacks from an exporter in the backlog
			// can be ignored.
			return
		}

		// current date is idle, wake up the poll loop
		select {
		case e.idle <- struct{}{}:
		default:
		}
	}

	// set up exporter
	exporter, err := NewDateExporter(DateExporterConfig{
		Client:        e.cfg.Client,
		Date:          date,
		Export:        e.cfg.Export,
		BatchExport:   e.cfg.BatchExport,
		OnIdle:        onIdle,
		PreviousState: state,
		Concurrency:   e.cfg.Concurrency,
		MaxBackoff:    e.cfg.MaxBackoff,
		PollInterval:  e.cfg.PollInterval,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// if a current export is in progress and is newer than this export,
	// add this export to the backlog.
	if e.current != nil && e.currentDate.After(date) {
		// historical export is being started, add to backlog
		e.previous[date] = exporter
		return nil
	}

	// bump previous export to backlog
	if e.current != nil {
		e.previous[e.currentDate] = e.current
	}
	e.current = exporter
	e.currentDate = date

	return nil
}

func (e *Exporter) withLock(fn func()) {
	e.mu.Lock()
	defer e.mu.Unlock()
	fn()
}

func (e *Exporter) withoutLock(fn func()) {
	e.mu.Unlock()
	defer e.mu.Lock()
	fn()
}

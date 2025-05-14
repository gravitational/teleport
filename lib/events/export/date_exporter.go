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
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/time/rate"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	auditlogpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/auditlog/v1"
	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/utils/interval"
)

// Client is the subset of the audit event client that is used by the date exporter.
type Client interface {
	ExportUnstructuredEvents(ctx context.Context, req *auditlogpb.ExportUnstructuredEventsRequest) stream.Stream[*auditlogpb.ExportEventUnstructured]
	GetEventExportChunks(ctx context.Context, req *auditlogpb.GetEventExportChunksRequest) stream.Stream[*auditlogpb.EventExportChunk]
}

// DateExporterConfig configures the date exporter.
type DateExporterConfig struct {
	// Client is the audit event client used to fetch and export events.
	Client Client
	// Date is the target date to export events from.
	Date time.Time
	// Export is the callback used to export events. Must be safe for concurrent use if
	// the Concurrency parameter is greater than 1.
	Export func(ctx context.Context, event *auditlogpb.ExportEventUnstructured) error
	// BatchExport is the callback with configuration used to export multiple
	// events in batches.
	BatchExport *BatchExportConfig

	// OnIdle is an optional callback that gets invoked periodically when the exporter is idle. Note that it is
	// safe to close the exporter or inspect its state from within this callback, but waiting on the exporter's
	// Done channel within this callback will deadlock.
	OnIdle func(ctx context.Context)
	// PreviousState is an optional parameter used to resume from a previous date export run.
	PreviousState DateExporterState
	// Concurrency sets the maximum number of event chunks that will be processed concurrently (defaults to 1).
	Concurrency int
	// MaxBackoff optionally overrides the default maximum backoff applied when errors are hit.
	MaxBackoff time.Duration
	// PollInterval optionally overrides the default poll interval used to fetch event chunks.
	PollInterval time.Duration
}

// CheckAndSetDefaults validates configuration and sets default values for optional parameters.
func (cfg *DateExporterConfig) CheckAndSetDefaults() error {
	if cfg.Client == nil {
		return trace.BadParameter("missing required parameter Client in DateExporterConfig")
	}
	if cfg.Export == nil && cfg.BatchExport == nil {
		return trace.BadParameter("missing required parameter Export or BatchExport in DateExporterConfig")
	}
	if cfg.BatchExport != nil && cfg.BatchExport.Callback == nil {
		return trace.BadParameter("missing parameter BatchExport.Callback in DateExporterConfig")
	}
	if cfg.Export != nil && cfg.BatchExport != nil {
		return trace.BadParameter("only one of Export or BatchExport may be set in ExporterConfig")
	}
	if cfg.Date.IsZero() {
		return trace.BadParameter("missing required parameter Date in DateExporterConfig")
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

// chunkEntry represents the state of a single event chunk being processed by the date exporter. Unlike
// the rest of the exporter which uses a basic mutex, the chunk entry uses atomic operations in order to
// minimize the potential for reads to affect event processing perf.
type chunkEntry struct {
	cursor atomic.Pointer[string]
	done   atomic.Bool
}

func (e *chunkEntry) getCursor() string {
	p := e.cursor.Load()
	if p == nil {
		return ""
	}
	return *p
}

func (e *chunkEntry) setCursor(cursor string) {
	e.cursor.Store(&cursor)
}

// DateExporterState represents the current state of the date exporter. State can be used to resume
// export from a previous run using the PreviousState parameter of the DateExporter config.
type DateExporterState struct {
	// Completed is an unordered list of the chunks for which all events have been consumed.
	Completed []string
	// Cursors is a map of chunk to cursor for partially completed chunks.
	Cursors map[string]string
}

// IsEmpty returns true if no state is defined.
func (s *DateExporterState) IsEmpty() bool {
	return len(s.Completed) == 0 && len(s.Cursors) == 0
}

// Clone returns a deep copy of the date exporter state.
func (s *DateExporterState) Clone() DateExporterState {
	cloned := DateExporterState{
		Completed: make([]string, len(s.Completed)),
		Cursors:   make(map[string]string, len(s.Cursors)),
	}
	copy(cloned.Completed, s.Completed)
	for chunk, cursor := range s.Cursors {
		cloned.Cursors[chunk] = cursor
	}
	return cloned
}

// DateExporter is a utility for exporting events for a given date using the chunked event export APIs. Note that
// it is specifically designed to prioritize performance and ensure that events aren't missed. It may not yield events
// in time order, and does not provide a mechanism to decide when export for a given date should be considered complete,
// since there is no 100% reliable way to determine when all events for a given date have been exported.
type DateExporter struct {
	cfg             DateExporterConfig
	log             *slog.Logger
	mainLogLimiter  *rate.Limiter
	chunkLogLimiter *rate.Limiter
	retry           retryutils.Retry
	sem             chan struct{}
	mu              sync.Mutex
	chunks          map[string]*chunkEntry
	idle            bool
	cancel          context.CancelFunc
	done            chan struct{}
}

// NewDateExporter creates a new date exporter and begin background processing of event chunks. Processing will continue
// until DateExporter.Stop is called, even if no new chunks are showing up. It is the caller's responsibility to decide
// when export for a given date should be considered complete by examining the the exporter's progress.
func NewDateExporter(cfg DateExporterConfig) (*DateExporter, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	retry, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
		First:  retryutils.FullJitter(cfg.MaxBackoff / 16),
		Driver: retryutils.NewExponentialDriver(cfg.MaxBackoff / 16),
		Max:    cfg.MaxBackoff,
		Jitter: retryutils.HalfJitter,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// date exporter should always present a correct chunk progress state. if state from
	// a previous run was provided as part of the configuration we want to set it up before
	// creating the exporter to ensure that any concurrent operations being used to
	// monitor/store progress are always shown the correct state.
	chunks := make(map[string]*chunkEntry)

	// set up entries for previously completed chunks
	for _, chunk := range cfg.PreviousState.Completed {
		entry := new(chunkEntry)
		entry.done.Store(true)
		chunks[chunk] = entry
	}

	// set up entries for partially completed chunks
	for chunk, cursor := range cfg.PreviousState.Cursors {
		entry := new(chunkEntry)
		entry.setCursor(cursor)
		chunks[chunk] = entry
	}

	ctx, cancel := context.WithCancel(context.Background())

	exporter := &DateExporter{
		cfg:             cfg,
		log:             slog.With("date", cfg.Date.Format(time.DateOnly)),
		mainLogLimiter:  rate.NewLimiter(rate.Every(time.Minute), 1),
		chunkLogLimiter: rate.NewLimiter(rate.Every(time.Minute), 3),
		retry:           retry,
		sem:             make(chan struct{}, cfg.Concurrency),
		chunks:          chunks,
		cancel:          cancel,
		done:            make(chan struct{}),
	}

	go exporter.run(ctx)

	return exporter, nil
}

// GetState loads the current state of the date exporter. Note that there may be concurrent export operations
// in progress, meaning that by the time state is observed it may already be outdated.
func (e *DateExporter) GetState() DateExporterState {
	e.mu.Lock()
	defer e.mu.Unlock()

	var completed []string
	cursors := make(map[string]string)

	for chunk, entry := range e.chunks {
		if entry.done.Load() {
			completed = append(completed, chunk)
		} else {
			if cursor := entry.getCursor(); cursor != "" {
				cursors[chunk] = cursor
			}
		}
	}

	return DateExporterState{
		Completed: completed,
		Cursors:   cursors,
	}
}

// IsIdle returns true if the date exporter has successfully discovered and processed all currently extant event chunks. Note that
// this does not imply that all events for a given date have been processed, since there may be replication delays and/or ongoing
// activity if the current date is being polled, but it is a strong indicator that all events have been discovered when the exporter
// is processing dates that are significantly in the past.
func (e *DateExporter) IsIdle() bool {
	var idle bool
	e.withLock(func() {
		idle = e.idle
	})
	return idle
}

// Close terminates all event processing. Note that shutdown is asynchronous. Any operation that needs to wait for export to fully
// terminate should wait on Done after calling Close.
func (e *DateExporter) Close() error {
	e.cancel()
	return nil
}

// Done provides a channel that will be closed when the date exporter has completed processing all event chunks. When saving the
// final state of the exporter for future resumption, this channel must be waited upon before state is loaded. Note that the date
// exporter never termiantes unless Close is called, so waiting on Done is only meaningful after Close has been called.
func (e *DateExporter) Done() <-chan struct{} {
	return e.done
}

func (e *DateExporter) run(ctx context.Context) {
	defer close(e.done)
	retry := e.retry.Clone()

	poll := interval.New(interval.Config{
		Duration:      e.cfg.PollInterval,
		FirstDuration: retryutils.FullJitter(e.cfg.PollInterval / 2),
		Jitter:        retryutils.SeventhJitter,
	})
	defer poll.Stop()

	var firstFullCycleCompleted bool

	// resume processing of any partially completed chunks prior to fetching new chunks
	for chunk, cursor := range e.GetState().Cursors {
		e.log.InfoContext(ctx, "resuming processing of partially completed chunk", "chunk", chunk, "cursor", cursor)
		if ok := e.startProcessingChunk(ctx, chunk, cursor); !ok {
			return
		}
	}

	for {
		n, err := e.fetchAndProcessChunks(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}

			if e.mainLogLimiter.Allow() {
				e.log.WarnContext(ctx, "fetch and process of event chunks failed", "error", err)
			}

			retry.Inc()
			select {
			case <-retry.After():
			case <-ctx.Done():
				return
			}
			continue
		}

		retry.Reset()

		// if no new chunks were processed, the exporter is considered idle
		idle := n == 0
		e.withLock(func() {
			e.idle = idle
		})
		if idle && e.cfg.OnIdle != nil {
			e.cfg.OnIdle(ctx)
		}

		// log first success, and periodically log subsequent non-idle cycles
		if (n > 0 && e.mainLogLimiter.Allow()) || !firstFullCycleCompleted {
			e.log.InfoContext(ctx, "successful fetch and process of event chunks", "chunks", n, "idle", idle)
		}

		firstFullCycleCompleted = true

		select {
		case <-poll.Next():
		case <-ctx.Done():
			return
		}
	}
}

// waitForInflightChunks blocks until all inflight chunks have been processed by acquiring all
// semaphore tokens and then releasing them. note that this operation does not accept a context,
// which is necessary in order to ensure that Done actually waits for all background processing
// to halt.
func (e *DateExporter) waitForInflightChunks() {
	// acquire all semaphore tokens to block until all inflight chunks have been processed
	for i := 0; i < e.cfg.Concurrency; i++ {
		e.sem <- struct{}{}
	}

	// release all semaphore tokens
	for i := 0; i < e.cfg.Concurrency; i++ {
		<-e.sem
	}
}

// fetchAndProcessChunks fetches and processes all chunks for the current date. if the function returns
// without error, all chunks have been successfully processed. note that all *currently known* chunks being
// processed does not necessarily imply that all events for a given date have been processed since there may
// be event replication delays, and/or ongoing activity if the current date is being polled.
func (e *DateExporter) fetchAndProcessChunks(ctx context.Context) (int, error) {
	// wait for inflight chunks before returning. in theory it would be fine (and possibly more performant)
	// to return immediately, but doing so makes it difficult to reason about when the exporter has fully exited
	// and/or how many complete export cycles have been performed.
	defer e.waitForInflightChunks()

	chunks := e.cfg.Client.GetEventExportChunks(ctx, &auditlogpb.GetEventExportChunksRequest{
		Date: timestamppb.New(e.cfg.Date),
	})

	var newChunks int

	for chunks.Next() {
		// known chunks should be skipped
		var skip bool
		e.withLock(func() {
			if _, ok := e.chunks[chunks.Item().Chunk]; ok {
				skip = true
				return
			}

			// so long as there is at least one undiscovered chunk, the exporter is not considered idle.
			e.idle = false
		})

		if skip {
			continue
		}

		if ok := e.startProcessingChunk(ctx, chunks.Item().Chunk, "" /* cursor */); !ok {
			return newChunks, trace.Wrap(ctx.Err())
		}

		newChunks++
	}

	if err := chunks.Done(); err != nil {
		return newChunks, trace.Wrap(err)
	}

	return newChunks, nil
}

// startProcessingChunk blocks until a semaphore token is acquired, then starts background processing of the
// supplied chunk. returns false if the context is canceled before background processing can be started.
func (e *DateExporter) startProcessingChunk(ctx context.Context, chunk, cursor string) (ok bool) {
	// acquire semaphore to start concurrent chunk processing
	select {
	case e.sem <- struct{}{}:
	case <-ctx.Done():
		return false
	}

	// set up entry so chunk processing status can be tracked
	entry := new(chunkEntry)
	if cursor != "" {
		entry.setCursor(cursor)
	}

	e.withLock(func() {
		e.chunks[chunk] = entry
	})

	// process chunk concurrently
	go func() {
		defer func() {
			<-e.sem
		}()

		e.processChunk(ctx, chunk, entry)
	}()

	return true
}

// processChunk attempts to export events from a given chunk. it will continuously retry until the context is canceled
// or all events have been successfully exported.
func (e *DateExporter) processChunk(ctx context.Context, chunk string, entry *chunkEntry) {
	// note: this retry is never reset since we return on first successful stream consumption
	retry := e.retry.Clone()
	var failures int
Outer:
	for {

		events := e.cfg.Client.ExportUnstructuredEvents(ctx, &auditlogpb.ExportUnstructuredEventsRequest{
			Date:   timestamppb.New(e.cfg.Date),
			Chunk:  chunk,
			Cursor: entry.getCursor(),
		})

		var err error
		if e.cfg.Export != nil {
			err = e.exportEvents(ctx, events, entry)
		} else {
			err = e.batchExportEvents(ctx, events, entry, chunk)
		}
		if err != nil {
			failures++

			if e.chunkLogLimiter.Allow() {
				e.log.WarnContext(ctx, "event chunk export failed", "chunk", chunk, "failures", failures, "error", err)
			}
			retry.Inc()

			select {
			case <-retry.After():
			case <-ctx.Done():
				return
			}
			continue Outer
		}

		entry.done.Store(true)
		return
	}
}

type batch struct {
	maxSize int
	chunk   string
	entry   *chunkEntry

	size   int
	events []*auditlogpb.EventUnstructured
	cursor string
}

func (b *batch) addEvent(event *auditlogpb.ExportEventUnstructured) bool {
	e := event.GetEvent()
	size := proto.Size(e)
	if b.size+size <= b.maxSize || len(b.events) == 0 {
		b.size += size
		b.events = append(b.events, e)
		b.cursor = event.GetCursor()
		return true
	}
	return false
}

func (b *batch) reset() {
	b.events = nil
	b.size = 0
	b.cursor = ""
}

func (b *batch) isEmpty() bool {
	return len(b.events) == 0
}

// batchExportEvents reads events from the provided stream and exports them in batches.
// Batching adheres to the export configuration: batches are sent either when
// they reach the configured maximum size (MaxSize) or after the maximum delay
// (MaxDelay) has passed with pending events. Processing continues until the
// stream closes or an error occurs.
func (e *DateExporter) batchExportEvents(ctx context.Context, stream stream.Stream[*auditlogpb.ExportEventUnstructured], entry *chunkEntry, chunk string) error {
	exportEventCh := make(chan *auditlogpb.ExportEventUnstructured, 1)
	go func() {
		for stream.Next() {
			exportEventCh <- stream.Item()
		}
		close(exportEventCh)
	}()

	batch := &batch{
		maxSize: e.cfg.BatchExport.MaxSize,
		chunk:   chunk,
		entry:   entry,
	}
	timer := time.NewTimer(e.cfg.BatchExport.MaxDelay)
	defer timer.Stop()
loop:
	for {
		var unprocessedEvent *auditlogpb.ExportEventUnstructured
		select {
		case exportEvent, ok := <-exportEventCh:
			if !ok {
				// all events have been processed
				break loop
			}
			if batch.addEvent(exportEvent) {
				continue
			}
			unprocessedEvent = exportEvent
		case <-timer.C:
			if batch.isEmpty() {
				timer.Reset(e.cfg.BatchExport.MaxDelay)
				continue
			}
		}
		err := e.batchExport(ctx, batch, false /*completed*/)
		if err != nil {
			stream.Done()
			return trace.Wrap(err)
		}

		batch.reset()
		timer.Reset(e.cfg.BatchExport.MaxDelay)

		if unprocessedEvent != nil {
			batch.addEvent(unprocessedEvent)
		}
	}

	if err := stream.Done(); err != nil {
		return trace.Wrap(err)
	}

	// One final call back with completed set to true, even if batch is empty.
	err := e.batchExport(ctx, batch, true /*completed*/)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// batchExport calls the batch export callback with the given batched events and
// the derived resume state.
func (e *DateExporter) batchExport(ctx context.Context, batch *batch, completed bool) error {
	cursor := batch.cursor
	if completed {
		cursor = ""
	}
	resumeState := BulkExportResumeState{
		Chunk:     batch.chunk,
		Cursor:    cursor,
		Date:      e.cfg.Date,
		Completed: completed,
	}
	batch.entry.setCursor(cursor)
	return e.cfg.BatchExport.Callback(ctx, batch.events, resumeState)
}

// exportEvents exports all events from the provided stream, updating the supplied entry on each successful export.
func (e *DateExporter) exportEvents(ctx context.Context, events stream.Stream[*auditlogpb.ExportEventUnstructured], entry *chunkEntry) error {
	for events.Next() {
		if err := e.cfg.Export(ctx, events.Item()); err != nil {
			events.Done()
			return trace.Wrap(err)
		}

		entry.setCursor(events.Item().Cursor)
	}
	return trace.Wrap(events.Done())
}

func (e *DateExporter) withLock(fn func()) {
	e.mu.Lock()
	defer e.mu.Unlock()
	fn()
}

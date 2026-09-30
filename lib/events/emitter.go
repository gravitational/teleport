/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package events

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	auditlogpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/auditlog/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events/auditqueue"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/log"
)

const (
	// AsyncBufferSize is a default buffer size for async emitters
	AsyncBufferSize = 1024
	// auditQueueDir is the directory where the audit queue resides.
	auditQueueDir = "audit-queue"
	// Matches the audit queue dequeue batch size so one delivery cycle is one round of requests.
	sealedSubmitConcurrency = 25
	maxEmitRequestBytes     = 3 * 1024 * 1024
)

// AsyncEmitterConfig provides parameters for emitter
type AsyncEmitterConfig struct {
	// Inner emits events to the underlying store
	Inner apievents.Emitter
	// BufferSize is a default buffer size for emitter
	BufferSize int
	// DisableAuditQueue bypasses the audit queue and forwards events through
	// an in-memory channel instead.
	DisableAuditQueue bool
	// DataDir is the Teleport data directory. This is required for sqlite
	// backed queues.
	DataDir string
	// AuditQueueCfg holds the options from the Teleport yaml config.
	AuditQueueCfg auditqueue.Config
	// AuditQueueBackends is the ordered list of backends to try on startup.
	AuditQueueBackends []auditqueue.Kind
	// Sealer is used to encrypt audit log events before writing them to disk.
	Sealer auditqueue.Sealer
	// SealedSubmitter delivers sealed audit event batches to the auth server.
	SealedSubmitter SealedBatchSubmitter
}

// SealedBatchSubmitter delivers sealed audit event batches to the auth
// server. A nil error means the whole batch was accepted. An error means the
// whole batch must be retried.
type SealedBatchSubmitter interface {
	SubmitAuditQueueBatch(ctx context.Context, req *auditlogpb.SubmitAuditQueueBatchRequest) error
}

// CheckAndSetDefaults checks and sets default values
func (c *AsyncEmitterConfig) CheckAndSetDefaults() error {
	if c.Inner == nil {
		return trace.BadParameter("missing parameter Inner")
	}
	if c.BufferSize == 0 {
		c.BufferSize = AsyncBufferSize
	}
	return nil
}

// NewAsyncEmitter returns emitter that submits events
// without blocking the caller. It will start losing events
// when the queue is full.
func NewAsyncEmitter(cfg AsyncEmitterConfig) (*AsyncEmitter, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	var queue auditqueue.Queue
	if cfg.DisableAuditQueue {
		slog.InfoContext(context.Background(), "Using default in-memory audit event channel. Audit queue is disabled.")
	} else {
		cfg.AuditQueueCfg.Sealer = cfg.Sealer
		var err error
		queue, err = makeQueue(cfg.DataDir, cfg.AuditQueueCfg, cfg.AuditQueueBackends)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if _, ok := cfg.Inner.(apievents.BatchEmitter); !ok {
			slog.WarnContext(context.TODO(),
				"Audit queue inner emitter does not support batched delivery. Events will be forwarded one at a time.",
				"inner_type", log.TypeAttr(cfg.Inner),
			)
		}
	}

	deliveryTimeout := cfg.AuditQueueCfg.DeliveryTimeout
	if deliveryTimeout <= 0 {
		deliveryTimeout = auditqueue.DefaultDeliveryTimeout
	}

	ctx, cancel := context.WithCancel(context.Background())
	a := &AsyncEmitter{
		cancel:          cancel,
		ctx:             ctx,
		cfg:             cfg,
		queue:           queue,
		deliveryTimeout: deliveryTimeout,
	}
	if queue == nil {
		a.noQueueEventsCh = make(chan apievents.AuditEvent, cfg.BufferSize)
		a.wg.Go(a.forwardWithoutQueue)
		return a, nil
	}
	a.wg.Go(func() {
		if err := queue.Run(ctx, a.deliver); err != nil && ctx.Err() == nil {
			slog.ErrorContext(ctx, "Audit queue Run returned error.", "error", err)
		}
	})
	return a, nil
}

func makeQueue(dataDir string, queueCfg auditqueue.Config, backends []auditqueue.Kind) (auditqueue.Queue, error) {
	queueCfg.Path = filepath.Join(dataDir, auditQueueDir, uuid.NewString())

	if len(backends) == 0 {
		backends = []auditqueue.Kind{auditqueue.KindSQLiteDisk}
	}

	var lastErr error
	for i, backend := range backends {
		q, err := auditqueue.New(backend, queueCfg)
		if err == nil {
			if i > 0 {
				slog.WarnContext(context.TODO(),
					"Using fallback audit queue backend.",
					"backend", backend,
				)
			}
			return q, nil
		}
		slog.WarnContext(context.TODO(),
			"Failed to create audit queue backend, trying next.",
			"backend", backend,
			"error", err,
		)
		lastErr = err
	}
	return nil, trace.Wrap(lastErr)
}

// AsyncEmitter enqueues events to the audit queue and delivers them to the
// inner emitter in a separate goroutine without blocking the caller. When the
// audit queue is disabled, events are forwarded through an in-memory channel.
type AsyncEmitter struct {
	cfg             AsyncEmitterConfig
	noQueueEventsCh chan apievents.AuditEvent // Copied from legacy implementation, will be removed when we delete the legacy code.
	cancel          context.CancelFunc
	ctx             context.Context
	queue           auditqueue.Queue
	deliveryTimeout time.Duration
	wg              sync.WaitGroup
	sealedWarnOnce  sync.Once
}

// Close closes emitter and cancels all in flight events.
func (a *AsyncEmitter) Close() error {
	a.cancel()
	a.wg.Wait()
	var errs []error
	if a.queue != nil {
		errs = append(errs, a.queue.Close())
	}
	if a.cfg.Sealer != nil {
		errs = append(errs, a.cfg.Sealer.Close())
	}
	return trace.NewAggregate(errs...)
}

// Shutdown makes a best effort attempt to flush pending audit events to the
// inner emitter before closing.
func (a *AsyncEmitter) Shutdown(ctx context.Context) error {
	if a.queue == nil {
		return trace.Wrap(a.Close())
	}
	if err := a.queue.Drain(ctx); err != nil {
		slog.WarnContext(ctx,
			"Audit queue drain returned an error during graceful shutdown.",
			"error", err,
		)
	}
	return trace.Wrap(a.Close())
}

// Stats reports the current depth of the audit queue. It returns a zero Stats
// when the audit queue is disabled.
func (a *AsyncEmitter) Stats(ctx context.Context) (auditqueue.Stats, error) {
	if a.queue == nil {
		return auditqueue.Stats{}, nil
	}
	return a.queue.Stats(ctx)
}

// forwardWithoutQueue was copied from the legacy implementation, will be removed when we delete the legacy code.
func (a *AsyncEmitter) forwardWithoutQueue() {
	for {
		select {
		case <-a.ctx.Done():
			return
		case event := <-a.noQueueEventsCh:
			err := a.cfg.Inner.EmitAuditEvent(a.ctx, event)
			if err != nil {
				if a.ctx.Err() != nil {
					return
				}
				slog.ErrorContext(a.ctx, "Failed to emit audit event.", "error", err)
			}
		}
	}
}

func splitSealedItems(items []auditqueue.Item) (plain, sealed []auditqueue.Item) {
	for _, item := range items {
		if item.Sealed() {
			sealed = append(sealed, item)
			continue
		}
		plain = append(plain, item)
	}
	return plain, sealed
}

func itemSize(item auditqueue.Item) int {
	var size int
	for _, event := range item.Events {
		size += event.Size()
	}
	return size
}

func chunkItems(items []auditqueue.Item, maxBytes, maxEvents int) [][]auditqueue.Item {
	var chunks [][]auditqueue.Item
	var chunk []auditqueue.Item
	var chunkBytes, chunkEvents int
	for _, item := range items {
		size := itemSize(item)
		count := len(item.Events)
		if len(chunk) > 0 && (chunkBytes+size > maxBytes || chunkEvents+count > maxEvents) {
			chunks = append(chunks, chunk)
			chunk, chunkBytes, chunkEvents = nil, 0, 0
		}
		chunk = append(chunk, item)
		chunkBytes += size
		chunkEvents += count
	}
	if len(chunk) > 0 {
		chunks = append(chunks, chunk)
	}
	return chunks
}

// deliver is the handler function passed to queue.Run. It must return a list of
// the items it was able to successfully deliver.
func (a *AsyncEmitter) deliver(ctx context.Context, items []auditqueue.Item) ([]auditqueue.Item, error) {
	ctx, cancel := context.WithTimeout(ctx, a.deliveryTimeout)
	defer cancel()

	items, sealedItems := splitSealedItems(items)
	sealedDelivered, sealedErr := a.deliverSealed(ctx, sealedItems)

	batchEmitter, _ := a.cfg.Inner.(apievents.BatchEmitter)

	var delivered []auditqueue.Item
	if batchEmitter != nil {
		var fallback []auditqueue.Item
		for _, chunk := range chunkItems(items, maxEmitRequestBytes, apidefaults.MaxAuditEventsBatchSize) {
			var events []apievents.AuditEvent
			for _, item := range chunk {
				events = append(events, item.Events...)
			}
			err := batchEmitter.EmitAuditEvents(ctx, events)
			if err == nil {
				delivered = append(delivered, chunk...)
				continue
			}
			if ctx.Err() != nil {
				return append(delivered, sealedDelivered...), sealedErr
			}
			if trace.IsLimitExceeded(err) {
				return append(delivered, sealedDelivered...), errors.Join(sealedErr, err)
			}
			slog.WarnContext(ctx, "Failed to emit audit events as a single request, falling back to per-batch delivery.", "error", err)
			fallback = append(fallback, chunk...)
		}
		items = fallback
	}

	var failed int
	var plainErr error
	for _, item := range items {
		if ctx.Err() != nil {
			break
		}
		if err := a.deliverBatch(ctx, batchEmitter, item); err != nil {
			failed++
			if trace.IsLimitExceeded(err) {
				plainErr = err
				break
			}
			if plainErr == nil {
				plainErr = err
			}
			continue
		}
		delivered = append(delivered, item)
	}
	if failed > 0 && ctx.Err() == nil {
		if trace.IsLimitExceeded(plainErr) {
			slog.DebugContext(ctx, "Audit event delivery throttled by the auth server.",
				"count", failed,
				"error", plainErr,
			)
		} else {
			slog.ErrorContext(ctx, "Failed to emit audit events.",
				"count", failed,
				"error", plainErr,
			)
		}
	}
	return append(delivered, sealedDelivered...), errors.Join(sealedErr, plainErr)
}

func (a *AsyncEmitter) deliverSealed(ctx context.Context, sealed []auditqueue.Item) ([]auditqueue.Item, error) {
	if len(sealed) == 0 {
		return nil, nil
	}
	if a.cfg.SealedSubmitter == nil {
		a.sealedWarnOnce.Do(func() {
			slog.WarnContext(ctx,
				"Sealed audit event batches cannot be delivered because no sealed batch submitter is configured. They will be retried.",
			)
		})
		return nil, nil
	}

	var mu sync.Mutex
	var delivered []auditqueue.Item
	var failed int
	var throttled int
	var throttleErr error
	var g errgroup.Group
	defer g.Wait()
	g.SetLimit(sealedSubmitConcurrency)
	for _, item := range sealed {
		if ctx.Err() != nil {
			break
		}
		g.Go(func() error {
			err := a.submitSealed(ctx, item)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				delivered = append(delivered, item)
				return nil
			case trace.IsLimitExceeded(err):
				throttled++
				if throttleErr == nil {
					throttleErr = err
				}
				return nil
			default:
				failed++
				return err
			}
		})
	}
	firstErr := g.Wait()
	if ctx.Err() == nil {
		if failed > 0 {
			slog.ErrorContext(ctx, "Failed to submit sealed audit event batches.",
				"count", failed,
				"error", firstErr,
			)
		}
		if throttled > 0 {
			slog.DebugContext(ctx, "Sealed audit event batch submissions throttled by the auth server.",
				"count", throttled,
				"error", throttleErr,
			)
		}
	}
	if throttleErr != nil {
		return delivered, throttleErr
	}
	return delivered, firstErr
}

func (a *AsyncEmitter) submitSealed(ctx context.Context, item auditqueue.Item) error {
	format, ok := sealedWireFormat(item.Format)
	if !ok {
		return trace.BadParameter("sealed batch has unknown payload format %d", item.Format)
	}
	req := auditlogpb.SubmitAuditQueueBatchRequest_builder{
		Batch: auditlogpb.AuditQueueSealedBatch_builder{
			Payload:    item.Payload,
			Format:     format,
			EventCount: int64(item.EventCount),
		}.Build(),
	}.Build()
	return trace.Wrap(a.cfg.SealedSubmitter.SubmitAuditQueueBatch(ctx, req))
}

func sealedWireFormat(format int) (auditlogpb.AuditQueueBatchFormat, bool) {
	switch format {
	case auditqueue.FormatAgeV1:
		return auditlogpb.AuditQueueBatchFormat_AUDIT_QUEUE_BATCH_FORMAT_AGE_V1, true
	default:
		return auditlogpb.AuditQueueBatchFormat_AUDIT_QUEUE_BATCH_FORMAT_UNSPECIFIED, false
	}
}

func (a *AsyncEmitter) deliverBatch(ctx context.Context, batchEmitter apievents.BatchEmitter, item auditqueue.Item) error {
	if batchEmitter != nil {
		err := batchEmitter.EmitAuditEvents(ctx, item.Events)
		if err == nil {
			return nil
		}
		if ctx.Err() != nil || trace.IsLimitExceeded(err) {
			return trace.Wrap(err)
		}
		slog.WarnContext(ctx, "Failed to emit audit events as a batch, falling back to per-event delivery.", "error", err)
	}

	for _, event := range item.Events {
		if err := ctx.Err(); err != nil {
			return trace.Wrap(err)
		}
		if err := a.cfg.Inner.EmitAuditEvent(ctx, event); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// EmitAuditEvent enqueues the audit event without blocking on delivery. It
// fails when the queue is full. When the audit queue is disabled, the event is
// sent to the in-memory channel and dropped if the channel is full.
func (a *AsyncEmitter) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	if a.queue == nil {
		// This block was copied from the legacy implementation. It be removed when we delete the legacy code.
		select {
		case a.noQueueEventsCh <- event:
			return nil
		case <-ctx.Done():
			return trace.ConnectionProblem(ctx.Err(), "context canceled or closed")
		default:
			slog.ErrorContext(ctx, "Failed to emit audit event. This server's connection to the auth service appears to be slow.", "event_type", event.GetType(), "event_code", event.GetCode())
			return nil
		}
	}
	err := a.queue.Enqueue(event)
	if errors.Is(err, auditqueue.ErrQueueFull) {
		slog.ErrorContext(
			ctx,
			"Failed to emit audit event. Audit queue is full.",
			"event_type", event.GetType(),
			"event_code", event.GetCode(),
		)
		return trace.Wrap(err)
	}
	return trace.Wrap(err)
}

// CheckingEmitterConfig provides parameters for emitter
type CheckingEmitterConfig struct {
	// Inner emits events to the underlying store
	Inner apievents.Emitter
	// Clock is a clock interface, used in tests
	Clock clockwork.Clock
	// UIDGenerator is unique ID generator
	UIDGenerator utils.UID
	// ClusterName specifies the name of this teleport cluster
	// as configured on the auth server
	ClusterName string
}

// NewCheckingEmitter returns emitter that checks
// that all required fields are properly set
func NewCheckingEmitter(cfg CheckingEmitterConfig) (*CheckingEmitter, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &CheckingEmitter{
		CheckingEmitterConfig: cfg,
	}, nil
}

// CheckingEmitter ensures that event fields have been set properly
// and reports statistics for every wrapper
type CheckingEmitter struct {
	CheckingEmitterConfig
}

// CheckAndSetDefaults checks and sets default values
func (w *CheckingEmitterConfig) CheckAndSetDefaults() error {
	if w.Inner == nil {
		return trace.BadParameter("missing parameter Inner")
	}
	if w.Clock == nil {
		w.Clock = clockwork.NewRealClock()
	}
	if w.UIDGenerator == nil {
		w.UIDGenerator = utils.NewRealUID()
	}
	return nil
}

// EmitAuditEvent emits audit event
func (r *CheckingEmitter) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	ctx = context.WithoutCancel(ctx)
	auditEmitEvent.Inc()
	auditEmitEventSizes.Observe(float64(event.Size()))
	if err := checkAndSetEventFields(event, r.Clock, r.UIDGenerator, r.ClusterName); err != nil {
		slog.ErrorContext(ctx, "Failed to emit audit event.", "error", err)
		AuditFailedEmit.Inc()
		return trace.Wrap(err)
	}
	if err := r.Inner.EmitAuditEvent(ctx, event); err != nil {
		AuditFailedEmit.Inc()
		slog.ErrorContext(ctx, "Failed to emit audit event of type", "event_type", event.GetType(), "error", err)
		return trace.Wrap(err)
	}
	return nil
}

// EmitAuditEvents emits a batch of audit events
func (r *CheckingEmitter) EmitAuditEvents(ctx context.Context, events []apievents.AuditEvent) error {
	ctx = context.WithoutCancel(ctx)
	for _, event := range events {
		auditEmitEvent.Inc()
		auditEmitEventSizes.Observe(float64(event.Size()))
		if err := checkAndSetEventFields(event, r.Clock, r.UIDGenerator, r.ClusterName); err != nil {
			slog.ErrorContext(ctx, "Failed to emit audit event.", "error", err)
			AuditFailedEmit.Inc()
			return trace.Wrap(err)
		}
	}
	if err := EmitAuditEvents(ctx, r.Inner, events); err != nil {
		AuditFailedEmit.Inc()
		slog.ErrorContext(ctx, "Failed to emit audit events.", "error", err)
		return trace.Wrap(err)
	}
	return nil
}

// CheckingAsyncEmitter is a CheckingEmitter wrapping an AsyncEmitter. This is to
// ensure that audit log events have their fields set appropriately before they
// are enqueued.
type CheckingAsyncEmitter struct {
	*CheckingEmitter
	asyncEmitter *AsyncEmitter
}

func NewCheckingAsyncEmitter(checkingCfg CheckingEmitterConfig, asyncCfg AsyncEmitterConfig) (*CheckingAsyncEmitter, error) {
	asyncEmitter, err := NewAsyncEmitter(asyncCfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	checkingCfg.Inner = asyncEmitter
	checking, err := NewCheckingEmitter(checkingCfg)
	if err != nil {
		_ = asyncEmitter.Close()
		return nil, trace.Wrap(err)
	}
	emitter := CheckingAsyncEmitter{
		CheckingEmitter: checking,
		asyncEmitter:    asyncEmitter,
	}
	return &emitter, nil
}

// Close closes the underlying AsyncEmitter.
func (c *CheckingAsyncEmitter) Close() error {
	return c.asyncEmitter.Close()
}

// Shutdown attempts to drain the underlying AsyncEmitter before closing.
// See AsyncEmitter.Shutdown.
func (c *CheckingAsyncEmitter) Shutdown(ctx context.Context) error {
	return c.asyncEmitter.Shutdown(ctx)
}

// Stats reports the current depth of the underlying audit queue.
// See AsyncEmitter.Stats.
func (c *CheckingAsyncEmitter) Stats(ctx context.Context) (auditqueue.Stats, error) {
	return c.asyncEmitter.Stats(ctx)
}

// checkAndSetEventFields updates passed event fields with additional information
// common for all event types such as unique IDs, timestamps, codes, etc.
//
// This method is a "final stop" for various audit log implementations for
// updating event fields before it gets persisted in the backend.
func checkAndSetEventFields(event apievents.AuditEvent, clock clockwork.Clock, uid utils.UID, clusterName string) error {
	if err := checkBasicEventFields(event); err != nil {
		return trace.Wrap(err)
	}
	if event.GetID() == "" && event.GetType() != SessionPrintEvent && event.GetType() != DesktopRecordingEvent {
		event.SetID(uid.New())
	}
	if event.GetTime().IsZero() {
		event.SetTime(clock.Now().UTC().Round(time.Millisecond))
	}
	if event.GetClusterName() == "" {
		event.SetClusterName(clusterName)
	}
	return nil
}

func checkBasicEventFields(event apievents.AuditEvent) error {
	if event.GetType() == "" {
		return trace.BadParameter("missing mandatory event type field")
	}
	if event.GetCode() == "" && event.GetType() != SessionPrintEvent && event.GetType() != DesktopRecordingEvent {
		return trace.BadParameter("missing mandatory event code field for %v event", event.GetType())
	}
	return nil
}

// NewWriterEmitter returns a new instance of emitter writing to writer
func NewWriterEmitter(w io.WriteCloser) *WriterEmitter {
	return &WriterEmitter{
		w:         w,
		WriterLog: NewWriterLog(w),
	}
}

// WriterEmitter is an emitter that emits all events
// to the external writer
type WriterEmitter struct {
	w io.WriteCloser
	*WriterLog
}

// Close closes the underlying io.WriteCloser passed in NewWriterEmitter
func (w *WriterEmitter) Close() error {
	return trace.NewAggregate(
		w.w.Close(),
		w.WriterLog.Close())
}

// EmitAuditEvent writes the event to the writer
func (w *WriterEmitter) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	// line is the text to be logged
	line, err := utils.FastMarshal(event)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = fmt.Fprintln(w.w, string(line))
	return trace.ConvertSystemError(err)
}

// NewLoggingEmitter returns an emitter that logs all events to the console
// with the info level. Events are only logged for self-hosted installations,
// Teleport Cloud treats this as a no-op.
func NewLoggingEmitter(cloud bool) *LoggingEmitter {
	return &LoggingEmitter{
		emit: !modules.GetModules().Features().Cloud && !cloud,
	}
}

// LoggingEmitter logs all events with info level
type LoggingEmitter struct {
	emit bool
}

// EmitAuditEvent logs audit event, skips session print events, session
// disk events and app session request events, because they are very verbose.
func (l *LoggingEmitter) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	if !l.emit {
		return nil
	}

	switch event.GetType() {
	case ResizeEvent,
		SessionDiskEvent,
		SessionPrintEvent,
		AppSessionRequestEvent,
		AppSessionRequestAllowedEvent,
		AppSessionLLMRequestSuccessEvent,
		AppSessionLLMRequestFailureEvent,
		"":
		return nil
	}

	data, err := utils.FastMarshal(event)
	if err != nil {
		return trace.Wrap(err)
	}

	var fields map[string]any
	if err := utils.FastUnmarshal(data, &fields); err != nil {
		return trace.Wrap(err)
	}
	fields[teleport.ComponentKey] = teleport.ComponentAuditLog

	slog.InfoContext(ctx, "emitting audit event", "event_type", event.GetType(), "fields", fields)
	return nil
}

// EmitAuditEvents emits a batch of audit events to e.
func EmitAuditEvents(ctx context.Context, e apievents.Emitter, events []apievents.AuditEvent) error {
	if be, ok := e.(apievents.BatchEmitter); ok {
		return trace.Wrap(be.EmitAuditEvents(ctx, events))
	}
	for _, event := range events {
		if err := e.EmitAuditEvent(ctx, event); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// NewMultiEmitter returns emitter that writes
// events to all emitters
func NewMultiEmitter(emitters ...apievents.Emitter) *MultiEmitter {
	return &MultiEmitter{
		emitters: emitters,
	}
}

// MultiEmitter writes audit events to multiple emitters
type MultiEmitter struct {
	emitters []apievents.Emitter
}

// EmitAuditEvent emits audit event to all emitters
func (m *MultiEmitter) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	ctx = context.WithoutCancel(ctx)
	var errors []error
	for i := range m.emitters {
		err := m.emitters[i].EmitAuditEvent(ctx, event)
		if err != nil {
			errors = append(errors, err)
		}
	}
	return trace.NewAggregate(errors...)
}

// EmitAuditEvents emits a batch of audit events to all emitters.
func (m *MultiEmitter) EmitAuditEvents(ctx context.Context, events []apievents.AuditEvent) error {
	ctx = context.WithoutCancel(ctx)
	var errors []error
	for i := range m.emitters {
		if err := EmitAuditEvents(ctx, m.emitters[i], events); err != nil {
			errors = append(errors, err)
		}
	}
	return trace.NewAggregate(errors...)
}

var (
	_ apievents.BatchEmitter = (*MultiEmitter)(nil)
	_ apievents.BatchEmitter = (*CheckingEmitter)(nil)
)

// StreamerAndEmitter combines streamer and emitter to create stream emitter
type StreamerAndEmitter struct {
	Streamer
	apievents.Emitter
}

// NewCallbackEmitter returns an emitter that invokes a callback on every
// action, is used in tests to inject failures
func NewCallbackEmitter(cfg CallbackEmitterConfig) (*CallbackEmitter, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &CallbackEmitter{
		CallbackEmitterConfig: cfg,
	}, nil
}

// CallbackEmitterConfig provides parameters for emitter
type CallbackEmitterConfig struct {
	// OnEmitAuditEvent is called on emit audit event on a stream
	OnEmitAuditEvent func(ctx context.Context, event apievents.AuditEvent) error
}

// CheckAndSetDefaults checks and sets default values
func (c *CallbackEmitterConfig) CheckAndSetDefaults() error {
	if c.OnEmitAuditEvent == nil {
		return trace.BadParameter("missing parameter OnEmitAuditEvent")
	}
	return nil
}

// CallbackEmitter invokes a callback on every action, is used in tests to inject failures
type CallbackEmitter struct {
	CallbackEmitterConfig
}

func (c *CallbackEmitter) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	ctx = context.WithoutCancel(ctx)
	return c.OnEmitAuditEvent(ctx, event)
}

// NewCallbackStreamer returns streamer that invokes callback on every
// action, is used in tests to inject failures
func NewCallbackStreamer(cfg CallbackStreamerConfig) (*CallbackStreamer, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &CallbackStreamer{
		CallbackStreamerConfig: cfg,
	}, nil
}

// CallbackStreamerConfig provides parameters for streamer
type CallbackStreamerConfig struct {
	// Inner emits events to the underlying store
	Inner Streamer
	// OnCreateAuditStream is called on create audit stream
	OnCreateAuditStream func(ctx context.Context, sid session.ID, inner Streamer) (apievents.Stream, error)
	// OnResumeAuditStream is called on resuming audit stream
	OnResumeAuditStream func(ctx context.Context, sid session.ID, uploadID string, inner Streamer) (apievents.Stream, error)
	// OnRecordEvent is called on emit audit event on a stream
	OnRecordEvent func(ctx context.Context, sid session.ID, event apievents.PreparedSessionEvent) error
}

// CheckAndSetDefaults checks and sets default values
func (c *CallbackStreamerConfig) CheckAndSetDefaults() error {
	if c.Inner == nil {
		return trace.BadParameter("missing parameter Inner")
	}
	return nil
}

// CallbackStreamer ensures that event fields have been set properly
// and reports statistics for every wrapper
type CallbackStreamer struct {
	CallbackStreamerConfig
}

// CreateAuditStream creates audit event stream
func (s *CallbackStreamer) CreateAuditStream(ctx context.Context, sid session.ID) (apievents.Stream, error) {
	var stream apievents.Stream
	var err error
	if s.OnCreateAuditStream != nil {
		stream, err = s.OnCreateAuditStream(ctx, sid, s.Inner)
	} else {
		stream, err = s.Inner.CreateAuditStream(ctx, sid)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &CallbackStream{
		stream:   stream,
		streamer: s,
	}, nil
}

// ResumeAuditStream resumes audit event stream
func (s *CallbackStreamer) ResumeAuditStream(ctx context.Context, sid session.ID, uploadID string) (apievents.Stream, error) {
	var stream apievents.Stream
	var err error
	if s.OnResumeAuditStream != nil {
		stream, err = s.OnResumeAuditStream(ctx, sid, uploadID, s.Inner)
	} else {
		stream, err = s.Inner.ResumeAuditStream(ctx, sid, uploadID)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &CallbackStream{
		stream:    stream,
		sessionID: sid,
		streamer:  s,
	}, nil
}

// CallbackStream call
type CallbackStream struct {
	stream    apievents.Stream
	sessionID session.ID
	streamer  *CallbackStreamer
}

// Close flushes non-uploaded flight stream data without marking
// the stream completed and closes the stream instance
func (s *CallbackStream) Close(ctx context.Context) error {
	return s.stream.Close(ctx)
}

// Done returns channel closed when streamer is closed
// should be used to detect sending errors
func (s *CallbackStream) Done() <-chan struct{} {
	return s.stream.Done()
}

// Status returns channel receiving updates about stream status
// last event index that was uploaded and upload ID
func (s *CallbackStream) Status() <-chan apievents.StreamStatus {
	return s.stream.Status()
}

// Complete closes the stream and marks it finalized
func (s *CallbackStream) Complete(ctx context.Context) error {
	return s.stream.Complete(ctx)
}

// RecordEvent records a session event
func (s *CallbackStream) RecordEvent(ctx context.Context, event apievents.PreparedSessionEvent) error {
	if s.streamer.OnRecordEvent != nil {
		if err := s.streamer.OnRecordEvent(ctx, s.sessionID, event); err != nil {
			return trace.Wrap(err)
		}
	}
	return s.stream.RecordEvent(ctx, event)
}

// NewReportingStreamer reports upload events
// to the eventsC channel, if the channel is not nil.
func NewReportingStreamer(streamer Streamer, eventsC chan UploadEvent) *ReportingStreamer {
	return &ReportingStreamer{
		streamer: streamer,
		eventsC:  eventsC,
	}
}

// ReportingStreamer  reports upload events
// to the eventsC channel, if the channel is not nil.
type ReportingStreamer struct {
	streamer Streamer
	eventsC  chan UploadEvent
}

// CreateAuditStream creates audit event stream
func (s *ReportingStreamer) CreateAuditStream(ctx context.Context, sid session.ID) (apievents.Stream, error) {
	stream, err := s.streamer.CreateAuditStream(ctx, sid)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &ReportingStream{
		Stream:    stream,
		eventsC:   s.eventsC,
		sessionID: sid,
	}, nil
}

// ResumeAuditStream resumes audit event stream
func (s *ReportingStreamer) ResumeAuditStream(ctx context.Context, sid session.ID, uploadID string) (apievents.Stream, error) {
	stream, err := s.streamer.ResumeAuditStream(ctx, sid, uploadID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &ReportingStream{
		Stream:    stream,
		sessionID: sid,
		eventsC:   s.eventsC,
	}, nil
}

// ReportingStream reports status of uploads to the events channel
type ReportingStream struct {
	apievents.Stream
	sessionID session.ID
	eventsC   chan UploadEvent
}

// Complete closes the stream and marks it finalized
func (s *ReportingStream) Complete(ctx context.Context) error {
	err := s.Stream.Complete(ctx)
	if s.eventsC == nil {
		return trace.Wrap(err)
	}
	select {
	case s.eventsC <- UploadEvent{
		SessionID: string(s.sessionID),
		Error:     err,
	}:
	default:
		slog.WarnContext(ctx, "Skip send event on a blocked channel.")
	}
	return trace.Wrap(err)
}

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

package events

import (
	"context"
	"log/slog"
	"sync"

	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events/auditqueue"
	"github.com/gravitational/teleport/lib/observability/metrics"
)

// auditFallbackQueuedEvents counts locally-originated audit events diverted to
// the fallback queue after the primary backend rejected them.
var auditFallbackQueuedEvents = prometheus.NewCounter(
	prometheus.CounterOpts{
		Namespace: teleport.MetricNamespace,
		Name:      "audit_fallback_queued_events_total",
		Help:      "Total number of locally-originated audit events diverted to the fallback queue after the primary audit backend rejected them.",
	},
)

type forwardedEmitKey struct{}

// WithForwardedEmit marks ctx as carrying an audit event forwarded from an
// agent. A FallbackEmitter does not queue such events when delivery fails. The
// originating instance owns their durability and will retry from its own queue,
// so the delivery error is returned to it instead.
func WithForwardedEmit(ctx context.Context) context.Context {
	return context.WithValue(ctx, forwardedEmitKey{}, struct{}{})
}

// isForwardedEmit reports whether ctx was marked by WithForwardedEmit.
func isForwardedEmit(ctx context.Context) bool {
	return ctx.Value(forwardedEmitKey{}) != nil
}

// FallbackEmitterConfig configures a FallbackEmitter.
type FallbackEmitterConfig struct {
	// Primary is the audit backend that events are emitted to.
	Primary apievents.Emitter
	// DataDir is the Teleport data directory, used for the fallback queue.
	DataDir string
	// EnableAuditQueue enables the fallback queue. When false, the
	// FallbackEmitter simply delegates to Primary with no fallback behavior.
	EnableAuditQueue bool
	// AuditQueueCfg holds the queue options from the Teleport yaml config.
	AuditQueueCfg auditqueue.Config
	// AuditQueueBackends is the ordered list of queue backends to try.
	AuditQueueBackends []auditqueue.Kind
}

// CheckAndSetDefaults checks and sets default values.
func (c *FallbackEmitterConfig) CheckAndSetDefaults() error {
	if c.Primary == nil {
		return trace.BadParameter("missing parameter Primary")
	}
	return nil
}

// FallbackEmitter emits audit events directly to a primary backend. When the
// primary backend fails to accept a locally-originated event, the event is
// enqueued to the Audit Queue so it can be retried later. This behavior was
// requested during drafting the RFD found below:
//
// See: https://github.com/gravitational/teleport.e/blob/rfd/0254-sqlite-audit-log-event-queue/rfd/0254-sqlite-audit-log-event-queue.md#auth-server--kubernetes-and-cloud-considerations
type FallbackEmitter struct {
	cfg    FallbackEmitterConfig
	queue  auditqueue.Queue
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewFallbackEmitter returns a FallbackEmitter wrapping cfg.Primary.
func NewFallbackEmitter(cfg FallbackEmitterConfig) (*FallbackEmitter, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := metrics.RegisterPrometheusCollectors(auditFallbackQueuedEvents); err != nil {
		return nil, trace.Wrap(err)
	}

	var queue auditqueue.Queue
	if cfg.EnableAuditQueue {
		var err error
		queue, err = makeQueue(cfg.DataDir, cfg.AuditQueueCfg, cfg.AuditQueueBackends)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	f := &FallbackEmitter{
		cfg:    cfg,
		queue:  queue,
		ctx:    ctx,
		cancel: cancel,
	}
	if queue != nil {
		slog.InfoContext(ctx, "Audit fallback queue is enabled.")
		f.wg.Go(func() {
			if err := queue.Run(ctx, f.deliver); err != nil && ctx.Err() == nil {
				slog.ErrorContext(ctx, "Audit fallback queue Run returned error.", "error", err)
			}
		})
	}
	return f, nil
}

// EmitAuditEvent emits the event to the primary backend. On failure, a
// locally-originated event is enqueued to the fallback queue and a nil error is
// returned.
func (f *FallbackEmitter) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	err := f.cfg.Primary.EmitAuditEvent(ctx, event)
	if err == nil {
		return nil
	}
	if f.queue == nil || isForwardedEmit(ctx) {
		return trace.Wrap(err)
	}

	slog.WarnContext(ctx,
		"Failed to emit audit event to the audit backend, falling back to the local queue.",
		"event_type", event.GetType(),
		"event_code", event.GetCode(),
		"error", err,
	)
	if qerr := f.queue.Enqueue(ctx, event); qerr != nil {
		slog.ErrorContext(ctx,
			"Failed to enqueue audit event to the local fallback queue.",
			"event_type", event.GetType(),
			"event_code", event.GetCode(),
			"error", qerr,
		)
		return trace.Wrap(qerr)
	}
	auditFallbackQueuedEvents.Inc()
	return nil
}

// deliver is the queue.Run handler. It emits queued events to the primary
// backend and returns the subset that were successfully delivered.
func (f *FallbackEmitter) deliver(ctx context.Context, items []auditqueue.Item) []auditqueue.Item {
	var delivered []auditqueue.Item
	for _, item := range items {
		if ctx.Err() != nil {
			return delivered
		}
		if err := f.cfg.Primary.EmitAuditEvent(ctx, item.Event); err != nil {
			slog.ErrorContext(ctx, "Failed to re-emit queued audit event.", "error", err)
			continue
		}
		delivered = append(delivered, item)
	}
	return delivered
}

// Close shuts down the background consumer and releases the queue.
func (f *FallbackEmitter) Close() error {
	f.cancel()
	f.wg.Wait()
	if f.queue != nil {
		return trace.Wrap(f.queue.Close())
	}
	return nil
}

// Shutdown makes a best effort attempt to flush pending audit events to the
// inner emitter before closing.
func (f *FallbackEmitter) Shutdown(ctx context.Context) error {
	if f.queue != nil {
		if err := f.queue.Drain(ctx); err != nil {
			slog.WarnContext(ctx,
				"Audit fallback queue drain returned an error during graceful shutdown.",
				"error", err,
			)
		}
	}
	return trace.Wrap(f.Close())
}

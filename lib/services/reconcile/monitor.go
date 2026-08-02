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

package reconcile

import (
	"context"
	"iter"
	"log/slog"
	"sync"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/observability/metrics"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// Config configures a [Monitor]. K is the resource key,
// T is the owned resource type the consumer registers, and V is the read-only
// view type yielded by the desired-state iterator. Views are borrowed: the
// reconciler never retains one past its loop iteration and materializes an
// owned T only when a create or update is actually performed.
type Config[K comparable, T any, V any] struct {
	// Matcher decides whether a desired resource should be registered.
	Matcher func(V) bool
	// GetCurrent returns the currently registered resource for a key.
	GetCurrent func(K) (T, bool)
	// RangeCurrent iterates all currently registered resources. It is only
	// consumed for the removal pass; the reconciler collects removal
	// candidates before invoking OnDelete, so implementations may hold a read
	// lock for the duration of the iteration.
	RangeCurrent func() iter.Seq2[K, T]
	// RangeDesired iterates read-only views of the desired resource set. If
	// the same key is yielded more than once, the first occurrence wins;
	// consumers merging multiple sources should yield them in decreasing
	// precedence order. A yielded error aborts the reconciliation cycle.
	RangeDesired func() iter.Seq2[V, error]
	// KeyOf returns the key identifying a view.
	KeyOf func(V) K
	// Materialize converts a view into an owned T. It is invoked only for
	// views that will be created or updated.
	Materialize func(V) T
	// CompareWithCurrent compares a registered resource against a desired
	// view, returning [Equal] when no update is needed.
	CompareWithCurrent func(current T, view V) int
	// OnCreate is called when a new resource is detected.
	OnCreate func(context.Context, T) error
	// OnUpdate is called when an existing resource is updated.
	OnUpdate func(ctx context.Context, new, old T) error
	// OnDelete is called when an existing resource is deleted.
	OnDelete func(context.Context, T) error
	// AllowOriginChanges allows the reconciler to replace a resource with one
	// of a different origin. Disallowed by default to enforce segregation
	// between resources from different sources.
	AllowOriginChanges bool
	// Changes returns a channel that fires when the desired resource set may
	// have changed — typically a topology cache change pulse such as
	// [DatabasesCache.DatabaseChanges]. [Monitor.Run] arms it before
	// every cycle so a change landing mid-cycle wakes the next one. Optional:
	// when unset, only [Monitor.Trigger] and context cancellation
	// drive cycles.
	Changes func() <-chan struct{}
	// Logger emits log messages.
	Logger *slog.Logger
	// Metrics is an optional Metrics created and registered by the
	// caller; a noop instance is created when unset.
	Metrics *Metrics
}

// CheckAndSetDefaults validates the configuration and sets defaults.
func (c *Config[K, T, V]) CheckAndSetDefaults() error {
	if c.Matcher == nil {
		return trace.BadParameter("missing reconciler Matcher")
	}
	if c.GetCurrent == nil {
		return trace.BadParameter("missing reconciler GetCurrent")
	}
	if c.RangeCurrent == nil {
		return trace.BadParameter("missing reconciler RangeCurrent")
	}
	if c.RangeDesired == nil {
		return trace.BadParameter("missing reconciler RangeDesired")
	}
	if c.KeyOf == nil {
		return trace.BadParameter("missing reconciler KeyOf")
	}
	if c.Materialize == nil {
		return trace.BadParameter("missing reconciler Materialize")
	}
	if c.CompareWithCurrent == nil {
		return trace.BadParameter("missing reconciler CompareWithCurrent")
	}
	if c.OnCreate == nil {
		return trace.BadParameter("missing reconciler OnCreate")
	}
	if c.OnUpdate == nil {
		return trace.BadParameter("missing reconciler OnUpdate")
	}
	if c.OnDelete == nil {
		return trace.BadParameter("missing reconciler OnDelete")
	}
	if c.Logger == nil {
		c.Logger = slog.With(teleport.ComponentKey, "reconciler")
	}
	if c.Metrics == nil {
		var err error
		c.Metrics, err = NewMetrics(metrics.NoopRegistry().Wrap("unknown"))
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// Monitor reconciles currently registered resources against a stream
// of read-only desired-state views, creating/updating/deleting registered
// resources appropriately. Reconciliation is level-based: the current side is
// consulted every cycle, so failed applies are retried on the next cycle.
type Monitor[K comparable, T any, V any] struct {
	cfg     Config[K, T, V]
	logger  *slog.Logger
	metrics *Metrics
	stats   *stats

	// triggerC carries external reconcile requests from Trigger. Buffered
	// with capacity one so pending triggers coalesce.
	triggerC chan struct{}
	initOnce sync.Once
	initC    chan struct{}
}

// New creates a new Monitor over read-only views.
func New[K comparable, T any, V any](cfg Config[K, T, V]) (*Monitor[K, T, V], error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Monitor[K, T, V]{
		cfg:      cfg,
		logger:   cfg.Logger.With("component", "reconciler"),
		metrics:  cfg.Metrics,
		stats:    &stats{},
		triggerC: make(chan struct{}, 1),
		initC:    make(chan struct{}),
	}, nil
}

// Run reconciles continuously until the context is canceled: an immediate
// first cycle, then one cycle per wake. Wakes come from the configured
// Changes pulse — armed before each cycle so a change landing mid-cycle is
// not lost — and from [Monitor.Trigger]. Bursts coalesce into a
// single following cycle.
func (r *Monitor[K, T, V]) Run(ctx context.Context) {
	defer r.logger.DebugContext(ctx, "Reconciler done.")
	for {
		// Arm the change pulse BEFORE reconciling so that a change landing
		// during the cycle wakes the next one.
		var pulse <-chan struct{}
		if r.cfg.Changes != nil {
			pulse = r.cfg.Changes()
		}

		if err := r.Reconcile(ctx); err != nil {
			r.logger.ErrorContext(ctx, "Failed to reconcile.", "error", err)
		}
		r.initOnce.Do(func() { close(r.initC) })

		select {
		case <-pulse:
		case <-r.triggerC:
		case <-ctx.Done():
			return
		}
	}
}

// Trigger requests a reconciliation cycle from a running [Monitor.Run]
// loop. It never blocks; requests made while a cycle is already pending
// coalesce. Use it for desired-state sources that are not covered by the
// Changes pulse (e.g. cloud fetcher results).
func (r *Monitor[K, T, V]) Trigger() {
	select {
	case r.triggerC <- struct{}{}:
	default:
	}
}

// Initialized returns a channel that is closed after the first
// reconciliation cycle performed by [Monitor.Run] completes.
func (r *Monitor[K, T, V]) Initialized() <-chan struct{} {
	return r.initC
}

// Reconcile diffs the desired views against the currently registered
// resources and applies the difference. Duplicate desired keys are ignored
// after their first occurrence.
func (r *Monitor[K, T, V]) Reconcile(ctx context.Context) error {
	r.stats.reset()
	start := time.Now()

	var errs []error
	seen := make(map[K]struct{})
	for view, err := range r.cfg.RangeDesired() {
		if err != nil {
			return trace.Wrap(err, "reading desired resources")
		}

		key := r.cfg.KeyOf(view)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}

		if err := r.processDesired(ctx, key, view); err != nil {
			errs = append(errs, err)
		}
	}

	// Collect removal candidates before deleting: OnDelete mutates the
	// current set, which must not happen while RangeCurrent is iterating it.
	type removal struct {
		key        K
		registered T
	}
	var removals []removal
	for key, registered := range r.cfg.RangeCurrent() {
		if _, ok := seen[key]; !ok {
			removals = append(removals, removal{key: key, registered: registered})
		}
	}
	for _, rm := range removals {
		if err := r.processRemoved(ctx, rm.key, rm.registered); err != nil {
			errs = append(errs, err)
		}
	}

	if r.stats.hasChanges() {
		r.logger.InfoContext(ctx, "Reconciliation completed",
			"took", time.Since(start).String(),
			"stats", r.stats,
		)
	}

	return trace.NewAggregate(errs...)
}

// processDesired checks a desired view against the currently registered
// resources, creating or updating as needed.
func (r *Monitor[K, T, V]) processDesired(ctx context.Context, key K, view V) error {
	registered, ok := r.cfg.GetCurrent(key)
	if !ok {
		kind, err := types.GetKind(view)
		if err != nil {
			return trace.Wrap(err)
		}
		if !r.cfg.Matcher(view) {
			r.logger.DebugContext(ctx, "New resource doesn't match, not creating", "kind", kind, "name", key)
			return nil
		}
		r.logger.InfoContext(ctx, "New resource matches, creating", "kind", kind, "name", key)
		if err := r.onCreate(ctx, kind, r.cfg.Materialize(view)); err != nil {
			return trace.Wrap(err, "failed to create %v %v", kind, key)
		}
		return nil
	}

	if !r.cfg.AllowOriginChanges {
		// Don't overwrite a resource of a different origin (e.g., keep a
		// static resource from config and ignore the dynamic one).
		registeredOrigin, err := types.GetOrigin(registered)
		if err != nil {
			return trace.Wrap(err)
		}
		viewOrigin, err := types.GetOrigin(view)
		if err != nil {
			return trace.Wrap(err)
		}
		if registeredOrigin != viewOrigin {
			kind, _ := types.GetKind(view)
			r.logger.WarnContext(ctx, "New resource has different origin, not updating",
				"kind", kind, "name", key, "new_origin", viewOrigin, "existing_origin", registeredOrigin)
			return nil
		}
	}

	kind, err := types.GetKind(registered)
	if err != nil {
		return trace.Wrap(err)
	}
	if r.cfg.CompareWithCurrent(registered, view) == Equal {
		r.logger.Log(ctx, logutils.TraceLevel, "Existing resource is already registered", "kind", kind, "name", key)
		return nil
	}

	if r.cfg.Matcher(view) {
		r.logger.InfoContext(ctx, "Existing resource updated, updating", "kind", kind, "name", key)
		if err := r.onUpdate(ctx, kind, r.cfg.Materialize(view), registered); err != nil {
			return trace.Wrap(err, "failed to update %v %v", kind, key)
		}
		return nil
	}

	r.logger.InfoContext(ctx, "Existing resource updated and no longer matches, deleting", "kind", kind, "name", key)
	if err := r.onDelete(ctx, kind, registered); err != nil {
		if trace.IsNotFound(err) {
			r.logger.Log(ctx, logutils.TraceLevel, "Failed to delete resource", "kind", kind, "name", key, "err", err)
			return nil
		}
		return trace.Wrap(err, "failed to delete %v %v", kind, key)
	}
	return nil
}

// processRemoved deletes a registered resource that is no longer present in
// the desired set.
func (r *Monitor[K, T, V]) processRemoved(ctx context.Context, key K, registered T) error {
	kind, err := types.GetKind(registered)
	if err != nil {
		return trace.Wrap(err)
	}
	r.logger.InfoContext(ctx, "Resource was removed, deleting", "kind", kind, "name", key)
	if err := r.onDelete(ctx, kind, registered); err != nil {
		if trace.IsNotFound(err) {
			r.logger.Log(ctx, logutils.TraceLevel, "Failed to delete resource", "kind", kind, "name", key, "err", err)
			return nil
		}
		return trace.Wrap(err, "failed to delete %v %v", kind, key)
	}
	return nil
}

// onCreate wraps the OnCreate callback with metrics and stats observation.
func (r *Monitor[K, T, V]) onCreate(ctx context.Context, kind string, newT T) error {
	start := time.Now()
	err := r.cfg.OnCreate(ctx, newT)
	if err == nil {
		r.stats.created.Add(1)
	}
	r.metrics.Observe(kind, OperationCreate, start, err)
	return trace.Wrap(err)
}

// onUpdate wraps the OnUpdate callback with metrics and stats observation.
func (r *Monitor[K, T, V]) onUpdate(ctx context.Context, kind string, newT, registered T) error {
	start := time.Now()
	err := r.cfg.OnUpdate(ctx, newT, registered)
	if err == nil {
		r.stats.updated.Add(1)
	}
	r.metrics.Observe(kind, OperationUpdate, start, err)
	return trace.Wrap(err)
}

// onDelete wraps the OnDelete callback with metrics and stats observation.
func (r *Monitor[K, T, V]) onDelete(ctx context.Context, kind string, registered T) error {
	start := time.Now()
	err := r.cfg.OnDelete(ctx, registered)
	if err == nil {
		r.stats.deleted.Add(1)
	}
	r.metrics.Observe(kind, OperationDelete, start, err)
	return trace.Wrap(err)
}

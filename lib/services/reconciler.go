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

package services

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/observability/metrics"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// Matcher is used by reconciler to match resources.
type Matcher[T any] func(T) bool

// GenericReconcilerConfig is the resource reconciler configuration that allows
// any type implementing comparable to be a key.
type GenericReconcilerConfig[K comparable, T any] struct {
	// Matcher is used to match resources.
	Matcher Matcher[T]
	// GetCurrentResources returns currently registered resources. Note that the
	// map keys must be consistent across the current and new resources.
	GetCurrentResources func() map[K]T
	// GetNewResources returns resources to compare current resources against.
	// Note that the map keys must be consistent across the current and new
	// resources.
	GetNewResources func() map[K]T
	// Compare allows custom comparators without having to implement IsEqual.
	// Defaults to `CompareResources[T]` if not specified.
	CompareResources func(T, T) int
	// OnCreate is called when a new resource is detected.
	OnCreate func(context.Context, T) error
	// OnUpdate is called when an existing resource is updated.
	OnUpdate func(ctx context.Context, new, old T) error
	// OnDelete is called when an existing resource is deleted.
	OnDelete func(context.Context, T) error
	// Logger emits log messages.
	Logger *slog.Logger
	// Metrics is an optional ReconcilerMetrics created by the caller.
	// The caller is responsible for registering the metrics.
	// Metrics can be nil, in this case the generic reconciler will generate its
	// own metrics, which won't be registered.
	// Passing a metrics struct might look like a cumbersome API but we have 2 challenges:
	// - some parts of Teleport are using one-shot reconcilers. Registering
	//   metrics on every run would fail and we would lose the past reconciliation
	//   data.
	// - we have many reconcilers in Teleport and making the caller create the
	//   metrics beforehand allows them to specify the metric subsystem.
	Metrics *ReconcilerMetrics
	// AllowOriginChanges is a flag that allows the reconciler to change the
	// origin value of a reconciled resource. By default, origin changes are
	// disallowed to enforce segregation between of resources from different
	// sources.
	AllowOriginChanges bool
	// Concurrency sets the number of goroutines used to process resources
	// during reconciliation. When set to 0 or 1, resources are processed
	// sequentially. When set to a value greater than 1, resources are
	// processed concurrently using up to that many goroutines.
	// The OnCreate, OnUpdate, OnDelete, Matcher, and CompareResources
	// callbacks must be safe for concurrent use when Concurrency > 1.
	Concurrency int
}

// CheckAndSetDefaults validates the reconciler configuration and sets defaults.
func (c *GenericReconcilerConfig[K, T]) CheckAndSetDefaults() error {
	if c.Matcher == nil {
		return trace.BadParameter("missing reconciler Matcher")
	}
	if c.GetCurrentResources == nil {
		return trace.BadParameter("missing reconciler GetCurrentResources")
	}
	if c.GetNewResources == nil {
		return trace.BadParameter("missing reconciler GetNewResources")
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
	if c.CompareResources == nil {
		return trace.BadParameter("missing reconciler CompareResources")
	}
	if c.Logger == nil {
		c.Logger = slog.With(teleport.ComponentKey, "reconciler")
	}
	if c.Concurrency < 1 {
		c.Concurrency = 1
	}
	if c.Metrics == nil {
		var err error
		// If we are not given metrics, we create our own so we don't
		// panic when trying to increment/observe.
		c.Metrics, err = NewReconcilerMetrics(metrics.NoopRegistry().Wrap("unknown"))
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// ReconcilerMetrics is a set of metrics that the reconciler will update during
// its reconciliation cycle.
type ReconcilerMetrics struct {
	reconciliationTotal    *prometheus.CounterVec
	reconciliationDuration *prometheus.HistogramVec
}

const (
	metricLabelResult          = "result"
	metricLabelResultSuccess   = "success"
	metricLabelResultError     = "error"
	metricLabelResultNoop      = "noop"
	metricLabelOperation       = "operation"
	metricLabelOperationCreate = "create"
	metricLabelOperationUpdate = "update"
	metricLabelOperationDelete = "delete"
	metricLabelKind            = "kind"
)

// NewReconcilerMetrics creates subsystem-scoped metrics for the reconciler.
// The caller is responsible for registering them into an appropriate registry.
// The same ReconcilerMetrics can be used across different reconcilers.
// The metrics subsystem cannot be empty.
func NewReconcilerMetrics(reg *metrics.Registry) (*ReconcilerMetrics, error) {
	if reg == nil {
		return nil, trace.BadParameter("missing metrics registry (this is a bug)")
	}
	if reg.Subsystem() == "" {
		return nil, trace.BadParameter("missing metrics subsystem (this is a bug)")
	}
	return &ReconcilerMetrics{
		reconciliationTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: reg.Namespace(),
			Subsystem: reg.Subsystem(),
			Name:      "reconciliation_total",
			Help:      "Total number of individual resource reconciliations.",
		}, []string{metricLabelKind, metricLabelOperation, metricLabelResult}),
		reconciliationDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: reg.Namespace(),
			Subsystem: reg.Subsystem(),
			Name:      "reconciliation_duration_seconds",
			Help:      "The duration of individual resource reconciliation in seconds.",
		}, []string{metricLabelKind, metricLabelOperation}),
	}, nil
}

// Register metrics in the specified [prometheus.Registerer], returns an error
// if any metric fails, but still tries to register every metric before returning.
func (m *ReconcilerMetrics) Register(r prometheus.Registerer) error {
	return trace.NewAggregate(
		r.Register(m.reconciliationTotal),
		r.Register(m.reconciliationDuration),
	)
}

// NewGenericReconciler creates a new GenericReconciler with provided configuration.
func NewGenericReconciler[K comparable, T any](cfg GenericReconcilerConfig[K, T]) (*GenericReconciler[K, T], error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &GenericReconciler[K, T]{
		cfg:     cfg,
		logger:  cfg.Logger,
		metrics: cfg.Metrics,
		stats:   &reconcileStats{},
	}, nil
}

// GenericReconciler reconciles currently registered resources with new
// resources and creates/updates/deletes them appropriately.
//
// It's used in combination with watchers by agents (app, database, desktop)
// to enable dynamically registered resources.
type GenericReconciler[K comparable, T any] struct {
	cfg     GenericReconcilerConfig[K, T]
	logger  *slog.Logger
	metrics *ReconcilerMetrics
	stats   *reconcileStats
}

// reconcileStats tracks the number of resources created, updated, and deleted
// during a reconciliation cycle.
type reconcileStats struct {
	created atomic.Int64
	updated atomic.Int64
	deleted atomic.Int64
}

func (s *reconcileStats) reset() {
	s.created.Store(0)
	s.updated.Store(0)
	s.deleted.Store(0)
}

func (s *reconcileStats) hasChanges() bool {
	return s.created.Load() > 0 || s.updated.Load() > 0 || s.deleted.Load() > 0
}

// LogValue implements [slog.LogValuer].
func (s *reconcileStats) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Int64("created", s.created.Load()),
		slog.Int64("updated", s.updated.Load()),
		slog.Int64("deleted", s.deleted.Load()),
	)
}

// onCreate wraps the OnCreate callback with metrics and stats observation.
func (r *GenericReconciler[K, T]) onCreate(ctx context.Context, kind string, newT T) error {
	start := time.Now()
	err := r.cfg.OnCreate(ctx, newT)
	if err == nil {
		r.stats.created.Add(1)
	}
	r.observeMetrics(kind, metricLabelOperationCreate, start, err)
	return trace.Wrap(err)
}

// onUpdate wraps the OnUpdate callback with metrics and stats observation.
func (r *GenericReconciler[K, T]) onUpdate(ctx context.Context, kind string, newT, registered T) error {
	start := time.Now()
	err := r.cfg.OnUpdate(ctx, newT, registered)
	if err == nil {
		r.stats.updated.Add(1)
	}
	r.observeMetrics(kind, metricLabelOperationUpdate, start, err)
	return trace.Wrap(err)
}

// onDelete wraps the OnDelete callback with metrics and stats observation.
func (r *GenericReconciler[K, T]) onDelete(ctx context.Context, kind string, registered T) error {
	start := time.Now()
	err := r.cfg.OnDelete(ctx, registered)
	if err == nil {
		r.stats.deleted.Add(1)
	}
	r.observeMetrics(kind, metricLabelOperationDelete, start, err)
	return trace.Wrap(err)
}

func (r *GenericReconciler[K, T]) observeMetrics(kind, operation string, start time.Time, err error) {
	r.metrics.reconciliationDuration.With(prometheus.Labels{
		metricLabelKind:      kind,
		metricLabelOperation: operation,
	}).Observe(time.Since(start).Seconds())

	var result string
	switch {
	case err == nil:
		result = metricLabelResultSuccess
	// Only delete-not-found is a noop (resource already gone).
	// For create/update, NotFound is a real error (e.g. backend race).
	case operation == metricLabelOperationDelete && trace.IsNotFound(err):
		result = metricLabelResultNoop
	default:
		result = metricLabelResultError
	}
	r.metrics.reconciliationTotal.With(prometheus.Labels{
		metricLabelKind:      kind,
		metricLabelOperation: operation,
		metricLabelResult:    result,
	}).Inc()
}

// Reconcile reconciles currently registered resources with new resources and
// creates/updates/deletes them appropriately.
func (r *GenericReconciler[K, T]) Reconcile(ctx context.Context) error {
	r.stats.reset()

	currentResources := r.cfg.GetCurrentResources()
	newResources := r.cfg.GetNewResources()

	r.logger.DebugContext(ctx, "Reconciling current resources with new resources",
		"current_resource_count", len(currentResources), "new_resource_count", len(newResources))

	start := time.Now()

	var g errgroup.Group
	g.SetLimit(r.cfg.Concurrency)

	var (
		mu   sync.Mutex
		errs []error
	)

	// Process already registered resources to see if any of them were removed.
	for key, current := range currentResources {
		g.Go(func() error {
			if err := r.processRegisteredResource(ctx, newResources, key, current); err != nil {
				mu.Lock()
				errs = append(errs, trace.Wrap(err))
				mu.Unlock()
			}
			return nil
		})
	}

	// Add new resources if there are any or refresh those that were updated.
	for key, newResource := range newResources {
		g.Go(func() error {
			if err := r.processNewResource(ctx, currentResources, key, newResource); err != nil {
				mu.Lock()
				errs = append(errs, trace.Wrap(err))
				mu.Unlock()
			}
			return nil
		})
	}

	// Error are collected separately.
	_ = g.Wait()

	if r.stats.hasChanges() {
		r.logger.InfoContext(ctx, "Reconciliation completed",
			"kind", r.resourceKind(currentResources, newResources),
			"took", time.Since(start),
			"stats", r.stats,
		)
	}

	// TODO(zmb3): with a large number of resources, this can return a lengthy
	// error message that is difficult to parse
	return trace.NewAggregate(errs...)
}

// resourceKind extracts the resource kind from the first available resource.
func (r *GenericReconciler[K, T]) resourceKind(currentResources, newResources map[K]T) string {
	for _, res := range currentResources {
		kind, err := types.GetKind(res)
		if err == nil {
			return kind
		}
	}
	for _, res := range newResources {
		kind, err := types.GetKind(res)
		if err == nil {
			return kind
		}
	}
	return "unknown"
}

// processRegisteredResource checks the specified registered resource against the
// new list of resources.
func (r *GenericReconciler[K, T]) processRegisteredResource(ctx context.Context, newResources map[K]T, key K, registered T) error {
	// See if this registered resource is still present among "new" resources.
	if _, ok := newResources[key]; ok {
		return nil
	}

	kind, err := types.GetKind(registered)
	if err != nil {
		return trace.Wrap(err)
	}
	r.logger.InfoContext(ctx, "Resource was removed, deleting", "kind", kind, "name", key)
	err = r.onDelete(ctx, kind, registered)
	if err != nil {
		if trace.IsNotFound(err) {
			r.logger.Log(ctx, logutils.TraceLevel, "Failed to delete resource", "kind", kind, "name", key, "err", err)
			return nil
		}
		return trace.Wrap(err, "failed to delete %v %v", kind, key)
	}
	return nil
}

// processNewResource checks the provided new resource against currently
// registered resources.
func (r *GenericReconciler[K, T]) processNewResource(ctx context.Context, currentResources map[K]T, key K, newT T) error {
	// First see if the resource is already registered and if not, whether it
	// matches the selector labels and should be registered.
	registered, ok := currentResources[key]
	if !ok {
		kind, err := types.GetKind(newT)
		if err != nil {
			return trace.Wrap(err)
		}
		if r.cfg.Matcher(newT) {
			r.logger.InfoContext(ctx, "New resource matches, creating", "kind", kind, "name", key)
			if err := r.onCreate(ctx, kind, newT); err != nil {
				return trace.Wrap(err, "failed to create %v %v", kind, key)
			}
			return nil
		}
		r.logger.DebugContext(ctx, "New resource doesn't match, not creating", "kind", kind, "name", key)
		return nil
	}

	if !r.cfg.AllowOriginChanges {
		// Don't overwrite resource of a different origin (e.g., keep static resource from config and ignore dynamic resource)
		registeredOrigin, err := types.GetOrigin(registered)
		if err != nil {
			return trace.Wrap(err)
		}
		newOrigin, err := types.GetOrigin(newT)
		if err != nil {
			return trace.Wrap(err)
		}
		if registeredOrigin != newOrigin {
			kind, _ := types.GetKind(newT)
			r.logger.WarnContext(ctx, "New resource has different origin, not updating",
				"kind", kind, "name", key, "new_origin", newOrigin, "existing_origin", registeredOrigin)
			return nil
		}
	}

	// If the resource is already registered but was updated, see if its
	// labels still match.
	kind, err := types.GetKind(registered)
	if err != nil {
		return trace.Wrap(err)
	}
	if r.cfg.CompareResources(newT, registered) != Equal {
		if r.cfg.Matcher(newT) {
			r.logger.InfoContext(ctx, "Existing resource updated, updating", "kind", kind, "name", key)
			if err := r.onUpdate(ctx, kind, newT, registered); err != nil {
				return trace.Wrap(err, "failed to update %v %v", kind, key)
			}
			return nil
		}
		r.logger.InfoContext(ctx, "Existing resource updated and no longer matches, deleting", "kind", kind, "name", key)
		err := r.onDelete(ctx, kind, registered)
		if err != nil {
			if trace.IsNotFound(err) {
				r.logger.Log(ctx, logutils.TraceLevel, "Failed to delete resource", "kind", kind, "name", key, "err", err)
				return nil
			}
			return trace.Wrap(err, "failed to delete %v %v", kind, key)
		}
		return nil
	}

	r.logger.Log(ctx, logutils.TraceLevel, "Existing resource is already registered", "kind", kind, "name", key)
	return nil
}

// ReconcilerConfig holds the configuration for a reconciler
type ReconcilerConfig[T any] GenericReconcilerConfig[string, T]

// Reconciler reconciles currently registered resources with new resources and
// creates/updates/deletes them appropriately.
//
// This type exists for backwards compatibility, and is a simple wrapper around
// a GenericReconciler[string, T]
type Reconciler[T any] GenericReconciler[string, T]

// NewReconciler creates a new reconciler with provided configuration.
//
// Creates a new GenericReconciler[string, T] and wraps it in a Reconciler[T]
// for backwards compatibility.
func NewReconciler[T any](cfg ReconcilerConfig[T]) (*Reconciler[T], error) {
	embedded, err := NewGenericReconciler(GenericReconcilerConfig[string, T](cfg))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return (*Reconciler[T])(embedded), nil
}

// Reconcile reconciles currently registered resources with new resources and
// creates/updates/deletes them appropriately.
func (r *Reconciler[T]) Reconcile(ctx context.Context) error {
	return (*GenericReconciler[string, T])(r).Reconcile(ctx)
}

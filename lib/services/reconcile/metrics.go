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
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport/lib/observability/metrics"
)

// Comparison outcomes returned by [Config.CompareWithCurrent].
const (
	// Equal indicates that the registered resource needs no update.
	Equal = 0
	// Different indicates that the registered resource must be updated.
	Different = 1
)

// Operation label values accepted by [Metrics.Observe].
const (
	OperationCreate = "create"
	OperationUpdate = "update"
	OperationDelete = "delete"
)

const (
	metricLabelResult        = "result"
	metricLabelResultSuccess = "success"
	metricLabelResultError   = "error"
	metricLabelResultNoop    = "noop"
	metricLabelOperation     = "operation"
	metricLabelKind          = "kind"
)

// Metrics is the set of metrics updated during reconciliation cycles.
type Metrics struct {
	reconciliationTotal    *prometheus.CounterVec
	reconciliationDuration *prometheus.HistogramVec
}

// NewMetrics creates subsystem-scoped reconciliation metrics. The caller is
// responsible for registering them into an appropriate registry. The same
// Metrics can be shared across reconcilers. The metrics subsystem cannot be
// empty.
func NewMetrics(reg *metrics.Registry) (*Metrics, error) {
	if reg == nil {
		return nil, trace.BadParameter("missing metrics registry (this is a bug)")
	}
	if reg.Subsystem() == "" {
		return nil, trace.BadParameter("missing metrics subsystem (this is a bug)")
	}
	return &Metrics{
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

// Register registers the metrics in the specified [prometheus.Registerer]; it
// returns an error if any metric fails but still tries to register every
// metric before returning.
func (m *Metrics) Register(r prometheus.Registerer) error {
	return trace.NewAggregate(
		r.Register(m.reconciliationTotal),
		r.Register(m.reconciliationDuration),
	)
}

// Observe records the outcome of a single reconciliation operation.
func (m *Metrics) Observe(kind, operation string, start time.Time, err error) {
	m.reconciliationDuration.With(prometheus.Labels{
		metricLabelKind:      kind,
		metricLabelOperation: operation,
	}).Observe(time.Since(start).Seconds())

	var result string
	switch {
	case err == nil:
		result = metricLabelResultSuccess
	// Only delete-not-found is a noop (resource already gone).
	// For create/update, NotFound is a real error (e.g. backend race).
	case operation == OperationDelete && trace.IsNotFound(err):
		result = metricLabelResultNoop
	default:
		result = metricLabelResultError
	}

	m.reconciliationTotal.With(prometheus.Labels{
		metricLabelKind:      kind,
		metricLabelOperation: operation,
		metricLabelResult:    result,
	}).Inc()
}

// stats tracks the number of resources created, updated, and deleted during a
// reconciliation cycle.
type stats struct {
	created atomic.Int64
	updated atomic.Int64
	deleted atomic.Int64
}

func (s *stats) reset() {
	s.created.Store(0)
	s.updated.Store(0)
	s.deleted.Store(0)
}

func (s *stats) hasChanges() bool {
	return s.created.Load() > 0 || s.updated.Load() > 0 || s.deleted.Load() > 0
}

// LogValue implements [slog.LogValuer].
func (s *stats) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Int64("created", s.created.Load()),
		slog.Int64("updated", s.updated.Load()),
		slog.Int64("deleted", s.deleted.Load()),
	)
}

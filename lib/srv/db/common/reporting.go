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

package common

import (
	"context"
	"net"
	"time"

	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/observability/metrics"
)

type reporterConfig struct {
	engine    Engine
	component string
	database  types.Database
	labels    prometheus.Labels
}

func (r *reporterConfig) CheckAndSetDefaults() error {
	if r.engine == nil {
		return trace.BadParameter("missing parameter engine")
	}
	if r.database == nil {
		return trace.BadParameter("missing parameter database")
	}
	if r.component == "" {
		r.component = teleport.ComponentDatabase
	}
	r.labels = getLabels(r.component, r.database)
	return nil
}

type reportingEngine struct {
	reporterConfig
}

func init() {
	_ = metrics.RegisterPrometheusCollectors(prometheusCollectorsEngine...)
}

// newReportingEngine returns new reporting engine, which wraps a regular Engine but reports various usage metrics.
func newReportingEngine(cfg reporterConfig) (*reportingEngine, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &reportingEngine{cfg}, nil
}

func (r *reportingEngine) InitializeConnection(clientConn net.Conn, sessionCtx *Session) error {
	initializedConnections.With(r.labels).Inc()
	return r.engine.InitializeConnection(clientConn, sessionCtx)
}

func (r *reportingEngine) SendError(err error) {
	engineErrors.With(r.labels).Inc()
	r.engine.SendError(err)
}

func (r *reportingEngine) HandleConnection(ctx context.Context, sessionCtx *Session) error {
	activeConnections.With(r.labels).Inc()
	defer activeConnections.With(r.labels).Dec()

	start := time.Now()
	defer func() {
		connectionDurations.With(r.labels).Observe(time.Since(start).Seconds())
	}()

	return trace.Wrap(r.engine.HandleConnection(ctx, sessionCtx))
}

var _ Engine = (*reportingEngine)(nil)

var commonLabels = []string{teleport.ComponentLabel, "db_protocol", "db_type"}

func getLabels(component string, db types.Database) prometheus.Labels {
	return map[string]string{
		teleport.ComponentLabel: component,
		"db_protocol":           db.GetProtocol(),
		"db_type":               db.GetType(),
	}
}

var (
	messagesFromClient = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Name:      "messages_from_client_total",
			Subsystem: "db",
			Help:      "Number of messages (packets) received from the DB client",
		},
		commonLabels,
	)

	messagesFromServer = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Name:      "messages_from_server_total",
			Subsystem: "db",
			Help:      "Number of messages (packets) received from the DB server",
		},
		commonLabels,
	)

	methodCallCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Name:      "method_call_count_total",
			Subsystem: "db",
			Help:      "Number of times a DB method was called",
		},
		append([]string{"method"}, commonLabels...),
	)

	methodCallLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: teleport.MetricNamespace,
			Name:      "method_call_latency_seconds",
			Subsystem: "db",
			Help:      "Call latency for a DB method calls",
			// lowest bucket start of upper bound 0.001 sec (1 ms) with factor 2
			// highest bucket start of 0.001 sec * 2^15 == 32.768 sec
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
		},
		append([]string{"method"}, commonLabels...),
	)

	initializedConnections = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Name:      "initialized_connections_total",
			Subsystem: "db",
			Help:      "Number of initialized DB connections",
		},
		commonLabels,
	)

	activeConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Name:      "active_connections_total",
			Subsystem: "db",
			Help:      "Number of active DB connections",
		},
		commonLabels,
	)

	connectionDurations = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: teleport.MetricNamespace,
			Name:      "connection_durations_seconds",
			Subsystem: "db",
			Help:      "Duration of DB connection",
			// 1ms ... 14.5h
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 20),
		},
		commonLabels,
	)

	connectionSetupTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: teleport.MetricNamespace,
			Name:      "connection_setup_time_seconds",
			Subsystem: "db",
			Help:      "Initial time to setup DB connection, before any requests are handled.",
			// 1ms ... 14.5h
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 20),
		},
		commonLabels,
	)

	engineErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: teleport.MetricNamespace,
			Name:      "errors_total",
			Subsystem: "db",
			Help:      "Number of synthetic DB errors sent to the client",
		},
		commonLabels,
	)

	prometheusCollectorsEngine = []prometheus.Collector{
		messagesFromClient, messagesFromServer,
		methodCallCount, methodCallLatency,

		initializedConnections, activeConnections, connectionDurations, connectionSetupTime, engineErrors,
	}
)

func methodCallMetrics(method, component string, db types.Database) func() {
	start := time.Now()
	l := getLabels(component, db)
	l["method"] = method

	methodCallCount.With(l).Inc()
	return func() {
		methodCallLatency.With(l).Observe(time.Since(start).Seconds())
	}
}

// GetConnectionSetupTimeObserver returns a callback that will observe connection setup time metric.
// The value observed will be time between the call of this function and the invocation of the callback.
func GetConnectionSetupTimeObserver(db types.Database) func() {
	start := time.Now()
	return func() {
		connectionSetupTime.WithLabelValues(teleport.ComponentDatabase, db.GetProtocol(), db.GetType()).Observe(time.Since(start).Seconds())
	}
}

// GetMessagesFromClientMetric increments the messages from client metric.
func GetMessagesFromClientMetric(db types.Database) prometheus.Counter {
	return messagesFromClient.WithLabelValues(teleport.ComponentDatabase, db.GetProtocol(), db.GetType())
}

// GetMessagesFromServerMetric increments the messages from server metric.
func GetMessagesFromServerMetric(db types.Database) prometheus.Counter {
	return messagesFromServer.WithLabelValues(teleport.ComponentDatabase, db.GetProtocol(), db.GetType())
}

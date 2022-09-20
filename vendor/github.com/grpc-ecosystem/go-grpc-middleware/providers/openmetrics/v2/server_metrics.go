// Copyright (c) The go-grpc-middleware Authors.
// Licensed under the Apache License 2.0.

package metrics

import (
	openmetrics "github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors"
)

// ServerMetrics represents a collection of metrics to be registered on a
// Prometheus metrics registry for a gRPC server.
type ServerMetrics struct {
	serverStartedCounter    *openmetrics.CounterVec
	serverHandledCounter    *openmetrics.CounterVec
	serverStreamMsgReceived *openmetrics.CounterVec
	serverStreamMsgSent     *openmetrics.CounterVec
	// serverHandledHistogram can be nil
	serverHandledHistogram *openmetrics.HistogramVec
}

// NewServerMetrics returns a new ServerMetrics object.
func NewServerMetrics(opts ...ServerMetricsOption) *ServerMetrics {
	var config serverMetricsConfig
	config.apply(opts)
	return &ServerMetrics{
		serverStartedCounter: openmetrics.NewCounterVec(
			config.counterOpts.apply(openmetrics.CounterOpts{
				Name: "grpc_server_started_total",
				Help: "Total number of RPCs started on the server.",
			}), []string{"grpc_type", "grpc_service", "grpc_method"}),
		serverHandledCounter: openmetrics.NewCounterVec(
			config.counterOpts.apply(openmetrics.CounterOpts{
				Name: "grpc_server_handled_total",
				Help: "Total number of RPCs completed on the server, regardless of success or failure.",
			}), []string{"grpc_type", "grpc_service", "grpc_method", "grpc_code"}),
		serverStreamMsgReceived: openmetrics.NewCounterVec(
			config.counterOpts.apply(openmetrics.CounterOpts{
				Name: "grpc_server_msg_received_total",
				Help: "Total number of RPC stream messages received on the server.",
			}), []string{"grpc_type", "grpc_service", "grpc_method"}),
		serverStreamMsgSent: openmetrics.NewCounterVec(
			config.counterOpts.apply(openmetrics.CounterOpts{
				Name: "grpc_server_msg_sent_total",
				Help: "Total number of gRPC stream messages sent by the server.",
			}), []string{"grpc_type", "grpc_service", "grpc_method"}),
		serverHandledHistogram: config.serverHandledHistogram,
	}
}

// NewRegisteredServerMetrics returns a custom ServerMetrics object registered
// with the user's registry, and registers some common metrics associated
// with every instance.
func NewRegisteredServerMetrics(registry openmetrics.Registerer, opts ...ServerMetricsOption) *ServerMetrics {
	customServerMetrics := NewServerMetrics(opts...)
	customServerMetrics.MustRegister(registry)
	return customServerMetrics
}

// Register registers the metrics with the registry.
// returns error much like DefaultRegisterer of Prometheus.
func (m *ServerMetrics) Register(registry openmetrics.Registerer) error {
	for _, collector := range m.toRegister() {
		if err := registry.Register(collector); err != nil {
			return err
		}
	}
	return nil
}

// MustRegister registers the metrics with the registry
// and panics if any error occurs much like DefaultRegisterer of Prometheus.
func (m *ServerMetrics) MustRegister(registry openmetrics.Registerer) {
	registry.MustRegister(m.toRegister()...)
}

func (m *ServerMetrics) toRegister() []openmetrics.Collector {
	res := []openmetrics.Collector{
		m.serverStartedCounter,
		m.serverHandledCounter,
		m.serverStreamMsgReceived,
		m.serverStreamMsgSent,
	}
	if m.serverHandledHistogram != nil {
		res = append(res, m.serverHandledHistogram)
	}
	return res
}

// Describe sends the super-set of all possible descriptors of metrics
// collected by this Collector to the provided channel and returns once
// the last descriptor has been sent.
func (m *ServerMetrics) Describe(ch chan<- *openmetrics.Desc) {
	m.serverStartedCounter.Describe(ch)
	m.serverHandledCounter.Describe(ch)
	m.serverStreamMsgReceived.Describe(ch)
	m.serverStreamMsgSent.Describe(ch)
	if m.serverHandledHistogram != nil {
		m.serverHandledHistogram.Describe(ch)
	}
}

// Collect is called by the Prometheus registry when collecting
// metrics. The implementation sends each collected metric via the
// provided channel and returns once the last metric has been sent.
func (m *ServerMetrics) Collect(ch chan<- openmetrics.Metric) {
	m.serverStartedCounter.Collect(ch)
	m.serverHandledCounter.Collect(ch)
	m.serverStreamMsgReceived.Collect(ch)
	m.serverStreamMsgSent.Collect(ch)
	if m.serverHandledHistogram != nil {
		m.serverHandledHistogram.Collect(ch)
	}
}

// InitializeMetrics initializes all metrics, with their appropriate null
// value, for all gRPC methods registered on a gRPC server. This is useful, to
// ensure that all metrics exist when collecting and querying.
func (m *ServerMetrics) InitializeMetrics(server *grpc.Server) {
	serviceInfo := server.GetServiceInfo()
	for serviceName, info := range serviceInfo {
		for _, mInfo := range info.Methods {
			m.preRegisterMethod(serviceName, &mInfo)
		}
	}
}

// preRegisterMethod is invoked on Register of a Server, allowing all gRPC services labels to be pre-populated.
func (m *ServerMetrics) preRegisterMethod(serviceName string, mInfo *grpc.MethodInfo) {
	methodName := mInfo.Name
	methodType := string(typeFromMethodInfo(mInfo))
	// These are just references (no increments), as just referencing will create the labels but not set values.
	_, _ = m.serverStartedCounter.GetMetricWithLabelValues(methodType, serviceName, methodName)
	_, _ = m.serverStreamMsgReceived.GetMetricWithLabelValues(methodType, serviceName, methodName)
	_, _ = m.serverStreamMsgSent.GetMetricWithLabelValues(methodType, serviceName, methodName)
	if m.serverHandledHistogram != nil {
		_, _ = m.serverHandledHistogram.GetMetricWithLabelValues(methodType, serviceName, methodName)
	}
	for _, code := range interceptors.AllCodes {
		_, _ = m.serverHandledCounter.GetMetricWithLabelValues(methodType, serviceName, methodName, code.String())
	}
}

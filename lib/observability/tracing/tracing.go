// Copyright 2022 Gravitational, Inc
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

package tracing

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/url"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	oteltrace "go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/gravitational/teleport"
)

const (
	// DefaultExporterDialTimeout is the default timeout for dialing the exporter.
	DefaultExporterDialTimeout = 5 * time.Second

	// VersionKey is the attribute key for the teleport version.
	VersionKey = "teleport.version"

	// ProcessIDKey is attribute key for the process ID.
	ProcessIDKey = "teleport.process.id"

	// HostnameKey is the attribute key for the hostname.
	HostnameKey = "teleport.host.name"

	// HostIDKey is the attribute key for the host UUID.
	HostIDKey = "teleport.host.uuid"
)

// Config used to set up the tracing exporter and provider
type Config struct {
	// Service is the name of the service that will be reported to the tracing system.
	Service string
	// Attributes is a set of key value pairs that will be added to all spans.
	Attributes []attribute.KeyValue
	// ExporterURL is the URL of the exporter.
	ExporterURL string
	// SamplingRate determines how many spans are recorded and exported
	SamplingRate float64
	// TLSCert is the TLS configuration to use for the exporter.
	TLSConfig *tls.Config
	// DialTimeout is the timeout for dialing the exporter.
	DialTimeout time.Duration
	// Logger is the logger to use.
	Logger logrus.FieldLogger

	exporterURL *url.URL
}

// CheckAndSetDefaults checks the config and sets default values.
func (c *Config) CheckAndSetDefaults() error {
	if c.Service == "" {
		return trace.BadParameter("service name cannot be empty")
	}

	if c.ExporterURL == "" {
		return trace.BadParameter("exporter URL cannot be empty")
	}

	// first check if a network address is specified, if it was, default
	// to using grpc. If provided a URL, ensure that it is valid
	_, _, err := net.SplitHostPort(c.ExporterURL)
	if err == nil {
		c.exporterURL = &url.URL{
			Scheme: "grpc",
			Host:   c.ExporterURL,
		}
	} else {
		exporterURL, err := url.Parse(c.ExporterURL)
		if err != nil {
			return trace.BadParameter("failed to parse exporter URL: %v", err)
		}
		c.exporterURL = exporterURL

	}

	if c.DialTimeout <= 0 {
		c.DialTimeout = DefaultExporterDialTimeout
	}

	if c.Logger == nil {
		c.Logger = logrus.WithField(trace.Component, teleport.ComponentTracing)
	}

	return nil
}

// NewExporter returns a new exporter that is configured per the provided Config.
func NewExporter(ctx context.Context, cfg Config) (sdktrace.SpanExporter, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	var httpOptions []otlptracehttp.Option
	grpcOptions := []otlptracegrpc.Option{otlptracegrpc.WithDialOption(grpc.WithBlock())}

	if cfg.TLSConfig != nil {
		httpOptions = append(httpOptions, otlptracehttp.WithTLSClientConfig(cfg.TLSConfig.Clone()))
		grpcOptions = append(grpcOptions, otlptracegrpc.WithTLSCredentials(credentials.NewTLS(cfg.TLSConfig.Clone())))
	} else {
		httpOptions = append(httpOptions, otlptracehttp.WithInsecure())
		grpcOptions = append(grpcOptions, otlptracegrpc.WithInsecure())
	}

	var traceClient otlptrace.Client
	switch cfg.exporterURL.Scheme {
	case "http", "https":
		httpOptions = append(httpOptions, otlptracehttp.WithEndpoint(cfg.ExporterURL[len(cfg.exporterURL.Scheme)+3:]))
		traceClient = otlptracehttp.NewClient(httpOptions...)
	case "grpc":
		grpcOptions = append(grpcOptions, otlptracegrpc.WithEndpoint(cfg.ExporterURL[len(cfg.exporterURL.Scheme)+3:]))
		traceClient = otlptracegrpc.NewClient(grpcOptions...)
	default:
		return nil, trace.BadParameter("unsupported exporter scheme: %q", cfg.exporterURL.Scheme)
	}

	ctx, cancel := context.WithTimeout(ctx, cfg.DialTimeout)
	defer cancel()
	exporter, err := otlptrace.New(ctx, traceClient)
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return nil, trace.ConnectionProblem(err, "failed to connect to tracing exporter %s: %v", cfg.ExporterURL, err)
	case err != nil:
		return nil, trace.NewAggregate(err, traceClient.Stop(context.Background()))
	}

	return exporter, nil
}

// Provider wraps the OpenTelemetry tracing provider to provide common tags for all tracers.
type Provider struct {
	provider *sdktrace.TracerProvider
}

// Tracer returns a Tracer with the given name and options. If a Tracer for
// the given name and options does not exist it is created, otherwise the
// existing Tracer is returned.
//
// If name is empty, DefaultTracerName is used instead.
//
// This method is safe to be called concurrently.
func (p *Provider) Tracer(instrumentationName string, opts ...oteltrace.TracerOption) oteltrace.Tracer {
	opts = append(opts, oteltrace.WithInstrumentationVersion(teleport.Version))

	return p.provider.Tracer(instrumentationName, opts...)
}

// Shutdown shuts down the span processors in the order they were registered.
func (p *Provider) Shutdown(ctx context.Context) error {
	return trace.NewAggregate(p.provider.ForceFlush(ctx), p.provider.Shutdown(ctx))
}

// NoopProvider creates a new Provider that never samples any spans.
func NoopProvider() *Provider {
	return &Provider{provider: sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.NeverSample()),
	)}
}

// NewTraceProvider creates a new Provider that is configured per the provided Config.
func NewTraceProvider(ctx context.Context, cfg Config) (*Provider, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	exporter, err := NewExporter(ctx, cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	attrs := []attribute.KeyValue{
		// the service name used to display traces in backends
		semconv.ServiceNameKey.String(cfg.Service),
		attribute.String(VersionKey, teleport.Version),
	}
	attrs = append(attrs, cfg.Attributes...)

	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithProcess(),
		resource.WithProcessPID(),
		resource.WithProcessExecutableName(),
		resource.WithProcessExecutablePath(),
		resource.WithProcessOwner(),
		resource.WithProcessRuntimeName(),
		resource.WithProcessRuntimeVersion(),
		resource.WithProcessRuntimeDescription(),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithAttributes(attrs...),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// set global propagator, the default is no-op.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	// override the global logging handled with one that uses the
	// configured logger instead
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		cfg.Logger.WithError(err).Warnf("failed to export traces.")
	}))

	// set global provider to our provider wrapper to have all tracers use the common TracerOptions
	provider := &Provider{provider: sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SamplingRate))),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(sdktrace.NewBatchSpanProcessor(exporter)),
	)}
	otel.SetTracerProvider(provider)

	return provider, nil
}

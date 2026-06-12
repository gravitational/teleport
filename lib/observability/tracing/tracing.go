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

package tracing

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.22.0"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/embedded"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/observability/tracing"
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
	Logger *slog.Logger
	// Client is the client to use to export traces. This takes precedence over creating a
	// new client with the ExporterURL. Ownership of the client is transferred to the
	// tracing provider. It should **NOT** be closed by the caller.
	Client *tracing.Client

	// exporterURL is the parsed value of ExporterURL that is populated
	// by CheckAndSetDefaults
	exporterURL *url.URL
}

// CheckAndSetDefaults checks the config and sets default values.
func (c *Config) CheckAndSetDefaults() error {
	if c.Service == "" {
		return trace.BadParameter("service name cannot be empty")
	}

	if c.Client == nil && c.ExporterURL == "" {
		return trace.BadParameter("exporter URL cannot be empty")
	}

	if c.DialTimeout <= 0 {
		c.DialTimeout = DefaultExporterDialTimeout
	}

	if c.Logger == nil {
		c.Logger = slog.With(teleport.ComponentKey, teleport.ComponentTracing)
	}

	if c.Client != nil {
		return nil
	}

	// first check if a network address is specified, if it was, default
	// to using grpc. If provided a URL, ensure that it is valid
	h, _, err := net.SplitHostPort(c.ExporterURL)
	if err != nil || h == "file" {
		exporterURL, err := url.Parse(c.ExporterURL)
		if err != nil {
			return trace.BadParameter("failed to parse exporter URL: %v", err)
		}
		c.exporterURL = exporterURL
		return nil
	}

	c.exporterURL = &url.URL{
		Scheme: "grpc",
		Host:   c.ExporterURL,
	}
	return nil
}

// Endpoint provides the properly formatted endpoint that the otlp client libraries
// are expecting.
func (c *Config) Endpoint() string {
	uri := *c.exporterURL

	if uri.Scheme == "file" {
		uri.RawQuery = ""
	}
	uri.Scheme = ""

	s := uri.String()
	if strings.HasPrefix(s, "//") {
		return s[2:]
	}
	return s
}

// Provider wraps the OpenTelemetry tracing provider to provide common tags for all tracers.
type Provider struct {
	provider *sdktrace.TracerProvider

	embedded.TracerProvider
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
	return &Provider{
		provider: sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sdktrace.NeverSample()),
		),
	}
}

// NoopTracer creates a new Tracer that never samples any spans.
func NoopTracer(instrumentationName string) oteltrace.Tracer {
	return NoopProvider().Tracer(instrumentationName)
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
		resource.WithProcessExecutableName(),
		resource.WithProcessRuntimeName(),
		resource.WithProcessRuntimeVersion(),
		resource.WithProcessRuntimeDescription(),
		resource.WithTelemetrySDK(),
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
		cfg.Logger.WarnContext(ctx, "Failed to export traces", "error", err)
	}))

	// set global provider to our provider wrapper to have all tracers use the common TracerOptions
	provider := &Provider{
		provider: sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SamplingRate))),
			sdktrace.WithResource(res),
			sdktrace.WithSpanProcessor(sdktrace.NewBatchSpanProcessor(exporter)),
		),
	}

	otel.SetTracerProvider(provider)

	return provider, nil
}

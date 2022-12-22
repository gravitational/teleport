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
	"errors"
	"io"

	"github.com/gravitational/trace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// NewExporter returns a new exporter that is configured per the provided Config.
func NewExporter(ctx context.Context, cfg Config) (sdktrace.SpanExporter, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	traceClient, err := NewClient(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
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

	if cfg.Client == nil {
		return exporter, nil
	}

	return &wrappedExporter{
		exporter: exporter,
		closer:   cfg.Client,
	}, nil
}

// wrappedExporter is a sdktrace.SpanExporter wrapper that is used to ensure that any
// io.Closer that are provided to NewExporter are closed when the Exporter is
// Shutdown.
type wrappedExporter struct {
	closer   io.Closer
	exporter sdktrace.SpanExporter
}

// ExportSpans forwards the spans to the wrapped exporter.
func (w *wrappedExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	return w.exporter.ExportSpans(ctx, spans)
}

// Shutdown shuts down the wrapped exporter and closes the client.
func (w *wrappedExporter) Shutdown(ctx context.Context) error {
	return trace.NewAggregate(w.exporter.Shutdown(ctx), w.closer.Close())
}

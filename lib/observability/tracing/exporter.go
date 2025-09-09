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
	"io"

	"github.com/gravitational/trace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// newExporter returns a new exporter that is configured per the provided Config.
// It also returns a *bufferedClient if one is required because the resource
// attributes or the real client are not yet available, otherwise it will be
// nil.
func newExporter(ctx context.Context, cfg Config) (sdktrace.SpanExporter, *bufferedClient, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	var (
		traceClient otlptrace.Client
		bufClient   *bufferedClient
		err         error
	)
	if cfg.WaitForDelayedResourceAttrs || cfg.WaitForDelayedClient {
		// newBufferedClient will create the client if possible, or wait for a
		// delayed client to be provided later.
		bufClient, err = newBufferedClient(cfg)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		traceClient = bufClient
	} else {
		traceClient, err = NewClient(cfg)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
	}

	ctx, cancel := context.WithTimeout(ctx, cfg.DialTimeout)
	defer cancel()
	exporter, err := otlptrace.New(ctx, traceClient)
	if err != nil {
		return nil, nil, trace.NewAggregate(err, traceClient.Stop(context.Background()))
	}

	if cfg.Client == nil {
		return exporter, bufClient, nil
	}

	return &wrappedExporter{
		exporter: exporter,
		closer:   cfg.Client,
	}, bufClient, nil
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

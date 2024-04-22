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

	"github.com/gravitational/trace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// PropagationContext contains tracing information to be passed across service boundaries
type PropagationContext map[string]string

// TraceParent is the name of the header or query parameter that contains
// tracing context across service boundaries.
const TraceParent = "traceparent"

// PropagationContextFromContext creates a PropagationContext from the given context.Context. If the context
// does not contain any tracing information, the PropagationContext will be empty.
func PropagationContextFromContext(ctx context.Context, opts ...Option) PropagationContext {
	carrier := propagation.MapCarrier{}
	NewConfig(opts).TextMapPropagator.Inject(ctx, &carrier)
	return PropagationContext(carrier)
}

// WithPropagationContext injects any tracing information from the given PropagationContext into the
// given context.Context.
func WithPropagationContext(ctx context.Context, pc PropagationContext, opts ...Option) context.Context {
	return NewConfig(opts).TextMapPropagator.Extract(ctx, propagation.MapCarrier(pc))
}

// DefaultProvider returns the global default TracerProvider.
func DefaultProvider() oteltrace.TracerProvider {
	return otel.GetTracerProvider()
}

// NewTracer creates a new [oteltrace.Tracer] from the global default
// [oteltrace.TracerProvider] with the provided name.
func NewTracer(name string) oteltrace.Tracer {
	return DefaultProvider().Tracer(name)
}

// EndSpan ends the given span and if an error has occurred, set's the span's
// status to error and additionally records the error.
//
// Example usage:
//
//	func myFunc() (err error) {
//	  ctx, span := tracer.Start(ctx, "myFunc")
//	  defer func() { tracing.EndSpan(span, err) }()
//	  ...
//	}
func EndSpan(span oteltrace.Span, err error) {
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(trace.Unwrap(err))
	}
	span.End()
}

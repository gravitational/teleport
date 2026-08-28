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
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// Option applies an option value for a Config.
type Option interface {
	apply(*Config)
}

// Config stores tracing related properties to customize
// creating Tracers and extracting TraceContext
type Config struct {
	TracerProvider    oteltrace.TracerProvider
	TextMapPropagator propagation.TextMapPropagator
}

// NewConfig returns a Config configured with all the passed Option.
func NewConfig(opts []Option) *Config {
	c := &Config{
		TracerProvider:    otel.GetTracerProvider(),
		TextMapPropagator: otel.GetTextMapPropagator(),
	}
	for _, o := range opts {
		o.apply(c)
	}
	return c
}

type tracerProviderOption struct{ tp oteltrace.TracerProvider }

func (o tracerProviderOption) apply(c *Config) {
	if o.tp != nil {
		c.TracerProvider = o.tp
	}
}

// WithTracerProvider returns an Option to use the trace.TracerProvider when
// creating a trace.Tracer.
func WithTracerProvider(tp oteltrace.TracerProvider) Option {
	return tracerProviderOption{tp: tp}
}

type propagatorOption struct{ p propagation.TextMapPropagator }

func (o propagatorOption) apply(c *Config) {
	if o.p != nil {
		c.TextMapPropagator = o.p
	}
}

// WithTextMapPropagator returns an Option to use the propagation.TextMapPropagator when extracting
// and injecting trace context.
func WithTextMapPropagator(p propagation.TextMapPropagator) Option {
	return propagatorOption{p: p}
}

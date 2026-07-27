// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package llm

import (
	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/observability/metrics"
)

const (
	// llmSubsystem is used to prefix Prometheus metrics for this subsystem.
	//
	// See https://prometheus.io/docs/practices/naming/#subsystem-name
	llmSubsystem = "llm"
)

// llmMetrics contains all metrics related to LLM access.
type llmMetrics struct {
	// requestDuration keeps track of the request duration, in seconds.
	requestDuration *prometheus.HistogramVec
	// requestSize keeps track of the request size, in bytes.
	requestSize *prometheus.HistogramVec
	// providerRequestDuration keeps track of the provider request duration, in
	// seconds.
	providerRequestDuration *prometheus.HistogramVec
	// providerRequestDuration keeps track of the provider response size, in
	// bytes.
	providerResponseSize *prometheus.HistogramVec
	// inputTokens keeps track of the LLM input token count.
	inputTokens *prometheus.CounterVec
	// outputTokens keeps track of the LLM output token count.
	outputTokens *prometheus.CounterVec
}

func newMetrics(reg *metrics.Registry) (*llmMetrics, error) {
	m := &llmMetrics{
		requestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: reg.Namespace(),
				Subsystem: llmSubsystem,
				Name:      "request_duration_seconds",
				Help:      "Latency distribution of the total request time.",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{teleport.ComponentLabel, "format", "provider"},
		),
		requestSize: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: reg.Namespace(),
				Subsystem: llmSubsystem,
				Name:      "request_size_bytes",
				Help:      "Distribution of the request size.",
				// 1KiB ... 32MiB
				Buckets: prometheus.ExponentialBuckets(1024, 2, 16),
			},
			[]string{teleport.ComponentLabel, "format", "provider"},
		),
		providerRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: reg.Namespace(),
				Subsystem: llmSubsystem,
				Name:      "provider_request_duration_seconds",
				Help:      "Latency distribution of the provider request time.",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{teleport.ComponentLabel, "format", "provider", "streaming"},
		),
		providerResponseSize: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: reg.Namespace(),
				Subsystem: llmSubsystem,
				Name:      "provider_response_size_bytes",
				Help:      "Distribution of the provider response size.",
				// Responses doesn't have a hard limit, meaning they can go up to
				// any size. We might need to tweak the bucket sizes in the future
				// to better match the actual response sizes.
				//
				// 1KiB ... 32MiB
				Buckets: prometheus.ExponentialBuckets(1024, 2, 16),
			},
			[]string{teleport.ComponentLabel, "format", "provider", "streaming"},
		),
		inputTokens: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: reg.Namespace(),
				Subsystem: llmSubsystem,
				Name:      "input_tokens_count",
				Help:      "Number of input tokens sent by clients.",
			},
			[]string{teleport.ComponentLabel, "format"},
		),
		outputTokens: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: reg.Namespace(),
				Subsystem: llmSubsystem,
				Name:      "output_tokens_count",
				Help:      "Number of output tokens sent by providers.",
			},
			[]string{teleport.ComponentLabel, "format"},
		),
	}

	return m, trace.NewAggregate(
		reg.Register(m.requestDuration),
		reg.Register(m.requestSize),
		reg.Register(m.providerRequestDuration),
		reg.Register(m.providerResponseSize),
		reg.Register(m.inputTokens),
		reg.Register(m.outputTokens),
	)
}

/*
Copyright 2024 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package transportlogger

import (
	"context"
	"net/http"

	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/utils/transportlogger/common"
	"github.com/gravitational/teleport/api/utils/transportlogger/providers/prometheus"
)

type options struct {
	roundTripper http.RoundTripper
	loggers      []LoggerFunc
	clock        clockwork.Clock
	serviceName  string
}

func (o *options) checkAndSetDefaults() error {
	if o.roundTripper == nil {
		o.roundTripper = http.DefaultTransport
	}
	if o.clock == nil {
		o.clock = clockwork.NewRealClock()
	}
	if len(o.loggers) == 0 {
		o.loggers = append(o.loggers, prometheus.Logger)
	}
	return nil
}

type OptionFunc func(o *options)

// WithRoundTripper sets the underlying http.RoundTripper to use for making requests.
func WithRoundTripper(rt http.RoundTripper) OptionFunc {
	return func(o *options) {
		o.roundTripper = rt
	}
}

// WithClock sets the underlying http.RoundTripper to use for making requests.
func WithClock(clock clockwork.Clock) OptionFunc {
	return func(o *options) {
		o.clock = clock
	}
}

// WithServiceName sets the service name to use for making requests.
func WithServiceName(name string) OptionFunc {
	return func(o *options) {
		o.serviceName = name
	}
}

// WithPrometheusLogger adds a Prometheus logger to the transport.
// This logger will report the result of the API call as Prometheus metrics.
func WithPrometheusLogger() OptionFunc {
	return func(o *options) {
		o.loggers = append(o.loggers, prometheus.Logger)
	}
}

// LoggerFunc is a function that logs the result of a call.
type LoggerFunc func(ctx context.Context, result *common.Result)

func (t *Transport) report(ctx context.Context, r *common.Result) {
	for _, logFn := range t.loggers {
		if logFn == nil {
			continue
		}
		logFn(ctx, r)
	}
}

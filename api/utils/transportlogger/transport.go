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
	"net/http"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/utils/transportlogger/common"
)

// NewTransport creates a new Transport.
func NewTransport(optionsFunc ...OptionFunc) (*Transport, error) {
	var settings options
	for _, option := range optionsFunc {
		option(&settings)
	}
	if err := settings.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Transport{
		roundTripper: settings.roundTripper,
		loggers:      settings.loggers,
		clock:        settings.clock,
		serviceName:  settings.serviceName,
	}, nil
}

// Transport is transport that logs the result of the API call.
type Transport struct {
	roundTripper http.RoundTripper
	loggers      []LoggerFunc
	clock        clockwork.Clock
	serviceName  string
}

// RoundTrip executes a single HTTP transaction and logs the result of the API call.
func (t *Transport) RoundTrip(r *http.Request) (*http.Response, error) {
	result := &common.Result{
		Name:    r.URL.Path,
		URL:     r.URL.String(),
		Host:    r.URL.Hostname(),
		Method:  r.Method,
		Service: t.serviceName,
	}

	if mi, ok := r.Context().Value(metricsInfoKey).(MetricsInfo); ok {
		result.Name = mi.CallName
	}

	start := time.Now()
	resp, err := t.roundTripper.RoundTrip(r)
	if err == nil {
		result.HttpStatusCode = resp.StatusCode
	}
	result.Err = err
	result.Duration = time.Since(start)

	// report the API call result.
	t.report(r.Context(), result)

	return resp, trace.Wrap(err)
}

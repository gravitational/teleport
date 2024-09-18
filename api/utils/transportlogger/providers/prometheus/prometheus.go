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

package prometheus

import (
	"context"
	"errors"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport/api/utils/transportlogger/common"
)

// Extended HTTP status by custom context errors
const (
	HTTPErrUnknownStatusLabelValue    = "unknown_status"
	HTTPErrDeadlineExceededLabelValue = "deadline_exceeded"
	HTTPErrContextCanceledLabelValue  = "context_canceled"
	HTTPErrRequestTimeoutLabelValue   = "request_timeout"
)

func Logger(ctx context.Context, result *common.Result) {
	if result == nil {
		return
	}
	ExternalAPICallMetric.With(prometheus.Labels{
		endpointLabel:   result.Name,
		httpStatusLabel: statusFromResult(result),
		httpMethodLabel: result.Method,
		serviceLabel:    result.Service,
	}).Inc()
	ExternalApiCallTimeMetric.With(prometheus.Labels{
		endpointLabel: result.Name,
	}).Observe(result.Duration.Seconds())
}

func statusFromResult(result *common.Result) string {
	if result.Err != nil {
		switch {
		case errors.Is(result.Err, context.DeadlineExceeded):
			return HTTPErrDeadlineExceededLabelValue
		case errors.Is(result.Err, context.Canceled):
			return HTTPErrContextCanceledLabelValue
		case errors.Is(result.Err, errTimeout),
			errors.Is(result.Err, errRequestCanceled),
			errors.Is(result.Err, errRequestCanceledConn):
			return HTTPErrRequestTimeoutLabelValue
		default:
			return HTTPErrUnknownStatusLabelValue
		}
	}
	return fmt.Sprintf("%d", result.HttpStatusCode)
}

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
	"github.com/prometheus/client_golang/prometheus"
)

const (
	endpointLabel   = "endpoint"
	hostLabel       = "endpoint"
	httpStatusLabel = "http_status"
	httpMethodLabel = "http_method"
	serviceLabel    = "service"
)

const (
	// NamespaceTeleport is a namespace for all teleport metrics.
	NamespaceTeleport = "teleport"
	// APICallStatusName is a label for API call status.
	APICallStatusName = "api_call_status"
	// APICallTimeName is a label for API call time.
	APICallTimeName = "api_call_time"
)

var (
	ExternalAPICallMetric = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: NamespaceTeleport,
		Name:      APICallStatusName,
		Help:      "Track calls to 3th party API",
	}, []string{endpointLabel, httpStatusLabel, httpMethodLabel, serviceLabel})

	ExternalApiCallTimeMetric = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: NamespaceTeleport,
		Name:      APICallTimeName,
		Help:      "API call time in seconds",
	}, []string{endpointLabel})
)

func init() {
	prometheus.MustRegister(ExternalAPICallMetric)
	prometheus.MustRegister(ExternalApiCallTimeMetric)
}

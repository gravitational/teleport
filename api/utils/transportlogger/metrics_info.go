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

import "context"

var metricsInfoKey struct{}

// MetricsInfo is a struct that contains information about the call that is being made.
type MetricsInfo struct {
	// CallName is the name of the call that is being made.
	// Many REST endpoint uses uuid in the URL, so it's for metrics entropy.
	// CallName allow to set limited values for the REST API call that will be used for metrics.
	CallName string
}

// WithMetricInfo sets the metrics information for the call.
func WithMetricInfo(ctx context.Context, info MetricsInfo) context.Context {
	return context.WithValue(ctx, metricsInfoKey, info)
}

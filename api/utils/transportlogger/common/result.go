/*
Copyright 2021 Gravitational, Inc.

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

package common

import "time"

// Result is the result of the HTTP API call.
type Result struct {
	// Information obtained from metricsInfoKey
	Name string
	// URL is the URL of the request.
	URL string
	// Host is the hostname of the request.
	Host string
	// Method is the HTTP method of the request.
	Method string
	// Err is the error that occurred during the call.
	Err error
	// HttpStatusCode is the HTTP status code of the response.
	HttpStatusCode int
	// Duration is the duration of the call.
	Duration time.Duration
	// Service  is the service name.
	Service string
}

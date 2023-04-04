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

package breaker

import (
	"net/http"
)

// RoundTripper wraps a http.RoundTripper with a CircuitBreaker
type RoundTripper struct {
	tripper http.RoundTripper
	cb      *CircuitBreaker
}

// NewRoundTripper returns a RoundTripper
func NewRoundTripper(cb *CircuitBreaker, tripper http.RoundTripper) *RoundTripper {
	return &RoundTripper{
		tripper: tripper,
		cb:      cb,
	}
}

// CloseIdleConnections ensures idle connections of the wrapped
// [http.RoundTripper] are closed.
func (t *RoundTripper) CloseIdleConnections() {
	type closeIdler interface {
		CloseIdleConnections()
	}
	if tr, ok := t.tripper.(closeIdler); ok {
		tr.CloseIdleConnections()
	}
}

// RoundTrip forwards the request on to the provided http.RoundTripper if
// the CircuitBreaker allows it
func (t *RoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	v, err := t.cb.Execute(func() (interface{}, error) {
		//nolint:bodyclose // The interface{} conversion to *http.Response trips the linter even though this
		// is merely a pass through function. Closing the body here would prevent the actual
		// consumer to not be able to read it. Copying here to satisfy the linter seems wasteful.
		return t.tripper.RoundTrip(request)
	})

	if v == nil {
		return nil, err
	}

	return v.(*http.Response), err
}

// Unwrap returns the inner round tripper.
func (t *RoundTripper) Unwrap() http.RoundTripper {
	return t.tripper
}

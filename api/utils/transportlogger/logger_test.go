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
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/transportlogger/providers/prometheus"
)

func TestLogger(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ok" {
			w.WriteHeader(http.StatusOK)
		} else if r.URL.Path == "/not_found" {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	transport, err := NewTransport(
		WithRoundTripper(srv.Client().Transport),
		WithServiceName("test_api"),
	)
	require.NoError(t, err)

	httpClient := &http.Client{
		Transport: transport,
	}
	ctx := context.Background()
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/ok", nil)
	require.NoError(t, err)
	if false {
		resp, err := httpClient.Do(req.WithContext(WithMetricInfo(ctx, MetricsInfo{CallName: "ok_endpoint"})))
		require.NoError(t, err)
		defer resp.Body.Close()

		req, err = http.NewRequest(http.MethodGet, srv.URL+"/not_found", nil)
		require.NoError(t, err)
		resp, err = httpClient.Do(req.WithContext(WithMetricInfo(ctx, MetricsInfo{CallName: "not_found_endpoint"})))
		require.NoError(t, err)
		defer resp.Body.Close()

		want := `
# HELP teleport_api_call_status Track calls to 3th party API
# TYPE teleport_api_call_status counter
teleport_api_call_status{endpoint="not_found_endpoint",http_method="GET",http_status="404",service="test_api"} 1
teleport_api_call_status{endpoint="ok_endpoint",http_method="POST",http_status="200",service="test_api"} 1
`
		err = testutil.CollectAndCompare(prometheus.ExternalAPICallMetric, bytes.NewBufferString(want))
		require.NoError(t, err)

		prometheus.ExternalAPICallMetric.Reset()
	}
	req, err = http.NewRequest(http.MethodGet, srv.URL+"/not_found", nil)
	require.NoError(t, err)
	resp, err := httpClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

}

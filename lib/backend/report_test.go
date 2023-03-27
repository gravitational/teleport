// Copyright 2021 Gravitational, Inc
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

package backend

import (
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestReporterTopRequestsLimit(t *testing.T) {
	// Test that a Reporter deletes older requests from metrics to limit memory
	// usage. For this test, we'll keep 10 requests.
	const topRequests = 10
	r, err := NewReporter(ReporterConfig{
		Backend:          &nopBackend{},
		Component:        "test",
		TopRequestsCount: topRequests,
	})
	require.NoError(t, err)

	countTopRequests := func() int {
		ch := make(chan prometheus.Metric)
		go func() {
			requests.Collect(ch)
			close(ch)
		}()

		var count int64
		for range ch {
			atomic.AddInt64(&count, 1)
		}
		return int(count)
	}
	t.Cleanup(requests.Reset)

	// At first, the metric should have no values.
	require.Equal(t, 0, countTopRequests())

	// Run through 1000 unique keys.
	for i := 0; i < 1000; i++ {
		r.trackRequest(types.OpGet, []byte(strconv.Itoa(i)), nil)
	}

	// Now the metric should have only 10 of the keys above.
	require.Equal(t, topRequests, countTopRequests())
}

func TestBuildKeyLabel(t *testing.T) {
	sensitivePrefixes := []string{"secret"}
	testCases := []struct {
		input  string
		masked string
	}{
		{"/secret/", "/secret/"},
		{"/secret/a", "/secret/a"},
		{"/secret/ab", "/secret/*b"},
		{"/secret/1b4d2844-f0e3-4255-94db-bf0e91883205", "/secret/***************************e91883205"},
		{"/secret/secret-role", "/secret/********ole"},
		{"/secret/graviton-leaf", "/secret/*********leaf"},
		{"/secret/graviton-leaf/sub1/sub2", "/secret/*********leaf"},
		{"/public/graviton-leaf", "/public/graviton-leaf"},
		{"/public/graviton-leaf/sub1/sub2", "/public/graviton-leaf"},
		{".data/secret/graviton-leaf", ".data/secret/graviton-leaf"},
	}
	for _, tc := range testCases {
		require.Equal(t, tc.masked, buildKeyLabel(tc.input, sensitivePrefixes))
	}
}

func TestBuildLabelKey_SensitiveBackendPrefixes(t *testing.T) {
	testCases := []struct {
		input  string
		masked string
	}{
		{"/tokens/1234-5678", "/tokens/******678"},
		{"/usertoken/1234-5678", "/usertoken/******678"},
		{"/access_requests/1234-5678", "/access_requests/******678"},

		{"/webauthn/sessionData/login/1234-5678", "/webauthn/sessionData"},
		{"/webauthn/sessionData/1234-5678", "/webauthn/sessionData"},
		{"/sessionData/1234-5678", "/sessionData/******678"},
	}
	for _, tc := range testCases {
		require.Equal(t, tc.masked, buildKeyLabel(tc.input, sensitiveBackendPrefixes))
	}
}

/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package backend

import (
	"context"
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
	for i := range 1000 {
		r.trackRequest(context.Background(), types.OpGet, NewKey(strconv.Itoa(i)), Key{})
	}

	// Now the metric should have only 10 of the keys above.
	require.Equal(t, topRequests, countTopRequests())
}

func TestBuildKeyLabel(t *testing.T) {
	sensitivePrefixes := []string{"secret"}
	singletonPrefixes := []string{"config"}
	testCases := []struct {
		input   string
		output  string
		isRange bool
	}{
		{
			input:   "/secret/",
			output:  "/secret",
			isRange: false,
		},
		{
			input:   "/secret/a",
			output:  "/secret",
			isRange: false,
		},
		{
			input:   "/secret/a/b",
			output:  "/secret/a",
			isRange: false,
		},
		{
			input:   "/secret/ab",
			output:  "/secret",
			isRange: false,
		},
		{
			input:   "/secret/ab/ba",
			output:  "/secret/*b",
			isRange: false,
		},
		{
			input:   "/secret/1b4d2844-f0e3-4255-94db-bf0e91883205",
			output:  "/secret",
			isRange: false,
		},
		{
			input:   "/secret/1b4d2844-f0e3-4255-94db-bf0e91883205/foobar",
			output:  "/secret/***************************e91883205",
			isRange: false,
		},
		{
			input:   "/secret/graviton-leaf",
			output:  "/secret",
			isRange: false,
		},
		{
			input:   "/secret/graviton-leaf/sub1/sub2",
			output:  "/secret/*********leaf",
			isRange: false,
		},
		{
			input:   "/public/graviton-leaf",
			output:  "/public",
			isRange: false,
		},
		{
			input:   "/public/graviton-leaf",
			output:  "/public/graviton-leaf",
			isRange: true,
		},
		{
			input:   "/public/graviton-leaf/sub1/sub2",
			output:  "/public/graviton-leaf",
			isRange: false,
		},
		{
			input:   ".data/secret/graviton-leaf",
			output:  ".data/secret",
			isRange: false,
		},
		{
			input:   "/config/example",
			output:  "/config/example",
			isRange: false,
		},
		{
			input:   "/config/example/something",
			output:  "/config/example",
			isRange: false,
		},
	}
	for _, tc := range testCases {
		require.Equal(t, tc.output, buildKeyLabel(
			tc.input,
			sensitivePrefixes,
			singletonPrefixes,
			tc.isRange,
		), "tc=%+v", tc)
	}
}

func TestBuildLabelKey_BackendPrefixes(t *testing.T) {
	testCases := []struct {
		input  string
		masked string
	}{
		{"/tokens/1234-5678/sub", "/tokens/******678"},
		{"/usertoken/1234-5678/sub", "/usertoken/******678"},
		{"/access_requests/1234-5678/sub", "/access_requests/******678"},

		{"/webauthn/sessionData/login/1234-5678", "/webauthn/sessionData"},
		{"/webauthn/sessionData/1234-5678", "/webauthn/sessionData"},
		{"/sessionData/1234-5678/sub", "/sessionData/******678"},
		{"/cluster_configuration/audit", "/cluster_configuration/audit"},
		{"/cluster_configuration/audit/foo", "/cluster_configuration/audit"},
	}
	for _, tc := range testCases {
		require.Equal(t, tc.masked, buildKeyLabel(tc.input, sensitiveBackendPrefixes, singletonBackendPrefixes, false))
	}
}

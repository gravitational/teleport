/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package readyz_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/tbot/readyz"
)

func TestReadyz(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Truncate(time.Second)
	clock := clockwork.NewFakeClockAt(now)

	reg := readyz.NewRegistry(readyz.WithClock(clock))

	a := reg.AddService("svc", "a")
	b := reg.AddService("svc", "b")

	srv := httptest.NewServer(readyz.HTTPHandler(reg))
	srv.URL = srv.URL + "/readyz"
	t.Cleanup(srv.Close)

	t.Run("initial state - overall", func(t *testing.T) {
		rsp, err := http.Get(srv.URL)
		require.NoError(t, err)
		defer rsp.Body.Close()

		require.Equal(t, http.StatusServiceUnavailable, rsp.StatusCode)

		var response readyz.OverallStatus
		err = json.NewDecoder(rsp.Body).Decode(&response)
		require.NoError(t, err)

		require.Equal(t,
			readyz.OverallStatus{
				Status: readyz.Unhealthy,
				Services: map[string]*readyz.ServiceStatus{
					"a": {Status: readyz.Initializing},
					"b": {Status: readyz.Initializing},
				},
			},
			response,
		)
	})

	t.Run("individual service", func(t *testing.T) {
		a.ReportReason(readyz.Unhealthy, "database is down")

		rsp, err := http.Get(srv.URL + "/a")
		require.NoError(t, err)
		defer rsp.Body.Close()

		require.Equal(t, http.StatusServiceUnavailable, rsp.StatusCode)

		var response readyz.ServiceStatus
		err = json.NewDecoder(rsp.Body).Decode(&response)
		require.NoError(t, err)

		require.Equal(t,
			readyz.ServiceStatus{
				Status:    readyz.Unhealthy,
				Reason:    "database is down",
				UpdatedAt: &now,
			},
			response,
		)
	})

	t.Run("mixed state", func(t *testing.T) {
		a.Report(readyz.Healthy)
		b.ReportReason(readyz.Unhealthy, "database is down")

		rsp, err := http.Get(srv.URL)
		require.NoError(t, err)
		defer rsp.Body.Close()

		require.Equal(t, http.StatusServiceUnavailable, rsp.StatusCode)

		var response readyz.OverallStatus
		err = json.NewDecoder(rsp.Body).Decode(&response)
		require.NoError(t, err)

		require.Equal(t,
			readyz.OverallStatus{
				Status: readyz.Unhealthy,
				Services: map[string]*readyz.ServiceStatus{
					"a": {Status: readyz.Healthy, UpdatedAt: &now},
					"b": {Status: readyz.Unhealthy, Reason: "database is down", UpdatedAt: &now},
				},
			},
			response,
		)
	})

	t.Run("all healthy", func(t *testing.T) {
		a.Report(readyz.Healthy)
		b.Report(readyz.Healthy)

		rsp, err := http.Get(srv.URL)
		require.NoError(t, err)
		defer rsp.Body.Close()

		require.Equal(t, http.StatusOK, rsp.StatusCode)

		var response readyz.OverallStatus
		err = json.NewDecoder(rsp.Body).Decode(&response)
		require.NoError(t, err)

		require.Equal(t,
			readyz.OverallStatus{
				Status: readyz.Healthy,
				Services: map[string]*readyz.ServiceStatus{
					"a": {Status: readyz.Healthy, UpdatedAt: &now},
					"b": {Status: readyz.Healthy, UpdatedAt: &now},
				},
			},
			response,
		)
	})

	t.Run("unknown service", func(t *testing.T) {
		rsp, err := http.Get(srv.URL + "/foo")
		require.NoError(t, err)
		defer rsp.Body.Close()

		require.Equal(t, http.StatusNotFound, rsp.StatusCode)
	})
}

func TestAllServicesReported(t *testing.T) {
	reg := readyz.NewRegistry()

	a := reg.AddService("svc", "a")
	b := reg.AddService("svc", "b")

	select {
	case <-reg.AllServicesReported():
		t.Fatal("AllServicesReported should be blocked")
	default:
	}

	a.Report(readyz.Healthy)

	select {
	case <-reg.AllServicesReported():
		t.Fatal("AllServicesReported should be blocked")
	default:
	}

	b.Report(readyz.Unhealthy)

	select {
	case <-reg.AllServicesReported():
	default:
		t.Fatal("AllServicesReported should not be blocked")
	}
}

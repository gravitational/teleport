/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package app

import (
	"log/slog"
	"testing"

	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
)

// gaugeValue reads the current value of the activeSessions gauge for
// the given app name.
func gaugeValue(t *testing.T, appName string) float64 {
	t.Helper()
	var m dto.Metric
	require.NoError(t, activeSessions.WithLabelValues(appName).Write(&m))
	return m.GetGauge().GetValue()
}

func TestActiveSessionsGauge(t *testing.T) {
	// Reset the gauge so parallel test runs do not interfere.
	activeSessions.Reset()

	c := &ConnectionsHandler{log: slog.Default()}
	t.Cleanup(c.cacheCloseWg.Wait)

	// Simulate two sessions being created for different apps.
	activeSessions.WithLabelValues("app-a").Inc()
	activeSessions.WithLabelValues("app-b").Inc()
	require.InDelta(t, 1.0, gaugeValue(t, "app-a"), 0)
	require.InDelta(t, 1.0, gaugeValue(t, "app-b"), 0)

	// Simulate session expiry via the cache eviction callback.
	sess := newSessionChunk(0)
	sess.appName = "app-a"
	c.onSessionExpired(t.Context(), "key1", sess)
	require.InDelta(t, 0.0, gaugeValue(t, "app-a"), 0)
	require.InDelta(t, 1.0, gaugeValue(t, "app-b"), 0)

	// Expire the second session.
	sess2 := newSessionChunk(0)
	sess2.appName = "app-b"
	c.onSessionExpired(t.Context(), "key2", sess2)
	require.InDelta(t, 0.0, gaugeValue(t, "app-b"), 0)
}

func TestActiveSessionsGaugeNonSessionChunk(t *testing.T) {
	// onSessionExpired should be a no-op when the expired value is
	// not a *sessionChunk. The gauge should not change.
	activeSessions.Reset()
	activeSessions.WithLabelValues("app-a").Set(1)

	c := &ConnectionsHandler{log: slog.Default()}
	c.onSessionExpired(t.Context(), "key", "not-a-session-chunk")
	require.InDelta(t, 1.0, gaugeValue(t, "app-a"), 0)
}

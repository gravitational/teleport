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
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/tlsca"
)

// gaugeValue reads the current value of the activeSessions gauge for the
// given app name.
func gaugeValue(t *testing.T, appName string) float64 {
	t.Helper()
	var m dto.Metric
	require.NoError(t, activeSessions.WithLabelValues(appName).Write(&m))
	return m.GetGauge().GetValue()
}

func TestActiveSessionsGauge(t *testing.T) {
	// Reset the gauge so previous test runs do not interfere.
	activeSessions.Reset()

	c := newConnectionsHandler(t)
	t.Cleanup(c.cacheCloseWg.Wait)

	identity := &tlsca.Identity{Username: "alice"}
	appA, err := types.NewAppV3(types.Metadata{Name: "app-a"}, types.AppSpecV3{URI: "http://localhost"})
	require.NoError(t, err)
	appB, err := types.NewAppV3(types.Metadata{Name: "app-b"}, types.AppSpecV3{URI: "http://localhost"})
	require.NoError(t, err)

	// Create two sessions for different apps.
	now := time.Now()
	sessA, err := c.newSessionChunk(t.Context(), identity, appA, now)
	require.NoError(t, err)
	sessB, err := c.newSessionChunk(t.Context(), identity, appB, now)
	require.NoError(t, err)
	require.Equal(t, 1.0, gaugeValue(t, "app-a")) //nolint:testifylint // Inc/Dec produce exact float64 integers
	require.Equal(t, 1.0, gaugeValue(t, "app-b")) //nolint:testifylint // Inc/Dec produce exact float64 integers

	// Expire the first session via the cache eviction callback. The gauge is
	// decremented asynchronously after the session finishes closing, so wait
	// for the background goroutine to complete before checking.
	c.onSessionExpired(t.Context(), "key1", sessA)
	c.cacheCloseWg.Wait()
	require.Equal(t, 0.0, gaugeValue(t, "app-a")) //nolint:testifylint // Inc/Dec produce exact float64 integers
	require.Equal(t, 1.0, gaugeValue(t, "app-b")) //nolint:testifylint // Inc/Dec produce exact float64 integers

	// Expire the second session.
	c.onSessionExpired(t.Context(), "key2", sessB)
	c.cacheCloseWg.Wait()
	require.Equal(t, 0.0, gaugeValue(t, "app-b")) //nolint:testifylint // Inc/Dec produce exact float64 integers
}

// newConnectionsHandler returns a ConnectionsHandler wired with minimal fakes
// so that newSessionChunk can be called without a full auth server.
func newConnectionsHandler(t *testing.T) *ConnectionsHandler {
	t.Helper()
	return &ConnectionsHandler{
		cfg: &ConnectionsHandlerConfig{
			Clock:       clockwork.NewRealClock(),
			DataDir:     t.TempDir(),
			Emitter:     events.NewDiscardEmitter(),
			AuthClient:  metricsAuthClient{},
			AccessPoint: metricsAccessPoint{},
		},
		closeContext: t.Context(),
		log:          slog.Default(),
	}
}

// metricsAuthClient is a minimal fake that satisfies authclient.ClientI for
// the calls made by newSessionChunk: session tracker creation, tracker state
// updates, and audit stream creation.
type metricsAuthClient struct {
	authclient.ClientI
}

func (metricsAuthClient) CreateSessionTracker(_ context.Context, st types.SessionTracker) (types.SessionTracker, error) {
	return st, nil
}

func (metricsAuthClient) UpdateSessionTracker(_ context.Context, _ *proto.UpdateSessionTrackerRequest) error {
	return nil
}

func (metricsAuthClient) CreateAuditStream(_ context.Context, _ session.ID) (apievents.Stream, error) {
	return events.NewDiscardRecorder(), nil
}

func (metricsAuthClient) ResumeAuditStream(_ context.Context, _ session.ID, _ string) (apievents.Stream, error) {
	return events.NewDiscardRecorder(), nil
}

// metricsAccessPoint is a minimal fake that satisfies
// [authclient.AppsAccessPoint] for the two calls made by newSessionRecorder.
type metricsAccessPoint struct {
	authclient.AppsAccessPoint
}

func (metricsAccessPoint) GetSessionRecordingConfig(_ context.Context) (types.SessionRecordingConfig, error) {
	return types.DefaultSessionRecordingConfig(), nil
}

func (metricsAccessPoint) GetClusterName(_ context.Context) (types.ClusterName, error) {
	return types.NewClusterName(types.ClusterNameSpecV2{ClusterName: "test", ClusterID: "test"})
}

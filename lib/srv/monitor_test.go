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

package srv

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
)

func newTestMonitor(ctx context.Context, t *testing.T, asrv *auth.TestAuthServer, mut ...func(*MonitorConfig)) (*mockTrackingConn, *events.MockEmitter, MonitorConfig) {
	conn := &mockTrackingConn{make(chan struct{})}
	emitter := &events.MockEmitter{}
	cfg := MonitorConfig{
		Context:     ctx,
		Conn:        conn,
		Emitter:     emitter,
		Clock:       asrv.Clock(),
		Tracker:     &mockActivityTracker{asrv.Clock()},
		Entry:       logrus.StandardLogger(),
		LockWatcher: asrv.LockWatcher,
		LockTargets: []types.LockTarget{{User: "test-user"}},
		LockingMode: constants.LockingModeBestEffort,
	}
	for _, f := range mut {
		f(&cfg)
	}
	require.NoError(t, StartMonitor(cfg))
	return conn, emitter, cfg
}

func TestMonitorLockInForce(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	asrv, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		Dir:   t.TempDir(),
		Clock: clockwork.NewFakeClock(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, asrv.Close()) })

	conn, emitter, cfg := newTestMonitor(ctx, t, asrv)
	select {
	case <-conn.closedC:
		t.Fatal("Connection is already closed.")
	default:
	}
	lock, err := types.NewLock("test-lock", types.LockSpecV2{Target: cfg.LockTargets[0]})
	require.NoError(t, err)
	require.NoError(t, asrv.AuthServer.UpsertLock(ctx, lock))
	select {
	case <-conn.closedC:
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for connection close.")
	}
	require.Equal(t, services.LockInForceAccessDenied(lock).Error(), emitter.LastEvent().(*apievents.ClientDisconnect).Reason)

	// Monitor should also detect preexistent locks.
	conn, emitter, cfg = newTestMonitor(ctx, t, asrv)
	select {
	case <-conn.closedC:
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for connection close.")
	}
	require.Equal(t, services.LockInForceAccessDenied(lock).Error(), emitter.LastEvent().(*apievents.ClientDisconnect).Reason)
}

func TestMonitorStaleLocks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	asrv, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		Dir:   t.TempDir(),
		Clock: clockwork.NewFakeClock(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, asrv.Close()) })

	conn, emitter, _ := newTestMonitor(ctx, t, asrv, func(cfg *MonitorConfig) {
		cfg.LockingMode = constants.LockingModeStrict
	})
	select {
	case <-conn.closedC:
		t.Fatal("Connection is already closed.")
	default:
	}

	select {
	case <-asrv.LockWatcher.LoopC:
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for LockWatcher loop check.")
	}
	select {
	case asrv.LockWatcher.StaleC <- struct{}{}:
	default:
		t.Fatal("No staleness event should be scheduled yet. This is a bug in the test.")
	}
	go asrv.Backend.CloseWatchers()
	select {
	case <-asrv.LockWatcher.ResetC:
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for LockWatcher reset.")
	}
	select {
	case <-conn.closedC:
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for connection close.")
	}
	require.Equal(t, services.StrictLockingModeAccessDenied.Error(), emitter.LastEvent().(*apievents.ClientDisconnect).Reason)
}

type mockTrackingConn struct {
	closedC chan struct{}
}

func (c *mockTrackingConn) LocalAddr() net.Addr  { return &net.IPAddr{IP: net.IPv6loopback} }
func (c *mockTrackingConn) RemoteAddr() net.Addr { return &net.IPAddr{IP: net.IPv6loopback} }
func (c *mockTrackingConn) Close() error {
	close(c.closedC)
	return nil
}

type mockActivityTracker struct {
	clock clockwork.Clock
}

func (t *mockActivityTracker) GetClientLastActive() time.Time {
	return t.clock.Now()
}
func (t *mockActivityTracker) UpdateClientActivity() {}

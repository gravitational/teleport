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

package srv

import (
	"context"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

func newTestMonitor(ctx context.Context, t *testing.T, asrv *auth.TestAuthServer, mut ...func(*MonitorConfig)) (*mockTrackingConn, *eventstest.ChannelEmitter, MonitorConfig) {
	ctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)

	conn := &mockTrackingConn{closedC: make(chan struct{})}
	emitter := eventstest.NewChannelEmitter(1)
	cfg := MonitorConfig{
		Context:        ctx,
		Conn:           conn,
		Emitter:        emitter,
		EmitterContext: ctx,
		Clock:          asrv.Clock(),
		Tracker:        &mockActivityTracker{asrv.Clock()},
		Logger:         utils.NewSlogLoggerForTests(),
		LockWatcher:    asrv.LockWatcher,
		LockTargets:    []types.LockTarget{{User: "test-user"}},
		LockingMode:    constants.LockingModeBestEffort,
	}
	for _, f := range mut {
		f(&cfg)
	}
	require.NoError(t, StartMonitor(cfg))
	return conn, emitter, cfg
}

func TestConnectionMonitorLockInForce(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	asrv, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		Dir:   t.TempDir(),
		Clock: clockwork.NewFakeClock(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, asrv.Close()) })

	// Create a connection monitor that points to our test
	// Auth server.
	emitter := eventstest.NewChannelEmitter(1)
	monitor, err := NewConnectionMonitor(ConnectionMonitorConfig{
		AccessPoint:    asrv.AuthServer,
		Emitter:        emitter,
		EmitterContext: ctx,
		Clock:          asrv.Clock(),
		Logger:         utils.NewSlogLoggerForTests(),
		LockWatcher:    asrv.LockWatcher,
		ServerID:       "test",
	})
	require.NoError(t, err)

	lock, err := types.NewLock("test-lock", types.LockSpecV2{Target: types.LockTarget{User: "test-user"}})
	require.NoError(t, err)

	identity := &authz.LocalUser{
		Username: "test-user",
		Identity: tlsca.Identity{
			Username: "test-user",
		},
	}

	authzCtx := &authz.Context{
		Checker:          mockChecker{},
		Identity:         identity,
		UnmappedIdentity: identity,
	}

	t.Run("lock created after connection has been established", func(t *testing.T) {
		// Create a fake connection and monitor it.
		tconn := &mockTrackingConn{closedC: make(chan struct{})}
		monitorCtx, _, err := monitor.MonitorConn(ctx, authzCtx, tconn)
		require.NoError(t, err)
		require.NoError(t, monitorCtx.Err())

		// Create a lock targeting the user that was connected above.
		require.NoError(t, asrv.AuthServer.UpsertLock(ctx, lock))

		// Assert that the connection was terminated.
		select {
		case <-tconn.closedC:
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for connection close.")
		}

		// Assert that the context was canceled and verify the cause.
		require.Error(t, monitorCtx.Err())
		cause := context.Cause(monitorCtx)
		require.True(t, trace.IsAccessDenied(cause))
		for _, contains := range []string{"lock", "in force"} {
			require.Contains(t, cause.Error(), contains)
		}

		// Validate that the disconnect event was logged.
		require.Equal(t, services.LockInForceAccessDenied(lock).Error(), (<-emitter.C()).(*apievents.ClientDisconnect).Reason)
	})

	t.Run("connection terminated if lock already exists", func(t *testing.T) {
		// Create another connection for the locked user and validate
		// that it is terminated right away.
		tconn := &mockTrackingConn{closedC: make(chan struct{})}
		monitorCtx, _, err := monitor.MonitorConn(ctx, authzCtx, tconn)
		require.NoError(t, err)

		// Assert that the context was canceled and that the connection was terminated.
		require.Error(t, monitorCtx.Err())
		select {
		case <-tconn.closedC:
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for connection close.")
		}

		// Validate that the disconnect event was logged.
		require.Equal(t, services.LockInForceAccessDenied(lock).Error(), (<-emitter.C()).(*apievents.ClientDisconnect).Reason)
	})
}

func TestMonitorLockInForce(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
	case disconnectEvent := <-emitter.C():
		reason := (disconnectEvent).(*apievents.ClientDisconnect).Reason
		require.Equal(t, services.LockInForceAccessDenied(lock).Error(), reason, "expected error matching client disconnect")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for connection close event.")
	}

	select {
	case <-conn.closedC:
		// connection closed, continue
	default:
		t.Fatal("Connection not yet closed.")
	}

	// Monitor should also detect preexistent locks.
	conn, emitter, cfg = newTestMonitor(ctx, t, asrv)
	select {
	case <-conn.closedC:
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for connection close.")
	}
	require.Equal(t, services.LockInForceAccessDenied(lock).Error(), (<-emitter.C()).(*apievents.ClientDisconnect).Reason)
}

func TestMonitorStaleLocks(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
	case <-time.After(15 * time.Second):
		t.Fatal("Timeout waiting for LockWatcher loop check.")
	}

	// ensure ResetC is drained
	select {
	case <-asrv.LockWatcher.ResetC:
	default:
	}
	go asrv.Backend.CloseWatchers()

	// wait for reset
	select {
	case <-asrv.LockWatcher.ResetC:
	case <-time.After(15 * time.Second):
		t.Fatal("Timeout waiting for LockWatcher reset.")
	}
	// StaleC is listened by multiple goroutines, so we need to close to ensure
	// that all of them are unblocked and the stale state is detected.
	close(asrv.LockWatcher.StaleC)
	require.Eventually(t, func() bool {
		return asrv.LockWatcher.IsStale()
	}, 15*time.Second, 100*time.Millisecond, "Timeout waiting for LockWatcher to be stale.")
	select {
	case <-conn.closedC:
	case <-time.After(15 * time.Second):
		t.Fatal("Timeout waiting for connection close.")
	}
	require.Equal(t, services.StrictLockingModeAccessDenied.Error(), (<-emitter.C()).(*apievents.ClientDisconnect).Reason)
}

func TestWritesDisconnectMessage(t *testing.T) {
	asrv, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		Dir:   t.TempDir(),
		Clock: clockwork.NewFakeClock(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, asrv.Close()) })

	var sw strings.Builder

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clock := clockwork.NewFakeClock()
	conn, _, _ := newTestMonitor(ctx, t, asrv, func(cfg *MonitorConfig) {
		cfg.ClientIdleTimeout = 1 * time.Second
		cfg.Clock = clock
		cfg.MessageWriter = &sw
	})
	clock.BlockUntil(1)
	clock.Advance(2 * time.Second)
	<-conn.closedC
	require.Contains(t, sw.String(), "exceeded idle timeout")
}

type mockTrackingConn struct {
	net.Conn
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

// TestMonitorDisconnectExpiredCertBeforeTimeNow test case where DisconnectExpiredCert
// is already before time.Now
func TestMonitorDisconnectExpiredCertBeforeTimeNow(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewRealClock()

	certExpirationTime := clock.Now().Add(-1 * time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	asrv, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		Dir:   t.TempDir(),
		Clock: clockwork.NewFakeClock(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, asrv.Close()) })

	conn, _, _ := newTestMonitor(ctx, t, asrv, func(config *MonitorConfig) {
		config.Clock = clock
		config.DisconnectExpiredCert = certExpirationTime
	})

	select {
	case <-conn.closedC:
	case <-time.After(5 * time.Second):
		t.Fatal("Client is still connected.")
	}
}

func TestTrackingReadConn(t *testing.T) {
	server, client := net.Pipe()
	defer client.Close()

	// Close the server to force client reads to instantly return EOF.
	require.NoError(t, server.Close())

	// Wrap the client in a TrackingReadConn.
	ctx, cancel := context.WithCancelCause(context.Background())
	tc, err := NewTrackingReadConn(TrackingReadConnConfig{
		Conn:    client,
		Clock:   clockwork.NewFakeClock(),
		Context: ctx,
		Cancel:  cancel,
	})
	require.NoError(t, err)

	t.Run("Read EOF", func(t *testing.T) {
		// Make sure it returns an EOF and not a wrapped exception.
		buf := make([]byte, 64)
		_, err = tc.Read(buf)
		require.Equal(t, io.EOF, err)
	})

	t.Run("CloseWithCause", func(t *testing.T) {
		require.NoError(t, tc.CloseWithCause(trace.AccessDenied("fake problem")))
		require.ErrorIs(t, context.Cause(ctx), trace.AccessDenied("fake problem"))
	})

	t.Run("Close", func(t *testing.T) {
		ctx, cancel := context.WithCancelCause(context.Background())
		tc, err := NewTrackingReadConn(TrackingReadConnConfig{
			Conn:    client,
			Clock:   clockwork.NewFakeClock(),
			Context: ctx,
			Cancel:  cancel,
		})
		require.NoError(t, err)
		require.NoError(t, tc.Close())
		require.ErrorIs(t, context.Cause(ctx), io.EOF)
	})
}

type mockChecker struct {
	services.AccessChecker
}

func (m mockChecker) AdjustDisconnectExpiredCert(disconnect bool) bool {
	return disconnect
}

func (m mockChecker) AdjustClientIdleTimeout(ttl time.Duration) time.Duration {
	return ttl
}

func (m mockChecker) LockingMode(defaultMode constants.LockingMode) constants.LockingMode {
	return defaultMode
}

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

package auth_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/auditqueue"
)

// toggleBackend is a controllable audit backend that records delivered events
// and can be switched to fail, standing in for an SQS outage.
type toggleBackend struct {
	mu   sync.Mutex
	fail bool                   // Simulate backend failures.
	got  []apievents.AuditEvent // All events that we received.
}

func (b *toggleBackend) setFail(fail bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.fail = fail
}

func (b *toggleBackend) count() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.got)
}

func (b *toggleBackend) EmitAuditEvent(_ context.Context, event apievents.AuditEvent) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.fail {
		return trace.ConnectionProblem(nil, "backend unavailable")
	}
	b.got = append(b.got, event)
	return nil
}

func newAuthFallback(t *testing.T, backend *toggleBackend) *events.FallbackEmitter {
	t.Helper()
	fe, err := events.NewFallbackEmitter(events.FallbackEmitterConfig{
		Primary:          backend,
		DataDir:          t.TempDir(),
		EnableAuditQueue: true,
		AuditQueueCfg: auditqueue.Config{
			MaxAttempts:             3,
			DeadLetterSweepInterval: 50 * time.Millisecond,
		},
		AuditQueueBackends: []auditqueue.Kind{auditqueue.KindSQLiteMemory},
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, fe.Close()) })
	return fe
}

func userLoginEvent() apievents.AuditEvent {
	return &apievents.UserLogin{
		Metadata: apievents.Metadata{
			Type: events.UserLoginEvent,
			Code: events.UserLocalLoginCode,
		},
		Status: apievents.Status{Success: true},
	}
}

func TestEmitAuditEvent_ForwardedNotAdopted_GRPC(t *testing.T) {
	ctx := context.Background()
	backend := &toggleBackend{}
	backend.setFail(true)

	srv := newTestTLSServer(t)
	srv.Auth().SetEmitter(newAuthFallback(t, backend))

	client, err := srv.NewClient(authtest.TestServerID(types.RoleNode, "test-node"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	require.Error(t, client.EmitAuditEvent(ctx, userLoginEvent()),
		"forwarded event delivery error must propagate to the client")

	require.NoError(t, srv.Auth().EmitAuditEvent(ctx, userLoginEvent()),
		"auth server must queue its own event on backend failure")

	backend.setFail(false)
	require.Eventually(t, func() bool {
		return backend.count() >= 1
	}, 5*time.Second, 50*time.Millisecond,
		"auth server should re-deliver its own queued event after recovery")
}

func TestEmitAuditEvent_AgentRetriesEndToEnd_GRPC(t *testing.T) {
	ctx := context.Background()
	backend := &toggleBackend{}
	backend.setFail(true)

	srv := newTestTLSServer(t)
	srv.Auth().SetEmitter(newAuthFallback(t, backend))

	client, err := srv.NewClient(authtest.TestServerID(types.RoleNode, "test-node"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	agent, err := events.NewCheckingAsyncEmitter(
		events.CheckingEmitterConfig{},
		events.AsyncEmitterConfig{
			Inner:            client,
			DataDir:          t.TempDir(),
			EnableAuditQueue: true,
			AuditQueueCfg: auditqueue.Config{
				MaxAttempts:             3,
				DeadLetterSweepInterval: 50 * time.Millisecond,
			},
			AuditQueueBackends: []auditqueue.Kind{auditqueue.KindSQLiteMemory},
		},
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, agent.Close()) })

	require.NoError(t, agent.EmitAuditEvent(ctx, userLoginEvent()))

	require.Never(t, func() bool {
		return backend.count() > 0
	}, 500*time.Millisecond, 50*time.Millisecond,
		"event must not be delivered while the backend is down")

	backend.setFail(false)
	require.Eventually(t, func() bool {
		return backend.count() >= 1
	}, 5*time.Second, 50*time.Millisecond,
		"agent should re-deliver the forwarded event through gRPC after recovery")
}

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

package postgres

import (
	"context"
	"io"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common"
)

const testWaitTimeout = 5 * time.Second

func TestHandleEstablishedConnectionClosesServerOnClientReadyError(t *testing.T) {
	clientConn, clientPeer := net.Pipe()
	require.NoError(t, clientPeer.Close())
	t.Cleanup(func() { clientConn.Close() })

	serverConn, serverPeer := net.Pipe()
	t.Cleanup(func() { serverConn.Close() })
	t.Cleanup(func() { serverPeer.Close() })

	engine := &Engine{
		EngineConfig: common.EngineConfig{
			Context: t.Context(),
			Log:     slog.New(slog.DiscardHandler),
		},
		client:        pgproto3.NewBackend(clientConn, clientConn),
		rawClientConn: clientConn,
		rawServerConn: serverConn,
	}
	hijackedConn := &pgconn.HijackedConn{
		Conn: serverConn,
		PID:  1,
	}

	err := engine.handleEstablishedConnection(
		t.Context(),
		&common.Session{},
		pgproto3.NewFrontend(serverConn, serverConn),
		hijackedConn,
		func() { t.Fatal("connection setup should not be observed") },
	)
	require.Error(t, err)

	requireConnectionClosed(t, serverPeer)
}

func TestHandleEstablishedConnectionClosesServerBeforeSessionEnd(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	database, err := types.NewDatabaseV3(types.Metadata{
		Name: "postgres",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
	})
	require.NoError(t, err)
	sessionCtx := &common.Session{Database: database}

	clientConn, clientPeer := net.Pipe()
	t.Cleanup(func() { clientConn.Close() })
	t.Cleanup(func() { clientPeer.Close() })
	go func() {
		_, _ = io.Copy(io.Discard, clientPeer)
	}()

	serverConn, serverPeer := net.Pipe()
	t.Cleanup(func() { serverConn.Close() })
	t.Cleanup(func() { serverPeer.Close() })

	unblockSessionEnd := make(chan struct{})
	var unblockOnce sync.Once
	unblockAudit := func() {
		unblockOnce.Do(func() { close(unblockSessionEnd) })
	}
	t.Cleanup(unblockAudit)
	audit := &blockingSessionEndAudit{
		sessionEndStarted: make(chan struct{}),
		unblockSessionEnd: unblockSessionEnd,
	}

	engine := &Engine{
		EngineConfig: common.EngineConfig{
			Audit:   audit,
			Context: t.Context(),
			Log:     slog.New(slog.DiscardHandler),
		},
		client:        pgproto3.NewBackend(clientConn, clientConn),
		rawClientConn: clientConn,
		rawServerConn: serverConn,
	}
	hijackedConn := &pgconn.HijackedConn{
		Conn: serverConn,
		PID:  1,
	}

	done := make(chan error, 1)
	go func() {
		done <- engine.handleEstablishedConnection(
			ctx,
			sessionCtx,
			pgproto3.NewFrontend(serverConn, serverConn),
			hijackedConn,
			cancel,
		)
	}()

	select {
	case <-audit.sessionEndStarted:
	case <-time.After(testWaitTimeout):
		t.Fatal("session end audit did not start")
	}
	requireConnectionClosed(t, serverPeer)

	unblockAudit()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(testWaitTimeout):
		t.Fatal("connection handler did not return")
	}
}

type blockingSessionEndAudit struct {
	common.Audit
	sessionEndStarted chan struct{}
	unblockSessionEnd <-chan struct{}
}

func (a *blockingSessionEndAudit) OnSessionStart(context.Context, *common.Session, error) {}

func (a *blockingSessionEndAudit) OnSessionEnd(context.Context, *common.Session) {
	close(a.sessionEndStarted)
	<-a.unblockSessionEnd
}

func requireConnectionClosed(t *testing.T, conn net.Conn) {
	t.Helper()

	readErrCh := make(chan error, 1)
	go func() {
		_, err := conn.Read(make([]byte, 1))
		readErrCh <- err
	}()
	select {
	case err := <-readErrCh:
		require.ErrorIs(t, err, io.EOF)
	case <-time.After(testWaitTimeout):
		t.Fatal("upstream database connection was not closed")
	}
}

/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package tbot

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/ssh/agent"

	proxyclient "github.com/gravitational/teleport/api/client/proxy"
)

type mockConn struct {
	mu     sync.Mutex
	closed bool
	net.Conn
}

func (mc *mockConn) Close() error {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.closed = true
	mc.Conn.Close()
	return nil
}

type mockHostDialer struct {
	t      *testing.T
	mu     sync.Mutex
	closed bool
	conns  []*mockConn
}

func (m *mockHostDialer) DialHost(
	_ context.Context, _ string, _ string, _ agent.ExtendedAgent,
) (net.Conn, proxyclient.ClusterDetails, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		m.t.Errorf("attempt to dial on closed host dialer")
		return nil, proxyclient.ClusterDetails{}, fmt.Errorf("closed")
	}
	conn := &mockConn{
		Conn: &net.TCPConn{},
	}
	m.conns = append(m.conns, conn)
	return conn, proxyclient.ClusterDetails{}, nil
}

func (m *mockHostDialer) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

type mockHostDialerTracker struct {
	t           *testing.T
	mu          sync.Mutex
	hostDialers []*mockHostDialer
}

func (m *mockHostDialerTracker) New(_ context.Context) (hostDialer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	hostDialer := &mockHostDialer{t: m.t}
	m.hostDialers = append(m.hostDialers, hostDialer)
	return hostDialer, nil
}

func (m *mockHostDialerTracker) count() (open int, closed int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, conn := range m.hostDialers {
		conn.mu.Lock()
		connClosed := conn.closed
		conn.mu.Unlock()

		if connClosed {
			closed++
		} else {
			open++
		}
	}
	return open, closed

}

func TestCyclingHostDialClient(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tracker := &mockHostDialerTracker{}
	cycler := &cyclingHostDialClient{
		max:          5,
		hostDialerFn: tracker.New,
	}

	var conns []net.Conn
	for range 10 {
		conn, _, err := cycler.DialHost(ctx, "", "", nil)
		assert.NoError(t, err)
		conns = append(conns, conn)
	}

	openDialers, closedDialers := tracker.count()
	assert.Equal(t, 2, openDialers)
	assert.Equal(t, 0, closedDialers)

	// Close the first connection, it should not close any dialer.
	_ = conns[0].Close()
	assert.EventuallyWithT(t, func(t *assert.CollectT) {
		openDialers, closedDialers = tracker.count()
		assert.Equal(t, 2, openDialers)
		assert.Equal(t, 0, closedDialers)
	}, time.Second, 100*time.Millisecond)

	// Close the next 4 connections, it should close the first dialer.
	for i := 1; i < 5; i++ {
		_ = conns[i].Close()
	}
	assert.EventuallyWithT(t, func(t *assert.CollectT) {
		openDialers, closedDialers = tracker.count()
		assert.Equal(t, 1, openDialers)
		assert.Equal(t, 1, closedDialers)
	}, time.Second, 100*time.Millisecond)

	// Close the next 5 connections, it should close the second dialer.
	for i := 5; i < 10; i++ {
		_ = conns[i].Close()
	}
	assert.EventuallyWithT(t, func(t *assert.CollectT) {
		openDialers, closedDialers = tracker.count()
		assert.Equal(t, 0, openDialers)
		assert.Equal(t, 2, closedDialers)
	}, time.Second, 100*time.Millisecond)

	// Now we want to validate a weirder case, let's create 4 connections,
	// close them and then create a fifth.
	for range 4 {
		conn, _, err := cycler.DialHost(ctx, "", "", nil)
		assert.NoError(t, err)
		_ = conn.Close()
	}
	conn, _, err := cycler.DialHost(ctx, "", "", nil)
	assert.NoError(t, err)
	assert.EventuallyWithT(t, func(t *assert.CollectT) {
		openDialers, closedDialers = tracker.count()
		assert.Equal(t, 1, openDialers)
		assert.Equal(t, 2, closedDialers)
	}, time.Second, 100*time.Millisecond)
	_ = conn.Close()
	assert.EventuallyWithT(t, func(t *assert.CollectT) {
		openDialers, closedDialers = tracker.count()
		assert.Equal(t, 0, openDialers)
		assert.Equal(t, 3, closedDialers)
	}, time.Second, 100*time.Millisecond)
}

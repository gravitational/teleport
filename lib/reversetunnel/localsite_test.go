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

package reversetunnel

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()

	os.Exit(m.Run())
}

func TestRemoteConnCleanup(t *testing.T) {
	t.Parallel()

	const clockBlockers = 3 //periodic ticker + heart beat timer + resync ticker

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clock := clockwork.NewFakeClock()

	clt := &mockLocalSiteClient{}
	watcher, err := services.NewProxyWatcher(ctx, services.ProxyWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: "test",
			Logger:    utils.NewSlogLoggerForTests(),
			Clock:     clock,
			Client:    clt,
		},
		ProxyGetter: clt,
		ProxiesC:    make(chan []types.Server, 2),
	})
	require.NoError(t, err)
	require.NoError(t, watcher.WaitInitialization())

	// set up the site
	srv := &server{
		ctx:              ctx,
		Config:           Config{Clock: clock},
		localAuthClient:  &mockLocalSiteClient{},
		logger:           utils.NewSlogLoggerForTests(),
		offlineThreshold: time.Second,
		proxyWatcher:     watcher,
	}

	site, err := newLocalSite(srv, "clustername", nil,
		withPeriodicFunctionInterval(time.Hour),
		withProxySyncInterval(time.Hour),
	)
	require.NoError(t, err)

	// add a connection
	rconn := &mockRemoteConnConn{}
	sconn := &mockedSSHConn{}
	conn1, err := site.addConn(uuid.NewString(), types.NodeTunnel, rconn, sconn)
	require.NoError(t, err)

	// create a fake session
	fakeSession := newSessionTrackingConn(conn1, &mockRemoteConnConn{})

	reqs := make(chan *ssh.Request)
	defer close(reqs)

	// terminated by too many missed heartbeats
	go func() {
		site.handleHeartbeat(ctx, conn1, nil, reqs)
		cancel()
	}()

	// set the heartbeat to a time in the past that is long enough
	// to consider the connection offline
	conn1.markValid()
	conn1.setLastHeartbeat(clock.Now().UTC().Add(site.offlineThreshold * missedHeartBeatThreshold * -2))

	// advance the clock to trigger a missed heartbeat
	clock.BlockUntil(clockBlockers)
	clock.Advance(srv.offlineThreshold)
	// wait until the missed heartbeat was processed to continue
	clock.BlockUntil(clockBlockers)

	// validate that the fake session prevented anything from closing
	// but that the connection was marked invalid
	require.False(t, conn1.closed.Load())
	require.False(t, sconn.closed.Load())
	require.True(t, conn1.isInvalid())

	// set the heartbeat to a time in the past that is long enough
	// to consider the connection offline
	conn1.markValid()
	conn1.setLastHeartbeat(clock.Now().UTC().Add(site.offlineThreshold * missedHeartBeatThreshold * -2))

	// close the fake session
	require.NoError(t, fakeSession.Close())

	// advance the clock to trigger a missed heartbeat
	clock.Advance(srv.offlineThreshold)

	// validate the missed heartbeat terminated the loop and closes the connection
	select {
	case <-ctx.Done():
		require.True(t, conn1.closed.Load())
		require.True(t, sconn.closed.Load())
	case <-time.After(15 * time.Second):
		t.Fatal("localSite heartbeat handler never terminated")
	}
}

func TestLocalSiteOverlap(t *testing.T) {
	t.Parallel()

	srv := &server{
		Config:          Config{Clock: clockwork.NewFakeClock()},
		ctx:             context.Background(),
		localAuthClient: &mockLocalSiteClient{},
	}

	site, err := newLocalSite(srv, "clustername", nil,
		withPeriodicFunctionInterval(time.Hour),
	)
	require.NoError(t, err)

	nodeID := uuid.NewString()
	connType := types.NodeTunnel
	dreq := &sshutils.DialReq{
		ServerID: nodeID,
		ConnType: connType,
	}

	// add a few connections for the same node id
	conn1, err := site.addConn(nodeID, connType, &mockRemoteConnConn{}, nil)
	require.NoError(t, err)

	conn2, err := site.addConn(nodeID, connType, &mockRemoteConnConn{}, nil)
	require.NoError(t, err)

	conn3, err := site.addConn(nodeID, connType, &mockRemoteConnConn{}, nil)
	require.NoError(t, err)

	// no heartbeats from any of them shouldn't return a connection
	c, err := site.getRemoteConn(dreq)
	require.True(t, trace.IsNotFound(err))
	require.Nil(t, c)

	// ensure conn1 is ready
	conn1.setLastHeartbeat(time.Now())

	// getRemoteConn returns the only healthy connection
	c, err = site.getRemoteConn(dreq)
	require.NoError(t, err)
	require.Equal(t, conn1, c)

	// ensure conn2 is ready
	conn2.setLastHeartbeat(time.Now())

	// getRemoteConn returns the newest healthy connection
	c, err = site.getRemoteConn(dreq)
	require.NoError(t, err)
	require.Equal(t, conn2, c)

	// mark conn2 invalid
	conn2.markInvalid(nil)

	// getRemoteConn returns the only healthy connection
	c, err = site.getRemoteConn(dreq)
	require.NoError(t, err)
	require.Equal(t, conn1, c)

	// mark conn1 invalid
	conn1.markInvalid(nil)

	// getRemoteConn returns the only healthy connection
	c, err = site.getRemoteConn(dreq)
	require.NoError(t, err)
	require.Equal(t, conn2, c)

	// remove conn2
	site.removeRemoteConn(conn2)

	// getRemoteConn returns the only invalid connection
	c, err = site.getRemoteConn(dreq)
	require.NoError(t, err)
	require.Equal(t, conn1, c)

	// remove conn1
	site.removeRemoteConn(conn1)

	// no ready connections exist
	c, err = site.getRemoteConn(dreq)
	require.True(t, trace.IsNotFound(err))
	require.Nil(t, c)

	// mark conn3 as ready
	conn3.setLastHeartbeat(time.Now())

	// getRemoteConn returns the only healthy connection
	c, err = site.getRemoteConn(dreq)
	require.NoError(t, err)
	require.Equal(t, conn3, c)
}

func TestProxyResync(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clock := clockwork.NewFakeClock()

	proxy1, err := types.NewServer(uuid.NewString(), types.KindProxy, types.ServerSpecV2{})
	require.NoError(t, err)

	proxy2, err := types.NewServer(uuid.NewString(), types.KindProxy, types.ServerSpecV2{})
	require.NoError(t, err)

	clt := &mockLocalSiteClient{
		proxies: []types.Server{proxy1, proxy2},
	}
	// set up the watcher and wait for it to be initialized
	watcher, err := services.NewProxyWatcher(ctx, services.ProxyWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: "test",
			Logger:    utils.NewSlogLoggerForTests(),
			Clock:     clock,
			Client:    clt,
		},
		ProxyGetter: clt,
		ProxiesC:    make(chan []types.Server, 2),
	})
	require.NoError(t, err)
	require.NoError(t, watcher.WaitInitialization())

	// set up the site
	srv := &server{
		ctx:              ctx,
		Config:           Config{Clock: clock},
		localAuthClient:  &mockLocalSiteClient{},
		logger:           utils.NewSlogLoggerForTests(),
		offlineThreshold: 24 * time.Hour,
		proxyWatcher:     watcher,
	}
	site, err := newLocalSite(srv, "clustername", nil,
		withProxySyncInterval(time.Second),
		withPeriodicFunctionInterval(24*time.Hour),
	)
	require.NoError(t, err)

	// create the ssh machinery to mock an agent
	discoveryCh := make(chan *discoveryRequest)

	reqHandler := func(name string, wantReply bool, payload []byte) (bool, error) {
		assert.Equal(t, chanDiscoveryReq, name)

		var req discoveryRequest
		assert.NoError(t, json.Unmarshal(payload, &req))
		discoveryCh <- &req
		return true, nil
	}
	channelCreator := func(name string) ssh.Channel {
		assert.Equal(t, chanDiscovery, name)
		return &mockedSSHChannel{reqHandler: reqHandler}
	}

	rconn := &mockRemoteConnConn{}
	sconn := &mockedSSHConn{
		channelFn: channelCreator,
	}

	// add a connection
	conn1, err := site.addConn(uuid.NewString(), types.NodeTunnel, rconn, sconn)
	require.NoError(t, err)

	reqs := make(chan *ssh.Request)

	// terminated by canceled context
	go func() {
		site.handleHeartbeat(ctx, conn1, nil, reqs)
	}()

	expected := []types.Server{proxy1, proxy2}
	sort.Slice(expected, func(i, j int) bool { return expected[i].GetName() < expected[j].GetName() })
	for i := 0; i < 5; i++ {
		// wait for the heartbeat loop to select
		clock.BlockUntil(3) // periodic ticker + heart beat timer + resync ticker = 3

		// advance the sync interval to force a discovery request to be sent
		clock.Advance(time.Second)

		// wait for the discovery request to be received
		select {
		case req := <-discoveryCh:
			require.NotNil(t, req)
			require.Len(t, req.Proxies, 2)

			sort.Slice(req.Proxies, func(i, j int) bool { return req.Proxies[i].Metadata.Name < req.Proxies[j].Metadata.Name })

			require.Equal(t, req.Proxies[0].Metadata.Name, expected[0].GetName())
			require.Equal(t, req.Proxies[1].Metadata.Name, expected[1].GetName())
		case <-time.After(10 * time.Second):
			t.Fatal("timed out waiting for discovery request")
		}
	}
}

type mockLocalSiteClient struct {
	authclient.Client

	proxies []types.Server
}

// called by (*localSite).sshTunnelStats() as part of (*localSite).periodicFunctions()
func (*mockLocalSiteClient) GetNodes(_ context.Context, _ string) ([]types.Server, error) {
	return nil, nil
}

func (m *mockLocalSiteClient) GetProxies() ([]types.Server, error) {
	return m.proxies, nil
}

type mockWatcher struct {
	events chan types.Event
}

func newMockWatcher() mockWatcher {
	ch := make(chan types.Event, 1)

	ch <- types.Event{
		Type: types.OpInit,
	}

	return mockWatcher{events: ch}
}

func (m mockWatcher) Events() <-chan types.Event {
	return m.events
}

func (m mockWatcher) Done() <-chan struct{} {
	return make(chan struct{})
}

func (m mockWatcher) Close() error {
	return nil
}

func (m mockWatcher) Error() error {
	return nil
}

// called by proxyWatcher
func (mockLocalSiteClient) NewWatcher(context.Context, types.Watch) (types.Watcher, error) {
	return newMockWatcher(), nil
}

type mockRemoteConnConn struct {
	net.Conn
	closed atomic.Bool
}

func (c *mockRemoteConnConn) Close() error {
	c.closed.Store(true)
	return nil
}

// called for logging by (*remoteConn).markInvalid()
func (*mockRemoteConnConn) RemoteAddr() net.Addr {
	return &utils.NetAddr{
		Addr:        "localhost",
		AddrNetwork: "tcp",
	}
}

type mockedSSHConn struct {
	ssh.Conn
	closed atomic.Bool

	channelFn func(string) ssh.Channel
}

func (c *mockedSSHConn) Close() error {
	c.closed.Store(true)
	return nil
}

func (c *mockedSSHConn) OpenChannel(name string, data []byte) (ssh.Channel, <-chan *ssh.Request, error) {
	ch := make(chan *ssh.Request)
	close(ch)

	if c.channelFn != nil {
		return c.channelFn(name), ch, nil
	}

	return &mockedSSHChannel{}, ch, nil
}

func (*mockedSSHConn) RemoteAddr() net.Addr {
	return &utils.NetAddr{
		Addr:        "localhost",
		AddrNetwork: "tcp",
	}
}

type mockedSSHChannel struct {
	ssh.Channel

	reqHandler func(name string, wantReply bool, payload []byte) (bool, error)
}

func (c *mockedSSHChannel) SendRequest(name string, wantReply bool, payload []byte) (bool, error) {
	if c.reqHandler == nil {
		return true, nil
	}

	return c.reqHandler(name, wantReply, payload)
}

func (*mockedSSHChannel) Close() error {
	return nil
}

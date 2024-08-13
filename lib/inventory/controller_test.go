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

package inventory

import (
	"context"
	"fmt"
	"os"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/inventory/metadata"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

type fakeAuth struct {
	mu             sync.Mutex
	failUpserts    int
	failKeepAlives int

	upserts    int
	keepalives int
	err        error

	expectAddr      string
	unexpectedAddrs int

	failUpsertInstance int

	lastInstance    types.Instance
	lastRawInstance []byte
}

func (a *fakeAuth) UpsertNode(_ context.Context, server types.Server) (*types.KeepAlive, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.upserts++
	if a.expectAddr != "" {
		if server.GetAddr() != a.expectAddr {
			a.unexpectedAddrs++
		}
	}
	if a.failUpserts > 0 {
		a.failUpserts--
		return nil, trace.Errorf("upsert failed as test condition")
	}
	return &types.KeepAlive{}, a.err
}

func (a *fakeAuth) UpsertApplicationServer(_ context.Context, server types.AppServer) (*types.KeepAlive, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.upserts++

	if a.failUpserts > 0 {
		a.failUpserts--
		return nil, trace.Errorf("upsert failed as test condition")
	}
	return &types.KeepAlive{}, a.err
}

func (a *fakeAuth) DeleteApplicationServer(ctx context.Context, namespace, hostID, name string) error {
	return nil
}

func (a *fakeAuth) KeepAliveServer(_ context.Context, _ types.KeepAlive) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.keepalives++
	if a.failKeepAlives > 0 {
		a.failKeepAlives--
		return trace.Errorf("keepalive failed as test condition")
	}
	return a.err
}

func (a *fakeAuth) UpsertInstance(ctx context.Context, instance types.Instance) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.failUpsertInstance > 0 {
		a.failUpsertInstance--
		return trace.Errorf("upsert instance failed as test condition")
	}

	a.lastInstance = instance.Clone()
	var err error
	a.lastRawInstance, err = utils.FastMarshal(instance)
	if err != nil {
		panic("fastmarshal of instance should be infallible")
	}

	return nil
}

// TestSSHServerBasics verifies basic expected behaviors for a single control stream heartbeating
// an ssh service.
func TestSSHServerBasics(t *testing.T) {
	const serverID = "test-server"
	const zeroAddr = "0.0.0.0:123"
	const peerAddr = "1.2.3.4:456"
	const wantAddr = "1.2.3.4:123"

	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := make(chan testEvent, 1024)

	auth := &fakeAuth{
		expectAddr: wantAddr,
	}

	controller := NewController(
		auth,
		usagereporter.DiscardUsageReporter{},
		withServerKeepAlive(time.Millisecond*200),
		withTestEventsChannel(events),
	)
	defer controller.Close()

	// set up fake in-memory control stream
	upstream, downstream := client.InventoryControlStreamPipe(client.ICSPipePeerAddr(peerAddr))

	controller.RegisterControlStream(upstream, proto.UpstreamInventoryHello{
		ServerID: serverID,
		Version:  teleport.Version,
		Services: []types.SystemRole{types.RoleNode},
	})

	// verify that control stream handle is now accessible
	handle, ok := controller.GetControlStream(serverID)
	require.True(t, ok)

	// verify that hb counter has been incremented
	require.Equal(t, int64(1), controller.instanceHBVariableDuration.Count())

	// send a fake ssh server heartbeat
	err := downstream.Send(ctx, proto.InventoryHeartbeat{
		SSHServer: &types.ServerV2{
			Metadata: types.Metadata{
				Name: serverID,
			},
			Spec: types.ServerSpecV2{
				Addr: zeroAddr,
			},
		},
	})
	require.NoError(t, err)

	// verify that heartbeat creates both an upsert and a keepalive
	awaitEvents(t, events,
		expect(sshUpsertOk, sshKeepAliveOk),
		deny(sshUpsertErr, sshKeepAliveErr, handlerClose),
	)

	// set up to induce some failures, but not enough to cause the control
	// stream to be closed.
	auth.mu.Lock()
	auth.failUpserts = 1
	auth.failKeepAlives = 2
	auth.mu.Unlock()

	// keepalive should fail twice, but since the upsert is already known
	// to have succeeded, we should not see an upsert failure yet.
	awaitEvents(t, events,
		expect(sshKeepAliveErr, sshKeepAliveErr),
		deny(sshUpsertErr, handlerClose),
	)

	err = downstream.Send(ctx, proto.InventoryHeartbeat{
		SSHServer: &types.ServerV2{
			Metadata: types.Metadata{
				Name: serverID,
			},
			Spec: types.ServerSpecV2{
				Addr: zeroAddr,
			},
		},
	})
	require.NoError(t, err)

	// we should now see an upsert failure, but no additional
	// keepalive failures, and the upsert should succeed on retry.
	awaitEvents(t, events,
		expect(sshKeepAliveOk, sshUpsertErr, sshUpsertRetryOk),
		deny(sshKeepAliveErr, handlerClose),
	)

	// launch goroutine to respond to a single ping
	go func() {
		select {
		case msg := <-downstream.Recv():
			downstream.Send(ctx, proto.UpstreamInventoryPong{
				ID: msg.(proto.DownstreamInventoryPing).ID,
			})
		case <-downstream.Done():
		case <-ctx.Done():
		}
	}()

	// limit time of ping call
	pingCtx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	// execute ping
	_, err = handle.Ping(pingCtx, 1)
	require.NoError(t, err)

	// set up to induce enough consecutive errors to cause stream closure
	auth.mu.Lock()
	auth.failUpserts = 5
	auth.mu.Unlock()

	err = downstream.Send(ctx, proto.InventoryHeartbeat{
		SSHServer: &types.ServerV2{
			Metadata: types.Metadata{
				Name: serverID,
			},
			Spec: types.ServerSpecV2{
				Addr: zeroAddr,
			},
		},
	})
	require.NoError(t, err)

	// both the initial upsert and the retry should fail, then the handle should
	// close.
	awaitEvents(t, events,
		expect(sshUpsertErr, sshUpsertRetryErr, handlerClose),
		deny(sshUpsertOk),
	)

	// verify that closure propagates to server and client side interfaces
	closeTimeout := time.After(time.Second * 10)
	select {
	case <-handle.Done():
	case <-closeTimeout:
		t.Fatal("timeout waiting for handle closure")
	}
	select {
	case <-downstream.Done():
	case <-closeTimeout:
		t.Fatal("timeout waiting for handle closure")
	}

	// verify that hb counter has been decremented (counter is decremented concurrently, but
	// always *before* closure is propagated to downstream handle, hence being safe to load
	// here).
	require.Equal(t, int64(0), controller.instanceHBVariableDuration.Count())

	// verify that the peer address of the control stream was used to override
	// zero-value IPs for heartbeats.
	auth.mu.Lock()
	unexpectedAddrs := auth.unexpectedAddrs
	auth.mu.Unlock()
	require.Zero(t, unexpectedAddrs)
}

// TestSSHServerBasics verifies basic expected behaviors for a single control stream heartbeating
// an app service.
func TestAppServerBasics(t *testing.T) {
	const serverID = "test-server"
	const appCount = 3

	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := make(chan testEvent, 1024)

	auth := &fakeAuth{}

	controller := NewController(
		auth,
		usagereporter.DiscardUsageReporter{},
		withServerKeepAlive(time.Millisecond*200),
		withTestEventsChannel(events),
	)
	defer controller.Close()

	// set up fake in-memory control stream
	upstream, downstream := client.InventoryControlStreamPipe()

	controller.RegisterControlStream(upstream, proto.UpstreamInventoryHello{
		ServerID: serverID,
		Version:  teleport.Version,
		Services: []types.SystemRole{types.RoleApp},
	})

	// verify that control stream handle is now accessible
	handle, ok := controller.GetControlStream(serverID)
	require.True(t, ok)

	// verify that hb counter has been incremented
	require.Equal(t, int64(1), controller.instanceHBVariableDuration.Count())

	// send a fake app server heartbeat
	for i := 0; i < appCount; i++ {
		err := downstream.Send(ctx, proto.InventoryHeartbeat{
			AppServer: &types.AppServerV3{
				Metadata: types.Metadata{
					Name: serverID,
				},
				Spec: types.AppServerSpecV3{
					HostID: serverID,
					App: &types.AppV3{
						Kind:    types.KindApp,
						Version: types.V3,
						Metadata: types.Metadata{
							Name: fmt.Sprintf("app-%d", i),
						},
						Spec: types.AppSpecV3{},
					},
				},
			},
		})
		require.NoError(t, err)
	}

	// verify that heartbeat creates both an upsert and a keepalive
	awaitEvents(t, events,
		expect(appUpsertOk, appKeepAliveOk, appUpsertOk, appKeepAliveOk, appUpsertOk, appKeepAliveOk),
		deny(appUpsertErr, appKeepAliveErr, handlerClose),
	)

	// set up to induce some failures, but not enough to cause the control
	// stream to be closed.
	auth.mu.Lock()
	auth.failUpserts = 1
	auth.failKeepAlives = 2
	auth.mu.Unlock()

	// keepalive should fail twice, but since the upsert is already known
	// to have succeeded, we should not see an upsert failure yet.
	awaitEvents(t, events,
		expect(appKeepAliveErr, appKeepAliveErr),
		deny(appUpsertErr, handlerClose),
	)

	for i := 0; i < appCount; i++ {
		err := downstream.Send(ctx, proto.InventoryHeartbeat{
			AppServer: &types.AppServerV3{
				Metadata: types.Metadata{
					Name: serverID,
				},
				Spec: types.AppServerSpecV3{
					HostID: serverID,
					App: &types.AppV3{
						Kind:    types.KindApp,
						Version: types.V3,
						Metadata: types.Metadata{
							Name: fmt.Sprintf("app-%d", i),
						},
						Spec: types.AppSpecV3{},
					},
				},
			},
		})
		require.NoError(t, err)
	}

	// we should now see an upsert failure, but no additional
	// keepalive failures, and the upsert should succeed on retry.
	awaitEvents(t, events,
		expect(appKeepAliveOk, appKeepAliveOk, appKeepAliveOk, appUpsertErr, appUpsertRetryOk),
		deny(appKeepAliveErr, handlerClose),
	)

	// launch goroutine to respond to a single ping
	go func() {
		select {
		case msg := <-downstream.Recv():
			downstream.Send(ctx, proto.UpstreamInventoryPong{
				ID: msg.(proto.DownstreamInventoryPing).ID,
			})
		case <-downstream.Done():
		case <-ctx.Done():
		}
	}()

	// limit time of ping call
	pingCtx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	// execute ping
	_, err := handle.Ping(pingCtx, 1)
	require.NoError(t, err)

	// ensure that local app keepalive states have reset to healthy by waiting
	// on a full cycle+ worth of keepalives without errors.
	awaitEvents(t, events,
		expect(keepAliveTick, keepAliveTick),
		deny(appKeepAliveErr, handlerClose),
	)

	// set up to induce enough consecutive keepalive errors to cause removal
	// of server-side keepalive state.
	auth.mu.Lock()
	auth.failKeepAlives = 3 * appCount
	auth.mu.Unlock()

	// expect that all app keepalives fail, then the app is removed.
	var expectedEvents []testEvent
	for i := 0; i < appCount; i++ {
		expectedEvents = append(expectedEvents, []testEvent{appKeepAliveErr, appKeepAliveErr, appKeepAliveErr, appKeepAliveDel}...)
	}

	// wait for failed keepalives to trigger removal
	awaitEvents(t, events,
		expect(expectedEvents...),
		deny(handlerClose),
	)

	// verify that further keepalive ticks to not result in attempts to keepalive
	// apps (successful or not).
	awaitEvents(t, events,
		expect(keepAliveTick, keepAliveTick, keepAliveTick),
		deny(appKeepAliveOk, appKeepAliveErr, handlerClose),
	)

	// set up to induce enough consecutive errors to cause stream closure
	auth.mu.Lock()
	auth.failUpserts = 5
	auth.mu.Unlock()

	err = downstream.Send(ctx, proto.InventoryHeartbeat{
		AppServer: &types.AppServerV3{
			Metadata: types.Metadata{
				Name: serverID,
			},
			Spec: types.AppServerSpecV3{
				HostID: serverID,
				App: &types.AppV3{
					Kind:     types.KindApp,
					Version:  types.V3,
					Metadata: types.Metadata{},
					Spec:     types.AppSpecV3{},
				},
			},
		},
	})
	require.NoError(t, err)

	// both the initial upsert and the retry should fail, then the handle should
	// close.
	awaitEvents(t, events,
		expect(appUpsertErr, appUpsertRetryErr, handlerClose),
		deny(appUpsertOk),
	)

	// verify that closure propagates to server and client side interfaces
	closeTimeout := time.After(time.Second * 10)
	select {
	case <-handle.Done():
	case <-closeTimeout:
		t.Fatal("timeout waiting for handle closure")
	}
	select {
	case <-downstream.Done():
	case <-closeTimeout:
		t.Fatal("timeout waiting for handle closure")
	}

	// verify that hb counter has been decremented (counter is decremented concurrently, but
	// always *before* closure is propagated to downstream handle, hence being safe to load
	// here).
	require.Equal(t, int64(0), controller.instanceHBVariableDuration.Count())
}

// TestInstanceHeartbeat verifies basic expected behaviors for instance heartbeat.
func TestInstanceHeartbeat_Disabled(t *testing.T) {
	const serverID = "test-instance"
	const peerAddr = "1.2.3.4:456"

	events := make(chan testEvent, 1024)

	auth := &fakeAuth{}

	controller := NewController(
		auth,
		usagereporter.DiscardUsageReporter{},
		withInstanceHBInterval(time.Millisecond*200),
		withTestEventsChannel(events),
	)
	defer controller.Close()

	// set up fake in-memory control stream
	upstream, _ := client.InventoryControlStreamPipe(client.ICSPipePeerAddr(peerAddr))

	controller.RegisterControlStream(upstream, proto.UpstreamInventoryHello{
		ServerID: serverID,
		Version:  teleport.Version,
		Services: []types.SystemRole{types.RoleNode},
	})

	// verify that control stream handle is now accessible
	_, ok := controller.GetControlStream(serverID)
	require.True(t, ok)

	// verify that no instance heartbeats are emitted
	awaitEvents(t, events,
		deny(instanceHeartbeatOk, instanceHeartbeatErr, instanceCompareFailed, handlerClose),
	)
}

func TestInstanceHeartbeatDisabledEnv(t *testing.T) {
	t.Setenv("TELEPORT_UNSTABLE_DISABLE_INSTANCE_HB", "yes")

	controller := NewController(
		&fakeAuth{},
		usagereporter.DiscardUsageReporter{},
	)
	defer controller.Close()

	require.False(t, controller.instanceHBEnabled)
}

func TestServerKeepaliveDisabledEnv(t *testing.T) {
	t.Setenv("TELEPORT_UNSTABLE_DISABLE_SERVER_KEEPALIVE", "yes")

	controller := NewController(
		&fakeAuth{},
		usagereporter.DiscardUsageReporter{},
	)
	defer controller.Close()

	require.False(t, controller.serverKeepAliveEnabled)
}

// TestInstanceHeartbeat verifies basic expected behaviors for instance heartbeat.
func TestInstanceHeartbeat(t *testing.T) {
	const serverID = "test-instance"
	const peerAddr = "1.2.3.4:456"

	events := make(chan testEvent, 1024)

	auth := &fakeAuth{}

	controller := NewController(
		auth,
		usagereporter.DiscardUsageReporter{},
		withInstanceHBInterval(time.Millisecond*200),
		withTestEventsChannel(events),
	)
	defer controller.Close()

	// set up fake in-memory control stream
	upstream, downstream := client.InventoryControlStreamPipe(client.ICSPipePeerAddr(peerAddr))

	controller.RegisterControlStream(upstream, proto.UpstreamInventoryHello{
		ServerID: serverID,
		Version:  teleport.Version,
		Services: []types.SystemRole{types.RoleNode, types.RoleApp},
	})

	// verify that control stream handle is now accessible
	handle, ok := controller.GetControlStream(serverID)
	require.True(t, ok)

	// verify that instance heartbeat succeeds
	awaitEvents(t, events,
		expect(instanceHeartbeatOk),
		deny(instanceHeartbeatErr, instanceCompareFailed, handlerClose),
	)

	// verify the service counter shows the correct number for the given services.
	require.Equal(t, uint64(1), controller.serviceCounter.get(types.RoleNode))
	require.Equal(t, uint64(1), controller.serviceCounter.get(types.RoleApp))

	// this service was not seen, so it should be 0.
	require.Equal(t, uint64(0), controller.serviceCounter.get(types.RoleOkta))

	// set up single failure of upsert. stream should recover.
	auth.mu.Lock()
	auth.failUpsertInstance = 1
	auth.mu.Unlock()

	// verify that heartbeat error occurs
	awaitEvents(t, events,
		expect(instanceHeartbeatErr),
		deny(instanceCompareFailed, handlerClose),
	)

	// verify that recovery happens
	awaitEvents(t, events,
		expect(instanceHeartbeatOk),
		deny(instanceHeartbeatErr, instanceCompareFailed, handlerClose),
	)

	// verify the service counter shows the correct number for the given services.
	require.Equal(t, map[types.SystemRole]uint64{
		types.RoleApp:  1,
		types.RoleNode: 1,
	}, controller.ConnectedServiceCounts())
	require.Equal(t, uint64(1), controller.ConnectedServiceCount(types.RoleNode))
	require.Equal(t, uint64(1), controller.ConnectedServiceCount(types.RoleApp))

	// set up double failure of CAS. stream should not recover.
	auth.mu.Lock()
	auth.failUpsertInstance = 2
	auth.mu.Unlock()

	// expect failure and handle closure
	awaitEvents(t, events,
		expect(instanceHeartbeatErr, handlerClose),
	)

	// verify that closure propagates to server and client side interfaces
	closeTimeout := time.After(time.Second * 10)
	select {
	case <-handle.Done():
	case <-closeTimeout:
		t.Fatal("timeout waiting for handle closure")
	}
	select {
	case <-downstream.Done():
	case <-closeTimeout:
		t.Fatal("timeout waiting for handle closure")
	}

	// verify the service counter now shows no connected services.
	require.Equal(t, map[types.SystemRole]uint64{
		types.RoleApp:  0,
		types.RoleNode: 0,
	}, controller.ConnectedServiceCounts())
	require.Equal(t, uint64(0), controller.ConnectedServiceCount(types.RoleNode))
	require.Equal(t, uint64(0), controller.ConnectedServiceCount(types.RoleApp))
}

// TestUpdateLabels verifies that an instance's labels can be updated over an
// inventory control stream.
func TestUpdateLabels(t *testing.T) {
	t.Parallel()
	const serverID = "test-instance"
	const peerAddr = "1.2.3.4:456"

	events := make(chan testEvent, 1024)

	auth := &fakeAuth{}

	controller := NewController(
		auth,
		usagereporter.DiscardUsageReporter{},
		withInstanceHBInterval(time.Millisecond*200),
		withTestEventsChannel(events),
	)
	defer controller.Close()

	// Set up fake in-memory control stream.
	upstream, downstream := client.InventoryControlStreamPipe(client.ICSPipePeerAddr(peerAddr))
	upstreamHello := proto.UpstreamInventoryHello{
		ServerID: serverID,
		Version:  teleport.Version,
		Services: []types.SystemRole{types.RoleNode},
	}
	downstreamHello := proto.DownstreamInventoryHello{
		Version:  teleport.Version,
		ServerID: "auth",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	downstreamHandle := NewDownstreamHandle(func(ctx context.Context) (client.DownstreamInventoryControlStream, error) {
		return downstream, nil
	}, upstreamHello)

	// Wait for upstream hello.
	select {
	case msg := <-upstream.Recv():
		require.Equal(t, upstreamHello, msg)
	case <-ctx.Done():
		require.Fail(t, "never got upstream hello")
	}
	require.NoError(t, upstream.Send(ctx, downstreamHello))
	controller.RegisterControlStream(upstream, upstreamHello)

	// Verify that control stream upstreamHandle is now accessible.
	upstreamHandle, ok := controller.GetControlStream(serverID)
	require.True(t, ok)

	// Update labels.
	labels := map[string]string{"a": "1", "b": "2"}
	require.NoError(t, upstreamHandle.UpdateLabels(ctx, proto.LabelUpdateKind_SSHServerCloudLabels, labels))

	require.Eventually(t, func() bool {
		require.Equal(t, labels, downstreamHandle.GetUpstreamLabels(proto.LabelUpdateKind_SSHServerCloudLabels))
		return true
	}, time.Second, 100*time.Millisecond)
}

// TestAgentMetadata verifies that an instance's agent metadata is received in
// inventory control stream.
func TestAgentMetadata(t *testing.T) {
	// set the install method to validate it was returned as agent metadata
	t.Setenv("TELEPORT_INSTALL_METHOD_AWSOIDC_DEPLOYSERVICE", "true")
	metadataGetter = func(ctx context.Context) (*metadata.Metadata, error) {
		return &metadata.Metadata{
			OS:                    "llamaOS",
			OSVersion:             "1.2.3",
			HostArchitecture:      "llama",
			GlibcVersion:          "llama.5.6.7",
			InstallMethods:        []string{"llama", "alpaca"},
			ContainerRuntime:      "test",
			ContainerOrchestrator: "test",
			CloudEnvironment:      "llama-cloud",
		}, nil
	}

	const serverID = "test-instance"
	const peerAddr = "1.2.3.4:456"

	events := make(chan testEvent, 1024)

	auth := &fakeAuth{}

	controller := NewController(
		auth,
		usagereporter.DiscardUsageReporter{},
		withInstanceHBInterval(time.Millisecond*200),
		withTestEventsChannel(events),
	)
	defer controller.Close()

	// Set up fake in-memory control stream.
	upstream, downstream := client.InventoryControlStreamPipe(client.ICSPipePeerAddr(peerAddr))
	upstreamHello := proto.UpstreamInventoryHello{
		ServerID: serverID,
		Version:  teleport.Version,
		Services: []types.SystemRole{types.RoleNode},
	}
	downstreamHello := proto.DownstreamInventoryHello{
		Version:  teleport.Version,
		ServerID: "auth",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	NewDownstreamHandle(func(ctx context.Context) (client.DownstreamInventoryControlStream, error) {
		return downstream, nil
	}, upstreamHello)

	// Wait for upstream hello.
	select {
	case msg := <-upstream.Recv():
		require.Equal(t, upstreamHello, msg)
	case <-ctx.Done():
		require.Fail(t, "never got upstream hello")
	}
	require.NoError(t, upstream.Send(ctx, downstreamHello))
	controller.RegisterControlStream(upstream, upstreamHello)

	// Verify that control stream upstreamHandle is now accessible.
	upstreamHandle, ok := controller.GetControlStream(serverID)
	require.True(t, ok)

	// Validate that the agent's metadata ends up in the auth server.
	require.Eventually(t, func() bool {
		return slices.Equal([]string{"llama", "alpaca"}, upstreamHandle.AgentMetadata().InstallMethods) &&
			upstreamHandle.AgentMetadata().OS == "llamaOS"
	}, 10*time.Second, 200*time.Millisecond)
}

func TestGoodbye(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		supportsGoodbye bool
	}{
		{
			name: "no goodbye",
		},
		{
			name:            "goodbye",
			supportsGoodbye: true,
		},
	}

	upstreamHello := proto.UpstreamInventoryHello{
		ServerID: "llama",
		Version:  teleport.Version,
		Services: []types.SystemRole{types.RoleNode, types.RoleApp},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			controller := NewController(
				&fakeAuth{},
				usagereporter.DiscardUsageReporter{},
				withInstanceHBInterval(time.Millisecond*200),
			)
			defer controller.Close()

			// Set up fake in-memory control stream.
			upstream, downstream := client.InventoryControlStreamPipe(client.ICSPipePeerAddr("127.0.0.1:8090"))

			downstreamHello := proto.DownstreamInventoryHello{
				Version:  teleport.Version,
				ServerID: "auth",
				Capabilities: &proto.DownstreamInventoryHello_SupportedCapabilities{
					AppCleanup:     test.supportsGoodbye,
					AppHeartbeats:  true,
					NodeHeartbeats: true,
				},
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			handle := NewDownstreamHandle(func(ctx context.Context) (client.DownstreamInventoryControlStream, error) {
				return downstream, nil
			}, upstreamHello)

			// Wait for upstream hello.
			select {
			case msg := <-upstream.Recv():
				require.Equal(t, upstreamHello, msg)
			case <-ctx.Done():
				require.Fail(t, "never got upstream hello")
			}
			require.NoError(t, upstream.Send(ctx, downstreamHello))

			// Attempt to send a goodbye.
			go func() {
				ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				assert.NoError(t, handle.SendGoodbye(ctx))
				// Close the handle to unblock receive below.
				assert.NoError(t, handle.Close())
			}()

			// Wait to see if a goodbye is received.
			timeoutC := time.After(10 * time.Second)
			for {
				select {
				case msg := <-upstream.Recv():
					switch msg.(type) {
					case proto.UpstreamInventoryHello, proto.InventoryHeartbeat,
						proto.UpstreamInventoryPong, proto.UpstreamInventoryAgentMetadata:
					case proto.UpstreamInventoryGoodbye:
						if test.supportsGoodbye {
							require.Equal(t, proto.UpstreamInventoryGoodbye{DeleteResources: true}, msg)
						} else {
							t.Fatalf("received an unexpected message %v", msg)
						}
						return
					}
				case <-upstream.Done():
					return
				case <-timeoutC:
					if test.supportsGoodbye {
						require.FailNow(t, "timeout waiting for goodbye message")
					} else {
						return
					}
				}
			}
		})
	}
}

func TestGetSender(t *testing.T) {

	controller := NewController(
		&fakeAuth{},
		usagereporter.DiscardUsageReporter{},
		withInstanceHBInterval(time.Millisecond*200),
	)
	defer controller.Close()

	// Set up fake in-memory control stream.
	upstream, downstream := client.InventoryControlStreamPipe(client.ICSPipePeerAddr("127.0.0.1:8090"))

	downstreamHello := proto.DownstreamInventoryHello{
		Version:  teleport.Version,
		ServerID: "auth",
		Capabilities: &proto.DownstreamInventoryHello_SupportedCapabilities{
			AppCleanup:     true,
			AppHeartbeats:  true,
			NodeHeartbeats: true,
		},
	}

	upstreamHello := proto.UpstreamInventoryHello{
		ServerID: "llama",
		Version:  teleport.Version,
		Services: []types.SystemRole{types.RoleNode, types.RoleApp},
	}

	handle := NewDownstreamHandle(func(ctx context.Context) (client.DownstreamInventoryControlStream, error) {
		return downstream, nil
	}, upstreamHello)

	// Validate that the sender is not present prior to
	// the stream becoming healthy.
	s, ok := handle.GetSender()
	require.False(t, ok)
	require.Nil(t, s)

	// Wait for upstream hello.
	select {
	case msg := <-upstream.Recv():
		require.Equal(t, upstreamHello, msg)
	case <-time.After(5 * time.Second):
		require.Fail(t, "never got upstream hello")
	}
	// Send the downstream hello so that the
	// sender becomes available.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, upstream.Send(ctx, downstreamHello))

	// Validate that once healthy the sender is provided.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		s, ok = handle.GetSender()
		assert.True(t, ok)
		assert.NotNil(t, s)
	}, 10*time.Second, 100*time.Millisecond)
}

type eventOpts struct {
	expect map[testEvent]int
	deny   map[testEvent]struct{}
}

type eventOption func(*eventOpts)

func expect(events ...testEvent) eventOption {
	return func(opts *eventOpts) {
		for _, event := range events {
			opts.expect[event] = opts.expect[event] + 1
		}
	}
}

func deny(events ...testEvent) eventOption {
	return func(opts *eventOpts) {
		for _, event := range events {
			opts.deny[event] = struct{}{}
		}
	}
}

func awaitEvents(t *testing.T, ch <-chan testEvent, opts ...eventOption) {
	options := eventOpts{
		expect: make(map[testEvent]int),
		deny:   make(map[testEvent]struct{}),
	}
	for _, opt := range opts {
		opt(&options)
	}

	timeout := time.After(time.Second * 30)
	for {
		if len(options.expect) == 0 {
			return
		}

		select {
		case event := <-ch:
			if _, ok := options.deny[event]; ok {
				require.Failf(t, "unexpected event", "event=%v", event)
			}

			options.expect[event] = options.expect[event] - 1
			if options.expect[event] < 1 {
				delete(options.expect, event)
			}
		case <-timeout:
			require.Failf(t, "timeout waiting for events", "expect=%+v", options.expect)
		}
	}
}

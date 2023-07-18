/*
Copyright 2022 Gravitational, Inc.

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

package inventory

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
	"github.com/gravitational/teleport/lib/utils"
)

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

	// verify that the peer address of the control stream was used to override
	// zero-value IPs for heartbeats.
	auth.mu.Lock()
	unexpectedAddrs := auth.unexpectedAddrs
	auth.mu.Unlock()
	require.Zero(t, unexpectedAddrs)
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

// TestInstanceHeartbeat verifies basic expected behaviors for instance heartbeat.
func TestInstanceHeartbeat(t *testing.T) {
	t.Setenv("TELEPORT_UNSTABLE_ENABLE_INSTANCE_HB", "yes")

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
	require.NoError(t, upstreamHandle.UpdateLabels(ctx, proto.LabelUpdateKind_SSHServer, labels))

	require.Eventually(t, func() bool {
		require.Equal(t, labels, downstreamHandle.GetUpstreamLabels(proto.LabelUpdateKind_SSHServer))
		return true
	}, time.Second, 100*time.Millisecond)
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

	timeout := time.After(time.Second * 5)
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

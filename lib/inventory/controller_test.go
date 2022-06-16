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

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

type fakeAuth struct {
	mu             sync.Mutex
	failUpserts    int
	failKeepAlives int

	upserts    int
	keepalives int
	err        error
}

func (a *fakeAuth) UpsertNode(_ context.Context, _ types.Server) (*types.KeepAlive, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.upserts++
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

// TestControllerBasics verifies basic expected behaviors for a single control stream.
func TestControllerBasics(t *testing.T) {
	const serverID = "test-server"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := make(chan testEvent, 1024)

	auth := &fakeAuth{}

	controller := NewController(
		auth,
		withServerKeepAlive(time.Millisecond*200),
		withTestEventsChannel(events),
	)
	defer controller.Close()

	// set up fake in-memory control stream
	upstream, downstream := client.InventoryControlStreamPipe()

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
	_, err = handle.Ping(pingCtx)
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

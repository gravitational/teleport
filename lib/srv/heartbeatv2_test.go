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
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/inventory"
	"github.com/gravitational/teleport/lib/inventory/metadata"
)

type fakeHeartbeatDriver struct {
	handle  inventory.DownstreamHandle
	streamC chan client.DownstreamInventoryControlStream

	mu            sync.Mutex
	pollCount     int
	fallbackCount int
	announceCount int

	// below fields set the next N calls to the corresponding method to return
	// its non-default value (changed=true/ok=false). Set by tests to trigger
	// limited traversal of unhappy path.

	pollChanged int
	fallbackErr int
	announceErr int

	disableFallback bool
}

func (h *fakeHeartbeatDriver) Poll(ctx context.Context) (changed bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.pollCount++
	if h.pollChanged > 0 {
		h.pollChanged--
		return true
	}
	return false
}

func (h *fakeHeartbeatDriver) FallbackAnnounce(ctx context.Context) (ok bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.SupportsFallback() {
		panic("FallbackAnnounce called when SupportsFallback is false")
	}
	h.fallbackCount++
	if h.fallbackErr > 0 {
		h.fallbackErr--
		return false
	}
	return true
}

func (h *fakeHeartbeatDriver) SupportsFallback() bool {
	return !h.disableFallback
}

func (h *fakeHeartbeatDriver) Announce(ctx context.Context, sender inventory.DownstreamSender) (ok bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.announceCount++
	if h.announceErr > 0 {
		h.announceErr--
		return false
	}
	return true
}

func (h *fakeHeartbeatDriver) newStream(ctx context.Context, t *testing.T) client.UpstreamInventoryControlStream {
	upstream, downstream := client.InventoryControlStreamPipe()

	t.Cleanup(func() {
		upstream.Close()
	})

	select {
	case h.streamC <- downstream:
	case <-ctx.Done():
		t.Fatalf("context canceled during fake stream setup")
	}

	var msg proto.UpstreamInventoryMessage
	select {
	case msg = <-upstream.Recv():
	case <-ctx.Done():
		t.Fatalf("context canceled during fake stream recv")
	}

	require.IsType(t, *new(*proto.UpstreamInventoryHello), msg)

	err := upstream.Send(ctx, &proto.DownstreamInventoryHello{
		ServerID: "test-auth",
		Version:  teleport.Version,
	})
	require.NoError(t, err)

	return upstream
}

func newFakeHeartbeatDriver(t *testing.T) *fakeHeartbeatDriver {
	// streamC is used to pass a fake control stream to the downstream handle's create func.
	streamC := make(chan client.DownstreamInventoryControlStream)

	hello := &proto.UpstreamInventoryHello{
		ServerID: "test-node",
		Version:  teleport.Version,
		Services: types.SystemRoles{types.RoleNode}.StringSlice(),
	}

	handle, err := inventory.NewDownstreamHandle(func(ctx context.Context) (client.DownstreamInventoryControlStream, error) {
		// we're emulating an inventory.DownstreamCreateFunc here, but those are typically
		// expected to return an error if no stream can be acquired. we're deliberately going
		// with a blocking strategy instead here to avoid dealing w/ backoff that could make the
		// test need to run longer.
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context canceled while waiting for next control stream")
		case stream := <-streamC:
			return stream, nil
		}
	}, func(ctx context.Context) (*proto.UpstreamInventoryHello, error) { return hello, nil })
	require.NoError(t, err)

	t.Cleanup(func() {
		handle.Close()
	})

	return &fakeHeartbeatDriver{
		handle:      handle,
		streamC:     streamC,
		pollChanged: 1, // first poll is always true
	}
}

// TestHeartbeatV2Basics verifies the basic functionality of HeartbeatV2 under various conditions by
// injecting a fake driver and attempting to force the HeartbeatV2 into all of its happy and unhappy
// states while monitoring test events to verify expected behaviors.
func TestHeartbeatV2Basics(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	// set up fake hb driver that lets us easily inject failures for
	// the diff steps and assists w/ faking inventory control handles.
	driver := newFakeHeartbeatDriver(t)

	hb := newHeartbeatV2(driver.handle, driver, heartbeatV2Config{
		announceInterval: time.Millisecond * 200,
		pollInterval:     time.Millisecond * 50,
		fallbackBackoff:  time.Millisecond * 400,
		testEvents:       make(chan hbv2TestEvent, 1028),
	})
	go hb.Run()
	defer hb.Close()

	// initial state: fallback announce and polling occur, but
	// no control stream is available yet, so we don't ever
	// use the control-stream announce. First poll always reads
	// as different, so expect that too.
	awaitEvents(t, hb.testEvents,
		expect(hbv2PollDiff, hbv2FallbackOk, hbv2Start, hbv2OnHeartbeatOk),
		deny(hbv2FallbackErr, hbv2FallbackBackoff, hbv2AnnounceOk, hbv2AnnounceErr, hbv2OnHeartbeatErr),
	)

	// verify that we're now polling "same" and that time-based announces
	// are being triggered (we set the announce interval very short, so these
	// should be going off a lot).
	awaitEvents(t, hb.testEvents,
		expect(hbv2PollSame, hbv2AnnounceInterval),
		deny(hbv2PollDiff),
	)

	// set up some fallback errs
	driver.mu.Lock()
	driver.fallbackErr = 2
	driver.mu.Unlock()

	// wait for fallback errors to happen, and confirm that we see fallback backoff
	// come into effect. we still expect no proper announce events.
	awaitEvents(t, hb.testEvents,
		expect(hbv2FallbackErr, hbv2FallbackErr, hbv2FallbackBackoff, hbv2FallbackOk, hbv2OnHeartbeatErr, hbv2OnHeartbeatOk),
		deny(hbv2AnnounceOk, hbv2AnnounceErr),
	)

	// confirm we resume non-err fallback calls (gotta check this separately
	// to establish ordering).
	awaitEvents(t, hb.testEvents,
		expect(hbv2FallbackOk, hbv2FallbackOk),
		deny(hbv2FallbackErr, hbv2AnnounceOk, hbv2AnnounceErr),
	)

	// make a stream available to the heartbeat instance
	// (note: we don't need to pull from our half of the stream since
	// fakeHeartbeatDriverInner doesn't actually send any messages across it).
	stream := driver.newStream(ctx, t)

	// wait for at least one announce to be certain that we're no longer
	// in fallback mode.
	awaitEvents(t, hb.testEvents,
		expect(hbv2AnnounceOk),
		deny(hbv2AnnounceErr),
	)

	// start denying fallbacks to make sure the change of modes stuck (kind of a silly
	// check given how trivial the control-flow is currently, but its good to have this here
	// in case we refactor anything later). Take this opportunity to re-check that our announces
	// are internval and not poll based.
	awaitEvents(t, hb.testEvents,
		expect(hbv2AnnounceOk, hbv2AnnounceOk, hbv2PollSame, hbv2AnnounceInterval, hbv2OnHeartbeatOk),
		deny(hbv2AnnounceErr, hbv2FallbackOk, hbv2FallbackErr, hbv2OnHeartbeatErr),
	)

	// set up a "changed" poll since we haven't traversed that path
	// in stream-based announce mode yet.
	driver.mu.Lock()
	driver.pollChanged = 1
	driver.mu.Unlock()

	// confirm poll diff
	awaitEvents(t, hb.testEvents,
		expect(hbv2PollDiff),
		deny(hbv2AnnounceErr, hbv2FallbackOk, hbv2FallbackErr),
	)

	// confirm healthy announce w/ happens-after relationship to
	// the poll diff.
	awaitEvents(t, hb.testEvents,
		expect(hbv2AnnounceOk),
		deny(hbv2AnnounceErr, hbv2FallbackOk, hbv2FallbackErr),
	)

	// force hb back into fallback mode
	stream.Close()

	// wait for first fallback call
	awaitEvents(t, hb.testEvents,
		expect(hbv2FallbackOk),
		deny(hbv2FallbackErr),
	)

	// confirm that we stay in fallback mode (this is more of a sanity-check for
	// our fakeHeartbeatDriver impl than a test of the actually hbv2).
	awaitEvents(t, hb.testEvents,
		expect(hbv2FallbackOk, hbv2FallbackOk),
		deny(hbv2FallbackErr, hbv2AnnounceOk, hbv2AnnounceErr),
	)

	// create a new stream
	_ = driver.newStream(ctx, t)

	// confirm that we go back into announce mode no problem.
	awaitEvents(t, hb.testEvents,
		expect(hbv2AnnounceOk),
		deny(hbv2AnnounceErr),
	)

	// confirm that we stay in announce mode.
	awaitEvents(t, hb.testEvents,
		expect(hbv2AnnounceOk),
		deny(hbv2AnnounceErr, hbv2FallbackOk, hbv2FallbackErr),
	)
}

func TestHeartbeatV2NoFallbackUnchecked(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	// set up fake hb driver that lets us easily inject failures for
	// the diff steps and assists w/ faking inventory control handles.
	driver := newFakeHeartbeatDriver(t)

	driver.mu.Lock()
	driver.disableFallback = true
	driver.mu.Unlock()

	// set up hb with very long degraded check interval to confirm expected
	// OnHeartbeat behavior outside of periodic checks.
	hb := newHeartbeatV2(driver.handle, driver, heartbeatV2Config{
		announceInterval:      time.Millisecond * 200,
		pollInterval:          time.Millisecond * 50,
		fallbackBackoff:       time.Millisecond * 400,
		degradedCheckInterval: time.Hour,
		testEvents:            make(chan hbv2TestEvent, 1028),
	})
	go hb.Run()
	defer hb.Close()

	// verify that we tick but don't ever emit a degraded state
	awaitEvents(t, hb.testEvents,
		expect(hbv2NoFallback, hbv2NoFallback),
		deny(hbv2OnHeartbeatOk, hbv2OnHeartbeatErr),
	)

	// make a stream available to the heartbeat instance
	// (note: we don't need to pull from our half of the stream since
	// fakeHeartbeatDriverInner doesn't actually send any messages across it).
	stream := driver.newStream(ctx, t)

	// verify heartbeats
	awaitEvents(t, hb.testEvents,
		expect(hbv2OnHeartbeatOk),
		deny(hbv2OnHeartbeatErr),
	)

	// verify that while heartbeating, we don't hit the "NoFallback" case
	awaitEvents(t, hb.testEvents,
		expect(hbv2OnHeartbeatOk, hbv2OnHeartbeatOk),
		deny(hbv2OnHeartbeatErr, hbv2NoFallback),
	)

	stream.Close()

	awaitEvents(t, hb.testEvents,
		expect(hbv2NoFallback),
		deny(hbv2OnHeartbeatErr),
	)
}

func TestHeartbeatV2NoFallbackChecked(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	// set up fake hb driver that lets us easily inject failures for
	// the diff steps and assists w/ faking inventory control handles.
	driver := newFakeHeartbeatDriver(t)

	driver.mu.Lock()
	driver.disableFallback = true
	driver.mu.Unlock()

	// set up hb with fast degraded check interval
	hb := newHeartbeatV2(driver.handle, driver, heartbeatV2Config{
		announceInterval:      time.Millisecond * 200,
		pollInterval:          time.Millisecond * 50,
		fallbackBackoff:       time.Millisecond * 400,
		degradedCheckInterval: time.Millisecond * 100,
		testEvents:            make(chan hbv2TestEvent, 1028),
	})
	go hb.Run()
	defer hb.Close()

	// verify that we tick and emit hb errs
	awaitEvents(t, hb.testEvents,
		expect(hbv2NoFallback, hbv2NoFallback, hbv2OnHeartbeatErr),
		deny(hbv2OnHeartbeatOk),
	)

	// make a stream available to the heartbeat instance
	// (note: we don't need to pull from our half of the stream since
	// fakeHeartbeatDriverInner doesn't actually send any messages across it).
	stream := driver.newStream(ctx, t)

	// verify heartbeats start
	awaitEvents(t, hb.testEvents,
		expect(hbv2OnHeartbeatOk),
	)

	// verify that the hb errs have stopped
	awaitEvents(t, hb.testEvents,
		expect(hbv2OnHeartbeatOk, hbv2OnHeartbeatOk),
		deny(hbv2OnHeartbeatErr),
	)

	stream.Close()

	// verify that closure causes errors to resume
	awaitEvents(t, hb.testEvents,
		expect(hbv2NoFallback, hbv2OnHeartbeatErr),
	)

	// verify that closure means OK events have stopped
	awaitEvents(t, hb.testEvents,
		expect(hbv2NoFallback, hbv2OnHeartbeatErr),
		deny(hbv2OnHeartbeatOk),
	)
}

type eventOpts struct {
	expect map[hbv2TestEvent]int
	deny   map[hbv2TestEvent]struct{}
}

type eventOption func(*eventOpts)

func expect(events ...hbv2TestEvent) eventOption {
	return func(opts *eventOpts) {
		for _, event := range events {
			opts.expect[event] = opts.expect[event] + 1
		}
	}
}

func deny(events ...hbv2TestEvent) eventOption {
	return func(opts *eventOpts) {
		for _, event := range events {
			opts.deny[event] = struct{}{}
		}
	}
}

func awaitEvents(t *testing.T, ch <-chan hbv2TestEvent, opts ...eventOption) {
	options := eventOpts{
		expect: make(map[hbv2TestEvent]int),
		deny:   make(map[hbv2TestEvent]struct{}),
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

type fakeDownstreamHandle struct {
	inventory.DownstreamHandle
}

func (f *fakeDownstreamHandle) CloseContext() context.Context {
	return context.Background()
}

func mockMetadataGetter() (metadataGetter, chan *metadata.Metadata) {
	ch := make(chan *metadata.Metadata, 1)
	return func(ctx context.Context) (*metadata.Metadata, error) {
		select {
		case meta := <-ch:
			if meta == nil {
				return nil, errors.New("error fetching metadata")
			}
			return meta, nil
		default:
		}

		select {
		case meta := <-ch:
			if meta == nil {
				return nil, errors.New("error fetching metadata")
			}
			return meta, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}, ch
}

func makeMetadata(id string) *metadata.Metadata {
	return &metadata.Metadata{
		CloudMetadata: &types.CloudMetadata{
			AWS: &types.AWSInfo{
				InstanceID: id,
			},
		},
	}
}

func TestNewHeartbeatFetchMetadata(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	heartbeat, err := NewSSHServerHeartbeat(HeartbeatV2Config[*types.ServerV2]{
		InventoryHandle: &fakeDownstreamHandle{},
		GetResource: func(context.Context) (*types.ServerV2, error) {
			return &types.ServerV2{
				Spec: types.ServerSpecV2{},
			}, nil
		},
	})
	require.NoError(t, err)
	metadataGetter, metaCh := mockMetadataGetter()
	t.Cleanup(func() { close(metaCh) })
	inner := heartbeat.inner.(*sshServerHeartbeatV2)
	inner.getMetadata = metadataGetter

	// Metadata won't be set before metadata getter returns.
	server, err := inner.getServer(ctx)
	require.NoError(t, err)
	assert.Nil(t, server.GetCloudMetadata(), "Metadata was set before background process returned")

	// Metadata won't be set if the getter fails.
	metaCh <- nil
	time.Sleep(100 * time.Millisecond) // Wait for goroutines to complete
	server, err = inner.getServer(ctx)
	require.NoError(t, err)
	assert.Nil(t, server.GetCloudMetadata(), "Metadata was set despite metadata getter failing")

	// getServer gets updated metadata value.
	metaCh <- makeMetadata("foo")
	time.Sleep(100 * time.Millisecond) // Wait for goroutines to complete
	server, err = inner.getServer(ctx)
	require.NoError(t, err)
	meta := server.GetCloudMetadata()
	assert.NotNil(t, meta, "Heartbeat never got metadata")
	assert.Equal(t, "foo", meta.AWS.InstanceID)
}

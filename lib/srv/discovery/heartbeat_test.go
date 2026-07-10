// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package discovery

import (
	"context"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	discoveryservicev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryservice/v1"
	"github.com/gravitational/teleport/api/utils/retryutils"
)

// TestBuildSelfHeartbeatDeterminism proves the heartbeat payload is identical
// across repeated builds with unchanged state — the property that
// announce-on-change diffing depends on.
func TestBuildSelfHeartbeatDeterminism(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()
	s := &Server{
		Config: &Config{
			ServerID:       "aaaaaaaa-1111-2222-3333-444444444444",
			Hostname:       "disc-1.example.com",
			DiscoveryGroup: "demo",
			PollInterval:   5 * time.Minute,
			clock:          clock,
		},
	}

	first := s.buildSelfHeartbeat()
	for range 10 {
		require.True(t, proto.Equal(first, s.buildSelfHeartbeat()),
			"heartbeat payload must be deterministic across builds with unchanged state")
	}

	require.Equal(t, s.ServerID, first.GetMetadata().GetName())
	require.Equal(t, "demo", first.GetSpec().GetDiscoveryGroup())
	require.Equal(t, "disc-1.example.com", first.GetSpec().GetHostname())
	require.NotNil(t, first.GetMetadata().GetExpires())
}

type fakeAnnouncer struct {
	calls chan struct{}
	err   atomic.Pointer[error]
}

func newFakeAnnouncer(err error) *fakeAnnouncer {
	fa := &fakeAnnouncer{calls: make(chan struct{}, 100)}
	fa.err.Store(&err)
	return fa
}

func (f *fakeAnnouncer) UpsertDiscoveryService(ctx context.Context, svc *discoveryservicev1.DiscoveryService) (*discoveryservicev1.DiscoveryService, error) {
	select {
	case f.calls <- struct{}{}:
	case <-ctx.Done():
	}
	return svc, *f.err.Load()
}

func newAnnouncerTestServer(t *testing.T, announcer Announcer, clock clockwork.Clock) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return &Server{
		Config: &Config{
			ServerID:                  "host-1",
			Log:                       slog.Default(),
			clock:                     clock,
			jitter:                    retryutils.SeventhJitter,
			DiscoveryServiceAnnouncer: announcer,
		},
		ctx:      ctx,
		cancelfn: cancel,
	}
}

func expectAnnounce(t *testing.T, fa *fakeAnnouncer, msg string) {
	t.Helper()
	select {
	case <-fa.calls:
	case <-time.After(5 * time.Second):
		t.Fatal(msg)
	}
}

func expectNoAnnounce(t *testing.T, fa *fakeAnnouncer, msg string) {
	t.Helper()
	select {
	case <-fa.calls:
		t.Fatal(msg)
	case <-time.After(100 * time.Millisecond):
	}
}

// tick advances the fake clock by one check period once the announcer is
// waiting on its ticker.
func tick(t *testing.T, clock *clockwork.FakeClock, d time.Duration) {
	t.Helper()
	require.NoError(t, clock.BlockUntilContext(t.Context(), 1))
	clock.Advance(d)
}

// TestAnnouncerRenews validates the steady-state loop: announce at start,
// silence between renewals, and a renewal after the announce period elapses.
func TestAnnouncerRenews(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()
	fa := newFakeAnnouncer(nil)
	s := newAnnouncerTestServer(t, fa, clock)

	s.startHeartbeatAnnouncer()
	expectAnnounce(t, fa, "expected an initial announce")

	// One check period later, nothing changed and the renewal isn't due.
	tick(t, clock, heartbeatCheckPeriod)
	expectNoAnnounce(t, fa, "must not announce when nothing changed and renewal is not due")

	// Advance past the maximum announce period: renewal must fire.
	tick(t, clock, heartbeatTTL/2+heartbeatTTL/10)
	expectAnnounce(t, fa, "expected a renewal announce after the announce period")
}

// TestAnnouncerAnnouncesOnChange validates change propagation: a spec change
// is announced within one check period, not at the next renewal.
func TestAnnouncerAnnouncesOnChange(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()
	fa := newFakeAnnouncer(nil)
	s := newAnnouncerTestServer(t, fa, clock)

	s.startHeartbeatAnnouncer()
	expectAnnounce(t, fa, "expected an initial announce")

	tick(t, clock, heartbeatCheckPeriod)
	expectNoAnnounce(t, fa, "must not announce when nothing changed")

	// Mutate state the payload derives from; the next check must announce.
	s.Hostname = "renamed.example.com"
	tick(t, clock, heartbeatCheckPeriod)
	expectAnnounce(t, fa, "expected a change-triggered announce within one check period")
}

// TestAnnouncerNotImplemented validates the compatibility behavior: on
// NotImplemented from an older auth server the announcer reports healthy,
// goes quiet, and probes again later so heartbeating resumes after an auth
// upgrade without an agent restart.
func TestAnnouncerNotImplemented(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()
	fa := newFakeAnnouncer(trace.NotImplemented("old auth"))
	var lastHeartbeatErr atomic.Pointer[error]
	s := newAnnouncerTestServer(t, fa, clock)
	s.OnHeartbeat = func(err error) { lastHeartbeatErr.Store(&err) }

	s.startHeartbeatAnnouncer()
	expectAnnounce(t, fa, "expected an initial announce attempt")

	// Service must still report healthy: no heartbeat support is not an error.
	require.Eventually(t, func() bool {
		p := lastHeartbeatErr.Load()
		return p != nil && *p == nil
	}, 5*time.Second, 10*time.Millisecond, "NotImplemented must report ready, not failing")

	// Quiet through the probe backoff window.
	tick(t, clock, 30*time.Minute)
	expectNoAnnounce(t, fa, "must not hammer an auth that lacks support")

	// Auth got upgraded: the next probe succeeds and heartbeating resumes.
	fa.err.Store(new(error))
	tick(t, clock, 31*time.Minute)
	expectAnnounce(t, fa, "expected a probe after the NotImplemented backoff")
}

// TestAnnouncerRetriesOnTransientError validates that ordinary announce
// failures retry on the retry period rather than every check period, and
// report unhealthy through OnHeartbeat.
func TestAnnouncerRetriesOnTransientError(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()
	fa := newFakeAnnouncer(trace.ConnectionProblem(nil, "boom"))
	var lastHeartbeatErr atomic.Pointer[error]
	s := newAnnouncerTestServer(t, fa, clock)
	s.OnHeartbeat = func(err error) { lastHeartbeatErr.Store(&err) }

	s.startHeartbeatAnnouncer()
	expectAnnounce(t, fa, "expected an initial announce attempt")

	require.Eventually(t, func() bool {
		p := lastHeartbeatErr.Load()
		return p != nil && *p != nil
	}, 5*time.Second, 10*time.Millisecond, "transient failure must report through OnHeartbeat")

	// No hammering on the very next check period...
	tick(t, clock, heartbeatCheckPeriod)
	expectNoAnnounce(t, fa, "failed announce must back off, not retry every check period")

	// ...but a retry lands within the retry period.
	tick(t, clock, heartbeatRetryPeriod)
	expectAnnounce(t, fa, "expected a retry within the retry period")
}

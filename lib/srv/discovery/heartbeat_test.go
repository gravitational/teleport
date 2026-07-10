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
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	discoveryservicev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryservice/v1"
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
	err   error
}

func (f *fakeAnnouncer) UpsertDiscoveryService(ctx context.Context, svc *discoveryservicev1.DiscoveryService) (*discoveryservicev1.DiscoveryService, error) {
	select {
	case f.calls <- struct{}{}:
	case <-ctx.Done():
	}
	return svc, f.err
}

func newAnnouncerTestServer(t *testing.T, announcer Announcer, clock clockwork.Clock) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return &Server{
		Config: &Config{
			ServerID:                  "host-1",
			Log:                       slog.Default(),
			clock:                     clock,
			DiscoveryServiceAnnouncer: announcer,
		},
		ctx:      ctx,
		cancelfn: cancel,
	}
}

// TestAnnouncerNotImplemented validates the compatibility behavior: on
// NotImplemented from an older auth server the announcer logs once and stops
// permanently — it must never crash the service or keep retrying.
func TestAnnouncerNotImplemented(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()
	fa := &fakeAnnouncer{calls: make(chan struct{}, 10), err: trace.NotImplemented("old auth")}
	s := newAnnouncerTestServer(t, fa, clock)

	s.startHeartbeatAnnouncer()

	select {
	case <-fa.calls:
	case <-time.After(5 * time.Second):
		t.Fatal("expected an initial announce attempt")
	}
	clock.Advance(10 * heartbeatAnnouncePeriod)
	select {
	case <-fa.calls:
		t.Fatal("announcer must stop permanently after NotImplemented")
	case <-time.After(100 * time.Millisecond):
	}
}

// TestAnnouncerRetriesOnTransientError validates that ordinary announce
// failures do not stop the loop: the next tick retries.
func TestAnnouncerRetriesOnTransientError(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()
	fa := &fakeAnnouncer{calls: make(chan struct{}, 10), err: trace.ConnectionProblem(nil, "boom")}
	s := newAnnouncerTestServer(t, fa, clock)

	s.startHeartbeatAnnouncer()

	select {
	case <-fa.calls:
	case <-time.After(5 * time.Second):
		t.Fatal("expected an initial announce attempt")
	}
	require.NoError(t, clock.BlockUntilContext(t.Context(), 1))
	clock.Advance(heartbeatAnnouncePeriod)
	select {
	case <-fa.calls:
	case <-time.After(5 * time.Second):
		t.Fatal("expected a retry on the next tick after a transient error")
	}
}

// TestAnnouncerRenews validates the steady-state loop: announce at start and
// on every tick.
func TestAnnouncerRenews(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()
	fa := &fakeAnnouncer{calls: make(chan struct{}, 10)}
	s := newAnnouncerTestServer(t, fa, clock)

	s.startHeartbeatAnnouncer()

	select {
	case <-fa.calls:
	case <-time.After(5 * time.Second):
		t.Fatal("expected an initial announce")
	}
	for range 3 {
		require.NoError(t, clock.BlockUntilContext(t.Context(), 1))
		clock.Advance(heartbeatAnnouncePeriod)
		select {
		case <-fa.calls:
		case <-time.After(5 * time.Second):
			t.Fatal("expected an announce per tick")
		}
	}
}

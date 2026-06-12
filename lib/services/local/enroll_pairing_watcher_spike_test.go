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

// This file is a throwaway spike. It sketches how the public Device Trust RPC
// will consume EnrollPairing events via the standard event stream - no custom
// watcher needed. Delete when the real public-RPC issue lands.
//
// The watcher is scoped to the pairing's name (the owning user), which is the
// only field a server-side filter can match on a delete - so the handler is
// notified both when its pairing transitions state (put) and when it's removed
// (delete). Removal covers both TTL expiry and an explicit deny: denying a
// pairing is modeled as deleting it from the backend, not as a DENIED state, so
// the delete path handles both.
//
// Because a same-user pairing can TTL-expire and be recreated with a fresh
// token, the handler also checks the token in-process on put events to avoid
// resolving its wait on the wrong pairing.

package local_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

// errPairingGone reports that the pairing the handler was waiting on is no
// longer the one it can resolve: it was removed (TTL expiry or deny), or
// replaced by a fresh pairing for the same user. In production the public RPC
// would translate this into a NotFound/expired response to the mobile app.
var errPairingGone = errors.New("enroll pairing is gone")

// TestEnrollPairingEventStream walks an EnrollPairing through its state machine
// while a goroutine modeling the public Device Trust RPC handler blocks on the
// standard event stream waiting for APPROVED.
func TestEnrollPairingEventStream(t *testing.T) {
	t.Parallel()
	const user = "alice"

	bk, err := memory.New(memory.Config{
		Context: t.Context(),
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = bk.Close() })

	service, err := local.NewEnrollPairingService(bk)
	require.NoError(t, err)
	events := local.NewEventsService(bk)

	// Authenticated side: Web UI opens the wizard, creates the pairing.
	created, err := service.CreateEnrollPairing(t.Context(), user)
	require.NoError(t, err)

	type result struct {
		pairing *devicepb.EnrollPairing
		err     error
	}
	approved := make(chan result, 1)
	ready := make(chan struct{})

	// Public RPC handler sketch. The mobile app's request carries only the token.
	// The handler resolves the pairing (and thus its name) by looking it up by
	// the token, CASes it to AWAITING_APPROVAL (both omitted here - the index on
	// token still needs to be implemented), then blocks for APPROVED via the
	// standard event stream and returns the resolved pairing.
	go func() {
		p, err := awaitApprovedSketch(t.Context(), events, created.GetMetadata().GetName(), created.GetStatus().GetToken(), ready)
		approved <- result{p, err}
	}()

	// Wait for the watcher to be subscribed so the puts below don't race
	// the OpInit handshake.
	select {
	case <-ready:
	case <-time.After(time.Second):
		t.Fatal("watcher did not initialize in time")
	}

	// Drive the state machine. In production, AWAITING_APPROVAL is written by the
	// public RPC's CAS and APPROVED by ApproveEnrollPairing on the authenticated
	// side. The spike just bypasses the RPCs and writes the states directly.
	awaitingApproval := proto.Clone(created).(*devicepb.EnrollPairing)
	awaitingApproval.GetStatus().SetState(devicepb.EnrollPairingState_ENROLL_PAIRING_STATE_AWAITING_APPROVAL)
	require.NoError(t, putForTest(t.Context(), bk, awaitingApproval))

	approvedPairing := proto.Clone(awaitingApproval).(*devicepb.EnrollPairing)
	approvedPairing.GetStatus().SetState(devicepb.EnrollPairingState_ENROLL_PAIRING_STATE_APPROVED)
	require.NoError(t, putForTest(t.Context(), bk, approvedPairing))

	select {
	case r := <-approved:
		require.NoError(t, r.err)
		require.Equal(t, user, r.pairing.GetMetadata().GetName())
		require.Equal(t,
			devicepb.EnrollPairingState_ENROLL_PAIRING_STATE_APPROVED,
			r.pairing.GetStatus().GetState())
	case <-time.After(2 * time.Second):
		t.Fatal("public RPC sketch did not observe APPROVED in time")
	}
}

// TestEnrollPairingDeniedDuringWait asserts the handler returns promptly when
// its pairing is denied. Deny is modeled as deleting the pairing, and the name
// filter routes the watched user's delete to the handler.
func TestEnrollPairingDeniedDuringWait(t *testing.T) {
	t.Parallel()
	const user = "alice"

	bk, err := memory.New(memory.Config{
		Context: t.Context(),
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = bk.Close() })

	service, err := local.NewEnrollPairingService(bk)
	require.NoError(t, err)
	events := local.NewEventsService(bk)

	created, err := service.CreateEnrollPairing(t.Context(), user)
	require.NoError(t, err)

	type result struct {
		pairing *devicepb.EnrollPairing
		err     error
	}
	done := make(chan result, 1)
	ready := make(chan struct{})

	go func() {
		p, err := awaitApprovedSketch(t.Context(), events, created.GetMetadata().GetName(), created.GetStatus().GetToken(), ready)
		done <- result{p, err}
	}()

	select {
	case <-ready:
	case <-time.After(time.Second):
		t.Fatal("watcher did not initialize in time")
	}

	// Deny deletes the pairing. DenyEnrollPairing doesn't exist yet, so go
	// through the raw backend.
	require.NoError(t, bk.Delete(t.Context(), backend.NewKey("devices", "enroll_pairing", user)))

	select {
	case r := <-done:
		require.ErrorIs(t, r.err, errPairingGone)
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not observe the deny in time")
	}
}

// TestEnrollPairingReplacedDuringWait covers a same-user replacement that
// passes the name filter but is a different pairing. The in-process token check
// must treat it as gone, not resolve the wait - even when the replacement is
// already APPROVED. We can't assume the old pairing's delete is observed before
// the new put: TTL-delete events are asynchronous and backend-dependent (see
// testEvents in lib/backend/test, which waits a separate, extendable
// ttlDeleteTimeout for them), so the expiry delete can lag the replacing
// create. The token check is therefore load-bearing. The memory backend's
// synchronous expiry hides this, so the test overwrites the key directly to
// stand in for that ordering.
func TestEnrollPairingReplacedDuringWait(t *testing.T) {
	t.Parallel()
	const user = "alice"

	bk, err := memory.New(memory.Config{
		Context: t.Context(),
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = bk.Close() })

	service, err := local.NewEnrollPairingService(bk)
	require.NoError(t, err)
	events := local.NewEventsService(bk)

	created, err := service.CreateEnrollPairing(t.Context(), user)
	require.NoError(t, err)

	type result struct {
		pairing *devicepb.EnrollPairing
		err     error
	}
	done := make(chan result, 1)
	ready := make(chan struct{})

	go func() {
		p, err := awaitApprovedSketch(t.Context(), events, created.GetMetadata().GetName(), created.GetStatus().GetToken(), ready)
		done <- result{p, err}
	}()

	select {
	case <-ready:
	case <-time.After(time.Second):
		t.Fatal("watcher did not initialize in time")
	}

	// A fresh pairing for the same user, already APPROVED, but with a
	// different token. Modeled as a direct overwrite of the same key.
	replacement := proto.Clone(created).(*devicepb.EnrollPairing)
	replacement.GetStatus().SetToken("a-different-token")
	replacement.GetStatus().SetState(devicepb.EnrollPairingState_ENROLL_PAIRING_STATE_APPROVED)
	require.NoError(t, putForTest(t.Context(), bk, replacement))

	select {
	case r := <-done:
		require.ErrorIs(t, r.err, errPairingGone,
			"an APPROVED put for a different token must not resolve the wait")
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not observe the replacement in time")
	}
}

// awaitApprovedSketch is what the public Device Trust RPC handler will look
// like in production. The mobile app's request supplies only the token. The
// handler resolves the pairing by looking it up by token (out of scope here),
// which yields the name (the owning user) and lets it CAS the state to
// AWAITING_APPROVAL, then opens an event watcher scoped to that name and
// blocks. The name and token passed in here stand in for that lookup result.
// It returns:
//   - the pairing, once it transitions to APPROVED;
//   - errPairingGone, if the pairing is deleted, or replaced by a fresh
//     pairing for the same user (detected by the token no longer matching).
//
// Two omissions vs. production: it should bound the wait by a deadline (expiry
// deletes can lag, so the deadline is the real expiry backstop) and reconcile
// state by token right after subscribing, to cover events before OpInit - as
// HeadlessAuthenticationWatcher does.
func awaitApprovedSketch(ctx context.Context, events *local.EventsService, name, token string, ready chan<- struct{}) (*devicepb.EnrollPairing, error) {
	filter := types.EnrollPairingFilter{Name: name}
	w, err := events.NewWatcher(ctx, types.Watch{
		Kinds: []types.WatchKind{{
			Kind:   types.KindEnrollPairing,
			Filter: filter.IntoMap(),
		}},
	})
	if err != nil {
		return nil, err
	}
	defer w.Close()

	for {
		select {
		case event := <-w.Events():
			switch event.Type {
			case types.OpInit:
				close(ready)
			case types.OpDelete:
				// The name filter guarantees this delete is for our user's
				// pairing, so the pairing we're waiting on is gone.
				return nil, errPairingGone
			case types.OpPut:
				pairing, err := types.ConvertResource[*devicepb.EnrollPairing](event.Resource)
				if err != nil {
					return nil, err
				}
				// The name filter lets every same-user put through, including
				// a fresh pairing that replaced ours after a TTL expiry. The
				// token is the only field that distinguishes them, and it's
				// available here on a put but never on a delete.
				if pairing.GetStatus().GetToken() != token {
					return nil, errPairingGone
				}
				if pairing.GetStatus().GetState() == devicepb.EnrollPairingState_ENROLL_PAIRING_STATE_APPROVED {
					return pairing, nil
				}
			}
		case <-w.Done():
			return nil, context.Canceled
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// TestEnrollPairingNameFilter asserts the parser's server-side name filter:
// events for other users are dropped, while both puts and deletes for the
// watched user are delivered. Delete delivery is the property token-based
// filtering couldn't provide - a delete carries no value, so name (which lives
// in the key) is the only field the server can match on.
func TestEnrollPairingNameFilter(t *testing.T) {
	t.Parallel()
	const watched = "alice"
	const other = "bob"

	bk, err := memory.New(memory.Config{
		Context: t.Context(),
		Clock:   clockwork.NewFakeClock(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = bk.Close() })

	service, err := local.NewEnrollPairingService(bk)
	require.NoError(t, err)
	events := local.NewEventsService(bk)

	filter := types.EnrollPairingFilter{Name: watched}
	w, err := events.NewWatcher(t.Context(), types.Watch{
		Kinds: []types.WatchKind{{
			Kind:   types.KindEnrollPairing,
			Filter: filter.IntoMap(),
		}},
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Close() })

	select {
	case event := <-w.Events():
		require.Equal(t, types.OpInit, event.Type)
	case <-time.After(time.Second):
		t.Fatal("did not see OpInit")
	}

	// A concurrent enrollment by another user must be dropped at the parser.
	_, err = service.CreateEnrollPairing(t.Context(), other)
	require.NoError(t, err)

	// The watched user's pairing: its put must be delivered.
	_, err = service.CreateEnrollPairing(t.Context(), watched)
	require.NoError(t, err)

	select {
	case event := <-w.Events():
		require.Equal(t, types.OpPut, event.Type)
		got, err := types.ConvertResource[*devicepb.EnrollPairing](event.Resource)
		require.NoError(t, err)
		require.Equal(t, watched, got.GetMetadata().GetName())
	case <-time.After(time.Second):
		t.Fatal("did not see the watched user's put event")
	}

	// Deleting the watched user's pairing must be delivered too: the property
	// that lets the handler learn its pairing was removed.
	require.NoError(t, bk.Delete(t.Context(), backend.NewKey("devices", "enroll_pairing", watched)))

	select {
	case event := <-w.Events():
		require.Equal(t, types.OpDelete, event.Type)
		require.Equal(t, watched, event.Resource.GetName())
	case <-time.After(time.Second):
		t.Fatal("did not see the watched user's delete event")
	}

	// No further events: bob's put and (on cleanup) delete are dropped at the
	// parser by name, so nothing but the watched user's events ever arrives.
	stop := time.After(100 * time.Millisecond)
	select {
	case event := <-w.Events():
		t.Fatalf("unexpected event for an unwatched user: %+v", event)
	case <-stop:
	}
}

func putForTest(ctx context.Context, bk backend.Backend, p *devicepb.EnrollPairing) error {
	value, err := services.MarshalEnrollPairing(p)
	if err != nil {
		return err
	}
	expires := p.GetMetadata().GetExpires()
	var exp time.Time
	if expires != nil {
		exp = expires.AsTime()
	} else {
		exp = bk.Clock().Now().Add(5 * time.Minute)
		p.GetMetadata().SetExpires(timestamppb.New(exp))
	}
	_, err = bk.Put(ctx, backend.Item{
		Key:     backend.NewKey("devices", "enroll_pairing", p.GetMetadata().GetName()),
		Value:   value,
		Expires: exp,
	})
	return err
}

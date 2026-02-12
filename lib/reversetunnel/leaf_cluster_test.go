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

package reversetunnel

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
)

func TestSyncValidatedMFAChallenges(t *testing.T) {
	t.Parallel()

	now := time.Now()

	// Create three validated challenges:
	//
	// 1. One for the leaf cluster
	// 2. One for a different leaf cluster
	// 3. One that is expired
	//
	// Only the first one should be replicated to the leaf.
	challengeForLeaf := newValidatedMFAChallenge(
		"challenge-for-leaf",
		"leaf.example.com",
		now.Add(time.Minute),
	)

	challengeForOtherLeaf := newValidatedMFAChallenge(
		"challenge-for-other-leaf",
		"other-leaf.example.com",
		now.Add(time.Minute),
	)

	expiredChallenge := newValidatedMFAChallenge(
		"expired-challenge",
		"leaf.example.com",
		now.Add(-time.Minute),
	)

	// Set up a channel to send events to the watcher and prime it with an init event.
	events := make(chan types.Event, 1)
	events <- types.Event{Type: types.OpInit}

	// Set up a mock watcher that will return the three challenges above.
	watcher, err := services.NewGenericResourceWatcher(
		t.Context(),
		services.GenericWatcherConfig[*mfav1.ValidatedMFAChallenge, *mfav1.ValidatedMFAChallenge]{
			ResourceKind: types.KindValidatedMFAChallenge,
			ResourceWatcherConfig: services.ResourceWatcherConfig{
				Component: "Watcher on the Wall",
				Client: &mockEvents{
					watcher: &mockMFAWatcher{
						events: events,
						done:   make(chan struct{}),
					},
				},
			},
			ResourceGetter: func(context.Context) ([]*mfav1.ValidatedMFAChallenge, error) {
				return []*mfav1.ValidatedMFAChallenge{
					challengeForLeaf,
					challengeForOtherLeaf,
					expiredChallenge,
				}, nil
			},
			ResourceKey: func(r *mfav1.ValidatedMFAChallenge) string {
				return r.GetMetadata().GetName()
			},
			CloneFunc: func(r *mfav1.ValidatedMFAChallenge) *mfav1.ValidatedMFAChallenge {
				return proto.Clone(r).(*mfav1.ValidatedMFAChallenge)
			},
			ReadOnlyFunc: func(r *mfav1.ValidatedMFAChallenge) *mfav1.ValidatedMFAChallenge {
				return proto.Clone(r).(*mfav1.ValidatedMFAChallenge)
			},
			ResourcesC: make(chan []*mfav1.ValidatedMFAChallenge, 2),
		},
	)
	require.NoError(t, err)
	t.Cleanup(watcher.Close)

	// Set up a mock MFA service client that will receive replicated challenges from the leaf cluster.
	leafMFAClient := &mockMFAServiceClient{
		requests: make([]*mfav1.ReplicateValidatedMFAChallengeRequest, 0),
	}

	// Set up a leaf cluster with the mock watcher and mock MFA client.
	leaf := &leafCluster{
		domainName:                   "leaf.example.com",
		logger:                       slog.Default(),
		clock:                        clockwork.NewFakeClockAt(now),
		leafClient:                   &mockLeafClient{mfaClient: leafMFAClient},
		validatedMFAChallengeWatcher: watcher,
	}

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	// Set up a channel to receive errors from the sync process.
	errC := make(chan error, 1)

	// Start syncing validated MFA challenges in a separate goroutine since it will block until the context is canceled.
	go func() {
		errC <- leaf.SyncValidatedMFAChallenges(
			ctx,
			retryutils.LinearConfig{
				First: time.Millisecond,
				Step:  time.Millisecond,
				Max:   10 * time.Millisecond,
			},
			watcher,
		)
	}()

	// Wait for the watcher to initialize before sending events to it.
	require.NoError(t, watcher.WaitInitialization())

	wantReqs := []*mfav1.ReplicateValidatedMFAChallengeRequest{
		{
			Name:          challengeForLeaf.GetMetadata().GetName(),
			Payload:       challengeForLeaf.GetSpec().GetPayload(),
			SourceCluster: challengeForLeaf.GetSpec().GetSourceCluster(),
			TargetCluster: challengeForLeaf.GetSpec().GetTargetCluster(),
			Username:      challengeForLeaf.GetSpec().GetUsername(),
		},
	}

	// The watcher will return the three challenges we set up above, and we expect the leaf cluster to replicate only
	// the valid challenge for its cluster.
	require.EventuallyWithT(
		t,
		func(c *assert.CollectT) {
			assert.Empty(
				c,
				cmp.Diff(
					wantReqs,
					leafMFAClient.Requests(),
				),
				"unexpected replicated challenge requests (-want +got)",
			)
		},
		time.Second, 10*time.Millisecond, "waiting for expected replicated challenge requests",
	)

	// Cancel the context to stop the sync process and verify that it exits without error and that the expected
	// challenge was replicated.
	cancel()
	require.NoError(t, <-errC)

	// Only the challenge intended for the leaf cluster should have been replicated, so we expect exactly one request to
	// have been made to the leaf's MFA service client.
	require.Len(t, leafMFAClient.Requests(), 1)
}

func newValidatedMFAChallenge(name, targetCluster string, expiry time.Time) *mfav1.ValidatedMFAChallenge {
	return &mfav1.ValidatedMFAChallenge{
		Kind:    types.KindValidatedMFAChallenge,
		Version: "v1",
		Metadata: &types.Metadata{
			Name:    name,
			Expires: &expiry,
		},
		Spec: &mfav1.ValidatedMFAChallengeSpec{
			Payload: &mfav1.SessionIdentifyingPayload{
				Payload: &mfav1.SessionIdentifyingPayload_SshSessionId{SshSessionId: []byte("session-id")},
			},
			SourceCluster: "root.example.com",
			TargetCluster: targetCluster,
			Username:      "alice",
		},
	}
}

type mockEvents struct {
	watcher types.Watcher
}

var _ types.Events = (*mockEvents)(nil)

func (m *mockEvents) NewWatcher(context.Context, types.Watch) (types.Watcher, error) {
	return m.watcher, nil
}

type mockMFAWatcher struct {
	events    chan types.Event
	done      chan struct{}
	closeOnce sync.Once
}

var _ types.Watcher = (*mockMFAWatcher)(nil)

func (m *mockMFAWatcher) Events() <-chan types.Event {
	return m.events
}

func (m *mockMFAWatcher) Done() <-chan struct{} {
	return m.done
}

func (m *mockMFAWatcher) Close() error {
	m.closeOnce.Do(
		func() {
			if m.done != nil {
				close(m.done)
			}
		},
	)

	return nil
}

func (*mockMFAWatcher) Error() error {
	return nil
}

type mockLeafClient struct {
	authclient.ClientI

	mfaClient mfav1.MFAServiceClient
}

func (m *mockLeafClient) MFAServiceClient() mfav1.MFAServiceClient {
	return m.mfaClient
}

type mockMFAServiceClient struct {
	mfav1.MFAServiceClient

	mu       sync.Mutex
	requests []*mfav1.ReplicateValidatedMFAChallengeRequest
}

func (m *mockMFAServiceClient) Requests() []*mfav1.ReplicateValidatedMFAChallengeRequest {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Return a copy of the requests slice to avoid potential race conditions.
	out := make([]*mfav1.ReplicateValidatedMFAChallengeRequest, len(m.requests))
	copy(out, m.requests)

	return out
}

func (m *mockMFAServiceClient) ReplicateValidatedMFAChallenge(
	_ context.Context,
	req *mfav1.ReplicateValidatedMFAChallengeRequest,
	_ ...grpc.CallOption,
) (*mfav1.ReplicateValidatedMFAChallengeResponse, error) {
	// Only one challenge is expected to be replicated.
	if req.GetName() == "challenge-for-leaf" {
		m.mu.Lock()
		m.requests = append(m.requests, req)
		m.mu.Unlock()

		return &mfav1.ReplicateValidatedMFAChallengeResponse{}, nil
	}

	return nil, trace.BadParameter("unexpected challenge: %s", req.GetName())
}

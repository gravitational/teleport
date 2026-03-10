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
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
)

func TestRunValidatedMFAChallengeSync(t *testing.T) {
	t.Parallel()

	// Create two identical challenges to test that only one of them gets replicated to the leaf cluster.
	chal := newValidatedMFAChallenge("challenge-for-leaf")
	duplicatedChal := proto.Clone(chal).(*mfav1.ValidatedMFAChallenge)

	// Set up a channel to send events to the watcher and prime it with an init event.
	events := make(chan types.Event, 1)
	events <- types.Event{Type: types.OpInit}

	clock := clockwork.NewFakeClock()

	// Set up a mock watcher that will return the challenge above.
	watcher, err := services.NewGenericResourceWatcher(
		t.Context(),
		services.GenericWatcherConfig[*mfav1.ValidatedMFAChallenge, *mfav1.ValidatedMFAChallenge]{
			ResourceKind: types.KindValidatedMFAChallenge,
			ResourceWatcherConfig: services.ResourceWatcherConfig{
				Component: "Watcher on the Wall",
				Clock:     clock,
				Client: &mockEvents{
					watcher: &mockMFAWatcher{
						events: events,
						done:   make(chan struct{}),
					},
				},
			},
			ResourceGetter: func(context.Context) ([]*mfav1.ValidatedMFAChallenge, error) {
				return []*mfav1.ValidatedMFAChallenge{
					chal,
					duplicatedChal,
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
			ResourcesC: make(chan []*mfav1.ValidatedMFAChallenge, 1),
		},
	)
	require.NoError(t, err)
	t.Cleanup(watcher.Close)

	// Set up a mock MFA service client that will receive replicated challenges from the leaf cluster.
	leafMFAClient := &mockMFAServiceClient{
		requests: make([]*mfav1.ReplicateValidatedMFAChallengeRequest, 0),
		called:   make(chan struct{}),
	}

	// Set up a leaf cluster with the mock watcher and the mock MFA client.
	leaf := &leafCluster{
		domainName:                   "leaf.example.com",
		logger:                       slog.Default(),
		clock:                        clock,
		leafClient:                   &mockLeafClient{mfaClient: leafMFAClient},
		validatedMFAChallengeWatcher: watcher,
	}

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	t.Cleanup(cancel)

	// Set up a channel to receive errors from the run sync process.
	errC := make(chan error, 1)

	// Start syncing validated MFA challenges in a separate goroutine since it will block until the context is canceled.
	go func() {
		errC <- leaf.runValidatedMFAChallengeSync(
			ctx,
			retryutils.LinearConfig{
				First: time.Second,
				Step:  time.Second,
				Max:   time.Second,
			},
		)
	}()

	// Wait for the replication attempt.
	select {
	case <-ctx.Done():
		t.Fatal("context deadline exceeded while waiting for replication attempt")

	case <-leafMFAClient.called:
		// Replication attempt was made, continue with assertions.
	}

	// Verify it exits without error.
	cancel()
	require.NoError(t, <-errC)

	wantReqs := []*mfav1.ReplicateValidatedMFAChallengeRequest{
		{
			Name:          chal.GetMetadata().GetName(),
			Payload:       chal.GetSpec().GetPayload(),
			SourceCluster: chal.GetSpec().GetSourceCluster(),
			TargetCluster: chal.GetSpec().GetTargetCluster(),
			Username:      chal.GetSpec().GetUsername(),
		},
	}
	require.Empty(
		t,
		cmp.Diff(
			wantReqs,
			leafMFAClient.Requests(),
		),
		"runValidatedMFAChallengeSync mismatch (-want +got)",
	)
}

func TestSyncValidatedMFAChallenges(t *testing.T) {
	t.Parallel()

	leafMFAClient := &mockMFAServiceClient{
		requests: make([]*mfav1.ReplicateValidatedMFAChallengeRequest, 0),
	}

	leaf := &leafCluster{
		domainName: "leaf.example.com",
		logger:     slog.Default(),
		clock:      clockwork.NewFakeClock(),
		leafClient: &mockLeafClient{mfaClient: leafMFAClient},
	}

	chal := newValidatedMFAChallenge("challenge")

	err := leaf.syncValidatedMFAChallenges(
		t.Context(),
		[]*mfav1.ValidatedMFAChallenge{
			chal,
		},
	)
	require.NoError(t, err)

	wantReqs := []*mfav1.ReplicateValidatedMFAChallengeRequest{
		{
			Name:          chal.GetMetadata().GetName(),
			Payload:       chal.GetSpec().GetPayload(),
			SourceCluster: chal.GetSpec().GetSourceCluster(),
			TargetCluster: chal.GetSpec().GetTargetCluster(),
			Username:      chal.GetSpec().GetUsername(),
		},
	}
	require.Empty(
		t,
		cmp.Diff(
			wantReqs,
			leafMFAClient.Requests(),
		),
		"syncValidatedMFAChallenges mismatch (-want +got)")
}

func newValidatedMFAChallenge(name string) *mfav1.ValidatedMFAChallenge {
	return &mfav1.ValidatedMFAChallenge{
		Kind:    types.KindValidatedMFAChallenge,
		Version: "v1",
		Metadata: &types.Metadata{
			Name: name,
		},
		Spec: &mfav1.ValidatedMFAChallengeSpec{
			Payload: &mfav1.SessionIdentifyingPayload{
				Payload: &mfav1.SessionIdentifyingPayload_SshSessionId{SshSessionId: []byte("session-id")},
			},
			SourceCluster: "root.example.com",
			TargetCluster: "leaf.example.com",
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

	requests []*mfav1.ReplicateValidatedMFAChallengeRequest
	mu       sync.Mutex

	called     chan struct{}
	calledOnce sync.Once
}

func (m *mockMFAServiceClient) Requests() []*mfav1.ReplicateValidatedMFAChallengeRequest {
	m.mu.Lock()
	out := make([]*mfav1.ReplicateValidatedMFAChallengeRequest, len(m.requests))
	copy(out, m.requests)
	m.mu.Unlock()

	return out
}

func (m *mockMFAServiceClient) ReplicateValidatedMFAChallenge(
	_ context.Context,
	req *mfav1.ReplicateValidatedMFAChallengeRequest,
	_ ...grpc.CallOption,
) (*mfav1.ReplicateValidatedMFAChallengeResponse, error) {
	m.mu.Lock()
	m.requests = append(m.requests, req)
	m.mu.Unlock()

	if m.called != nil {
		m.calledOnce.Do(func() {
			close(m.called)
		})
	}

	return &mfav1.ReplicateValidatedMFAChallengeResponse{}, nil
}

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

package reversetunnel_test

import (
	"context"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
)

func TestValidatedMFAChallengeWatcher_Success(t *testing.T) {
	t.Parallel()

	lister := &mockValidatedMFAChallengeLister{
		challenges: []*mfav1.ValidatedMFAChallenge{
			newValidatedMFAChallenge(),
		},
	}

	watcher, _ := newValidatedMFAChallengeWatcher(t, lister, "leaf")

	challenges, err := watcher.CurrentResources(t.Context())
	require.NoError(t, err)
	require.Len(t, challenges, 1)
	require.Len(t, lister.challenges, 1)

	require.NotSame(
		t,
		lister.challenges[0],
		challenges[0],
		"CurrentResources should return clones of the original challenges, not the same pointers",
	)
	require.Empty(t, cmp.Diff(lister.challenges[0], challenges[0]), "CurrentResources mismatch (-want +got)")
}

func TestValidatedMFAChallengeWatcher_UsesTargetClusterFilter(t *testing.T) {
	t.Parallel()

	leafAChallenge := newValidatedMFAChallengeWithTargetCluster("leaf-a-challenge", "leaf-a")
	leafBChallenge := newValidatedMFAChallengeWithTargetCluster("leaf-b-challenge", "leaf-b")

	lister := &mockValidatedMFAChallengeLister{
		challenges: []*mfav1.ValidatedMFAChallenge{
			leafAChallenge,
			leafBChallenge,
		},
	}

	watcher, watchClient := newValidatedMFAChallengeWatcher(t, lister, "leaf-a")

	watch := <-watchClient.called

	challenges, err := watcher.CurrentResources(t.Context())
	require.NoError(t, err)
	require.Len(t, challenges, 1)
	require.Equal(t, leafAChallenge.GetMetadata().GetName(), challenges[0].GetMetadata().GetName())
	require.Equal(t, "leaf-a", challenges[0].GetSpec().GetTargetCluster())
	require.Len(t, lister.requests, 1)
	require.Equal(t, "leaf-a", lister.requests[0].GetFilter().GetTargetCluster())

	require.Len(t, watch.Kinds, 1)
	require.Equal(
		t,
		(&types.ValidatedMFAChallengeFilter{TargetCluster: "leaf-a"}).IntoMap(),
		watch.Kinds[0].Filter,
	)
}

func TestNewValidatedMFAChallengeWatcher_Validation(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name    string
		cfg     reversetunnel.ValidatedMFAChallengeWatcherConfig
		wantErr error
	}{
		{
			name: "nil ValidatedMFAChallengeLister",
			cfg: reversetunnel.ValidatedMFAChallengeWatcherConfig{
				ValidatedMFAChallengeLister: nil,
				ClusterName:                 "leaf",
				ResourceWatcherConfig:       &services.ResourceWatcherConfig{},
			},
			wantErr: trace.BadParameter("cfg.ValidatedMFAChallengeLister must be set"),
		},
		{
			name: "empty ClusterName",
			cfg: reversetunnel.ValidatedMFAChallengeWatcherConfig{
				ValidatedMFAChallengeLister: &mockValidatedMFAChallengeLister{},
				ClusterName:                 "",
				ResourceWatcherConfig:       &services.ResourceWatcherConfig{},
			},
			wantErr: trace.BadParameter("cfg.ClusterName must be set"),
		},
		{
			name: "nil ResourceWatcherConfig",
			cfg: reversetunnel.ValidatedMFAChallengeWatcherConfig{
				ValidatedMFAChallengeLister: &mockValidatedMFAChallengeLister{},
				ClusterName:                 "leaf",
				ResourceWatcherConfig:       nil,
			},
			wantErr: trace.BadParameter("cfg.ResourceWatcherConfig must be set"),
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			watcher, err := reversetunnel.NewValidatedMFAChallengeWatcher(
				t.Context(),
				testCase.cfg,
			)
			require.ErrorIs(t, err, testCase.wantErr)
			require.Nil(t, watcher)
		})
	}
}

func newValidatedMFAChallengeWatcher(
	t *testing.T,
	lister *mockValidatedMFAChallengeLister,
	clusterName string,
) (*services.GenericWatcher[*mfav1.ValidatedMFAChallenge, *mfav1.ValidatedMFAChallenge], *mockNewWatcherer) {
	t.Helper()

	watchClient := &mockNewWatcherer{
		watcher: &mockWatcher{
			events: make(chan types.Event),
			done:   make(chan struct{}),
		},
		called: make(chan types.Watch, 1),
	}

	watcher, err := reversetunnel.NewValidatedMFAChallengeWatcher(
		t.Context(),
		reversetunnel.ValidatedMFAChallengeWatcherConfig{
			ValidatedMFAChallengeLister: lister,
			ClusterName:                 clusterName,
			ResourceWatcherConfig: &services.ResourceWatcherConfig{
				Client:    watchClient,
				Clock:     clockwork.NewFakeClock(),
				Component: "test-watcher",
			},
		},
	)
	require.NoError(t, err)
	t.Cleanup(watcher.Close)

	return watcher, watchClient
}

func newValidatedMFAChallenge() *mfav1.ValidatedMFAChallenge {
	return newValidatedMFAChallengeWithTargetCluster("test-challenge", "leaf")
}

func newValidatedMFAChallengeWithTargetCluster(name, targetCluster string) *mfav1.ValidatedMFAChallenge {
	return &mfav1.ValidatedMFAChallenge{
		Kind:    types.KindValidatedMFAChallenge,
		Version: "v1",
		Metadata: &types.Metadata{
			Name: name,
		},
		Spec: &mfav1.ValidatedMFAChallengeSpec{
			Payload: &mfav1.SessionIdentifyingPayload{
				Payload: &mfav1.SessionIdentifyingPayload_SshSessionId{
					SshSessionId: []byte("session-id"),
				},
			},
			SourceCluster: "root",
			TargetCluster: targetCluster,
			Username:      "alice",
		},
	}
}

type mockValidatedMFAChallengeLister struct {
	challenges []*mfav1.ValidatedMFAChallenge
	requests   []*mfav1.ListValidatedMFAChallengesRequest
}

var _ reversetunnel.ValidatedMFAChallengeLister = (*mockValidatedMFAChallengeLister)(nil)

func (m *mockValidatedMFAChallengeLister) ListValidatedMFAChallenges(
	ctx context.Context,
	req *mfav1.ListValidatedMFAChallengesRequest,
	opts ...grpc.CallOption,
) (*mfav1.ListValidatedMFAChallengesResponse, error) {
	m.requests = append(m.requests, req)

	challenges := make([]*mfav1.ValidatedMFAChallenge, 0, len(m.challenges))
	targetCluster := req.GetFilter().GetTargetCluster()

	for _, challenge := range m.challenges {
		if targetCluster != "" && challenge.GetSpec().GetTargetCluster() != targetCluster {
			continue
		}

		challenges = append(challenges, challenge)
	}

	return &mfav1.ListValidatedMFAChallengesResponse{
		ValidatedChallenges: challenges,
	}, nil
}

type mockNewWatcherer struct {
	watcher types.Watcher
	called  chan types.Watch
}

var _ types.Events = (*mockNewWatcherer)(nil)

func (m *mockNewWatcherer) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	if m.called != nil {
		m.called <- watch
	}

	return m.watcher, nil
}

type mockWatcher struct {
	events    chan types.Event
	done      chan struct{}
	closeOnce sync.Once
	err       error
}

var _ types.Watcher = (*mockWatcher)(nil)

func (m *mockWatcher) Events() <-chan types.Event {
	return m.events
}

func (m *mockWatcher) Done() <-chan struct{} {
	return m.done
}

func (m *mockWatcher) Close() error {
	m.closeOnce.Do(
		func() {
			if m.done != nil {
				close(m.done)
			}
		},
	)

	return nil
}

func (m *mockWatcher) Error() error {
	return m.err
}

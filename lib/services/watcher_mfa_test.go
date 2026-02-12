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

package services_test

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

func TestValidatedMFAChallengeWatcher_Success(t *testing.T) {
	t.Parallel()

	lister := &mockValidatedMFAChallengeLister{
		challenges: []*mfav1.ValidatedMFAChallenge{
			newValidatedMFAChallenge(),
		},
	}

	newWatcherer := &mockNewWatcherer{
		watcher: &mockWatcher{
			events: make(chan types.Event),
			done:   make(chan struct{}),
		},
	}

	watcher, err := services.NewValidatedMFAChallengeWatcher(
		t.Context(),
		services.ValidatedMFAChallengeWatcherConfig{
			ValidatedMFAChallengeLister: lister,
			ResourceWatcherConfig: &services.ResourceWatcherConfig{
				Client:    newWatcherer,
				Clock:     clockwork.NewRealClock(),
				Component: "test-watcher",
			},
		},
	)
	require.NoError(t, err)

	require.EventuallyWithTf(
		t,
		func(c *assert.CollectT) {
			challenges, err := watcher.CurrentResources(t.Context())
			assert.NoError(c, err)

			// Ensure that the challenges returned by CurrentResources are clones of the original challenges returned by
			// the lister, and not the same pointers.
			assert.False(
				c,
				slices.Equal(
					lister.challenges,
					challenges,
				),
				"CurrentResources should return clones of the original challenges, not the same pointers",
			)

			// Ensure that the challenges returned by CurrentResources are deeply equal to the original challenges
			// returned by the lister.
			assert.Empty(
				c,
				cmp.Diff(
					lister.challenges,
					challenges,
				),
				"CurrentResources mismatch (-want +got)",
			)
		},
		100*time.Millisecond,
		10*time.Millisecond,
		"CurrentResources did not return expected challenges",
	)
}

func TestNewValidatedMFAChallengeWatcher_Validation(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name    string
		cfg     services.ValidatedMFAChallengeWatcherConfig
		wantErr error
	}{
		{
			name: "nil ValidatedMFAChallengeLister",
			cfg: services.ValidatedMFAChallengeWatcherConfig{
				ValidatedMFAChallengeLister: nil,
				ResourceWatcherConfig:       &services.ResourceWatcherConfig{},
			},
			wantErr: trace.BadParameter("cfg.ValidatedMFAChallengeGetter must be set"),
		},
		{
			name: "nil ResourceWatcherConfig",
			cfg: services.ValidatedMFAChallengeWatcherConfig{
				ValidatedMFAChallengeLister: &mockValidatedMFAChallengeLister{},
				ResourceWatcherConfig:       nil,
			},
			wantErr: trace.BadParameter("cfg.ResourceWatcherConfig must be set"),
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			watcher, err := services.NewValidatedMFAChallengeWatcher(
				t.Context(),
				testCase.cfg,
			)
			require.ErrorIs(t, err, testCase.wantErr)
			require.Nil(t, watcher)
		})
	}
}

func newValidatedMFAChallenge() *mfav1.ValidatedMFAChallenge {
	return &mfav1.ValidatedMFAChallenge{
		Kind:    types.KindValidatedMFAChallenge,
		Version: "v1",
		Metadata: &types.Metadata{
			Name: "test-challenge",
		},
		Spec: &mfav1.ValidatedMFAChallengeSpec{
			Payload: &mfav1.SessionIdentifyingPayload{
				Payload: &mfav1.SessionIdentifyingPayload_SshSessionId{
					SshSessionId: []byte("session-id"),
				},
			},
			SourceCluster: "src",
			TargetCluster: "tgt",
			Username:      "alice",
		},
	}
}

type mockValidatedMFAChallengeLister struct {
	challenges []*mfav1.ValidatedMFAChallenge
}

var _ services.ValidatedMFAChallengeLister = (*mockValidatedMFAChallengeLister)(nil)

func (m *mockValidatedMFAChallengeLister) ListValidatedMFAChallenges(
	ctx context.Context,
	req *mfav1.ListValidatedMFAChallengesRequest,
	opts ...grpc.CallOption,
) (*mfav1.ListValidatedMFAChallengesResponse, error) {
	return &mfav1.ListValidatedMFAChallengesResponse{
		ValidatedChallenges: m.challenges,
	}, nil
}

type mockNewWatcherer struct {
	watcher types.Watcher
}

var _ types.Events = (*mockNewWatcherer)(nil)

func (m *mockNewWatcherer) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
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

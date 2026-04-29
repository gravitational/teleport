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
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

func TestRunValidatedMFAChallengeSync_Success(t *testing.T) {
	t.Parallel()

	synctest.Test(
		t,
		func(t *testing.T) {
			chal := newValidatedMFAChallenge("challenge-for-leaf")

			leafMFAClient := newMockMFAServiceClient()

			leaf := newLeafClusterWithMFAWatcher(t, leafMFAClient, chal)

			ctx, cancel := context.WithCancel(t.Context())
			t.Cleanup(cancel)

			errC := startValidatedMFAChallengeSync(
				t,
				leaf,
				ctx,
				newRetryConfig(time.Nanosecond),
			)

			synctest.Wait()

			cancel()
			require.NoError(t, <-errC)
			require.Len(t, leafMFAClient.Requests(), 1)
			assertReplicatedChallenges(t, leafMFAClient, chal)
		},
	)
}

func TestRunValidatedMFAChallengeSync_RetriesFailedChallenges(t *testing.T) {
	t.Parallel()

	synctest.Test(
		t,
		func(t *testing.T) {
			chal := newValidatedMFAChallenge("challenge-for-leaf")

			leafMFAClient := newMockMFAServiceClient()
			leafMFAClient.errByName[chal.GetMetadata().GetName()] = []error{
				trace.ConnectionProblem(nil, "some transient error"),
				nil,
			}

			leaf := newLeafClusterWithMFAWatcher(t, leafMFAClient, chal)

			ctx, cancel := context.WithCancel(t.Context())
			t.Cleanup(cancel)

			errC := startValidatedMFAChallengeSync(
				t,
				leaf,
				ctx,
				newRetryConfig(time.Nanosecond),
			)

			synctest.Wait()

			// First attempt should be made and should fail.
			require.Len(t, leafMFAClient.Requests(), 1)

			time.Sleep(time.Nanosecond)
			synctest.Wait()

			// Second attempt should succeed.
			cancel()
			require.NoError(t, <-errC)
			require.Len(t, leafMFAClient.Requests(), 2)
		},
	)
}

func TestRunValidatedMFAChallengeSync_UsesLatestDesiredState(t *testing.T) {
	t.Parallel()

	synctest.Test(
		t,
		func(t *testing.T) {
			stale := newValidatedMFAChallenge("stale-challenge")
			fresh := newValidatedMFAChallenge("fresh-challenge")

			releaseStaleAttempt := make(chan struct{})

			leafMFAClient := newMockMFAServiceClient()
			leafMFAClient.errByName[stale.GetMetadata().GetName()] = []error{
				trace.ConnectionProblem(nil, "replication failed"),
			}
			leafMFAClient.beforeReply = func(req *mfav1.ReplicateValidatedMFAChallengeRequest) {
				if req.GetName() == stale.GetMetadata().GetName() {
					<-releaseStaleAttempt
				}
			}

			leaf := newLeafClusterWithMFAWatcher(t, leafMFAClient, stale)

			ctx, cancel := context.WithCancel(t.Context())
			t.Cleanup(cancel)

			const retryDelay = time.Nanosecond

			errC := startValidatedMFAChallengeSync(
				t,
				leaf,
				ctx,
				newRetryConfig(retryDelay),
			)

			synctest.Wait()
			require.Len(t, leafMFAClient.Requests(), 1)

			// Update the watcher's state to include the fresh challenge before unblocking the stale attempt. The next
			// retry should then sync the latest desired state instead of retrying the stale one, but it must still wait
			// for the existing backoff to elapse before syncing.
			leaf.validatedMFAChallengeWatcher.ResourcesC <- []*mfav1.ValidatedMFAChallenge{fresh}
			close(releaseStaleAttempt)

			synctest.Wait()
			require.Len(t, leafMFAClient.Requests(), 1)

			time.Sleep(retryDelay)
			synctest.Wait()

			cancel()
			require.NoError(t, <-errC)
			assertReplicatedChallenges(t, leafMFAClient, stale, fresh)
		},
	)
}

func TestRunValidatedMFAChallengeSync_DropsExpiredFailedChallenges(t *testing.T) {
	t.Parallel()

	synctest.Test(
		t,
		func(t *testing.T) {
			expired := newValidatedMFAChallenge("challenge-for-leaf")
			expired.GetMetadata().SetExpiry(time.Now().Add(expiredValidatedMFAChallengeGracePeriod + time.Nanosecond))

			leafMFAClient := newMockMFAServiceClient()
			leafMFAClient.errByName[expired.GetMetadata().GetName()] = []error{
				trace.ConnectionProblem(nil, "some transient error"),
			}

			leaf := newLeafClusterWithMFAWatcher(t, leafMFAClient, expired)

			ctx, cancel := context.WithCancel(t.Context())
			t.Cleanup(cancel)

			const retryDelay = 2 * time.Nanosecond

			errC := startValidatedMFAChallengeSync(
				t,
				leaf,
				ctx,
				newRetryConfig(retryDelay),
			)

			// First attempt should fail.
			synctest.Wait()
			require.Len(t, leafMFAClient.Requests(), 1)

			time.Sleep(retryDelay)
			synctest.Wait()

			// Second attempt should not be made because the challenge should have expired, so we should observe no
			// additional requests.
			cancel()
			require.NoError(t, <-errC)
			require.Len(t, leafMFAClient.Requests(), 1)
		},
	)
}

func TestRunValidatedMFAChallengeSync_CoalescesUpdatesDuringBackoff(t *testing.T) {
	t.Parallel()

	synctest.Test(
		t,
		func(t *testing.T) {
			stale := newValidatedMFAChallenge("stale-challenge")
			fresh := newValidatedMFAChallenge("fresh-challenge")
			fresher := newValidatedMFAChallenge("fresher-challenge")
			freshest := newValidatedMFAChallenge("freshest-challenge")

			leafMFAClient := newMockMFAServiceClient()
			leafMFAClient.errByName[stale.GetMetadata().GetName()] = []error{
				trace.ConnectionProblem(nil, "replication failed"),
			}

			leaf := newLeafClusterWithMFAWatcher(t, leafMFAClient, stale)

			ctx, cancel := context.WithCancel(t.Context())
			t.Cleanup(cancel)

			const retryDelay = time.Nanosecond

			errC := startValidatedMFAChallengeSync(
				t,
				leaf,
				ctx,
				newRetryConfig(retryDelay),
			)

			synctest.Wait()
			require.Len(t, leafMFAClient.Requests(), 1)

			leaf.validatedMFAChallengeWatcher.ResourcesC <- []*mfav1.ValidatedMFAChallenge{fresh}
			synctest.Wait()
			require.Len(t, leafMFAClient.Requests(), 1)

			leaf.validatedMFAChallengeWatcher.ResourcesC <- []*mfav1.ValidatedMFAChallenge{fresher}
			synctest.Wait()
			require.Len(t, leafMFAClient.Requests(), 1)

			leaf.validatedMFAChallengeWatcher.ResourcesC <- []*mfav1.ValidatedMFAChallenge{freshest}
			synctest.Wait()
			require.Len(t, leafMFAClient.Requests(), 1)

			time.Sleep(retryDelay)
			synctest.Wait()

			cancel()
			require.NoError(t, <-errC)
			assertReplicatedChallenges(t, leafMFAClient, stale, freshest)
		},
	)
}

func TestCoalescingMailbox_StoreCoalescesPendingUpdates(t *testing.T) {
	t.Parallel()

	mailbox := newCoalescingMailbox()

	reallyStale := newValidatedMFAChallengeSet(newValidatedMFAChallenge("really-stale-challenge"))
	sortOfStale := newValidatedMFAChallengeSet(newValidatedMFAChallenge("sort-of-stale-challenge"))
	latest := newValidatedMFAChallengeSet(newValidatedMFAChallenge("latest-challenge"))

	mailbox.store(reallyStale)
	mailbox.store(sortOfStale)
	mailbox.store(latest)

	require.Equal(
		t,
		latest,
		mailbox.drain(newValidatedMFAChallengeSet()),
		"drained value should be the latest stored value",
	)
	require.Empty(t, mailbox.updatesC, "mailbox updatesC should be empty after draining")
}

func TestSyncValidatedMFAChallenges_Success(t *testing.T) {
	t.Parallel()

	leafMFAClient := newMockMFAServiceClient()

	leaf := newLeafClusterForSyncTest(leafMFAClient)

	chal := newValidatedMFAChallenge("challenge")

	failed := leaf.syncValidatedMFAChallenges(
		t.Context(),
		newValidatedMFAChallengeSet(
			chal,
		),
	)
	require.Empty(t, failed)
	assertReplicatedChallenges(t, leafMFAClient, chal)
}

func TestSyncValidatedMFAChallenges_IgnoresAlreadyExistsAndReturnsFailures(t *testing.T) {
	t.Parallel()

	leafMFAClient := newMockMFAServiceClient()

	leaf := newLeafClusterForSyncTest(leafMFAClient)

	existing := newValidatedMFAChallenge("already-exists")
	failing := newValidatedMFAChallenge("fails")

	leafMFAClient.errByName[existing.GetMetadata().GetName()] = []error{trace.AlreadyExists("already exists")}
	leafMFAClient.errByName[failing.GetMetadata().GetName()] = []error{trace.ConnectionProblem(nil, "replication failed")}

	failed := leaf.syncValidatedMFAChallenges(
		t.Context(),
		newValidatedMFAChallengeSet(
			existing,
			failing,
		),
	)

	require.Equal(
		t,
		failed,
		newValidatedMFAChallengeSet(
			failing,
		),
	)
	require.Len(t, leafMFAClient.Requests(), 2)
}

func TestSyncValidatedMFAChallenges_SkipsExpiredChallenges(t *testing.T) {
	t.Parallel()

	leafMFAClient := newMockMFAServiceClient()

	leaf := newLeafClusterForSyncTest(leafMFAClient)

	expired := newValidatedMFAChallenge("expired")
	expired.GetMetadata().SetExpiry(time.Now().Add(expiredValidatedMFAChallengeGracePeriod).Add(-time.Nanosecond))

	failed := leaf.syncValidatedMFAChallenges(
		t.Context(),
		newValidatedMFAChallengeSet(expired),
	)

	require.Empty(t, failed)
	require.Empty(t, leafMFAClient.Requests())
}

func newLeafClusterWithMFAWatcher(
	t *testing.T,
	mfaClient mfav1.MFAServiceClient,
	challenges ...*mfav1.ValidatedMFAChallenge,
) *leafCluster {
	t.Helper()

	clock := clockwork.NewRealClock()

	events := make(chan types.Event, 1)
	events <- types.Event{Type: types.OpInit}

	// Make a copy of the challenges to avoid tests mutating the ones owned by the watcher.
	copiedChallenges := make([]*mfav1.ValidatedMFAChallenge, 0, len(challenges))
	for _, challenge := range challenges {
		copiedChallenges = append(copiedChallenges, apiutils.CloneProtoMsg(challenge))
	}

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
				return copiedChallenges, nil
			},
			ResourceKey: func(r *mfav1.ValidatedMFAChallenge) string {
				return backend.NewKey(
					r.GetSpec().GetTargetCluster(),
					r.GetMetadata().GetName(),
				).String()
			},
			CloneFunc:    apiutils.CloneProtoMsg[*mfav1.ValidatedMFAChallenge],
			ReadOnlyFunc: apiutils.CloneProtoMsg[*mfav1.ValidatedMFAChallenge],
			ResourcesC:   make(chan []*mfav1.ValidatedMFAChallenge, 1),
		},
	)
	require.NoError(t, err)
	t.Cleanup(watcher.Close)

	return &leafCluster{
		domainName:                   "leaf.example.com",
		logger:                       slog.Default(),
		clock:                        clock,
		leafClient:                   &mockLeafClient{mfaClient: mfaClient},
		validatedMFAChallengeWatcher: watcher,
	}
}

func newLeafClusterForSyncTest(
	mfaClient mfav1.MFAServiceClient,
) *leafCluster {
	return &leafCluster{
		domainName: "leaf.example.com",
		logger:     slog.Default(),
		clock:      clockwork.NewRealClock(),
		leafClient: &mockLeafClient{mfaClient: mfaClient},
	}
}

func startValidatedMFAChallengeSync(
	t *testing.T,
	leaf *leafCluster,
	ctx context.Context,
	cfg retryutils.LinearConfig,
) chan error {
	t.Helper()

	errC := make(chan error, 1)

	go func() {
		errC <- leaf.runValidatedMFAChallengeSync(ctx, cfg)
	}()

	return errC
}

func newRetryConfig(delay time.Duration) retryutils.LinearConfig {
	return retryutils.LinearConfig{
		First: delay,
		Step:  delay,
		Max:   delay,
	}
}

func assertReplicatedChallenges(
	t *testing.T,
	client *mockMFAServiceClient,
	challenges ...*mfav1.ValidatedMFAChallenge,
) {
	t.Helper()

	wantReqs := make([]*mfav1.ReplicateValidatedMFAChallengeRequest, 0, len(challenges))
	for _, challenge := range challenges {
		wantReqs = append(wantReqs, replicateValidatedMFAChallengeRequest(challenge))
	}

	require.Empty(
		t,
		cmp.Diff(client.Requests(), wantReqs),
		"replicated challenges mismatch (-got +want)",
	)
}

func replicateValidatedMFAChallengeRequest(
	chal *mfav1.ValidatedMFAChallenge,
) *mfav1.ReplicateValidatedMFAChallengeRequest {
	return &mfav1.ReplicateValidatedMFAChallengeRequest{
		Name:          chal.GetMetadata().GetName(),
		Payload:       chal.GetSpec().GetPayload(),
		SourceCluster: chal.GetSpec().GetSourceCluster(),
		TargetCluster: chal.GetSpec().GetTargetCluster(),
		Username:      chal.GetSpec().GetUsername(),
	}
}

func newValidatedMFAChallenge(name string) *mfav1.ValidatedMFAChallenge {
	expires := time.Now().Add(local.ValidatedMFAChallengeExpiry)

	return &mfav1.ValidatedMFAChallenge{
		Kind:    types.KindValidatedMFAChallenge,
		Version: "v1",
		Metadata: &types.Metadata{
			Name:    name,
			Expires: &expires,
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

	requests    []*mfav1.ReplicateValidatedMFAChallengeRequest
	errByName   map[string][]error
	beforeReply func(*mfav1.ReplicateValidatedMFAChallengeRequest)
	mu          sync.Mutex
}

func newMockMFAServiceClient() *mockMFAServiceClient {
	return &mockMFAServiceClient{
		requests:  make([]*mfav1.ReplicateValidatedMFAChallengeRequest, 0),
		errByName: make(map[string][]error),
	}
}

func (m *mockMFAServiceClient) Requests() []*mfav1.ReplicateValidatedMFAChallengeRequest {
	m.mu.Lock()
	defer m.mu.Unlock()

	out := make([]*mfav1.ReplicateValidatedMFAChallengeRequest, len(m.requests))
	copy(out, m.requests)

	return out
}

func (m *mockMFAServiceClient) ReplicateValidatedMFAChallenge(
	_ context.Context,
	req *mfav1.ReplicateValidatedMFAChallengeRequest,
	_ ...grpc.CallOption,
) (*mfav1.ReplicateValidatedMFAChallengeResponse, error) {
	m.mu.Lock()

	// Record request for later inspection by tests.
	m.requests = append(m.requests, req)

	// Determine if we should return an error for this request based on the challenge name.
	var err error
	if errs := m.errByName[req.GetName()]; len(errs) > 0 {
		err = errs[0]

		m.errByName[req.GetName()] = errs[1:]
	}

	beforeReply := m.beforeReply
	m.mu.Unlock()

	if beforeReply != nil {
		beforeReply(req)
	}

	if err != nil {
		return nil, err
	}

	return &mfav1.ReplicateValidatedMFAChallengeResponse{}, nil
}

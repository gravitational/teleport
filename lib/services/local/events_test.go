/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package local

import (
	"context"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/suite"
	"github.com/gravitational/teleport/lib/utils"
)

func fetchEvent(t *testing.T, w types.Watcher, timeout time.Duration) types.Event {
	t.Helper()
	timeoutC := time.After(timeout)
	var ev types.Event
	select {
	case <-timeoutC:
		require.Fail(t, "Timeout waiting for event", w.Error())
	case <-w.Done():
		require.Fail(t, "Watcher exited with error", w.Error())
	case ev = <-w.Events():
	}
	return ev
}

func newTestContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return ctx
}

func unwrapResource153[T any](t *testing.T, r types.Resource) T {
	u, ok := r.(types.Resource153Unwrapper)
	require.True(t, ok, "expected event to implement Resource153Unwrapper")

	dst, ok := u.Unwrap().(T)
	require.True(t, ok, "expected event to cast to %T", dst)
	return dst
}

func TestWatchers(t *testing.T) {
	const fetchTimeout = 3 * time.Second
	type actionFn func(context.Context, *testing.T, backend.Backend)
	type validateFn func(context.Context, *testing.T, types.Watcher)

	t.Parallel()

	testCases := []struct {
		name           string
		kind           string
		filter         map[string]string
		init           actionFn
		causeEvents    actionFn
		validateEvents validateFn
	}{
		{
			name: "CA (unfiltered)",
			kind: types.KindCertAuthority,
			causeEvents: func(subtestCtx context.Context, subtestT *testing.T, backend backend.Backend) {
				// GIVEN an empty backend, WHEN I create 3 new CAs
				userCA := suite.NewTestCA(types.UserCA, "example.com")
				hostCA := suite.NewTestCA(types.HostCA, "example.com")
				hostCARemote := suite.NewTestCA(types.HostCA, "remote.com")
				require.NoError(subtestT, CreateResources(subtestCtx, backend, userCA, hostCA, hostCARemote))
			},
			validateEvents: func(subtestCtx context.Context, subtestT *testing.T, watcher types.Watcher) {
				// EXPECT that we receive at least 3 events notifying us of the
				// CA creations
				gotCertAuthIDSet := utils.NewSet[types.CertAuthID]()
				for range 3 {
					event := fetchEvent(subtestT, watcher, fetchTimeout)

					// EXPECT that the resource attached to the event is a CA
					ca, ok := event.Resource.(*types.CertAuthorityV2)
					require.True(t, ok)

					gotCertAuthIDSet.Add(ca.GetID())
				}

				// EXPECT that we received events for all 3 created CAs
				expected := []types.CertAuthID{
					{Type: types.UserCA, DomainName: "example.com"},
					{Type: types.HostCA, DomainName: "example.com"},
					{Type: types.HostCA, DomainName: "remote.com"},
				}
				require.ElementsMatch(t, expected, gotCertAuthIDSet.Elements())
			},
		},
		{
			name:   "CA (filtered)",
			kind:   types.KindCertAuthority,
			filter: types.CertAuthorityFilter{types.HostCA: "example.com"}.IntoMap(),
			causeEvents: func(subtestCtx context.Context, subtestT *testing.T, backend backend.Backend) {
				// GIVEN an empty backend, WHEN I create some new CAs
				userCA := suite.NewTestCA(types.UserCA, "example.com")
				hostCA := suite.NewTestCA(types.HostCA, "example.com")
				hostCARemote := suite.NewTestCA(types.HostCA, "remote.com")
				require.NoError(subtestT, CreateResources(subtestCtx, backend, userCA, hostCA, hostCARemote))
			},
			validateEvents: func(subtestCtx context.Context, subtestT *testing.T, watcher types.Watcher) {
				// EXPECT that we receive at least one event notifying us of the
				//        CA creation
				event := fetchEvent(t, watcher, fetchTimeout)
				require.Equal(t, types.OpPut, event.Type)

				// EXPECT that the attached resource is a CA
				ca, ok := event.Resource.(*types.CertAuthorityV2)
				require.True(t, ok)

				// EXPECT that the resource we're being notified about matches
				// the filter we specified in the test case1
				require.Equal(t, types.CertAuthID{
					Type:       types.HostCA,
					DomainName: "example.com",
				}, ca.GetID())
			},
		},
		{
			name: "provisioning principal state PUT",
			kind: types.KindProvisioningPrincipalState,
			causeEvents: func(subtestCtx context.Context, subtestT *testing.T, backend backend.Backend) {
				// GIVEN an empty backend, WHEN I create a new provisioning
				// PrincipalState
				svc, err := NewProvisioningStateService(backend)
				require.NoError(subtestT, err)

				_, err = svc.CreateProvisioningState(subtestCtx, mkUserProvisioningState(
					"alice",
					services.DownstreamID("foocorp"),
					provisioningv1.ProvisioningState_PROVISIONING_STATE_STALE))
				require.NoError(subtestT, err)
			},
			validateEvents: func(subtestCtx context.Context, subtestT *testing.T, watcher types.Watcher) {
				// EXPECT that the watcher gets an event notifying us about the
				// creation
				event := fetchEvent(t, watcher, fetchTimeout)
				require.Equal(t, types.OpPut, event.Type)

				// EXPECT that the resource attached event represents the created
				// PrincipalState record
				s0 := unwrapResource153[*provisioningv1.PrincipalState](subtestT, event.Resource)
				require.Equal(subtestT, "foocorp", s0.Spec.DownstreamId)
				require.Equal(subtestT, "u-alice", s0.Metadata.Name)
			},
		},
		{
			name: "provisioning principal state DELETE",
			kind: types.KindProvisioningPrincipalState,
			init: func(subtestCtx context.Context, subtestT *testing.T, backend backend.Backend) {
				// GIVEN an existing provisioning PrincipalState
				svc, err := NewProvisioningStateService(backend)
				require.NoError(subtestT, err)
				_, err = svc.CreateProvisioningState(subtestCtx, mkUserProvisioningState(
					"alice",
					services.DownstreamID("foocorp"),
					provisioningv1.ProvisioningState_PROVISIONING_STATE_STALE))
				require.NoError(subtestT, err)
			},
			causeEvents: func(subtestCtx context.Context, subtestT *testing.T, backend backend.Backend) {
				// WHEN I delete all provisioning PrincipalState records
				svc, err := NewProvisioningStateService(backend)
				require.NoError(subtestT, err)
				require.NoError(subtestT, svc.DeleteAllProvisioningStates(subtestCtx))
			},
			validateEvents: func(subtestCtx context.Context, subtestT *testing.T, watcher types.Watcher) {
				// EXPECT to receive a DELETE event
				event := fetchEvent(t, watcher, fetchTimeout)
				require.Equal(t, types.OpDelete, event.Type)

				// EXPECT that the event targets our pre-created record
				m := event.Resource.GetMetadata()
				require.Equal(subtestT, "u-alice", m.Name)

				// EXPECT that the supplied resource is a PrincipalState record
				// containing the downstream ID of the deleted resource
				principalState := unwrapResource153[*provisioningv1.PrincipalState](subtestT, event.Resource)
				require.NotNil(t, principalState.Spec)
				require.Equal(subtestT, "foocorp", principalState.Spec.DownstreamId)
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx := newTestContext(t)

			// GIVEN an empty back-end
			clock := clockwork.NewFakeClock()
			bk, err := memory.New(memory.Config{
				Clock: clock,
			})
			require.NoError(t, err)
			t.Cleanup(func() { bk.Close() })
			eventsSvc := NewEventsService(bk)

			// ALSO GIVEN a possibly-customized backend state
			if test.init != nil {
				test.init(ctx, t, bk)
			}

			// WHEN I Create a new Watcher
			watcher, err := eventsSvc.NewWatcher(ctx, types.Watch{Kinds: []types.WatchKind{{
				Kind:        test.kind,
				Filter:      test.filter,
				LoadSecrets: false,
			}}})
			require.NoError(t, err)
			t.Cleanup(func() { watcher.Close() })

			// EXPECT that we will receive a cache init event
			event := fetchEvent(t, watcher, fetchTimeout)
			require.Equal(t, types.OpInit, event.Type)

			// WHEN I perform an action
			test.causeEvents(ctx, t, bk)

			// EXPECT that the appropriate events are emitted
			test.validateEvents(ctx, t, watcher)
		})
	}
}

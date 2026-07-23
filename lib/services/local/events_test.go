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

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	linuxdesktopv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/linuxdesktop/v1"
	mfav2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v2"
	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/lib/auth/authcatest"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/set"
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

func unwrapResource153[T types.Resource153](t *testing.T, r types.Resource) T {
	u, ok := r.(types.Resource153UnwrapperT[T])
	require.True(t, ok, "expected event to implement Resource153Unwrapper")

	dst := u.UnwrapT()
	return dst
}

func mustEncodeScopeForKey(t *testing.T, scope string) string {
	t.Helper()

	encoded, err := scopes.EncodeForKey(scope)
	require.NoError(t, err)
	return encoded
}

func TestAccessListParserScopedDelete(t *testing.T) {
	const scope = "/eng/platform"
	key := backend.NewKey(scopedPrefix, accessListPrefix, mustEncodeScopeForKey(t, scope), "reviewed")
	event := backend.Event{
		Type: types.OpDelete,
		Item: backend.Item{Key: key},
	}

	parser := newAccessListParser()
	require.True(t, parser.match(key))

	resource, err := parser.parse(event)
	require.NoError(t, err)

	accessList, ok := resource.(*accesslist.AccessList)
	require.True(t, ok)
	require.Equal(t, types.KindAccessList, accessList.GetKind())
	require.Equal(t, "reviewed", accessList.GetName())
	require.Equal(t, scope, accessList.Scope)
}

func TestAccessListMemberParserScopedDelete(t *testing.T) {
	const listScope = "/eng/platform"

	tests := []struct {
		name                   string
		member                 scopes.QualifiedName
		expectedMembershipKind string
	}{
		{
			name:                   "unscoped member",
			member:                 scopes.QualifiedName{Name: "alice"},
			expectedMembershipKind: accesslist.MembershipKindUnspecified,
		},
		{
			name:                   "scoped list member",
			member:                 scopes.QualifiedName{Scope: "/eng", Name: "team"},
			expectedMembershipKind: accesslist.MembershipKindScopedList,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			key := backend.NewKey(
				scopedPrefix,
				accessListMemberPrefix,
				mustEncodeScopeForKey(t, listScope),
				"reviewed",
				mustEncodeScopeForKey(t, test.member.Scope),
				test.member.Name,
			)
			event := backend.Event{
				Type: types.OpDelete,
				Item: backend.Item{Key: key},
			}

			parser := newAccessListMemberParser()
			require.True(t, parser.match(key))

			resource, err := parser.parse(event)
			require.NoError(t, err)

			member, ok := resource.(*accesslist.AccessListMember)
			require.True(t, ok)
			require.Equal(t, types.KindAccessListMember, member.GetKind())
			require.Equal(t, test.member.String(), member.GetName())
			require.Equal(t, listScope, member.Scope)
			require.Equal(t, scopes.QualifiedName{Scope: listScope, Name: "reviewed"}.String(), member.Spec.AccessList)
			require.Equal(t, test.member.String(), member.Spec.Name)
			require.Equal(t, test.expectedMembershipKind, member.Spec.MembershipKind)
		})
	}
}

func TestAccessListReviewParserScopedDelete(t *testing.T) {
	const scope = "/eng/platform"
	key := backend.NewKey(scopedPrefix, accessListReviewPrefix, mustEncodeScopeForKey(t, scope), "reviewed", "review-1")
	event := backend.Event{
		Type: types.OpDelete,
		Item: backend.Item{Key: key},
	}

	parser := newAccessListReviewParser()
	require.True(t, parser.match(key))

	resource, err := parser.parse(event)
	require.NoError(t, err)

	review, ok := resource.(*accesslist.Review)
	require.True(t, ok)
	require.Equal(t, types.KindAccessListReview, review.GetKind())
	require.Equal(t, "review-1", review.GetName())
	require.Equal(t, scope, review.Scope)
	require.Equal(t, scopes.QualifiedName{Scope: scope, Name: "reviewed"}.String(), review.Spec.AccessList)
}

func TestAccessListScopedDeleteParserRejectsMalformedKeys(t *testing.T) {
	tests := []struct {
		name   string
		parser resourceParser
		key    backend.Key
	}{
		{
			name:   "access list missing name",
			parser: newAccessListParser(),
			key:    backend.NewKey(scopedPrefix, accessListPrefix, mustEncodeScopeForKey(t, "/eng/platform")),
		},
		{
			name:   "member missing member name",
			parser: newAccessListMemberParser(),
			key: backend.NewKey(
				scopedPrefix,
				accessListMemberPrefix,
				mustEncodeScopeForKey(t, "/eng/platform"),
				"reviewed",
				mustEncodeScopeForKey(t, "/eng"),
			),
		},
		{
			name:   "review has bad encoded scope",
			parser: newAccessListReviewParser(),
			key:    backend.NewKey(scopedPrefix, accessListReviewPrefix, "bad-scope", "reviewed", "review-1"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.True(t, test.parser.match(test.key))
			_, err := test.parser.parse(backend.Event{
				Type: types.OpDelete,
				Item: backend.Item{Key: test.key},
			})
			require.Error(t, err)
		})
	}
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
				userCA, err := authcatest.NewCA(types.UserCA, "example.com")
				require.NoError(t, err)
				hostCA, err := authcatest.NewCA(types.HostCA, "example.com")
				require.NoError(t, err)
				hostCARemote, err := authcatest.NewCA(types.HostCA, "remote.com")
				require.NoError(t, err)
				require.NoError(subtestT, CreateResources(subtestCtx, backend, userCA, hostCA, hostCARemote))
			},
			validateEvents: func(subtestCtx context.Context, subtestT *testing.T, watcher types.Watcher) {
				// EXPECT that we receive at least 3 events notifying us of the
				// CA creations
				gotCertAuthIDSet := set.New[types.CertAuthID]()
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
				userCA, err := authcatest.NewCA(types.UserCA, "example.com")
				require.NoError(t, err)
				hostCA, err := authcatest.NewCA(types.HostCA, "example.com")
				require.NoError(t, err)
				hostCARemote, err := authcatest.NewCA(types.HostCA, "remote.com")
				require.NoError(t, err)
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
				require.Equal(subtestT, "foocorp", s0.GetSpec().GetDownstreamId())
				require.Equal(subtestT, "u-alice", s0.GetMetadata().GetName())
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
				require.NotNil(t, principalState.GetSpec())
				require.Equal(subtestT, "foocorp", principalState.GetSpec().GetDownstreamId())
			},
		},
		{
			name: "linux desktop PUT",
			kind: types.KindLinuxDesktop,
			causeEvents: func(subtestCtx context.Context, subtestT *testing.T, backend backend.Backend) {
				// GIVEN an empty backend, WHEN I create a new Linux desktop
				svc, err := NewLinuxDesktopService(backend)
				require.NoError(subtestT, err)

				desktop := linuxdesktopv1.LinuxDesktop_builder{
					Kind:    types.KindLinuxDesktop,
					Version: types.V1,
					Metadata: headerv1.Metadata_builder{
						Name: "desktop-1",
						Labels: map[string]string{
							"env":  "test",
							"team": "engineering",
						},
					}.Build(),
					Spec: linuxdesktopv1.LinuxDesktopSpec_builder{
						Addr:     "127.0.0.1:22",
						Hostname: "test-host",
					}.Build(),
				}.Build()

				_, err = svc.CreateLinuxDesktop(subtestCtx, desktop)
				require.NoError(subtestT, err)
			},
			validateEvents: func(subtestCtx context.Context, subtestT *testing.T, watcher types.Watcher) {
				// EXPECT that the watcher gets an event notifying us about the creation
				event := fetchEvent(t, watcher, fetchTimeout)
				require.Equal(t, types.OpPut, event.Type)

				// EXPECT that the resource attached to the event is a Linux desktop
				desktop := unwrapResource153[*linuxdesktopv1.LinuxDesktop](subtestT, event.Resource)
				require.Equal(subtestT, "desktop-1", desktop.GetMetadata().GetName())
				require.Equal(subtestT, "127.0.0.1:22", desktop.GetSpec().GetAddr())
				require.Equal(subtestT, "test-host", desktop.GetSpec().GetHostname())
				require.Equal(subtestT, map[string]string{
					"env":  "test",
					"team": "engineering",
				}, desktop.GetMetadata().GetLabels())
			},
		},
		{
			name: "linux desktop DELETE",
			kind: types.KindLinuxDesktop,
			init: func(subtestCtx context.Context, subtestT *testing.T, backend backend.Backend) {
				// GIVEN an existing Linux desktop
				svc, err := NewLinuxDesktopService(backend)
				require.NoError(subtestT, err)

				desktop := linuxdesktopv1.LinuxDesktop_builder{
					Kind:    types.KindLinuxDesktop,
					Version: types.V1,
					Metadata: headerv1.Metadata_builder{
						Name: "desktop-to-delete",
						Labels: map[string]string{
							"env": "staging",
						},
					}.Build(),
					Spec: linuxdesktopv1.LinuxDesktopSpec_builder{
						Addr:     "192.168.1.10:22",
						Hostname: "delete-me",
					}.Build(),
				}.Build()

				_, err = svc.CreateLinuxDesktop(subtestCtx, desktop)
				require.NoError(subtestT, err)
			},
			causeEvents: func(subtestCtx context.Context, subtestT *testing.T, backend backend.Backend) {
				// WHEN I delete the Linux desktop
				svc, err := NewLinuxDesktopService(backend)
				require.NoError(subtestT, err)
				require.NoError(subtestT, svc.DeleteLinuxDesktop(subtestCtx, "desktop-to-delete"))
			},
			validateEvents: func(subtestCtx context.Context, subtestT *testing.T, watcher types.Watcher) {
				// EXPECT to receive a DELETE event
				event := fetchEvent(t, watcher, fetchTimeout)
				require.Equal(t, types.OpDelete, event.Type)

				// EXPECT that the event targets our pre-created desktop
				m := event.Resource.GetMetadata()
				require.Equal(subtestT, "desktop-to-delete", m.Name)

				// EXPECT that the resource is a ResourceHeader with the correct kind
				require.Equal(subtestT, types.KindLinuxDesktop, event.Resource.GetKind())
			},
		},
		{
			name: "validated MFA challenge PUT",
			kind: types.KindValidatedMFAChallenge,
			causeEvents: func(subtestCtx context.Context, subtestT *testing.T, bk backend.Backend) {
				svc, err := NewMFAService(bk)
				require.NoError(subtestT, err)

				_, err = svc.CreateValidatedMFAChallenge(
					subtestCtx,
					"leaf.example.com",
					mfav2.ValidatedMFAChallenge_builder{
						Kind:    types.KindValidatedMFAChallenge,
						Version: types.V1,
						Metadata: headerv1.Metadata_builder{
							Name: "test-challenge",
						}.Build(),
						Spec: mfav2.ValidatedMFAChallengeSpec_builder{
							Payload: mfav2.SessionIdentifyingPayload_builder{
								SshSessionId: []byte("session-id"),
							}.Build(),
							SourceCluster: "root.example.com",
							TargetCluster: "leaf.example.com",
							Username:      "alice",
						}.Build(),
					}.Build(),
				)
				require.NoError(subtestT, err)
			},
			validateEvents: func(subtestCtx context.Context, subtestT *testing.T, watcher types.Watcher) {
				event := fetchEvent(subtestT, watcher, fetchTimeout)
				require.Equal(subtestT, types.OpPut, event.Type)

				chal, err := types.ConvertResource[*mfav2.ValidatedMFAChallenge](event.Resource)
				require.NoError(subtestT, err)
				require.Equal(subtestT, "test-challenge", chal.GetMetadata().GetName())
				require.Equal(subtestT, "leaf.example.com", chal.GetSpec().GetTargetCluster())
			},
		},
		{
			name: "validated MFA challenge DELETE",
			kind: types.KindValidatedMFAChallenge,
			init: func(subtestCtx context.Context, subtestT *testing.T, bk backend.Backend) {
				svc, err := NewMFAService(bk)
				require.NoError(subtestT, err)

				_, err = svc.CreateValidatedMFAChallenge(subtestCtx, "leaf.example.com", mfav2.ValidatedMFAChallenge_builder{
					Kind:    types.KindValidatedMFAChallenge,
					Version: types.V1,
					Metadata: headerv1.Metadata_builder{
						Name: "test-challenge",
					}.Build(),
					Spec: mfav2.ValidatedMFAChallengeSpec_builder{
						Payload: mfav2.SessionIdentifyingPayload_builder{
							SshSessionId: []byte("session-id"),
						}.Build(),
						SourceCluster: "root.example.com",
						TargetCluster: "leaf.example.com",
						Username:      "alice",
					}.Build(),
				}.Build())
				require.NoError(subtestT, err)
			},
			causeEvents: func(subtestCtx context.Context, subtestT *testing.T, bk backend.Backend) {
				err := bk.Delete(subtestCtx, backend.NewKey(types.KindValidatedMFAChallenge, "leaf.example.com", "test-challenge"))
				require.NoError(subtestT, err)
			},
			validateEvents: func(subtestCtx context.Context, subtestT *testing.T, watcher types.Watcher) {
				event := fetchEvent(subtestT, watcher, fetchTimeout)
				require.Equal(subtestT, types.OpDelete, event.Type)

				chal, err := types.ConvertResource[*mfav2.ValidatedMFAChallenge](event.Resource)
				require.NoError(subtestT, err)
				require.Equal(subtestT, types.KindValidatedMFAChallenge, chal.GetKind())
				require.Equal(subtestT, "test-challenge", chal.GetMetadata().GetName())
				require.Equal(subtestT, "leaf.example.com", chal.GetSpec().GetTargetCluster())
			},
		},
		{
			name: "PendingCSRRequest PUT/DELETE",
			kind: types.KindPendingCSRRequest,
			causeEvents: func(ctx context.Context, t *testing.T, bk backend.Backend) {
				service, err := NewSubCAService(SubCAServiceParams{
					Backend: bk,
				})
				require.NoError(t, err)

				// PUT.
				res, err := service.CreatePendingCSRRequest(
					ctx,
					subcav1.PendingCSRRequest_builder{
						Kind:    types.KindPendingCSRRequest,
						Version: types.V1,
						Metadata: headerv1.Metadata_builder{
							Name: "2f878e0f-115c-4b48-a4f6-f4deae8efb6f",
						}.Build(),
						Spec: subcav1.PendingCSRRequestSpec_builder{
							ClusterName: "example.com",
							CaType:      string(types.WindowsCA),
							PublicKeyHashes: []*subcav1.PublicKeyHash{
								subcav1.PublicKeyHash_builder{
									Value: "ea16c3a8c1f31943019ecc9bfb2899b60e8ec156874bdf4606a899c95392cef3",
								}.Build(),
							},
						}.Build(),
					}.Build(),
				)
				require.NoError(t, err)

				// DELETE.
				require.NoError(t,
					service.DeletePendingCSRRequest(ctx, res.GetMetadata().GetName()),
				)
			},
			validateEvents: func(ctx context.Context, t *testing.T, watcher types.Watcher) {
				const wantName = "2f878e0f-115c-4b48-a4f6-f4deae8efb6f"

				// PUT.
				event := fetchEvent(t, watcher, fetchTimeout)
				require.Equal(t, types.OpPut, event.Type)
				res, err := types.ConvertResource[*subcav1.PendingCSRRequest](event.Resource)
				require.NoError(t, err)
				require.Equal(t, wantName, res.GetMetadata().GetName())

				// DELETE.
				event = fetchEvent(t, watcher, fetchTimeout)
				require.Equal(t, types.OpDelete, event.Type)
				res, err = types.ConvertResource[*subcav1.PendingCSRRequest](event.Resource)
				require.NoError(t, err)
				require.Equal(t, wantName, res.GetMetadata().GetName())
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
